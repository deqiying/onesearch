package providers

import (
	"context"
	"strings"
	"time"
)

type DDG struct {
	MCP MCPStdio
}

type DDGSearchOptions struct {
	MaxResults int
	Region     string
}

type DDGFetchOptions struct {
	StartIndex int
	MaxLength  int
	Backend    string
}

func (p DDG) Search(ctx context.Context, query string, options DDGSearchOptions) map[string]any {
	start := time.Now()
	args := map[string]any{"query": query}
	if options.MaxResults > 0 {
		args["max_results"] = options.MaxResults
	}
	if strings.TrimSpace(options.Region) != "" {
		args["region"] = strings.TrimSpace(options.Region)
	}
	result, err := p.MCP.CallTool(ctx, "search", args)
	if err != nil {
		return mcpProviderError("ddg", "search", start, map[string]any{"query": query}, err)
	}
	out := result.Envelope()
	out["ok"] = result.Content != "" || len(result.Results) > 0
	out["provider"] = "ddg"
	out["tool"] = "search"
	out["query"] = query
	out["elapsed_ms"] = Elapsed(start)
	if !truthy(out["ok"]) {
		out["error_type"] = "empty_result"
		out["error"] = "ddg search returned no content or results"
	}
	return out
}

func (p DDG) FetchContent(ctx context.Context, targetURL string, options DDGFetchOptions) map[string]any {
	start := time.Now()
	args := map[string]any{"url": targetURL}
	if options.StartIndex > 0 {
		args["start_index"] = options.StartIndex
	}
	if options.MaxLength > 0 {
		args["max_length"] = options.MaxLength
	}
	if strings.TrimSpace(options.Backend) != "" {
		args["backend"] = strings.TrimSpace(options.Backend)
	}
	result, err := p.MCP.CallTool(ctx, "fetch_content", args)
	if err != nil {
		return mcpProviderError("ddg", "fetch_content", start, map[string]any{"url": targetURL}, err)
	}
	out := result.Envelope()
	out["ok"] = result.Content != ""
	out["provider"] = "ddg"
	out["tool"] = "fetch_content"
	out["url"] = targetURL
	out["elapsed_ms"] = Elapsed(start)
	if !truthy(out["ok"]) {
		out["error_type"] = "empty_result"
		out["error"] = "ddg fetch_content returned no content"
	}
	return out
}

func mcpProviderError(provider, tool string, start time.Time, fields map[string]any, err error) map[string]any {
	errorType, message := ErrorPayload(err)
	out := map[string]any{
		"ok":         false,
		"provider":   provider,
		"tool":       tool,
		"error_type": errorType,
		"error":      message,
		"elapsed_ms": Elapsed(start),
	}
	for key, value := range fields {
		out[key] = value
	}
	return out
}
