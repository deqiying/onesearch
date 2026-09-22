package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/deqiying/onesearch/internal/mcpclient"
)

type AnySearch struct {
	APIURL      string
	APIKey      string
	Timeout     time.Duration
	SessionMode string
}

func (p AnySearch) Call(ctx context.Context, name string, arguments map[string]any) map[string]any {
	start := time.Now()
	client := mcpclient.NewHTTP(mcpclient.Config{Endpoint: p.APIURL, APIKey: p.APIKey, Timeout: p.Timeout, SessionMode: p.SessionMode})
	defer client.Close(context.Background())
	resolvedName := name
	var snapshot mcpclient.ToolSnapshot
	if strings.EqualFold(strings.TrimSpace(p.SessionMode), "auto") {
		var discoverErr error
		snapshot, discoverErr = client.ListTools(ctx)
		if discoverErr != nil {
			errorType, message := mcpClientErrorPayload(discoverErr)
			return map[string]any{"ok": false, "provider": "anysearch", "tool": name, "error_type": errorType, "error": message, "elapsed_ms": Elapsed(start)}
		}
		var found bool
		resolvedName, found = mcpclient.ResolveTool(snapshot, name)
		if !found {
			return map[string]any{"ok": false, "provider": "anysearch", "tool": name, "error_type": "capability_unavailable", "error": "AnySearch MCP tool not found: " + name, "elapsed_ms": Elapsed(start)}
		}
		for _, tool := range snapshot.Tools {
			if tool.Name == resolvedName {
				var validationErr error
				arguments, validationErr = filterMCPArguments(arguments, tool.InputSchema)
				if validationErr != nil {
					return map[string]any{"ok": false, "provider": "anysearch", "tool": name, "error_type": "parameter_error", "error": validationErr.Error(), "elapsed_ms": Elapsed(start)}
				}
				break
			}
		}
	}
	data, err := client.CallTool(ctx, resolvedName, arguments)
	if err != nil {
		errorType, message := mcpClientErrorPayload(err)
		return map[string]any{"ok": false, "provider": "anysearch", "tool": name, "error_type": errorType, "error": message, "elapsed_ms": Elapsed(start)}
	}
	result := data
	text := extractAnySearchText(result)
	boundedText, textTruncated := boundedMCPText(text)
	isError, _ := result["isError"].(bool)
	content := boundedText
	contentTruncated := textTruncated
	pageTitle := ""
	parsed := []map[string]any{}
	if !isError {
		if isAnySearchExtractTool(resolvedName) {
			// extract 返回单页内容，不能按 markdown 搜索结果列表解析。
			pageURL, extractTitle, pageContent, ok := anySearchExtractPayload(result, boundedText)
			if ok {
				if pageURL == "" {
					pageURL = stringValue(arguments["url"])
				}
				// 正文可能来自未被 boundedMCPText 约束的 structuredContent，
				// 因此按最终返回的正文重新判定截断标记。
				content, contentTruncated = boundedMCPText(pageContent)
				pageTitle = extractTitle
				parsed = append(parsed, map[string]any{
					"title":         firstNonEmpty(pageTitle, pageURL, "extract page content"),
					"url":           pageURL,
					"description":   truncate(content, 500),
					"evidence_type": "page_extract",
				})
			}
		} else {
			parsed = parseAnySearchMarkdownResults(boundedText)
		}
		if len(parsed) == 0 && strings.TrimSpace(boundedText) != "" {
			parsed = append(parsed, map[string]any{
				"title":         name + " structured evidence",
				"url":           "",
				"description":   truncate(boundedText, 500),
				"evidence_type": "structured",
				"raw_content":   boundedText,
			})
		}
	}
	out := map[string]any{
		"ok":            !isError,
		"provider":      "anysearch",
		"tool":          name,
		"resolved_tool": resolvedName,
		"content":       content,
		"raw_content":   content,
		"results":       parsed,
		"total":         len(parsed),
		"elapsed_ms":    Elapsed(start),
		"mcp":           map[string]any{"protocol_version": client.ProtocolVersion(), "session_mode": client.SessionMode(), "tool_name": resolvedName},
	}
	if pageTitle != "" {
		out["title"] = pageTitle
	}
	if contentTruncated {
		out["content_truncated"] = true
		out["raw_content_truncated"] = true
	}
	for _, key := range []string{"query", "domain", "sub_domain", "url"} {
		if value, ok := arguments[key]; ok && stringValue(value) != "" {
			out[key] = value
		}
	}
	if isError {
		out["error_type"] = "provider_error"
		out["error"] = firstNonEmpty(boundedText, "AnySearch tool returned isError=true")
	}
	return out
}

