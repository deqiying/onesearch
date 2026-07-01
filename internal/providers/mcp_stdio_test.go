package providers

import "testing"

func TestNormalizeMCPToolResultParsesJSONContent(t *testing.T) {
	got := normalizeMCPToolResult(map[string]any{
		"content": []any{map[string]any{"type": "text", "text": `{"results":[{"title":"Doc","url":"https://example.com","snippet":"summary"}],"markdown":"body"}`}},
	}, "search", nil, "")
	if got.Content != "body" {
		t.Fatalf("content = %q, want body", got.Content)
	}
	if len(got.Results) != 1 || got.Results[0]["url"] != "https://example.com" || got.Results[0]["description"] != "summary" {
		t.Fatalf("results = %#v", got.Results)
	}
}

func TestNormalizeMCPToolResultExtractsURLsFromPlainText(t *testing.T) {
	got := normalizeMCPToolResult(map[string]any{
		"content": []any{map[string]any{"type": "text", "text": "See https://example.com/a and https://example.com/a."}},
	}, "search", nil, "")
	if len(got.Results) != 1 || got.Results[0]["url"] != "https://example.com/a" {
		t.Fatalf("results = %#v", got.Results)
	}
}
