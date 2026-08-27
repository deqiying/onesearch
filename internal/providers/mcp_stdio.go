package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/deqiying/onesearch/internal/mcpstdio"
)

type MCPStdio struct {
	Provider        string
	Command         string
	Args            []string
	Env             map[string]string
	Tools           map[string]string
	Timeout         time.Duration
	ProtocolVersion string
	SessionMode     string
}

type MCPNormalizedResult struct {
	Content    string
	RawContent string
	Results    []map[string]any
	Pages      []map[string]any
	RawResult  map[string]any
	MCP        map[string]any
}

func NewMCPStdio(provider string, settings map[string]any) MCPStdio {
	timeoutSeconds := floatSetting(settings, "timeout_seconds", 60)
	protocolVersion := stringSetting(settings, "protocol_version", "")
	sessionMode := stringSetting(settings, "session_mode", "")
	if nested := mapSetting(settings, "mcp"); nested != nil {
		protocolVersion = stringSetting(nested, "protocol_version", protocolVersion)
		sessionMode = stringSetting(nested, "session_mode", sessionMode)
	}
	return MCPStdio{
		Provider:        provider,
		Command:         stringSetting(settings, "command", ""),
		Args:            stringListSetting(settings, "args"),
		Env:             stringMapSetting(settings, "env"),
		Tools:           stringMapSetting(settings, "tools"),
		Timeout:         time.Duration(timeoutSeconds * float64(time.Second)),
		ProtocolVersion: protocolVersion,
		SessionMode:     sessionMode,
	}
}

func (p MCPStdio) ToolName(publicTool string) string {
	if p.Tools != nil {
		if value := strings.TrimSpace(p.Tools[publicTool]); value != "" {
			return value
		}
	}
	return strings.TrimSpace(publicTool)
}

func (p MCPStdio) CallTool(ctx context.Context, publicTool string, arguments map[string]any) (MCPNormalizedResult, error) {
	toolName := p.ToolName(publicTool)
	if toolName == "" {
		return MCPNormalizedResult{}, &ProviderError{Type: "config_error", Message: "missing mcp_stdio tool mapping for " + publicTool}
	}
	call, err := mcpstdio.CallTool(ctx, mcpstdio.Config{Command: p.Command, Args: p.Args, Env: p.Env, Timeout: p.Timeout, ProtocolVersion: p.ProtocolVersion, SessionMode: p.SessionMode}, toolName, compactArgs(arguments))
	if err != nil {
		return MCPNormalizedResult{}, normalizeMCPStdioError(err)
	}
	if truthy(call.Result["isError"]) {
		normalized := normalizeMCPToolResult(call.Result, toolName, call.Tools, call.Stderr)
		if normalized.MCP == nil {
			normalized.MCP = map[string]any{}
		}
		if call.ProtocolVersion != "" {
			normalized.MCP["protocol_version"] = call.ProtocolVersion
		}
		if call.SessionMode != "" {
			normalized.MCP["session_mode"] = call.SessionMode
		}
		message := firstNonEmpty(normalized.Content, normalized.RawContent, "mcp tool returned isError=true")
		return normalized, &ProviderError{Type: "provider_error", Message: message}
	}
	normalized := normalizeMCPToolResult(call.Result, toolName, call.Tools, call.Stderr)
	if normalized.MCP == nil {
		normalized.MCP = map[string]any{}
	}
	if call.ProtocolVersion != "" {
		normalized.MCP["protocol_version"] = call.ProtocolVersion
	}
	if call.SessionMode != "" {
		normalized.MCP["session_mode"] = call.SessionMode
	}
	return normalized, nil
}

func (r MCPNormalizedResult) Envelope() map[string]any {
	out := map[string]any{
		"content":     r.Content,
		"raw_content": r.RawContent,
		"raw_result":  r.RawResult,
	}
	if len(r.Results) > 0 {
		out["results"] = r.Results
		out["total"] = len(r.Results)
	}
	if len(r.Pages) > 0 {
		out["pages"] = r.Pages
		out["pages_count"] = len(r.Pages)
	}
	if len(r.MCP) > 0 {
		out["mcp"] = r.MCP
	}
	return out
}

func normalizeMCPStdioError(err error) error {
	var mcpErr *mcpstdio.Error
	if errors.As(err, &mcpErr) {
		return &ProviderError{Type: normalizeMCPErrorType(mcpErr.Type), Message: mcpErr.Error()}
	}
	return err
}

func normalizeMCPErrorType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "missing_command":
		return "config_error"
	case "config_error", "protocol_error", "provider_error", "timeout":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		if strings.TrimSpace(value) != "" {
			return strings.ToLower(strings.TrimSpace(value))
		}
		return "network_error"
	}
}

