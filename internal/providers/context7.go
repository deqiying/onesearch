package providers

import (
	"context"
	"net/url"
	"strings"
	"time"
)

type Context7 struct {
	APIURL  string
	APIKey  string
	Timeout time.Duration
}

func (p Context7) Library(ctx context.Context, name, query string) map[string]any {
	start := time.Now()
	requestQuery := strings.TrimSpace(name + " " + query)
	endpoint := strings.TrimRight(p.APIURL, "/") + "/api/v2/search?query=" + url.QueryEscape(requestQuery)
	var data any
	err := GetJSON(ctx, Client(p.Timeout), endpoint, p.headers(), &data)
	if err != nil {
		errorType, message := ErrorPayload(err)
		return map[string]any{"ok": false, "query": requestQuery, "provider": "context7", "error_type": errorType, "error": message, "elapsed_ms": Elapsed(start)}
	}
	var raw []map[string]any
	if list, ok := data.([]any); ok {
		raw = asMaps(list)
	} else if m, ok := data.(map[string]any); ok {
		raw = asMaps(m["results"])
	}
	results := normalizeContext7Libraries(raw)
	return map[string]any{"ok": true, "query": requestQuery, "provider": "context7", "results": results, "total": len(results), "elapsed_ms": Elapsed(start)}
}

func (p Context7) Docs(ctx context.Context, libraryID, query string) map[string]any {
	start := time.Now()
	endpoint := strings.TrimRight(p.APIURL, "/") + "/api/v2/context?libraryId=" + url.QueryEscape(libraryID) + "&query=" + url.QueryEscape(query)
	var data map[string]any
	err := GetJSON(ctx, Client(p.Timeout), endpoint, p.headers(), &data)
	if err != nil {
		errorType, message := ErrorPayload(err)
		return map[string]any{"ok": false, "library_id": libraryID, "query": query, "provider": "context7", "error_type": errorType, "error": message, "elapsed_ms": Elapsed(start)}
	}
	code := asAnySlice(data["codeSnippets"])
	info := asAnySlice(data["infoSnippets"])
	results := append(append([]any{}, code...), info...)
	return map[string]any{
		"ok":            true,
		"library_id":    libraryID,
		"query":         query,
		"provider":      "context7",
		"code_snippets": code,
		"info_snippets": info,
		"results":       results,
		"total":         len(results),
		"content":       mustJSON(data),
		"elapsed_ms":    Elapsed(start),
	}
}

func (p Context7) headers() map[string]string {
	headers := map[string]string{"X-Context7-Source": "onesearch"}
	if p.APIKey != "" {
		headers["Authorization"] = "Bearer " + p.APIKey
	}
	return headers
}

func normalizeContext7Libraries(items []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]any{
			"id":              stringValue(item["id"]),
			"title":           stringValue(item["title"]),
			"description":     stringValue(item["description"]),
			"trust_score":     item["trustScore"],
			"benchmark_score": item["benchmarkScore"],
			"total_snippets":  item["totalSnippets"],
			"stars":           item["stars"],
			"provider":        "context7",
		})
	}
	return out
}
