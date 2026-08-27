package mcpclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPModernCallCarriesRevisionHeadersAndMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("MCP-Protocol-Version") != ModernProtocolVersion || r.Header.Get("Mcp-Method") != "tools/call" || r.Header.Get("Mcp-Name") != "search" {
			t.Fatalf("headers = %#v", r.Header)
		}
		var request Request
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		meta, ok := request.Params["_meta"].(map[string]any)
		if !ok || meta["io.modelcontextprotocol/protocolVersion"] != ModernProtocolVersion {
			t.Fatalf("meta = %#v", request.Params["_meta"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": map[string]any{"content": []any{map[string]any{"type": "text", "text": "ok"}}}})
	}))
	defer server.Close()
	result, err := NewHTTP(Config{Endpoint: server.URL}).CallTool(context.Background(), "search", map[string]any{"query": "go"})
	if err != nil || result["content"] == nil {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestHTTPParsesMultilineSSEAndRejectsMismatchedID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"ok\"}]}}\n\n"))
	}))
	defer server.Close()
	result, err := NewHTTP(Config{Endpoint: server.URL}).CallTool(context.Background(), "search", nil)
	if err != nil || result == nil {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if !strings.Contains(string(mustJSON(result)), "ok") {
		t.Fatalf("result=%#v", result)
	}
}

func TestHTTPFallsBackToLegacyLifecycleOnlyOnEmpty400(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request Request
		_ = json.NewDecoder(r.Body).Decode(&request)
		switch request.Method {
		case "tools/call":
			if r.Header.Get("MCP-Protocol-Version") != "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": map[string]any{"content": []any{map[string]any{"type": "text", "text": "legacy"}}}})
		case "initialize":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": map[string]any{"protocolVersion": LegacyProtocolVersion}})
		default:
			w.WriteHeader(http.StatusAccepted)
		}
	}))
	defer server.Close()
	result, err := NewHTTP(Config{Endpoint: server.URL}).CallTool(context.Background(), "search", nil)
	if err != nil || result == nil {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func mustJSON(value any) []byte { data, _ := json.Marshal(value); return data }