// isAnySearchExtractTool 判断解析后的工具名是否指向单页抽取，兼容 mcp__<server>__extract 形式。
func isAnySearchExtractTool(name string) bool {
	parts := strings.Split(strings.TrimSpace(name), "__")
	return strings.EqualFold(strings.TrimSpace(parts[len(parts)-1]), "extract")
}

// anySearchExtractPayload 把 extract 的返回解包成 url/title/content。
// MCP 端点把单页结果放在扁平 JSON 文本（{"url","title","content"}）或
// result.structuredContent 中，REST /v1/extract 再用 data 包一层，三者都归一。
// 解包失败时返回 ok=false，由调用方保留原始文本作为内容。
func anySearchExtractPayload(result map[string]any, text string) (string, string, string, bool) {
	if url, title, content, ok := anySearchExtractFields(result["structuredContent"]); ok {
		return url, title, content, true
	}
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || !json.Valid([]byte(trimmed)) {
		return "", "", "", false
	}
	var decoded any
	if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
		return "", "", "", false
	}
	return anySearchExtractFields(decoded)
}

func anySearchExtractFields(value any) (string, string, string, bool) {
	switch typed := value.(type) {
	case map[string]any:
		if data, ok := typed["data"].(map[string]any); ok && len(data) > 0 {
			if url, title, content, found := anySearchExtractFields(data); found {
				return url, title, content, true
			}
		}
		content := anySearchExtractString(typed["content"], typed["markdown"])
		if content == "" {
			return "", "", "", false
		}
		return strings.TrimSpace(stringValue(typed["url"])), strings.TrimSpace(stringValue(typed["title"])), content, true
	case []any:
		for _, item := range typed {
			if url, title, content, ok := anySearchExtractFields(item); ok {
				return url, title, content, true
			}
		}
	}
	return "", "", "", false
}

