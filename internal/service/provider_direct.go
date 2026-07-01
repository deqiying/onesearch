package service

import (
	"context"
	"strings"
	"time"

	"github.com/deqiying/onesearch/internal/config"
	"github.com/deqiying/onesearch/internal/providers"
)

func (s *Service) ExaFetch(ctx context.Context, urls []string, options providers.ExaFetchOptions) map[string]any {
	provider, ok := s.providerByID("exa")
	if !ok || provider.APIKey == "" {
		return s.providerToolConfigError("exa", "web_fetch_exa")
	}
	return providers.Exa{APIURL: provider.BaseURL, APIKey: provider.APIKey, Timeout: durationSeconds(provider.SettingFloat("timeout_seconds", 30))}.Fetch(ctx, urls, options)
}

func (s *Service) TavilySearch(ctx context.Context, query string, options providers.TavilySearchOptions) map[string]any {
	provider, ok := s.providerByID("tavily")
	if !ok || provider.APIKey == "" {
		return s.providerToolConfigError("tavily", "tavily_search")
	}
	return providers.Tavily{APIURL: provider.BaseURL, APIKey: provider.APIKey, Timeout: durationSeconds(provider.SettingFloat("timeout_seconds", 90))}.SearchResult(ctx, query, options)
}

func (s *Service) TavilyExtract(ctx context.Context, urls []string, options providers.TavilyExtractOptions) map[string]any {
	provider, ok := s.providerByID("tavily")
	if !ok || provider.APIKey == "" {
		return s.providerToolConfigError("tavily", "tavily_extract")
	}
	timeout := durationSeconds(provider.SettingFloat("timeout_seconds", 60))
	if options.TimeoutSeconds > 0 {
		timeout = time.Duration(options.TimeoutSeconds * float64(time.Second))
	}
	return providers.Tavily{APIURL: provider.BaseURL, APIKey: provider.APIKey, Timeout: timeout}.ExtractResult(ctx, urls, options)
}

func (s *Service) TavilyMap(ctx context.Context, targetURL string, options providers.TavilyMapOptions) map[string]any {
	provider, ok := s.providerByID("tavily")
	if !ok || provider.APIKey == "" {
		return s.providerToolConfigError("tavily", "tavily_map")
	}
	data := providers.Tavily{
		APIURL:  provider.BaseURL,
		APIKey:  provider.APIKey,
		Timeout: durationSeconds(provider.SettingFloat("timeout_seconds", 150)),
	}.MapWithOptions(ctx, targetURL, options)
	if truthy(data["ok"]) && !options.AllowExternal {
		data["results"] = sameHostMapResults(targetURL, data["results"])
	}
	if truthy(data["ok"]) && mapResultCount(data["results"]) == 0 {
		data["ok"] = false
		data["error_type"] = "empty_result"
		data["error"] = "Tavily map returned no same-domain results"
	}
	return data
}

func (s *Service) TavilyCrawl(ctx context.Context, targetURL string, options providers.TavilyCrawlOptions) map[string]any {
	provider, ok := s.providerByID("tavily")
	if !ok || provider.APIKey == "" {
		return s.providerToolConfigError("tavily", "tavily_crawl")
	}
	data := providers.Tavily{
		APIURL:  provider.BaseURL,
		APIKey:  provider.APIKey,
		Timeout: durationSeconds(provider.SettingFloat("timeout_seconds", 150)),
	}.Crawl(ctx, targetURL, options)
	if truthy(data["ok"]) && !options.AllowExternal {
		data["results"] = sameHostResultMaps(targetURL, asMapSlice(data["results"]))
		data["total"] = len(asMapSlice(data["results"]))
		data["content"] = joinedContent(asMapSlice(data["results"]))
	}
	return data
}

func (s *Service) FirecrawlSearch(ctx context.Context, query string, limit int) map[string]any {
	provider, ok := s.providerByID("firecrawl")
	if !ok || provider.APIKey == "" {
		return s.providerToolConfigError("firecrawl", "firecrawl_search")
	}
	return providers.Firecrawl{APIURL: provider.BaseURL, APIKey: provider.APIKey}.SearchResult(ctx, query, limit)
}

func (s *Service) FirecrawlScrape(ctx context.Context, targetURL string, attempts int) map[string]any {
	provider, ok := s.providerByID("firecrawl")
	if !ok || provider.APIKey == "" {
		return s.providerToolConfigError("firecrawl", "firecrawl_scrape")
	}
	return providers.Firecrawl{APIURL: provider.BaseURL, APIKey: provider.APIKey}.ScrapeResult(ctx, targetURL, attempts)
}

func (s *Service) FirecrawlMap(ctx context.Context, targetURL string, limit int) map[string]any {
	provider, ok := s.providerByID("firecrawl")
	if !ok || provider.APIKey == "" {
		return s.providerToolConfigError("firecrawl", "firecrawl_map")
	}
	data := providers.Firecrawl{APIURL: provider.BaseURL, APIKey: provider.APIKey}.Map(ctx, targetURL, limit)
	data["tool"] = "firecrawl_map"
	return data
}

