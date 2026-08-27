package providers

import (
	"context"
	"strings"
	"time"
)

type Freecrawl struct {
	MCP MCPStdio
}

type FreecrawlSearchOptions struct {
	NumResults    int
	ScrapeResults bool
	SearchEngine  string
}

type FreecrawlScrapeOptions struct {
	Formats    []string
	Javascript bool
	AntiBot    bool
	Cache      bool
	Timeout    int
	WaitFor    string
}

type FreecrawlCrawlOptions struct {
	MaxDepth        int
	MaxPages        int
	SameDomainOnly  bool
	IncludePatterns []string
	ExcludePatterns []string
}

type FreecrawlDeepResearchOptions struct {
	NumSources      int
	MaxDepth        int
	IncludeAcademic bool
	SearchQueries   []string
}

func (p Freecrawl) Search(ctx context.Context, query string, options FreecrawlSearchOptions) map[string]any {
	start := time.Now()
	args := map[string]any{"query": query}
	if options.NumResults > 0 {
		args["num_results"] = options.NumResults
	}
	if options.ScrapeResults {
		args["scrape_results"] = true
	}
	if strings.TrimSpace(options.SearchEngine) != "" {
		args["search_engine"] = strings.TrimSpace(options.SearchEngine)
	}
	result, err := p.MCP.CallTool(ctx, "search", args)
	if err != nil {
		return mcpProviderError("freecrawl", "search", start, map[string]any{"query": query}, err)
	}
	out := result.Envelope()
	out["ok"] = result.Content != "" || len(result.Results) > 0
	out["provider"] = "freecrawl"
	out["tool"] = "search"
	out["query"] = query
	out["elapsed_ms"] = Elapsed(start)
	if !truthy(out["ok"]) {
		out["error_type"] = "empty_result"
		out["error"] = "freecrawl search returned no content or results"
	}
	return out
}

func (p Freecrawl) Scrape(ctx context.Context, targetURL string, options FreecrawlScrapeOptions) map[string]any {
	start := time.Now()
	args := map[string]any{"url": targetURL}
	if len(options.Formats) > 0 {
		args["formats"] = options.Formats
	}
	if options.Javascript {
		args["javascript"] = true
	}
	if options.AntiBot {
		args["anti_bot"] = true
	}
	if options.Cache {
		args["cache"] = true
	}
	if options.Timeout > 0 {
		args["timeout"] = options.Timeout
	}
	if strings.TrimSpace(options.WaitFor) != "" {
		args["wait_for"] = strings.TrimSpace(options.WaitFor)
	}
	result, err := p.MCP.CallTool(ctx, "scrape", args)
	if err != nil {
		return mcpProviderError("freecrawl", "scrape", start, map[string]any{"url": targetURL}, err)
	}
	out := result.Envelope()
	out["ok"] = result.Content != ""
	out["provider"] = "freecrawl"
	out["tool"] = "scrape"
	out["url"] = targetURL
	out["elapsed_ms"] = Elapsed(start)
	if !truthy(out["ok"]) {
		out["error_type"] = "empty_result"
		out["error"] = "freecrawl scrape returned no content"
	}
	return out
}

func (p Freecrawl) Crawl(ctx context.Context, targetURL string, options FreecrawlCrawlOptions) map[string]any {
	start := time.Now()
	args := map[string]any{"start_url": targetURL}
	if options.MaxDepth > 0 {
		args["max_depth"] = options.MaxDepth
	}
	if options.MaxPages > 0 {
		args["max_pages"] = options.MaxPages
	}
	if options.SameDomainOnly {
		args["same_domain_only"] = true
	}
	if len(options.IncludePatterns) > 0 {
		args["include_patterns"] = options.IncludePatterns
	}
	if len(options.ExcludePatterns) > 0 {
		args["exclude_patterns"] = options.ExcludePatterns
	}
	result, err := p.MCP.CallTool(ctx, "crawl", args)
	if err != nil {
		return mcpProviderError("freecrawl", "crawl", start, map[string]any{"url": targetURL}, err)
	}
	out := result.Envelope()
	out["ok"] = result.Content != "" || len(result.Results) > 0 || len(result.Pages) > 0
	out["provider"] = "freecrawl"
	out["tool"] = "crawl"
	out["url"] = targetURL
	out["elapsed_ms"] = Elapsed(start)
	if !truthy(out["ok"]) {
		out["error_type"] = "empty_result"
		out["error"] = "freecrawl crawl returned no content or pages"
	}
	return out
}

func (p Freecrawl) DeepResearch(ctx context.Context, topic string, options FreecrawlDeepResearchOptions) map[string]any {
	start := time.Now()
	args := map[string]any{"topic": topic}
	if options.NumSources > 0 {
		args["num_sources"] = options.NumSources
	}
	if options.MaxDepth > 0 {
		args["max_depth"] = options.MaxDepth
	}
	if options.IncludeAcademic {
		args["include_academic"] = true
	}
	if len(options.SearchQueries) > 0 {
		args["search_queries"] = options.SearchQueries
	}
	result, err := p.MCP.CallTool(ctx, "deep_research", args)
	if err != nil {
		return mcpProviderError("freecrawl", "deep_research", start, map[string]any{"query": topic}, err)
	}
	out := result.Envelope()
	out["ok"] = result.Content != "" || len(result.Results) > 0
	out["provider"] = "freecrawl"
	out["tool"] = "deep_research"
	out["query"] = topic
	out["elapsed_ms"] = Elapsed(start)
	if !truthy(out["ok"]) {
		out["error_type"] = "empty_result"
		out["error"] = "freecrawl deep_research returned no content or results"
	}
	return out
}
