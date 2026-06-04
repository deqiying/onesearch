package providers

import (
	"context"
	"strings"
	"time"
)

type DeepWiki struct {
	APIURL  string
	APIKey  string
	Timeout time.Duration
}

func (p DeepWiki) Ask(ctx context.Context, repoName, question string) map[string]any {
	return p.call(ctx, "ask_question", map[string]any{"repoName": repoName, "question": question})
}

func (p DeepWiki) Structure(ctx context.Context, repoName string) map[string]any {
	return p.call(ctx, "read_wiki_structure", map[string]any{"repoName": repoName})
}

func (p DeepWiki) Contents(ctx context.Context, repoName string) map[string]any {
	return p.call(ctx, "read_wiki_contents", map[string]any{"repoName": repoName})
}

func (p DeepWiki) call(ctx context.Context, name string, arguments map[string]any) map[string]any {
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
	err := PostJSON(ctx, Client(p.Timeout), strings.TrimRight(p.APIURL, "/"), headers, payload, &data)
	if err != nil {
		errorType, message := ErrorPayload(err)
		return map[string]any{"ok": false, "provider": "deepwiki", "tool": name, "error_type": errorType, "error": message, "elapsed_ms": Elapsed(start)}
	}
	if rawError, ok := data["error"]; ok {
		return map[string]any{"ok": false, "provider": "deepwiki", "tool": name, "error_type": "provider_error", "error": stringValue(rawError), "elapsed_ms": Elapsed(start)}
	}
	result, _ := data["result"].(map[string]any)
	text := extractMCPText(result)
	isError, _ := result["isError"].(bool)
	ok := !isError && strings.TrimSpace(text) != ""
	out := map[string]any{
		"ok":          ok,
		"provider":    "deepwiki",
		"tool":        name,
		"repo":        stringValue(arguments["repoName"]),
		"content":     text,
		"raw_content": text,
		"elapsed_ms":  Elapsed(start),
	}
	if question := stringValue(arguments["question"]); question != "" {
		out["query"] = question
	}
	if isError {
		out["error_type"] = "provider_error"
		out["error"] = firstNonEmpty(text, "DeepWiki tool returned isError=true")
	} else if strings.TrimSpace(text) == "" {
		out["error_type"] = "empty_result"
		out["error"] = "DeepWiki tool returned no text content"
	}
	return out
}

func extractMCPText(result map[string]any) string {
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
		return strings.TrimSpace(firstNonEmpty(stringValue(result["text"]), stringValue(result["result"])))
	}
}
