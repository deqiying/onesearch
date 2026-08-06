package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIEndpointURLAddsV1WhenMissing(t *testing.T) {
	got := openAIEndpointURL("https://relay.example.com/openai", "responses")
	want := "https://relay.example.com/openai/v1/responses"
	if got != want {
		t.Fatalf("endpoint = %q, want %q", got, want)
	}
}

func TestOpenAIEndpointURLDoesNotDuplicateV1(t *testing.T) {
	got := openAIEndpointURL("https://relay.example.com/openai/v1/", "/responses")
	want := "https://relay.example.com/openai/v1/responses"
	if got != want {
		t.Fatalf("endpoint = %q, want %q", got, want)
	}
}

func TestOpenAICompatibleParsesJSONAndSSE(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %q, want /v1/chat/completions", r.URL.Path)
		}
		if calls == 1 {
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			if _, ok := payload["tools"]; ok {
				t.Fatalf("default chat payload should not include tools: %#v", payload)
			}
			if _, ok := payload["tool_choice"]; ok {
				t.Fatalf("default chat payload should not include tool_choice: %#v", payload)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"json ok"}}]}`))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"sse\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\" ok\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	jsonText, err := (OpenAICompatible{APIURL: server.URL, APIKey: "key", Model: "gpt-test"}).Search(context.Background(), "q", "")
	if err != nil {
		t.Fatal(err)
	}
	if jsonText != "json ok" {
		t.Fatalf("json response = %q", jsonText)
	}
	sseText, err := (OpenAICompatible{APIURL: server.URL, APIKey: "key", Model: "gpt-test", Stream: true}).Search(context.Background(), "q", "")
	if err != nil {
		t.Fatal(err)
	}
	if sseText != "sse ok" {
		t.Fatalf("sse response = %q", sseText)
	}
}

func TestOpenAICompatibleSendsConfiguredToolsAndToolChoice(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["tool_choice"] != "auto" {
			t.Fatalf("tool_choice = %#v, want auto", payload["tool_choice"])
		}
		if tools, ok := payload["tools"].([]any); !ok || len(tools) != 1 || tools[0].(map[string]any)["type"] != "web_search" {
			t.Fatalf("tools = %#v, want web_search", payload["tools"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer server.Close()

	got, err := (OpenAICompatible{APIURL: server.URL, APIKey: "key", Model: "gpt-test", Tools: []map[string]any{{"type": "web_search"}}, ToolChoice: "auto"}).Search(context.Background(), "q", "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "ok" {
		t.Fatalf("response = %q, want ok", got)
	}
}

func TestHTTPErrorClassifiesUpstreamJSON(t *testing.T) {
	err := &HTTPError{
		StatusCode:   http.StatusBadGateway,
		Status:       "502 Bad Gateway",
		Body:         `{"error":{"type":"upstream_error","message":"Upstream request failed"}}`,
		ProviderType: "upstream_error",
		Message:      "Upstream request failed",
	}
	errorType, message := ErrorPayload(err)
	if errorType != "upstream_error" || !strings.Contains(message, "HTTP 502") {
		t.Fatalf("errorType=%q message=%q", errorType, message)
	}
}
