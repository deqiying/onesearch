// Package execution 将已校验且已应用默认值的命令参数映射到服务调用。
package execution

import (
	"context"
	"fmt"
	"time"

	"github.com/deqiying/onesearch/internal/providers"
	"github.com/deqiying/onesearch/internal/service"
)

// Backend 是共享执行层所需的服务接口。
type Backend interface {
	Search(context.Context, string, service.SearchOptions) map[string]any
	Fetch(context.Context, string, service.FetchOptions) map[string]any
	Map(context.Context, string, service.MapOptions) map[string]any
	Crawl(context.Context, string, service.CrawlOptions) map[string]any
	RepoWiki(context.Context, string, string, service.RepoWikiOptions) map[string]any
	ExaSearch(context.Context, string, providers.ExaOptions) map[string]any
	Context7Library(context.Context, string, string) map[string]any
	Context7Docs(context.Context, string, string) map[string]any
}

// Execute 接收规范化的参数，不解析命令行、不应用默认值，也不执行输出。
func Execute(ctx context.Context, svc Backend, id string, values map[string]any) map[string]any {
	v := arguments(values)
	switch id {
	case "search", "fetch", "map", "crawl", "repo-wiki", "exa.web-search", "context7.resolve-library-id", "context7.query-docs":
	default:
		return parameterError("unsupported command binding: " + id)
	}
	// 仅检查传入的上下文；保留未取消调用的原有 CLI 超时与服务调用顺序。
	if err := ctx.Err(); err != nil {
		if err == context.DeadlineExceeded {
			if data := deadlineResult(id, v); data != nil {
				return data
			}
		}
		return map[string]any{"ok": false, "error_type": "network_error", "error": err.Error()}
	}
	switch id {
	case "search":
		return executeSearch(ctx, svc, v)
	case "fetch":
		return svc.Fetch(ctx, v.text("url"), service.FetchOptions{Provider: v.text("provider")})
	case "map":
		return svc.Map(ctx, v.text("url"), service.MapOptions{Instructions: v.text("instructions"), MaxDepth: v.integer("max_depth"), MaxBreadth: v.integer("max_breadth"), Limit: v.integer("limit"), Timeout: v.integer("timeout"), Provider: v.text("provider")})
	case "crawl":
		timedCtx, cancel := context.WithTimeout(ctx, time.Duration(v.integer("timeout"))*time.Second)
		defer cancel()
		data := svc.Crawl(timedCtx, v.text("url"), service.CrawlOptions{MaxDepth: v.integer("max_depth"), Limit: v.integer("limit"), Timeout: v.integer("timeout"), Provider: v.text("provider")})
		if timedCtx.Err() == context.DeadlineExceeded {
			return deadlineResult(id, v)
		}
		return data
	case "repo-wiki":
		timedCtx, cancel := context.WithTimeout(ctx, time.Duration(v.number("timeout")*float64(time.Second)))
		defer cancel()
		data := svc.RepoWiki(timedCtx, v.text("repo"), v.text("question"), service.RepoWikiOptions{Mode: v.text("mode"), Provider: v.text("provider")})
		if timedCtx.Err() == context.DeadlineExceeded {
			return deadlineResult(id, v)
		}
		return data
	case "exa.web-search":
		numResults := v.integer("num_results")
		if maxResults := v.integer("max_results"); maxResults > 0 {
			numResults = maxResults
		}
		return svc.ExaSearch(ctx, v.text("query"), providers.ExaOptions{NumResults: numResults, SearchType: v.text("search_type"), IncludeText: v.boolean("include_text"), IncludeHighlights: v.boolean("include_highlights"), StartPublishedDate: v.text("start_published_date"), IncludeDomains: v.strings("include_domains"), ExcludeDomains: v.strings("exclude_domains"), Category: v.text("category")})
	case "context7.resolve-library-id":
		return svc.Context7Library(ctx, v.text("name"), v.text("query"))
	default: // context7.query-docs
		return svc.Context7Docs(ctx, v.text("library_id"), v.text("query"))
	}
}

func executeSearch(ctx context.Context, svc Backend, v arguments) map[string]any {
	providersFilter, filters, err := parseSearchProviderFilters(v.text("providers"))
	if err != nil {
		return parameterError(err.Error())
	}
	for _, override := range []struct{ capability, key string }{
		{"answer_search", "answer_providers"}, {"source_search", "source_providers"},
		{"docs_search", "docs_providers"}, {"page_fetch", "fetch_providers"}, {"repo_wiki", "repo_providers"},
	} {
		filters = overlayProviderFilter(filters, override.capability, v.text(override.key))
	}
	var stream *bool
	if v.boolean("stream") {
		value := true
		stream = &value
	}
	if v.boolean("no_stream") {
		value := false
		stream = &value
	}
	timedCtx, cancel := context.WithTimeout(ctx, time.Duration(v.number("timeout")*float64(time.Second)))
	defer cancel()
	data := svc.Search(timedCtx, v.text("query"), service.SearchOptions{
		Platform: v.text("platform"), Model: v.text("model"), ExtraSources: v.integer("extra_sources"), FetchSources: v.integer("fetch_sources"),
		Validation: v.text("validation"), Fallback: v.text("fallback"), Providers: providersFilter, ProviderFilters: filters, Stream: stream,
		RepoWiki: v.text("repo_wiki"), RepoWikiMode: v.text("repo_wiki_mode"), RepoWikiQuery: v.text("repo_wiki_query"),
	})
	if timedCtx.Err() == context.DeadlineExceeded {
		return deadlineResult("search", v)
	}
	return data
}

func deadlineResult(id string, v arguments) map[string]any {
	switch id {
	case "search":
		timeout := v.number("timeout")
		return map[string]any{"ok": false, "error_type": "network_error", "error": fmt.Sprintf("Search timed out after %g seconds", timeout), "query": v.text("query"), "content": "", "sources": []map[string]any{}, "sources_count": 0, "timeout_seconds": timeout}
	case "crawl":
		timeout := v.integer("timeout")
		return map[string]any{"ok": false, "error_type": "network_error", "error": fmt.Sprintf("Crawl timed out after %d seconds", timeout), "url": v.text("url"), "timeout_seconds": timeout}
	case "repo-wiki":
		timeout := v.number("timeout")
		return map[string]any{"ok": false, "error_type": "network_error", "error": fmt.Sprintf("Repo wiki timed out after %g seconds", timeout), "repo": v.text("repo"), "timeout_seconds": timeout}
	default:
		return nil
	}
}

func parameterError(message string) map[string]any {
	return map[string]any{"ok": false, "error_type": "parameter_error", "error": message}
}

type arguments map[string]any

func (v arguments) text(key string) string  { value, _ := v[key].(string); return value }
func (v arguments) boolean(key string) bool { value, _ := v[key].(bool); return value }
func (v arguments) integer(key string) int {
	switch value := v[key].(type) {
	case int:
		return value
	case float64:
		return int(value)
	default:
		return 0
	}
}
func (v arguments) number(key string) float64 {
	switch value := v[key].(type) {
	case float64:
		return value
	case int:
		return float64(value)
	default:
		return 0
	}
}
func (v arguments) strings(key string) []string {
	value, _ := v[key].([]string)
	return append([]string{}, value...)
}