func (s *Service) FirecrawlCrawl(ctx context.Context, targetURL string, options CrawlOptions) map[string]any {
	provider, ok := s.providerByID("firecrawl")
	if !ok || provider.APIKey == "" {
		return s.providerToolConfigError("firecrawl", "firecrawl_crawl")
	}
	data := providers.Firecrawl{APIURL: provider.BaseURL, APIKey: provider.APIKey}.Crawl(ctx, targetURL, options.MaxDepth, options.Limit)
	data["tool"] = "firecrawl_crawl"
	return data
}

func (s *Service) DDGSearch(ctx context.Context, query string, options providers.DDGSearchOptions) map[string]any {
	mcp, errData, ok := s.mcpStdioProvider("ddg", "search")
	if !ok {
		return errData
	}
	return providers.DDG{MCP: mcp}.Search(ctx, query, options)
}

func (s *Service) DDGFetchContent(ctx context.Context, targetURL string, options providers.DDGFetchOptions) map[string]any {
	mcp, errData, ok := s.mcpStdioProvider("ddg", "fetch_content")
	if !ok {
		return errData
	}
	return providers.DDG{MCP: mcp}.FetchContent(ctx, targetURL, options)
}

func (s *Service) FreecrawlSearch(ctx context.Context, query string, options providers.FreecrawlSearchOptions) map[string]any {
	mcp, errData, ok := s.mcpStdioProvider("freecrawl", "search")
	if !ok {
		return errData
	}
	return providers.Freecrawl{MCP: mcp}.Search(ctx, query, options)
}

func (s *Service) FreecrawlScrape(ctx context.Context, targetURL string, options providers.FreecrawlScrapeOptions) map[string]any {
	mcp, errData, ok := s.mcpStdioProvider("freecrawl", "scrape")
	if !ok {
		return errData
	}
	return providers.Freecrawl{MCP: mcp}.Scrape(ctx, targetURL, options)
}

func (s *Service) FreecrawlCrawl(ctx context.Context, targetURL string, options providers.FreecrawlCrawlOptions) map[string]any {
	mcp, errData, ok := s.mcpStdioProvider("freecrawl", "crawl")
	if !ok {
		return errData
	}
	return providers.Freecrawl{MCP: mcp}.Crawl(ctx, targetURL, options)
}

func (s *Service) FreecrawlDeepResearch(ctx context.Context, topic string, options providers.FreecrawlDeepResearchOptions) map[string]any {
	mcp, errData, ok := s.mcpStdioProvider("freecrawl", "deep_research")
	if !ok {
		return errData
	}
	return providers.Freecrawl{MCP: mcp}.DeepResearch(ctx, topic, options)
}

func (s *Service) mcpStdioProvider(providerID, publicTool string) (providers.MCPStdio, map[string]any, bool) {
	provider, ok := s.providerByID(providerID)
	if !ok {
		return providers.MCPStdio{}, mcpStdioConfigError(providerID, publicTool, "provider "+providerID+" 不存在；请检查 config.json 的 providers。"), false
	}
	if enabledIsFalse(provider.Enabled) {
		return providers.MCPStdio{}, mcpStdioConfigError(providerID, publicTool, "provider "+providerID+" 已禁用；请在 config.json 中设置 providers."+providerID+".enabled=true。"), false
	}
	if provider.Adapter != config.AdapterMCPStdio {
		return providers.MCPStdio{}, mcpStdioConfigError(providerID, publicTool, "provider "+providerID+" adapter 不是 mcp_stdio。"), false
	}
	mcp := providers.NewMCPStdio(providerID, provider.Settings)
	if strings.TrimSpace(mcp.Command) == "" {
		return providers.MCPStdio{}, mcpStdioConfigError(providerID, publicTool, "provider "+providerID+" 缺少 settings.command。"), false
	}
	if strings.TrimSpace(mcp.ToolName(publicTool)) == "" {
		return providers.MCPStdio{}, mcpStdioConfigError(providerID, publicTool, "provider "+providerID+" 缺少 settings.tools."+publicTool+"。"), false
	}
	return mcp, nil, true
}

func enabledIsFalse(value any) bool {
	if b, ok := value.(bool); ok {
		return !b
	}
	switch strings.ToLower(strings.TrimSpace(stringValue(value))) {
	case "false", "0", "off", "no":
		return true
	default:
		return false
	}
}

func mcpStdioConfigError(providerID, tool, message string) map[string]any {
	data := configError(message)
	data["provider"] = providerID
	data["tool"] = tool
	return data
}

func sameHostResultMaps(targetURL string, items []map[string]any) []map[string]any {
	targetHost := normalizedURLHost(targetURL)
	if targetHost == "" {
		return items
	}
	var out []map[string]any
	for _, item := range items {
		if sameOrSubHost(normalizedURLHost(stringValue(item["url"])), targetHost) {
			out = append(out, item)
		}
	}
	return out
}

func joinedContent(items []map[string]any) string {
	var parts []string
	for _, item := range items {
		if text := strings.TrimSpace(stringValue(item["content"])); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func (s *Service) providerToolConfigError(providerID, tool string) map[string]any {
	data := s.providerConfigError(providerID)
	data["provider"] = providerID
	data["tool"] = tool
	return data
}
