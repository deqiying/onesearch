// Package mcpserver 提供本地 stdio MCP 入口，不依赖 CLI 输出或配置写入。
package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"

	"github.com/deqiying/onesearch/internal/app"
	"github.com/deqiying/onesearch/internal/config"
	"github.com/deqiying/onesearch/internal/execution"
	"github.com/deqiying/onesearch/internal/service"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var protocolVersions = []string{"2026-07-28", "2025-11-25", "2025-06-18", "2025-03-26"}

type executeFunc func(context.Context, *service.Service, string, map[string]any) map[string]any

type handler struct {
	load    func() (*config.Config, error)
	execute executeFunc
	slots   chan struct{}
}

func newServer(load func() (*config.Config, error), execute executeFunc) (*mcp.Server, error) {
	catalog, err := buildCatalog()
	if err != nil {
		return nil, err
	}
	h := &handler{load: load, execute: execute, slots: make(chan struct{}, 4)}
	s := mcp.NewServer(&mcp.Implementation{Name: app.Name, Version: app.Version}, &mcp.ServerOptions{
		Capabilities:              &mcp.ServerCapabilities{Tools: &mcp.ToolCapabilities{}},
		SupportedProtocolVersions: append([]string{}, protocolVersions...),
		// SDK 日志可能含原始协议内容；本层仅向调用方返回固定诊断。
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	for _, definition := range catalog {
		s.AddTool(definition.tool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return h.call(ctx, definition, req.Params.Arguments, req.ProtocolVersion()), nil
		})
	}
	s.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			result, err := next(ctx, method, req)
			if err != nil {
				return result, err
			}
			if request, ok := req.(*mcp.ListToolsRequest); ok && request.ProtocolVersion() == "2025-03-26" {
				if listed, ok := result.(*mcp.ListToolsResult); ok {
					copyResult := *listed
					copyResult.Tools = make([]*mcp.Tool, len(listed.Tools))
					for i, tool := range listed.Tools {
						copyTool := *tool
						copyTool.OutputSchema = nil
						copyResult.Tools[i] = &copyTool
					}
					return &copyResult, nil
				}
			}
			return result, nil
		}
	})
	return s, nil
}

func (h *handler) call(ctx context.Context, definition toolDefinition, raw json.RawMessage, version string) (result *mcp.CallToolResult) {
	name, operation := definition.tool.Name, ""
	defer func() {
		if recover() != nil {
			result = makeResult(name, operation, version, failure("internal_error", "Tool operation failed unexpectedly"), nil)
		}
	}()
	fail := func(kind, message string) *mcp.CallToolResult {
		return makeResult(name, operation, version, failure(kind, message), nil)
	}
	var args map[string]any
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	if len(raw) > maxFrameBytes || json.Unmarshal(raw, &args) != nil || args == nil {
		return fail("parameter_error", "Arguments must be a JSON object")
	}
	operation = operationFor(name, args)
	if definition.input.Validate(args) != nil {
		return fail("parameter_error", "Arguments do not match the tool input schema")
	}
	in, err := bind(name, args)
	if err != nil {
		return fail("parameter_error", "Invalid operation parameters")
	}
	if ctx.Err() != nil {
		return fail("cancelled", "Tool call was cancelled")
	}
	select {
	case h.slots <- struct{}{}:
		defer func() { <-h.slots }()
	default:
		return fail("server_busy", "At most four tool calls may execute concurrently")
	}
	timed, cancel := context.WithTimeout(ctx, in.timeout)
	defer cancel()
	cfg, err := h.load()
	if cfg == nil {
		return fail("config_error", "Configuration snapshot is unavailable")
	}
	svc := service.New(cfg)
	if err != nil && name != "onesearch_status" {
		return fail("config_error", "Configuration could not be loaded; repair it outside MCP.")
	}
	var data map[string]any
	if name == "onesearch_status" {
		data = statusData(cfg, svc)
	} else {
		filter, _ := in.input.Values["provider"].(string)
		if !operationReady(cfg, svc, in.operation, filter) {
			return fail("config_error", "The selected operation is not configured or is disabled; inspect onesearch_status.")
		}
		data = h.execute(timed, svc, in.id, in.input.Values)
	}
	if timed.Err() == context.DeadlineExceeded {
		data = failure("timeout", "Tool call exceeded its overall timeout")
	} else if timed.Err() != nil {
		data = failure("cancelled", "Tool call was cancelled")
	}
	secrets := svc.OutputSecretValues()
	info := cfg.PathInfo()
	secrets = append(secrets, privatePaths(info.ConfigFile, info.ConfigDir)...)
	return makeResult(name, in.operation, version, data, secrets)
}

// Run 占用 reader/writer 直到 EOF 或关闭；stdout 仅由 SDK 写入 JSON-RPC。
func Run(ctx context.Context, reader io.ReadCloser, writer io.WriteCloser) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	s, err := newServer(config.LoadReadOnlySnapshot, func(ctx context.Context, svc *service.Service, id string, values map[string]any) map[string]any {
		return execution.Execute(ctx, svc, id, values)
	})
	if err != nil {
		return errors.New("MCP server initialization failed")
	}
	r := &closingReader{ReadCloser: reader, cancel: cancel}
	err = s.Run(ctx, stdioTransport{reader: r, writer: writer})
	if r.eof.Load() || errors.Is(err, context.Canceled) {
		return nil
	}
	if err != nil {
		return errors.New("MCP transport failed")
	}
	return nil
}

type closingReader struct {
	io.ReadCloser
	cancel context.CancelFunc
	eof    atomic.Bool
}

func (r *closingReader) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	if errors.Is(err, io.EOF) {
		r.eof.Store(true)
		r.cancel()
	}
	return n, err
}

type boundedWriter struct{ io.WriteCloser }

func (w boundedWriter) Write(p []byte) (int, error) {
	if len(p) > maxResponseBytes {
		return 0, errors.New("MCP response exceeds limit")
	}
	return w.WriteCloser.Write(p)
}
