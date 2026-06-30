package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExaFetchSendsContentsPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/contents" {
			t.Fatalf("path = %q, want /contents", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "exa-key" {
			t.Fatalf("x-api-key = %q", r.Header.Get("x-api-key"))
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		urls := payload["urls"].([]any)
		if len(urls) != 2 || urls[0] != "https://example.com/a" || urls[1] != "https://example.com/b" {
			t.Fatalf("urls payload = %#v", payload["urls"])
		}
		text := payload["text"].(map[string]any)
		if text["maxCharacters"] != float64(1234) {
			t.Fatalf("text payload = %#v", text)
		}
		_, _ = w.Write([]byte(`{"results":[{"id":"1","url":"https://example.com/a","title":"A","text":"alpha"},{"id":"2","url":"https://example.com/b","title":"B","text":"beta"}]}`))
	}))
	defer server.Close()

	got := (Exa{APIURL: server.URL, APIKey: "exa-key"}).Fetch(context.Background(), []string{"https://example.com/a", "https://example.com/b"}, ExaFetchOptions{MaxCharacters: 1234})
	if got["ok"] != true || got["provider"] != "exa" || got["tool"] != "web_fetch_exa" {
		t.Fatalf("fetch result envelope = %#v", got)
	}
	if got["content"] != "alpha\n\nbeta" {
		t.Fatalf("content = %#v", got["content"])
	}
}
