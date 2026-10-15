package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// SDK v1.8.0 会回送已取消 handler 的结果。现代 stdio 必须停止该请求的输出；
// 在 SDK 聚合完响应后过滤，不能提前丢弃 Connection.Write 而悬挂批量响应。
type stdioTransport struct {
	reader io.ReadCloser
	writer io.WriteCloser
}

func (t stdioTransport) Connect(ctx context.Context) (mcp.Connection, error) {
	state := &cancellationState{requests: map[jsonrpc.ID]bool{}}
	transport := &mcp.IOTransport{Reader: t.reader, Writer: &cancelWriter{WriteCloser: boundedWriter{t.writer}, state: state}, MaxLineLength: maxFrameBytes}
	connection, err := transport.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &observedConnection{Connection: connection, state: state}, nil
}

type cancellationState struct {
	mu       sync.Mutex
	requests map[jsonrpc.ID]bool
}
type observedConnection struct {
	mcp.Connection
	state *cancellationState
}

func (c *observedConnection) Read(ctx context.Context) (jsonrpc.Message, error) {
	message, err := c.Connection.Read(ctx)
	request, ok := message.(*jsonrpc.Request)
	if err != nil || !ok {
		return message, err
	}
	var params struct {
		Meta      map[string]any `json:"_meta"`
		RequestID any            `json:"requestId"`
	}
	if json.Unmarshal(request.Params, &params) != nil {
		return message, nil
	}
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	if request.IsCall() && params.Meta["io.modelcontextprotocol/protocolVersion"] == "2026-07-28" {
		c.state.requests[request.ID] = false
	} else if !request.IsCall() && request.Method == "notifications/cancelled" {
		if id, err := jsonrpc.MakeID(params.RequestID); err == nil {
			if _, exists := c.state.requests[id]; exists {
				c.state.requests[id] = true
			}
		}
	}
	return message, nil
}

type cancelWriter struct {
	io.WriteCloser
	state *cancellationState
}

func (w *cancelWriter) Write(body []byte) (int, error) {
	w.state.mu.Lock()
	defer w.state.mu.Unlock()
	if len(w.state.requests) == 0 {
		return w.WriteCloser.Write(body)
	}
	trimmed := bytes.TrimSpace(body)
	batch := len(trimmed) > 0 && trimmed[0] == '['
	messages := []json.RawMessage{trimmed}
	if batch {
		if err := json.Unmarshal(trimmed, &messages); err != nil {
			return 0, err
		}
	}
	kept := make([]json.RawMessage, 0, len(messages))
	for _, raw := range messages {
		message, err := jsonrpc.DecodeMessage(raw)
		if err != nil {
			return 0, err
		}
		if response, ok := message.(*jsonrpc.Response); ok {
			cancelled := w.state.requests[response.ID]
			delete(w.state.requests, response.ID)
			if cancelled {
				continue
			}
		}
		kept = append(kept, raw)
	}
	if len(kept) == 0 {
		return len(body), nil
	}
	if len(kept) == len(messages) {
		return w.WriteCloser.Write(body)
	}
	filtered, err := json.Marshal(kept)
	if err != nil {
		return 0, err
	}
	if !batch {
		filtered = kept[0]
	}
	_, err = w.WriteCloser.Write(append(filtered, '\n'))
	if err != nil {
		return 0, err
	}
	return len(body), nil
}
