package providers

import (
	"context"
	"strings"
	"time"

	"github.com/deqiying/onesearch/internal/mcpclient"
)

type DeepWiki struct {
	APIURL      string
	APIKey      string
	Timeout     time.Duration
	SessionMode string
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
	client := mcpclient.NewHTTP(mcpclient.Config{Endpoint: strings.TrimRight(p.APIURL, "/"), APIKey: p.APIKey, Timeout: p.Timeout, SessionMode: p.SessionMode})
	defer client.Close(context.Background())
	resolvedName := name
	var snapshot mcpclient.ToolSnapshot
	if strings.EqualFold(strings.TrimSpace(p.SessionMode), "auto") {
		var discoverErr error
		snapshot, discoverErr = client.ListTools(ctx)
		if discoverErr != nil {
			errorType, message := mcpClientErrorPayload(discoverErr)
			return map[string]any{"ok": false, "provider": "deepwiki", "tool": name, "error_type": errorType, "error": message, "elapsed_ms": Elapsed(start)}
		}
		var found bool
		resolvedName, found = mcpclient.ResolveTool(snapshot, name)
		if !found {
			return map[string]any{"ok": false, "provider": "deepwiki", "tool": name, "error_type": "capability_unavailable", "error": "DeepWiki MCP tool not found: " + name, "elapsed_ms": Elapsed(start)}
		}
	}
	data, err := client.CallTool(ctx, resolvedName, arguments)
	if err != nil {
		errorType, message := mcpClientErrorPayload(err)
		return map[string]any{"ok": false, "provider": "deepwiki", "tool": name, "error_type": errorType, "error": message, "elapsed_ms": Elapsed(start)}
	}
	result := data
	text := extractMCPText(result)
	boundedText, textTruncated := boundedMCPText(text)
	isError, _ := result["isError"].(bool)
	ok := !isError && strings.TrimSpace(boundedText) != ""
	out := map[string]any{
		"ok":            ok,
		"provider":      "deepwiki",
		"tool":          name,
		"resolved_tool": resolvedName,
		"repo":          stringValue(arguments["repoName"]),
		"content":       boundedText,
		"raw_content":   boundedText,
		"elapsed_ms":    Elapsed(start),
		"mcp":           map[string]any{"protocol_version": client.ProtocolVersion(), "session_mode": client.SessionMode(), "tool_name": resolvedName},
	}
	if textTruncated {
		out["content_truncated"] = true
		out["raw_content_truncated"] = true
	}
	if question := stringValue(arguments["question"]); question != "" {
		out["query"] = question
	}
	if isError {
		out["error_type"] = "provider_error"
		out["error"] = firstNonEmpty(boundedText, "DeepWiki tool returned isError=true")
	} else if strings.TrimSpace(boundedText) == "" {
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
