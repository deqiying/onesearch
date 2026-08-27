package providers

import (
	"context"
	"strings"
	"time"
)

type Exa struct {
	APIURL  string
	APIKey  string
	Timeout time.Duration
}

type ExaOptions struct {
	NumResults         int
	SearchType         string
	IncludeText        bool
	IncludeHighlights  bool
	StartPublishedDate string
	IncludeDomains     []string
	ExcludeDomains     []string
	Category           string
}

type ExaFetchOptions struct {
	MaxCharacters int
}

func (p Exa) Search(ctx context.Context, query string, options ExaOptions) map[string]any {
	start := time.Now()
	if options.NumResults <= 0 {
		options.NumResults = 5
	}
	if options.NumResults > 100 {
		return map[string]any{"ok": false, "provider": "exa", "query": query, "error_type": "parameter_error", "error": "Exa num_results must be between 1 and 100", "elapsed_ms": Elapsed(start)}
	}
	if options.SearchType == "" {
		options.SearchType = "auto"
	}
	if strings.EqualFold(options.SearchType, "neural") {
		options.SearchType = "auto"
	}
	options.SearchType = strings.ToLower(strings.TrimSpace(options.SearchType))
	if !containsExaSearchType(options.SearchType) {
		return map[string]any{"ok": false, "provider": "exa", "query": query, "error_type": "parameter_error", "error": "unsupported Exa search type: " + options.SearchType, "elapsed_ms": Elapsed(start)}
	}
	payload := map[string]any{
		"query":      query,
		"numResults": options.NumResults,
		"type":       options.SearchType,
	}
	if options.IncludeText || options.IncludeHighlights {
		payload["contents"] = map[string]any{
			"text":       options.IncludeText,
			"highlights": options.IncludeHighlights,
		}
	}
	if options.StartPublishedDate != "" {
		payload["startPublishedDate"] = options.StartPublishedDate
	}
	if len(options.IncludeDomains) > 0 {
		payload["includeDomains"] = options.IncludeDomains
	}
	if len(options.ExcludeDomains) > 0 {
		payload["excludeDomains"] = options.ExcludeDomains
	}
	if options.Category != "" {
		payload["category"] = options.Category
	}
	var data map[string]any
	err := PostJSON(ctx, Client(p.Timeout), strings.TrimRight(p.APIURL, "/")+"/search", map[string]string{"x-api-key": p.APIKey}, payload, &data)
	if err != nil {
		errorType, message := ErrorPayload(err)
		return map[string]any{"ok": false, "provider": "exa", "query": query, "error_type": errorType, "error": message, "elapsed_ms": Elapsed(start)}
	}
	results := normalizeExaResults(asMaps(data["results"]), options.IncludeText, options.IncludeHighlights)
	out := map[string]any{
		"ok":          true,
		"provider":    "exa",
		"query":       query,
		"search_type": firstNonEmpty(stringValue(data["searchType"]), options.SearchType),
		"results":     results,
		"total":       len(results),
		"elapsed_ms":  Elapsed(start),
	}
	for source, target := range map[string]string{"requestId": "request_id", "costDollars": "cost_dollars", "output": "output", "grounding": "grounding"} {
		if value := data[source]; value != nil {
			out[target] = value
		}
	}
	return out
}

func containsExaSearchType(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "auto", "fast", "instant", "deep-lite", "deep", "deep-reasoning":
		return true
	default:
		return false
	}
}

