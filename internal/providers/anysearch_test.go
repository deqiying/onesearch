package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func anySearchTextResult(text string) map[string]any {
	return map[string]any{"content": []any{map[string]any{"type": "text", "text": text}}}
}

// anySearchTestServer 返回固定的 tools/call 结果，使测试不依赖真实 AnySearch 端点。
func anySearchTestServer(t *testing.T, result map[string]any) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if method := r.Header.Get("Mcp-Method"); method != "tools/call" {
			t.Fatalf("Mcp-Method = %q, want tools/call", method)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": result})
	}))
	t.Cleanup(server.Close)
	return server
}

func TestAnySearchExtractUnwrapsMCPTextPayload(t *testing.T) {
	payload := `{"url":"https://example.com/page","title":"Example Page","content":"# Heading\n\nbody"}`
	server := anySearchTestServer(t, anySearchTextResult(payload))

	got := (AnySearch{APIURL: server.URL}).Call(context.Background(), "extract", map[string]any{"url": "https://example.com/page"})

	if got["ok"] != true || got["provider"] != "anysearch" || got["tool"] != "extract" || got["resolved_tool"] != "extract" {
		t.Fatalf("extract envelope = %#v", got)
	}
	if got["content"] != "# Heading\n\nbody" || got["raw_content"] != "# Heading\n\nbody" {
		t.Fatalf("content = %#v, raw_content = %#v", got["content"], got["raw_content"])
	}
	if got["title"] != "Example Page" || got["url"] != "https://example.com/page" {
		t.Fatalf("title = %#v, url = %#v", got["title"], got["url"])
	}
	results := asMapSlice(t, got["results"])
	if len(results) != 1 || got["total"] != 1 {
		t.Fatalf("results = %#v, total = %#v", results, got["total"])
	}
	if results[0]["title"] != "Example Page" || results[0]["url"] != "https://example.com/page" {
		t.Fatalf("evidence item = %#v", results[0])
	}
	if results[0]["evidence_type"] != "page_extract" || results[0]["description"] != "# Heading\n\nbody" {
		t.Fatalf("evidence item = %#v", results[0])
	}
}

func TestAnySearchExtractUnwrapsRESTEnvelope(t *testing.T) {
	payload := `{"data":{"url":"https://example.com/rest","title":"REST Page","content":"rest body"}}`
	server := anySearchTestServer(t, anySearchTextResult(payload))

	got := (AnySearch{APIURL: server.URL}).Extract(context.Background(), "https://example.com/rest", 20000)

	if got["ok"] != true || got["content"] != "rest body" || got["title"] != "REST Page" {
		t.Fatalf("extract result = %#v", got)
	}
	results := asMapSlice(t, got["results"])
	if len(results) != 1 || results[0]["url"] != "https://example.com/rest" {
		t.Fatalf("evidence item = %#v", results)
	}
}

func TestAnySearchExtractPrefersStructuredContent(t *testing.T) {
	server := anySearchTestServer(t, map[string]any{
		"structuredContent": map[string]any{"url": "https://example.com/structured", "title": "Structured", "content": "structured body"},
		"content":           []any{map[string]any{"type": "text", "text": "unparsable note"}},
	})

	got := (AnySearch{APIURL: server.URL}).Call(context.Background(), "mcp__anysearch__extract", map[string]any{"url": "https://example.com/structured"})

	if got["content"] != "structured body" || got["title"] != "Structured" {
		t.Fatalf("extract result = %#v", got)
	}
	if got["resolved_tool"] != "mcp__anysearch__extract" {
		t.Fatalf("resolved_tool = %#v", got["resolved_tool"])
	}
}