func normalizeMCPToolResult(raw map[string]any, toolName string, tools []mcpstdio.Tool, stderr string) MCPNormalizedResult {
	rawContent := extractMCPContentText(raw["content"])
	parsed := raw["structuredContent"]
	if isEmptyValue(parsed) && looksJSON(rawContent) {
		var decoded any
		if err := json.Unmarshal([]byte(rawContent), &decoded); err == nil {
			parsed = decoded
		}
	}
	content := firstNonEmpty(contentFromAny(parsed), rawContent)
	results := resultsFromAny(parsed)
	if len(results) == 0 {
		results = urlResults(rawContent)
	}
	pages := pagesFromAny(parsed)
	if len(pages) == 0 && len(results) > 0 && strings.Contains(strings.ToLower(toolName), "crawl") {
		pages = results
	}
	mcp := map[string]any{"tool_name": toolName, "tools_count": len(tools)}
	if len(tools) > 0 {
		names := make([]string, 0, len(tools))
		for _, tool := range tools {
			names = append(names, tool.Name)
		}
		mcp["tools"] = names
	}
	if strings.TrimSpace(stderr) != "" {
		mcp["stderr_present"] = true
		mcp["stderr_summary"] = truncate(redactDiagnostic(strings.Map(func(r rune) rune {
			if r < 0x20 && r != '\t' {
				return ' '
			}
			return r
		}, stderr)), 500)
	}
	return MCPNormalizedResult{
		Content:    strings.TrimSpace(content),
		RawContent: strings.TrimSpace(rawContent),
		Results:    results,
		Pages:      pages,
		RawResult:  boundedRawResult(raw),
		MCP:        mcp,
	}
}

func redactDiagnostic(text string) string {
	pattern := regexp.MustCompile(`(?i)(authorization|cookie|token|api[_-]?key|secret)=?[^\s;,]+`)
	return pattern.ReplaceAllString(text, "$1=[REDACTED]")
}

func mapSetting(settings map[string]any, key string) map[string]any {
	if settings == nil {
		return nil
	}
	if value, ok := settings[key].(map[string]any); ok {
		return value
	}
	return nil
}

func boundedRawResult(raw map[string]any) map[string]any {
	if raw == nil {
		return map[string]any{}
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return map[string]any{"redacted": true}
	}
	if len(data) <= 64*1024 {
		return redactMap(raw)
	}
	return map[string]any{"truncated": true, "bytes": len(data)}
}

func redactMap(raw map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range raw {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "authorization") || strings.Contains(lower, "cookie") || strings.Contains(lower, "token") || strings.Contains(lower, "api_key") || strings.Contains(lower, "secret") {
			out[key] = "[REDACTED]"
			continue
		}
		switch typed := value.(type) {
		case map[string]any:
			out[key] = redactMap(typed)
		default:
			out[key] = value
		}
	}
	return out
}

func extractMCPContentText(value any) string {
	items, ok := value.([]any)
	if !ok {
		return ""
	}
	parts := make([]string, 0, len(items))
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if strings.TrimSpace(stringValue(item["type"])) != "" && stringValue(item["type"]) != "text" {
			continue
		}
		if text := strings.TrimSpace(stringValue(item["text"])); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func contentFromAny(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range []string{"markdown", "text", "content", "raw_content", "rawContent", "summary"} {
			if text := strings.TrimSpace(stringValue(typed[key])); text != "" {
				return text
			}
		}
		for _, key := range []string{"data", "result", "page"} {
			if text := contentFromAny(typed[key]); text != "" {
				return text
			}
		}
	case []any:
		var parts []string
		for _, item := range typed {
			if text := contentFromAny(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n\n"))
	}
	return ""
}

func resultsFromAny(value any) []map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range []string{"results", "items", "web", "links"} {
			if results := resultListFromAny(typed[key]); len(results) > 0 {
				return results
			}
		}
		if nested, ok := typed["data"].(map[string]any); ok {
			if results := resultsFromAny(nested); len(results) > 0 {
				return results
			}
		}
		if results := resultListFromAny(typed["data"]); len(results) > 0 {
			return results
		}
		if hasResultTarget(typed) {
			return []map[string]any{normalizeResultMap(typed)}
		}
	case []any:
		return resultListFromAny(typed)
	}
	return nil
}

func pagesFromAny(value any) []map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range []string{"pages", "crawl", "documents"} {
			if pages := resultListFromAny(typed[key]); len(pages) > 0 {
				return pages
			}
		}
		if nested, ok := typed["data"].(map[string]any); ok {
			if pages := pagesFromAny(nested); len(pages) > 0 {
				return pages
			}
		}
	}
	return nil
}

