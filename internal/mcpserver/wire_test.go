package mcpserver

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deqiying/onesearch/internal/config"
	"github.com/deqiying/onesearch/internal/service"
)

type wireClient struct {
	t       *testing.T
	conn    net.Conn
	reader  *bufio.Reader
	version string
}

func startWire(t *testing.T, version string, load func() (*config.Config, error), execute executeFunc) *wireClient {
	t.Helper()
	s, err := newServer(load, execute)
	if err != nil {
		t.Fatal(err)
	}
	client, server := net.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- s.Run(ctx, stdioTransport{reader: &closingReader{ReadCloser: server, cancel: cancel}, writer: server})
	}()
	t.Cleanup(func() {
		client.Close()
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("server did not stop")
		}
	})
	return &wireClient{t: t, conn: client, reader: bufio.NewReader(client), version: version}
}

func (w *wireClient) send(id any, method string, params map[string]any) {
	w.t.Helper()
	if params == nil {
		params = map[string]any{}
	}
	if w.version == "2026-07-28" {
		params["_meta"] = map[string]any{"io.modelcontextprotocol/protocolVersion": w.version, "io.modelcontextprotocol/clientCapabilities": map[string]any{}}
	}
	request := map[string]any{"jsonrpc": "2.0", "method": method, "params": params}
	if id != nil {
		request["id"] = id
	}
	body, err := json.Marshal(request)
	if err != nil {
		w.t.Fatal(err)
	}
	w.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err = w.conn.Write(append(body, '\n')); err != nil {
		w.t.Fatal(err)
	}
}
func (w *wireClient) read() map[string]any {
	w.t.Helper()
	w.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	body, err := w.reader.ReadBytes('\n')
	if err != nil {
		w.t.Fatal(err)
	}
	if len(body) > maxResponseBytes {
		w.t.Fatal("response exceeded limit")
	}
	var response map[string]any
	if err = json.Unmarshal(body, &response); err != nil {
		w.t.Fatalf("non-protocol stdout: %s", body)
	}
	return response
}
func (w *wireClient) request(id any, method string, params map[string]any) map[string]any {
	w.t.Helper()
	w.send(id, method, params)
	response := w.read()
	want, _ := json.Marshal(id)
	got, _ := json.Marshal(response["id"])
	if string(want) != string(got) {
		w.t.Fatalf("id = %s, want %s", got, want)
	}
	return response
}
func (w *wireClient) initialize() map[string]any {
	w.t.Helper()
	if w.version == "2026-07-28" {
		return w.request("discover", "server/discover", nil)
	}
	result := w.request(1, "initialize", map[string]any{"protocolVersion": w.version, "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "raw-wire-test", "version": "1"}})
	w.send(nil, "notifications/initialized", nil)
	return result
}

func TestRawWireFourVersions(t *testing.T) {
	for _, version := range protocolVersions {
		t.Run(version, func(t *testing.T) {
			t.Setenv(config.ConfigDirEnvName, t.TempDir())
			var loads atomic.Int32
			w := startWire(t, version, func() (*config.Config, error) { loads.Add(1); return config.LoadReadOnlySnapshot() }, func(context.Context, *service.Service, string, map[string]any) map[string]any {
				t.Error("unexpected backend call")
				return nil
			})
			init := w.initialize()
			if init["error"] != nil {
				t.Fatalf("initialize: %#v", init)
			}
			body, _ := json.Marshal(init)
			for _, unsupported := range []string{`"logging"`, `"prompts"`, `"resources"`, `"tasks"`, `"listChanged":true`} {
				if strings.Contains(string(body), unsupported) {
					t.Fatalf("unsupported capability: %s", body)
				}
			}
			list := w.request("list", "tools/list", nil)
			result, ok := list["result"].(map[string]any)
			if !ok {
				t.Fatalf("list: %#v", list)
			}
			tools := result["tools"].([]any)
			if len(tools) != 5 {
				t.Fatalf("tools: %d", len(tools))
			}
			for _, tool := range tools {
				_, has := tool.(map[string]any)["outputSchema"]
				if has != (version != "2025-03-26") {
					t.Fatalf("outputSchema for %s: %v", version, has)
				}
			}
			if loads.Load() != 0 {
				t.Fatal("discovery loaded configuration")
			}
			bad := w.request(3, "tools/call", map[string]any{"name": "onesearch_search", "arguments": map[string]any{"mode": "not-a-mode", "query": "q"}})
			assertWireTool(t, bad, version, "parameter_error")
			if loads.Load() != 0 {
				t.Fatal("invalid args loaded configuration")
			}
			status := w.request("status", "tools/call", map[string]any{"name": "onesearch_status", "arguments": map[string]any{}})
			assertWireTool(t, status, version, "")
			if loads.Load() != 1 {
				t.Fatalf("loads = %d", loads.Load())
			}
			ping := w.request(4, "ping", nil)
			if version == "2026-07-28" {
				if ping["error"] == nil {
					t.Fatal("modern protocol unexpectedly exposes ping")
				}
			} else if ping["error"] != nil {
				t.Fatalf("ping: %#v", ping)
			}
			unknown := w.request(5, "not/a/method", nil)
			if unknown["error"].(map[string]any)["code"] != float64(-32601) {
				t.Fatalf("unknown method: %#v", unknown)
			}
			tool := w.request(6, "tools/call", map[string]any{"name": "onesearch_shell", "arguments": map[string]any{}})
			if tool["error"] == nil {
				t.Fatalf("unknown tool not rejected: %#v", tool)
			}
			status = w.request(7, "tools/call", map[string]any{"name": "onesearch_status"})
			assertWireTool(t, status, version, "")
		})
	}
}

func assertWireTool(t *testing.T, response map[string]any, version, kind string) map[string]any {
	t.Helper()
	result, ok := response["result"].(map[string]any)
	if !ok {
		t.Fatalf("not tool result: %#v", response)
	}
	content := result["content"].([]any)
	var text map[string]any
	if err := json.Unmarshal([]byte(content[0].(map[string]any)["text"].(string)), &text); err != nil {
		t.Fatal(err)
	}
	if version == "2025-03-26" {
		if result["structuredContent"] != nil {
			t.Fatal("legacy structuredContent")
		}
	} else if !reflect.DeepEqual(text, result["structuredContent"]) {
		t.Fatal("text/structured result mismatch")
	}
	if kind == "" {
		if text["ok"] != true || result["isError"] == true {
			t.Fatalf("failure: %#v", text)
		}
	} else if text["error_type"] != kind || result["isError"] != true {
		t.Fatalf("wrong error: %#v", text)
	}
	schema, err := resolveSchema(resultSchema())
	if err != nil {
		t.Fatal(err)
	}
	if err = schema.Validate(text); err != nil {
		t.Fatal(err)
	}
	return text
}

func TestRawWireModernDirectAndUnsupportedVersion(t *testing.T) {
	t.Setenv(config.ConfigDirEnvName, t.TempDir())
	w := startWire(t, "2026-07-28", config.LoadReadOnlySnapshot, nil)
	// 现代请求无需 initialize，也不强制先 discovery。
	assertWireTool(t, w.request("direct", "tools/call", map[string]any{"name": "onesearch_status"}), w.version, "")
	w.version = "unsupported"
	response := w.request(2, "tools/list", map[string]any{"_meta": map[string]any{"io.modelcontextprotocol/protocolVersion": "2099-01-01", "io.modelcontextprotocol/clientCapabilities": map[string]any{}}})
	if response["error"] == nil {
		t.Fatalf("unsupported version accepted: %#v", response)
	}
}

func TestRawWireCancellation(t *testing.T) {
	cfg := testSnapshot(t, true)
	for _, version := range []string{"2025-11-25", "2026-07-28"} {
		t.Run(version, func(t *testing.T) {
			started, stopped := make(chan struct{}), make(chan struct{})
			w := startWire(t, version, func() (*config.Config, error) { return cfg, nil }, func(ctx context.Context, _ *service.Service, _ string, _ map[string]any) map[string]any {
				close(started)
				<-ctx.Done()
				close(stopped)
				return failure("network_error", "cancelled")
			})
			w.initialize()
			w.send("cancel-me", "tools/call", map[string]any{"name": "onesearch_search", "arguments": map[string]any{"mode": "docs", "query": "q", "docs_options": map[string]any{"library_name": "test"}}})
			select {
			case <-started:
			case <-time.After(3 * time.Second):
				t.Fatal("backend did not start")
			}
			w.send(nil, "notifications/cancelled", map[string]any{"requestId": "cancel-me", "reason": "test cancellation"})
			select {
			case <-stopped:
			case <-time.After(3 * time.Second):
				t.Fatal("cancellation not propagated")
			}
			if version != "2026-07-28" {
				response := w.read()
				assertWireTool(t, response, version, "cancelled")
			}
			if w.request("still-alive", "tools/list", nil)["error"] != nil {
				t.Fatal("session failed after cancellation")
			}
			if version == "2026-07-28" {
				w.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
				body, err := w.reader.ReadBytes('\n')
				if err == nil || len(body) != 0 {
					t.Fatalf("unexpected post-cancel response: %s, %v", body, err)
				}
				if timeout, ok := err.(net.Error); !ok || !timeout.Timeout() {
					t.Fatalf("session closed after cancellation: %v", err)
				}
			}
		})
	}
}
