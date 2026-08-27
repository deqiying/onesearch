package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

type Context7 struct {
	APIURL               string
	APIKey               string
	Timeout              time.Duration
	LegacySearchEndpoint bool
}

func (p Context7) Library(ctx context.Context, name, query string) map[string]any {
	start := time.Now()
	name = strings.TrimSpace(name)
	query = strings.TrimSpace(query)
	requestQuery := strings.TrimSpace(name + " " + query)
	values := url.Values{}
	values.Set("libraryName", name)
	values.Set("query", query)
	endpoint := strings.TrimRight(p.APIURL, "/") + "/api/v2/libs/search?" + values.Encode()
	var data any
	err := GetJSON(ctx, Client(p.Timeout), endpoint, p.headers(), &data)
	legacyFallback := false
	if err != nil && p.LegacySearchEndpoint {
		var httpErr *HTTPError
		if AsHTTPError(err, &httpErr) && (httpErr.StatusCode == 404 || httpErr.StatusCode == 405) {
			legacyEndpoint := strings.TrimRight(p.APIURL, "/") + "/api/v2/search?query=" + url.QueryEscape(requestQuery)
			err = GetJSON(ctx, Client(p.Timeout), legacyEndpoint, p.headers(), &data)
			legacyFallback = err == nil
		}
	}
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
	return map[string]any{"ok": true, "query": requestQuery, "provider": "context7", "results": results, "total": len(results), "raw_result": boundedMap(data), "legacy_search_fallback": legacyFallback, "elapsed_ms": Elapsed(start)}
}

func (p Context7) Docs(ctx context.Context, libraryID, query string) map[string]any {
	start := time.Now()
	values := url.Values{}
	values.Set("libraryId", libraryID)
	values.Set("query", query)
	values.Set("type", "json")
	endpoint := strings.TrimRight(p.APIURL, "/") + "/api/v2/context?" + values.Encode()
	var rawData any
	err := GetJSON(ctx, Client(p.Timeout), endpoint, p.headers(), &rawData)
	if err != nil {
		errorType, message := ErrorPayload(err)
		return map[string]any{"ok": false, "library_id": libraryID, "query": query, "provider": "context7", "error_type": errorType, "error": message, "elapsed_ms": Elapsed(start)}
	}
	data, _ := rawData.(map[string]any)
	code := asAnySlice(data["codeSnippets"])
	info := asAnySlice(data["infoSnippets"])
	results := append(append([]any{}, code...), info...)
	if len(results) == 0 {
		results = asAnySlice(data["results"])
	}
	if len(results) == 0 {
		if docs := asAnySlice(data["documentation"]); len(docs) > 0 {
			results = docs
		}
	}
	if len(results) == 0 {
		if docs := asAnySlice(data["docs"]); len(docs) > 0 {
			results = docs
		}
	}
	if len(results) == 0 {
		if docs, ok := rawData.([]any); ok {
			results = docs
		}
	}
	results = normalizeContext7Docs(results)
	return map[string]any{
		"ok":            true,
		"library_id":    libraryID,
		"query":         query,
		"provider":      "context7",
		"code_snippets": code,
		"info_snippets": info,
		"results":       results,
		"total":         len(results),
		"content":       mustJSON(rawData),
		"raw_result":    boundedMap(rawData),
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
			"id":              firstNonEmpty(stringValue(item["id"]), stringValue(item["libraryId"]), stringValue(item["library_id"])),
			"title":           firstNonEmpty(stringValue(item["title"]), stringValue(item["name"])),
			"description":     firstNonEmpty(stringValue(item["description"]), stringValue(item["summary"])),
			"trust_score":     item["trustScore"],
			"benchmark_score": item["benchmarkScore"],
			"total_snippets":  item["totalSnippets"],
			"stars":           item["stars"],
			"provider":        "context7",
		})
	}
	return out
}

func boundedMap(value any) map[string]any {
	m, ok := value.(map[string]any)
	if !ok {
		return map[string]any{"value": fmt.Sprint(value)}
	}
	data, err := json.Marshal(m)
	if err != nil {
		return map[string]any{"redacted": true}
	}
	if len(data) > 64*1024 {
		return map[string]any{"truncated": true, "bytes": len(data)}
	}
	return redactContextMap(m)
}

func redactContextMap(input map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range input {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "authorization") || strings.Contains(lower, "token") || strings.Contains(lower, "api_key") || strings.Contains(lower, "secret") {
			out[key] = "[REDACTED]"
			continue
		}
		if nested, ok := value.(map[string]any); ok {
			out[key] = redactContextMap(nested)
		} else {
			out[key] = value
		}
	}
	return out
}

func normalizeContext7Docs(items []any) []any {
	out := make([]any, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			out = append(out, item)
			continue
		}
		copy := map[string]any{}
		for key, value := range m {
			copy[key] = value
		}
		if copy["id"] == nil {
			copy["id"] = firstNonEmpty(stringValue(m["libraryId"]), stringValue(m["source"]))
		}
		if copy["title"] == nil {
			copy["title"] = firstNonEmpty(stringValue(m["name"]), stringValue(m["heading"]))
		}
		if copy["description"] == nil {
			copy["description"] = firstNonEmpty(stringValue(m["content"]), stringValue(m["text"]), stringValue(m["summary"]))
		}
		out = append(out, copy)
	}
	return out
}