func TestAnySearchExtractKeepsRawTextWhenPayloadIsNotJSON(t *testing.T) {
	text := "# Plain page\n\nno json envelope here"
	server := anySearchTestServer(t, anySearchTextResult(text))

	got := (AnySearch{APIURL: server.URL}).Extract(context.Background(), "https://example.com/plain", 20000)

	if got["ok"] != true || got["content"] != text || got["raw_content"] != text {
		t.Fatalf("extract result = %#v", got)
	}
	if _, ok := got["title"]; ok {
		t.Fatalf("unexpected title = %#v", got["title"])
	}
	results := asMapSlice(t, got["results"])
	if len(results) != 1 || results[0]["url"] != "" || results[0]["evidence_type"] != "structured" {
		t.Fatalf("evidence item = %#v", results)
	}
	if results[0]["title"] != "extract structured evidence" {
		t.Fatalf("evidence title = %#v", results[0]["title"])
	}
}

func TestAnySearchExtractTruncatesLocally(t *testing.T) {
	payload := `{"url":"https://example.com/long","title":"Long","content":"0123456789ABCDEF"}`
	server := anySearchTestServer(t, anySearchTextResult(payload))

	got := (AnySearch{APIURL: server.URL}).Extract(context.Background(), "https://example.com/long", 10)

	if got["content"] != "0123456789" || got["raw_content"] != "0123456789" {
		t.Fatalf("truncated content = %#v", got["content"])
	}
	for _, key := range []string{"content_truncated", "raw_content_truncated"} {
		if _, ok := got[key]; ok {
			t.Fatalf("local max_length truncation must not set %s", key)
		}
	}
}

func TestAnySearchSearchStillParsesMarkdownResults(t *testing.T) {
	text := "### 1. Title One\n- **URL**: https://example.com/a\nsnippet text\n"
	server := anySearchTestServer(t, anySearchTextResult(text))

	got := (AnySearch{APIURL: server.URL}).Call(context.Background(), "search", map[string]any{"query": "anything"})

	if got["content"] != strings.TrimSpace(text) {
		t.Fatalf("search content = %#v", got["content"])
	}
	results := asMapSlice(t, got["results"])
	if len(results) != 1 || results[0]["url"] != "https://example.com/a" || results[0]["title"] != "Title One" {
		t.Fatalf("search results = %#v", results)
	}
}

func TestAnySearchExtractErrorResultStaysUntouched(t *testing.T) {
	server := anySearchTestServer(t, map[string]any{
		"isError": true,
		"content": []any{map[string]any{"type": "text", "text": "extract failed upstream"}},
	})

	got := (AnySearch{APIURL: server.URL}).Extract(context.Background(), "https://example.com/broken", 20000)

	if got["ok"] != false || got["error_type"] != "provider_error" || got["error"] != "extract failed upstream" {
		t.Fatalf("error result = %#v", got)
	}
	if len(asMapSlice(t, got["results"])) != 0 {
		t.Fatalf("error results = %#v", got["results"])
	}
}