func (p Exa) Fetch(ctx context.Context, urls []string, options ExaFetchOptions) map[string]any {
	start := time.Now()
	urls = nonEmptyStrings(urls)
	if len(urls) == 0 {
		return map[string]any{"ok": false, "provider": "exa", "tool": "web_fetch_exa", "error_type": "parameter_error", "error": "web_fetch_exa requires at least one url", "elapsed_ms": Elapsed(start)}
	}
	textOption := any(true)
	if options.MaxCharacters > 0 {
		textOption = map[string]any{"maxCharacters": options.MaxCharacters}
	}
	payload := map[string]any{
		"urls": urls,
		"text": textOption,
	}
	var data map[string]any
	err := PostJSON(ctx, Client(p.Timeout), strings.TrimRight(p.APIURL, "/")+"/contents", map[string]string{"x-api-key": p.APIKey}, payload, &data)
	if err != nil {
		errorType, message := ErrorPayload(err)
		return map[string]any{"ok": false, "provider": "exa", "tool": "web_fetch_exa", "urls": urls, "error_type": errorType, "error": message, "elapsed_ms": Elapsed(start)}
	}
	results := normalizeExaFetchResults(asMaps(data["results"]))
	content := joinedResultContent(results)
	out := map[string]any{
		"ok":         true,
		"provider":   "exa",
		"tool":       "web_fetch_exa",
		"urls":       urls,
		"results":    results,
		"total":      len(results),
		"content":    content,
		"elapsed_ms": Elapsed(start),
	}
	if len(urls) == 1 {
		out["url"] = urls[0]
	}
	if data["statuses"] != nil {
		out["statuses"] = data["statuses"]
	}
	for source, target := range map[string]string{"requestId": "request_id", "costDollars": "cost_dollars"} {
		if data[source] != nil {
			out[target] = data[source]
		}
	}
	if statuses := asMaps(data["statuses"]); len(statuses) > 0 {
		out["statuses"] = statuses
		failed := 0
		for _, status := range statuses {
			state := strings.ToLower(firstNonEmpty(stringValue(status["status"]), stringValue(status["state"])))
			if state != "" && state != "success" && state != "ok" && state != "completed" {
				failed++
			}
		}
		if failed > 0 {
			out["partial"] = failed < len(statuses) || len(results) > 0
			if len(results) == 0 {
				out["ok"] = false
				out["error_type"] = "partial"
				out["error"] = "Exa contents returned no successful results"
			}
		}
	}
	return out
}

func (p Exa) Similar(ctx context.Context, url string, numResults int) map[string]any {
	start := time.Now()
	if numResults <= 0 {
		numResults = 5
	}
	payload := map[string]any{"url": url, "numResults": numResults}
	var data map[string]any
	err := PostJSON(ctx, Client(p.Timeout), strings.TrimRight(p.APIURL, "/")+"/findSimilar", map[string]string{"x-api-key": p.APIKey}, payload, &data)
	if err != nil {
		errorType, message := ErrorPayload(err)
		return map[string]any{"ok": false, "url": url, "error_type": errorType, "error": message, "elapsed_ms": Elapsed(start)}
	}
	results := normalizeExaResults(asMaps(data["results"]), false, false)
	return map[string]any{"ok": true, "provider": "exa", "tool": "exa_similar", "deprecated": true, "url": url, "results": results, "total": len(results), "elapsed_ms": Elapsed(start)}
}

func normalizeExaFetchResults(items []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result := map[string]any{
			"id":            item["id"],
			"title":         stringValue(item["title"]),
			"url":           firstNonEmpty(stringValue(item["url"]), stringValue(item["id"])),
			"publishedDate": item["publishedDate"],
			"author":        stringValue(item["author"]),
		}
		for _, key := range []string{"text", "summary", "image", "favicon"} {
			if value := item[key]; stringValue(value) != "" {
				result[key] = value
			}
		}
		if highlights, ok := item["highlights"].([]any); ok {
			result["highlights"] = highlights
		}
		if scores, ok := item["highlightScores"].([]any); ok {
			result["highlightScores"] = scores
		}
		if subpages, ok := item["subpages"].([]any); ok {
			result["subpages"] = subpages
		}
		out = append(out, result)
	}
	return out
}

func normalizeExaResults(items []map[string]any, includeText, includeHighlights bool) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result := map[string]any{
			"id":            item["id"],
			"title":         stringValue(item["title"]),
			"url":           firstNonEmpty(stringValue(item["url"]), stringValue(item["id"])),
			"publishedDate": item["publishedDate"],
			"author":        stringValue(item["author"]),
			"score":         item["score"],
		}
		if includeText {
			result["text"] = stringValue(item["text"])
		}
		if includeHighlights {
			if highlights, ok := item["highlights"].([]any); ok {
				result["highlights"] = highlights
			}
		}
		if item["image"] != nil {
			result["image"] = item["image"]
		}
		if item["favicon"] != nil {
			result["favicon"] = item["favicon"]
		}
		out = append(out, result)
	}
	return out
}

func nonEmptyStrings(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if text := strings.TrimSpace(item); text != "" {
			out = append(out, text)
		}
	}
	return out
}

func joinedResultContent(items []map[string]any) string {
	var parts []string
	for _, item := range items {
		if text := strings.TrimSpace(stringValue(item["text"])); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}