// anySearchExtractString 只接受字符串正文。非字符串值宁可回退到原始文本，
// 也不用 fmt.Sprint 把对象或数组变成难以察觉的脏内容。
func anySearchExtractString(values ...any) string {
	for _, value := range values {
		if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

func (p AnySearch) Domains(ctx context.Context, domain string) map[string]any {
	args := map[string]any{}
	if domain != "" {
		args["domain"] = domain
	}
	return p.Call(ctx, "get_sub_domains", args)
}

func (p AnySearch) DomainsList(ctx context.Context, domains []string) map[string]any {
	clean := nonEmptyStrings(domains)
	if len(clean) == 0 || len(clean) > 5 {
		return map[string]any{"ok": false, "provider": "anysearch", "tool": "get_sub_domains", "error_type": "parameter_error", "error": "AnySearch domains requires 1 to 5 non-empty domains", "elapsed_ms": 0}
	}
	if len(clean) == 1 {
		return p.Domains(ctx, clean[0])
	}
	return p.Call(ctx, "get_sub_domains", map[string]any{"domains": clean})
}

func (p AnySearch) Search(ctx context.Context, query, domain, subDomain string, maxResults int) map[string]any {
	return p.SearchWithParams(ctx, query, domain, subDomain, nil, maxResults)
}

func (p AnySearch) SearchWithParams(ctx context.Context, query, domain, subDomain string, subDomainParams map[string]any, maxResults int) map[string]any {
	if maxResults <= 0 {
		maxResults = 5
	}
	if maxResults > 10 {
		return map[string]any{"ok": false, "provider": "anysearch", "tool": "search", "error_type": "parameter_error", "error": "AnySearch max_results must be between 1 and 10", "elapsed_ms": 0}
	}
	args := map[string]any{"query": query, "max_results": maxResults}
	domain, subDomain = splitDomain(domain, subDomain)
	if domain != "" {
		args["domain"] = domain
	}
	if subDomain != "" {
		args["sub_domain"] = subDomain
	}
	if len(subDomainParams) > 0 {
		args["sub_domain_params"] = subDomainParams
	}
	return p.Call(ctx, "search", args)
}

func (p AnySearch) Extract(ctx context.Context, targetURL string, maxLength int) map[string]any {
	result := p.Call(ctx, "extract", map[string]any{"url": targetURL})
	if maxLength > 0 && stringValue(result["content"]) != "" {
		result["content"] = truncate(stringValue(result["content"]), maxLength)
		result["raw_content"] = result["content"]
	}
	return result
}

func (p AnySearch) Batch(ctx context.Context, queries []string, maxResults int) map[string]any {
	if len(queries) == 0 || len(queries) > 5 {
		return map[string]any{"ok": false, "provider": "anysearch", "tool": "batch_search", "error_type": "parameter_error", "error": "AnySearch batch requires 1 to 5 queries", "elapsed_ms": 0}
	}
	maxResults = minAnySearchResults(maxResults)
	items := make([]map[string]any, 0, len(queries))
	for _, query := range queries {
		items = append(items, map[string]any{"query": query, "max_results": maxResults})
	}
	return p.Call(ctx, "batch_search", map[string]any{"queries": items})
}

func (p AnySearch) BatchObjects(ctx context.Context, queries []map[string]any, maxResults int) map[string]any {
	if len(queries) == 0 || len(queries) > 5 {
		return map[string]any{"ok": false, "provider": "anysearch", "tool": "batch_search", "error_type": "parameter_error", "error": "AnySearch batch requires 1 to 5 query objects", "elapsed_ms": 0}
	}
	items := make([]map[string]any, 0, len(queries))
	for _, item := range queries {
		copy := map[string]any{}
		for _, key := range []string{"query", "domain", "sub_domain", "sub_domain_params", "max_results"} {
			if value, ok := item[key]; ok {
				copy[key] = value
			}
		}
		if strings.TrimSpace(stringValue(copy["query"])) == "" {
			return map[string]any{"ok": false, "provider": "anysearch", "tool": "batch_search", "error_type": "parameter_error", "error": "each AnySearch batch item requires query", "elapsed_ms": 0}
		}
		if value, ok := copy["sub_domain_params"]; ok {
			if _, valid := value.(map[string]any); !valid {
				return map[string]any{"ok": false, "provider": "anysearch", "tool": "batch_search", "error_type": "parameter_error", "error": "sub_domain_params must be an object", "elapsed_ms": 0}
			}
		}
		if _, ok := copy["max_results"]; !ok && maxResults > 0 {
			copy["max_results"] = minAnySearchResults(maxResults)
		}
		items = append(items, copy)
	}
	return p.Call(ctx, "batch_search", map[string]any{"queries": items})
}

func minAnySearchResults(value int) int {
	if value <= 0 {
		return 5
	}
	if value > 10 {
		return 10
	}
	return value
}

func boundedMCPText(text string) (string, bool) {
	const limit = 64 * 1024
	if len(text) <= limit {
		return text, false
	}
	return truncate(text, limit), true
}

func filterMCPArguments(arguments map[string]any, schema map[string]any) (map[string]any, error) {
	if len(schema) == 0 {
		return arguments, nil
	}
	properties, _ := schema["properties"].(map[string]any)
	if len(properties) == 0 {
		return arguments, nil
	}
	filtered := map[string]any{}
	for key, value := range arguments {
		if _, ok := properties[key]; ok {
			filtered[key] = value
		}
	}
	if required, ok := schema["required"].([]any); ok {
		for _, item := range required {
			key := strings.TrimSpace(stringValue(item))
			if key != "" && isEmptyValue(filtered[key]) {
				return nil, fmt.Errorf("AnySearch MCP tool requires %s", key)
			}
		}
	}
	return filtered, nil
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

func mcpClientErrorPayload(err error) (string, string) {
	if mcpErr, ok := err.(*mcpclient.Error); ok {
		switch mcpErr.Type {
		case "config_error", "parameter_error", "auth_error", "rate_limited", "timeout", "protocol_error", "session_error", "capability_unavailable", "provider_error":
			return mcpErr.Type, mcpErr.Error()
		}
	}
	return ErrorPayload(err)
}