func TestIsAnySearchExtractTool(t *testing.T) {
	cases := map[string]bool{
		"extract":                      true,
		"Extract":                      true,
		" mcp__anysearch__extract ":    true,
		"mcp__anysearch__batch_search": false,
		"search":                       false,
		"get_sub_domains":              false,
		"":                             false,
	}
	for name, want := range cases {
		if got := isAnySearchExtractTool(name); got != want {
			t.Fatalf("isAnySearchExtractTool(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestAnySearchExtractFallsBackToArgumentURL(t *testing.T) {
	// 上游只回正文、不回 url 时，用调用参数补全证据 URL。
	payload := `{"title":"No URL","content":"body only"}`
	server := anySearchTestServer(t, anySearchTextResult(payload))

	got := (AnySearch{APIURL: server.URL}).Extract(context.Background(), "https://example.com/arg", 0)

	if got["url"] != "https://example.com/arg" || got["title"] != "No URL" {
		t.Fatalf("extract result = %#v", got)
	}
	results := asMapSlice(t, got["results"])
	if len(results) != 1 || results[0]["url"] != "https://example.com/arg" {
		t.Fatalf("evidence item = %#v", results)
	}
}

func TestAnySearchExtractIgnoresUnrelatedJSON(t *testing.T) {
	cases := map[string]string{
		"search-shaped":  `{"results":[{"title":"A","url":"https://example.com/a"}]}`,
		"error-shaped":   `{"error":"rate limited","code":429}`,
		"object-content": `{"url":"https://example.com/obj","content":{"text":"nested"}}`,
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			server := anySearchTestServer(t, anySearchTextResult(payload))

			got := (AnySearch{APIURL: server.URL}).Extract(context.Background(), "https://example.com/x", 0)

			if got["content"] != payload || got["raw_content"] != payload {
				t.Fatalf("content = %#v", got["content"])
			}
			if _, ok := got["title"]; ok {
				t.Fatalf("unexpected title = %#v", got["title"])
			}
			results := asMapSlice(t, got["results"])
			if len(results) != 1 || results[0]["evidence_type"] != "structured" || results[0]["url"] != "" {
				t.Fatalf("evidence item = %#v", results)
			}
		})
	}
}

func TestAnySearchExtractAcceptsArrayAndMarkdownPayloads(t *testing.T) {
	arrayPayload := `[{"url":"https://example.com/array","content":"array body"}]`
	server := anySearchTestServer(t, anySearchTextResult(arrayPayload))

	got := (AnySearch{APIURL: server.URL}).Call(context.Background(), "extract", map[string]any{"url": "https://example.com/array"})

	if got["content"] != "array body" || got["url"] != "https://example.com/array" {
		t.Fatalf("array payload result = %#v", got)
	}

	markdownPayload := `{"markdown":"markdown body"}`
	server = anySearchTestServer(t, anySearchTextResult(markdownPayload))

	got = (AnySearch{APIURL: server.URL}).Call(context.Background(), "extract", map[string]any{"url": "https://example.com/md"})

	if got["content"] != "markdown body" {
		t.Fatalf("markdown payload result = %#v", got)
	}
}

func TestAnySearchExtractTruncationFlagFollowsUnwrappedContent(t *testing.T) {
	// 文本超过 boundedMCPText 上限时 JSON 已不可用，正文改由 structuredContent 提供，
	// 因此不能沿用“文本被截断”的标记位。
	large := strings.Repeat("x", 70*1024)
	server := anySearchTestServer(t, map[string]any{
		"structuredContent": map[string]any{"url": "https://example.com/big", "title": "Big", "content": "structured body"},
		"content":           []any{map[string]any{"type": "text", "text": `{"url":"https://example.com/big","content":"` + large + `"}`}},
	})

	got := (AnySearch{APIURL: server.URL}).Call(context.Background(), "extract", map[string]any{"url": "https://example.com/big"})

	if got["content"] != "structured body" || got["title"] != "Big" {
		t.Fatalf("unwrapped result = %#v", got)
	}
	for _, key := range []string{"content_truncated", "raw_content_truncated"} {
		if _, ok := got[key]; ok {
			t.Fatalf("unwrapped structured content must not set %s", key)
		}
	}

	// 没有 structuredContent 时，被截断的文本无法解包，只能回退并如实标注截断。
	server = anySearchTestServer(t, anySearchTextResult(`{"url":"https://example.com/big","content":"`+large+`"}`))

	got = (AnySearch{APIURL: server.URL}).Call(context.Background(), "extract", map[string]any{"url": "https://example.com/big"})

	if got["content_truncated"] != true || got["raw_content_truncated"] != true {
		t.Fatalf("truncated raw text must keep flags: %#v", got)
	}
	results := asMapSlice(t, got["results"])
	if len(results) != 1 || results[0]["evidence_type"] != "structured" {
		t.Fatalf("evidence item = %#v", results)
	}
}

func asMapSlice(t *testing.T, value any) []map[string]any {
	t.Helper()
	items, ok := value.([]map[string]any)
	if !ok {
		t.Fatalf("value is not []map[string]any: %#v", value)
	}
	return items
}
