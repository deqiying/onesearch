package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDeepWikiAskUsesMCPToolCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["method"] != "tools/call" {
			t.Fatalf("method = %#v, want tools/call", payload["method"])
		}
		params := payload["params"].(map[string]any)
		if params["name"] != "ask_question" {
			t.Fatalf("tool name = %#v, want ask_question", params["name"])
		}
		args := params["arguments"].(map[string]any)
		if args["repoName"] != "microsoft/playwright" || args["question"] != "architecture?" {
			t.Fatalf("arguments = %#v", args)
		}
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"Playwright wiki answer"}]}}`))
	}))
	defer server.Close()

	got := DeepWiki{APIURL: server.URL, Timeout: 0}.Ask(context.Background(), "microsoft/playwright", "architecture?")
	if got["ok"] != true || got["content"] != "Playwright wiki answer" || got["repo"] != "microsoft/playwright" {
		t.Fatalf("result = %#v", got)
	}
}

func TestDeepWikiParsesSSEResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"Wiki structure\"}]}}\n\n"))
	}))
	defer server.Close()

	got := DeepWiki{APIURL: server.URL, Timeout: 0}.Structure(context.Background(), "microsoft/playwright")
	if got["ok"] != true || got["content"] != "Wiki structure" {
		t.Fatalf("result = %#v", got)
	}
}

func TestDeepWikiParsesFinalSSEResponseAfterProgressEvents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/progress\",\"params\":{\"message\":\"working\"}}\n\n"))
		_, _ = w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"Final wiki answer\"}]}}\n\n"))
	}))
	defer server.Close()

	got := DeepWiki{APIURL: server.URL, Timeout: 0}.Ask(context.Background(), "microsoft/playwright", "architecture?")
	if got["ok"] != true || got["content"] != "Final wiki answer" {
		t.Fatalf("result = %#v", got)
	}
}

func TestDeepWikiTreatsEmptyTextAsFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[]}}`))
	}))
	defer server.Close()

	got := DeepWiki{APIURL: server.URL, Timeout: 0}.Ask(context.Background(), "microsoft/playwright", "architecture?")
	if got["ok"] == true || got["error_type"] != "empty_result" {
		t.Fatalf("empty result should fail: %#v", got)
	}
}
