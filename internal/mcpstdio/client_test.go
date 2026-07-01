package mcpstdio

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestCallToolWithFakeServer(t *testing.T) {
	result, err := CallTool(t.Context(), helperConfig("ok"), "search", map[string]any{"query": "golang"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Tools) != 1 || result.Tools[0].Name != "search" {
		t.Fatalf("tools = %#v", result.Tools)
	}
	content := result.Result["content"].([]any)
	text := content[0].(map[string]any)["text"]
	if !strings.Contains(text.(string), "https://example.com/golang") {
		t.Fatalf("tool content = %#v", result.Result)
	}
}

func TestCallToolAcceptsCodexNamespacedToolName(t *testing.T) {
	result, err := CallTool(t.Context(), helperConfig("ok"), "mcp__freecrawl__search", map[string]any{"query": "golang"})
	if err != nil {
		t.Fatal(err)
	}
	content := result.Result["content"].([]any)
	text := content[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "https://example.com/golang") {
		t.Fatalf("tool content = %#v", result.Result)
	}
}

func TestCallToolReportsJSONRPCError(t *testing.T) {
	_, err := CallTool(t.Context(), helperConfig("rpc_error"), "search", map[string]any{"query": "golang"})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error = %v, want boom", err)
	}
}

func TestCallToolRejectsNonJSONStdout(t *testing.T) {
	_, err := CallTool(t.Context(), helperConfig("non_json"), "search", map[string]any{"query": "golang"})
	if err == nil || !strings.Contains(err.Error(), "non-JSON stdout") {
		t.Fatalf("error = %v, want protocol error", err)
	}
}

func helperConfig(mode string) Config {
	return Config{
		Command: os.Args[0],
		Args:    []string{"-test.run=TestMCPStdioHelperProcess"},
		Env: map[string]string{
			"GO_WANT_MCPSTDIO_HELPER": "1",
			"MCPSTDIO_HELPER_MODE":    mode,
		},
		Timeout: 2 * time.Second,
	}
}

func TestMCPStdioHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_MCPSTDIO_HELPER") != "1" {
		return
	}
	mode := os.Getenv("MCPSTDIO_HELPER_MODE")
	if mode == "non_json" {
		os.Stdout.WriteString("not json\n")
		os.Exit(0)
	}
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var request map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &request); err != nil {
			os.Exit(2)
		}
		id, hasID := request["id"]
		if !hasID {
			continue
		}
		switch request["method"] {
		case "initialize":
			helperRespond(id, map[string]any{"protocolVersion": defaultProtocolVersion, "capabilities": map[string]any{}})
		case "tools/list":
			helperRespond(id, map[string]any{"tools": []map[string]any{{"name": "search", "description": "fake search"}}})
		case "tools/call":
			if mode == "rpc_error" {
				helperRespondError(id, -32000, "boom")
				continue
			}
			params := request["params"].(map[string]any)
			args := params["arguments"].(map[string]any)
			query := args["query"].(string)
			helperRespond(id, map[string]any{
				"content": []map[string]any{{"type": "text", "text": `{"results":[{"title":"Go","url":"https://example.com/` + query + `"}]}`}},
			})
		default:
			helperRespondError(id, -32601, "method not found")
		}
	}
	os.Exit(0)
}

func helperRespond(id any, result any) {
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
	os.Stdout.Write(append(body, '\n'))
}

func helperRespondError(id any, code int, message string) {
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": code, "message": message}})
	os.Stdout.Write(append(body, '\n'))
}