func resultListFromAny(value any) []map[string]any {
	switch items := value.(type) {
	case []any:
		out := make([]map[string]any, 0, len(items))
		for _, raw := range items {
			switch item := raw.(type) {
			case map[string]any:
				if normalized := normalizeResultMap(item); len(normalized) > 0 {
					out = append(out, normalized)
				}
			case string:
				if strings.HasPrefix(strings.ToLower(strings.TrimSpace(item)), "http://") || strings.HasPrefix(strings.ToLower(strings.TrimSpace(item)), "https://") {
					out = append(out, map[string]any{"url": strings.TrimSpace(item)})
				}
			}
		}
		return out
	case []map[string]any:
		out := make([]map[string]any, 0, len(items))
		for _, item := range items {
			if normalized := normalizeResultMap(item); len(normalized) > 0 {
				out = append(out, normalized)
			}
		}
		return out
	case []string:
		out := make([]map[string]any, 0, len(items))
		for _, item := range items {
			if strings.HasPrefix(strings.ToLower(strings.TrimSpace(item)), "http://") || strings.HasPrefix(strings.ToLower(strings.TrimSpace(item)), "https://") {
				out = append(out, map[string]any{"url": strings.TrimSpace(item)})
			}
		}
		return out
	}
	return nil
}

func normalizeResultMap(input map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range input {
		out[key] = value
	}
	if url := firstNonEmpty(stringValue(input["url"]), stringValue(input["link"]), stringValue(input["href"])); url != "" {
		out["url"] = url
	}
	if title := firstNonEmpty(stringValue(input["title"]), stringValue(input["name"])); title != "" {
		out["title"] = title
	}
	if desc := firstNonEmpty(stringValue(input["description"]), stringValue(input["snippet"]), stringValue(input["body"])); desc != "" {
		out["description"] = desc
	}
	if content := firstNonEmpty(stringValue(input["content"]), stringValue(input["text"]), stringValue(input["markdown"]), stringValue(input["raw_content"]), stringValue(input["rawContent"])); content != "" {
		out["content"] = content
	}
	for key, value := range out {
		if strings.TrimSpace(key) != "" && !isEmptyValue(value) {
			return out
		}
	}
	return nil
}

func hasResultTarget(input map[string]any) bool {
	for _, key := range []string{"url", "link", "href", "title", "content", "markdown", "text"} {
		if strings.TrimSpace(stringValue(input[key])) != "" {
			return true
		}
	}
	return false
}

var urlPattern = regexp.MustCompile(`https?://[^\s<>"')\]}]+`)

func urlResults(text string) []map[string]any {
	matches := urlPattern.FindAllString(text, -1)
	seen := map[string]struct{}{}
	out := make([]map[string]any, 0, len(matches))
	for _, match := range matches {
		match = strings.TrimRight(match, ".,;:!?")
		if _, ok := seen[match]; ok {
			continue
		}
		seen[match] = struct{}{}
		out = append(out, map[string]any{"url": match})
	}
	return out
}

func compactArgs(input map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range input {
		if strings.TrimSpace(key) == "" || isEmptyValue(value) {
			continue
		}
		out[key] = value
	}
	return out
}

func stringSetting(settings map[string]any, key, fallback string) string {
	if settings == nil {
		return fallback
	}
	if value := strings.TrimSpace(stringValue(settings[key])); value != "" {
		return value
	}
	return fallback
}

func floatSetting(settings map[string]any, key string, fallback float64) float64 {
	if settings == nil {
		return fallback
	}
	switch value := settings[key].(type) {
	case float64:
		return value
	case float32:
		return float64(value)
	case int:
		return float64(value)
	case int64:
		return float64(value)
	case string:
		var parsed float64
		if _, err := fmt.Sscanf(strings.TrimSpace(value), "%f", &parsed); err == nil {
			return parsed
		}
	}
	return fallback
}

func stringListSetting(settings map[string]any, key string) []string {
	if settings == nil {
		return nil
	}
	switch items := settings[key].(type) {
	case []string:
		return append([]string{}, items...)
	case []any:
		out := make([]string, 0, len(items))
		for _, item := range items {
			if text := strings.TrimSpace(stringValue(item)); text != "" {
				out = append(out, text)
			}
		}
		return out
	case string:
		var out []string
		for _, item := range strings.Split(items, ",") {
			if text := strings.TrimSpace(item); text != "" {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}

func stringMapSetting(settings map[string]any, key string) map[string]string {
	if settings == nil {
		return nil
	}
	switch items := settings[key].(type) {
	case map[string]string:
		out := map[string]string{}
		for key, value := range items {
			if strings.TrimSpace(key) != "" && strings.TrimSpace(value) != "" {
				out[key] = value
			}
		}
		return out
	case map[string]any:
		out := map[string]string{}
		for key, value := range items {
			if strings.TrimSpace(key) != "" && strings.TrimSpace(stringValue(value)) != "" {
				out[key] = stringValue(value)
			}
		}
		return out
	default:
		return nil
	}
}

func looksJSON(text string) bool {
	text = strings.TrimSpace(text)
	return strings.HasPrefix(text, "{") || strings.HasPrefix(text, "[")
}

func isEmptyValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(typed) == ""
	case []string:
		return len(typed) == 0
	case []any:
		return len(typed) == 0
	case map[string]any:
		return len(typed) == 0
	default:
		return false
	}
}
