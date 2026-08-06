package providers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestXAIResponsesKeepsResponsesJSONAndSSEWire(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
		want        string
	}{
		{
			name:        "json",
			contentType: "application/json",
			body:        `{"output":[{"type":"message","content":[{"type":"output_text","text":"json answer","annotations":[{"type":"url_citation","url":"https://example.com","title":"Example"}]}]}]}`,
			want:        "json answer",
		},
		{
			name:        "sse",
			contentType: "text/event-stream",
			body: "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"sse\"}\n\n" +
				"event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\" answer\"}\n\n" +
				"event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"output\":[]}}\n\n",
			want: "sse answer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/responses" {
					t.Fatalf("path = %q, want /v1/responses", r.URL.Path)
				}
				if !strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
					t.Fatalf("Accept = %q", r.Header.Get("Accept"))
				}
				var payload map[string]any
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				if payload["model"] != "grok-test" || payload["instructions"] != SearchPrompt || payload["stream"] != false {
					t.Fatalf("responses payload = %#v", payload)
				}
				if payload["tool_choice"] != "required" {
					t.Fatalf("tool_choice = %#v", payload["tool_choice"])
				}
				tools, ok := payload["tools"].([]any)
				if !ok || len(tools) != 1 || tools[0].(map[string]any)["type"] != "web_search" {
					t.Fatalf("tools = %#v", payload["tools"])
				}
				input, ok := payload["input"].([]any)
				if !ok || len(input) != 1 || !strings.Contains(input[0].(map[string]any)["content"].(string), "query") {
					t.Fatalf("input = %#v", payload["input"])
				}
				w.Header().Set("Content-Type", tt.contentType)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			got, err := (XAIResponses{
				APIURL:     server.URL,
				APIKey:     "key",
				Model:      "grok-test",
				Tools:      []map[string]any{{"type": "web_search"}},
				ToolChoice: "required",
			}).Search(t.Context(), "query", "GitHub")
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(got, tt.want) {
				t.Fatalf("response = %q, want text %q", got, tt.want)
			}
		})
	}
}
