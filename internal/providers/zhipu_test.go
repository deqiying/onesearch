package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestZhipuGlobalProfileUsesGlobalFieldAllowlist(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["search_engine"] != "search-prime" || payload["count"] != float64(3) {
			t.Fatalf("payload = %#v", payload)
		}
		if _, ok := payload["search_intent"]; ok {
			t.Fatalf("global payload contains CN field: %#v", payload)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "req-1", "created": 1, "request_id": "req-1", "search_result": []any{}})
	}))
	defer server.Close()
	got := (Zhipu{APIURL: server.URL, APIKey: "z-key", ProtocolProfile: "zai_global", SearchEngine: "search-prime"}).Search(context.Background(), "global query", ZhipuOptions{Count: 3})
	if got["ok"] != true || got["protocol_profile"] != "zai_global" || got["request_id"] != "req-1" {
		t.Fatalf("result = %#v", got)
	}
}

func TestZhipuRejectsCountAboveProtocolLimit(t *testing.T) {
	got := (Zhipu{ProtocolProfile: "bigmodel_cn"}).Search(context.Background(), "q", ZhipuOptions{Count: 51})
	if got["ok"] != false || got["error_type"] != "parameter_error" {
		t.Fatalf("result = %#v", got)
	}
}
