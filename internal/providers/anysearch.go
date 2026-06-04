package providers

import (
	"context"
	"regexp"
	"strings"
	"time"
)

type AnySearch struct {
	APIURL  string
	APIKey  string
	Timeout time.Duration
}

func (p AnySearch) Call(ctx context.Context, name string, arguments map[string]any) map[string]any {
	start := time.Now()
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  map[string]any{"name": name, "arguments": arguments},
	}
	headers := map[string]string{"Accept": "application/json, text/event-stream"}
	if p.APIKey != "" {
		headers["Authorization"] = "Bearer " + p.APIKey
	}
	var data map[string]any
	err := PostJSON(ctx, Client(p.Timeout), p.APIURL, headers, payload, &data)
	if err != nil {
		errorType, message := ErrorPayload(err)
		return map[string]any{"ok": false, "provider": "anysearch", "tool": name, "error_type": errorType, "error": message, "elapsed_ms": Elapsed(start)}
	}
	if rawError, ok := data["error"]; ok {
		return map[string]any{"ok": false, "provider": "anysearch", "tool": name, "error_type": "provider_error", "error": stringValue(rawError), "elapsed_ms": Elapsed(start)}
	}
	result, _ := data["result"].(map[string]any)
	text := extractAnySearchText(result)
	isError, _ := result["isError"].(bool)
	parsed := []map[string]any{}
	if !isError {
		parsed = parseAnySearchMarkdownResults(text)
		if len(parsed) == 0 && strings.TrimSpace(text) != "" {
			parsed = append(parsed, map[string]any{
				"title":         name + " structured evidence",
				"url":           "",
				"description":   truncate(text, 500),
				"evidence_type": "structured",
				"raw_content":   text,
			})
		}
	}
	out := map[string]any{
		"ok":          !isError,
		"provider":    "anysearch",
		"tool":        name,
		"content":     text,
		"raw_content": text,
		"results":     parsed,
		"total":       len(parsed),
		"elapsed_ms":  Elapsed(start),
	}
	for _, key := range []string{"query", "domain", "sub_domain", "url"} {
		if value, ok := arguments[key]; ok && stringValue(value) != "" {
			out[key] = value
		}
	}
	if isError {
		out["error_type"] = "provider_error"
		out["error"] = firstNonEmpty(text, "AnySearch tool returned isError=true")
	}
	return out
}

func (p AnySearch) Domains(ctx context.Context, domain string) map[string]any {
	args := map[string]any{}
	if domain != "" {
		args["domain"] = domain
	}
	return p.Call(ctx, "list_domains", args)
}

func (p AnySearch) Search(ctx context.Context, query, domain, subDomain string, maxResults int) map[string]any {
	args := map[string]any{"query": query, "max_results": maxResults}
	domain, subDomain = splitDomain(domain, subDomain)
	if domain != "" {
		args["domain"] = domain
	}
	if subDomain != "" {
		args["sub_domain"] = subDomain
	}
	return p.Call(ctx, "search", args)
}

func (p AnySearch) Extract(ctx context.Context, targetURL string, maxLength int) map[string]any {
	return p.Call(ctx, "extract", map[string]any{"url": targetURL, "max_length": maxLength})
}

func (p AnySearch) Batch(ctx context.Context, queries []string, maxResults int) map[string]any {
	if len(queries) > 5 {
		return map[string]any{"ok": false, "provider": "anysearch", "tool": "batch_search", "error_type": "parameter_error", "error": "too many queries: " + stringValue(len(queries)) + " (max 5)", "elapsed_ms": 0}
	}
	items := make([]map[string]any, 0, len(queries))
	for _, query := range queries {
		items = append(items, map[string]any{"query": query, "max_results": maxResults})
	}
	return p.Call(ctx, "batch_search", map[string]any{"queries": items})
}

func extractAnySearchText(result map[string]any) string {
	content := result["content"]
	switch value := content.(type) {
	case string:
		return strings.TrimSpace(value)
	case []any:
		var parts []string
		for _, item := range value {
			if m, ok := item.(map[string]any); ok {
				if text := stringValue(m["text"]); text != "" {
					parts = append(parts, text)
				}
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n"))
	default:
		return ""
	}
}

func parseAnySearchMarkdownResults(text string) []map[string]any {
	var out []map[string]any
	var current map[string]any
	heading := regexp.MustCompile(`^###\s+\d+\.\s+(.+?)\s*$`)
	urlLine := regexp.MustCompile(`^-\s+\*\*URL\*\*:\s+(\S+)`)
	for _, line := range strings.Split(text, "\n") {
		if match := heading.FindStringSubmatch(line); match != nil {
			if current != nil {
				out = append(out, current)
			}
			current = map[string]any{"title": strings.TrimSpace(match[1]), "url": "", "description": ""}
			continue
		}
		if current == nil {
			continue
		}
		if match := urlLine.FindStringSubmatch(line); match != nil {
			current["url"] = strings.TrimSpace(match[1])
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "- **URL**") {
			desc := strings.TrimSpace(stringValue(current["description"]) + " " + trimmed)
			current["description"] = desc
		}
	}
	if current != nil {
		out = append(out, current)
	}
	if len(out) > 0 {
		return out
	}
	urlRe := regexp.MustCompile(`https?://[^\s)>\]]+`)
	seen := map[string]struct{}{}
	for _, found := range urlRe.FindAllString(text, -1) {
		if _, ok := seen[found]; ok {
			continue
		}
		seen[found] = struct{}{}
		out = append(out, map[string]any{"title": found, "url": found, "description": ""})
	}
	return out
}

func splitDomain(domain, subDomain string) (string, string) {
	if subDomain != "" || !strings.Contains(domain, ".") {
		return domain, subDomain
	}
	parts := strings.SplitN(domain, ".", 2)
	return parts[0], parts[1]
}

func truncate(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	return text[:limit]
}
