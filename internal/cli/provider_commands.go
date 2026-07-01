package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/deqiying/onesearch/internal/providers"
	"github.com/deqiying/onesearch/internal/service"
)

var providerToolAliases = map[string]map[string]string{
	"exa": {
		"web-search": "web_search_exa",
		"web-fetch":  "web_fetch_exa",
		"similar":    "find_similar",
	},
	"tavily": {
		"search":  "tavily_search",
		"extract": "tavily_extract",
		"map":     "tavily_map",
		"crawl":   "tavily_crawl",
	},
	"firecrawl": {
		"search": "firecrawl_search",
		"scrape": "firecrawl_scrape",
		"map":    "firecrawl_map",
		"crawl":  "firecrawl_crawl",
	},
	"context7": {
		"resolve-library-id": "resolve_library_id",
		"query-docs":         "query_docs",
	},
	"deepwiki": {
		"ask-question":        "ask_question",
		"read-wiki-structure": "read_wiki_structure",
		"read-wiki-contents":  "read_wiki_contents",
	},
	"anysearch": {
		"domains": "domains",
		"search":  "search",
		"extract": "extract",
		"batch":   "batch",
	},
	"zhipu": {
		"search": "zhipu_search",
	},
}

func shouldDispatchProviderCommand(command string, args []string) bool {
	_, ok := providerToolAliases[command]
	return ok
}

func runProviderCommand(ctx context.Context, svc *service.Service, provider string, args []string) int {
	if len(args) == 0 {
		return printProviderError(provider, "provider command requires subcommand", args, svc)
	}
	if isHelpToken(args[0]) {
		printProviderHelp(provider)
		return 0
	}
	tool, ok := canonicalProviderTool(provider, args[0])
	if !ok {
		return printProviderError(provider, fmt.Sprintf("unknown %s subcommand: %s", provider, args[0]), args[1:], svc)
	}
	switch provider {
	case "exa":
		return runExaGroup(ctx, svc, tool, args[1:])
	case "tavily":
		return runTavilyGroup(ctx, svc, tool, args[1:])
	case "firecrawl":
		return runFirecrawlGroup(ctx, svc, tool, args[1:])
	case "context7":
		return runContext7Group(ctx, svc, tool, args[1:])
	case "deepwiki":
		return runDeepWikiGroup(ctx, svc, tool, args[1:])
	case "anysearch":
		return runAnySearchGroup(ctx, svc, tool, args[1:])
	case "zhipu":
		return runZhipuGroup(ctx, svc, tool, args[1:])
	default:
		return printProviderError(provider, "unsupported provider command: "+provider, args[1:], svc)
	}
}

func runExaGroup(ctx context.Context, svc *service.Service, tool string, args []string) int {
	switch tool {
	case "web_search_exa":
		fs := flagSet("exa web-search")
		numResults := fs.Int("num-results", 5, "")
		maxResults := fs.Int("max-results", 0, "")
		searchType := fs.String("search-type", "neural", "")
		includeText := fs.Bool("include-text", false, "")
		includeHighlights := fs.Bool("include-highlights", false, "")
		startDate := fs.String("start-published-date", "", "")
		category := fs.String("category", "", "")
		includeDomains := stringListFlag{}
		excludeDomains := stringListFlag{}
		fs.Var(&includeDomains, "include-domains", "")
		fs.Var(&excludeDomains, "exclude-domains", "")
		outputFlags := addOutputFlags(fs)
		if err := parse(fs, args); err != nil {
			return printProviderToolParameterError("exa", tool, err.Error(), outputFlags, svc)
		}
		if fs.NArg() < 1 {
			return printProviderToolParameterError("exa", tool, "web_search_exa requires query", outputFlags, svc)
		}
		if *maxResults > 0 {
			*numResults = *maxResults
		}
		data := annotateProviderTool(svc.ExaSearch(ctx, fs.Arg(0), providers.ExaOptions{NumResults: *numResults, SearchType: *searchType, IncludeText: *includeText, IncludeHighlights: *includeHighlights, StartPublishedDate: *startDate, IncludeDomains: includeDomains.values, ExcludeDomains: excludeDomains.values, Category: *category}), "exa", tool)
		return printCommand("exa", data, makeFormatOutput(outputFlags, svc))
	case "web_fetch_exa":
		fs := flagSet("exa web-fetch")
		maxCharacters := fs.Int("max-characters", 20000, "")
		urlsFlag := stringListFlag{}
		fs.Var(&urlsFlag, "urls", "")
		outputFlags := addOutputFlags(fs)
		if err := parse(fs, args); err != nil {
			return printProviderToolParameterError("exa", tool, err.Error(), outputFlags, svc)
		}
		urls := append([]string{}, urlsFlag.values...)
		urls = append(urls, fs.Args()...)
		if len(nonEmptyArgs(urls)) == 0 {
			return printProviderToolParameterError("exa", tool, "web_fetch_exa requires at least one url", outputFlags, svc)
		}
		data := annotateProviderTool(svc.ExaFetch(ctx, urls, providers.ExaFetchOptions{MaxCharacters: *maxCharacters}), "exa", tool)
		return printCommand("exa", data, makeFormatOutput(outputFlags, svc))
	case "find_similar":
		fs := flagSet("exa similar")
		numResults := fs.Int("num-results", 5, "")
		outputFlags := addOutputFlags(fs)
		if err := parse(fs, args); err != nil {
			return printProviderToolParameterError("exa", tool, err.Error(), outputFlags, svc)
		}
		if fs.NArg() < 1 {
			return printProviderToolParameterError("exa", tool, "similar requires url", outputFlags, svc)
		}
		data := annotateProviderTool(svc.ExaSimilar(ctx, fs.Arg(0), *numResults), "exa", tool)
		return printCommand("exa", data, makeFormatOutput(outputFlags, svc))
	default:
		return printProviderError("exa", "unsupported exa tool: "+tool, args, svc)
	}
}

func runTavilyGroup(ctx context.Context, svc *service.Service, tool string, args []string) int {
	switch tool {
	case "tavily_search":
		fs := flagSet("tavily search")
		maxResults := fs.Int("max-results", 6, "")
		searchDepth := fs.String("search-depth", "advanced", "")
		topic := fs.String("topic", "", "")
		timeRange := fs.String("time-range", "", "")
		startDate := fs.String("start-date", "", "")
		endDate := fs.String("end-date", "", "")
		country := fs.String("country", "", "")
		includeRawContent := fs.Bool("include-raw-content", false, "")
		includeImages := fs.Bool("include-images", false, "")
		includeFavicon := fs.Bool("include-favicon", false, "")
		includeDomains := stringListFlag{}
		excludeDomains := stringListFlag{}
		fs.Var(&includeDomains, "include-domains", "")
		fs.Var(&excludeDomains, "exclude-domains", "")
		outputFlags := addOutputFlags(fs)
		if err := parse(fs, args); err != nil {
			return printProviderToolParameterError("tavily", tool, err.Error(), outputFlags, svc)
		}
		if fs.NArg() < 1 {
			return printProviderToolParameterError("tavily", tool, "tavily_search requires query", outputFlags, svc)
		}
		data := annotateProviderTool(svc.TavilySearch(ctx, fs.Arg(0), providers.TavilySearchOptions{MaxResults: *maxResults, SearchDepth: *searchDepth, Topic: *topic, TimeRange: *timeRange, StartDate: *startDate, EndDate: *endDate, Country: *country, IncludeRawContent: *includeRawContent, IncludeImages: *includeImages, IncludeFavicon: *includeFavicon, IncludeDomains: includeDomains.values, ExcludeDomains: excludeDomains.values}), "tavily", tool)
		return printCommand("tavily", data, makeFormatOutput(outputFlags, svc))
	case "tavily_extract":
		fs := flagSet("tavily extract")
		contentFormat := fs.String("extract-format", "markdown", "")
		extractDepth := fs.String("extract-depth", "", "")
		query := fs.String("query", "", "")
		includeImages := fs.Bool("include-images", false, "")
		includeFavicon := fs.Bool("include-favicon", false, "")
		timeoutSeconds := fs.Float64("timeout", 0, "")
		urlsFlag := stringListFlag{}
		fs.Var(&urlsFlag, "urls", "")
		outputFlags := addOutputFlags(fs)
		if err := parse(fs, args); err != nil {
			return printProviderToolParameterError("tavily", tool, err.Error(), outputFlags, svc)
		}
		urls := append([]string{}, urlsFlag.values...)
		urls = append(urls, fs.Args()...)
		if len(nonEmptyArgs(urls)) == 0 {
			return printProviderToolParameterError("tavily", tool, "tavily_extract requires at least one url", outputFlags, svc)
		}
		data := annotateProviderTool(svc.TavilyExtract(ctx, urls, providers.TavilyExtractOptions{Format: *contentFormat, ExtractDepth: *extractDepth, Query: *query, IncludeImages: *includeImages, IncludeFavicon: *includeFavicon, TimeoutSeconds: *timeoutSeconds}), "tavily", tool)
		return printCommand("tavily", data, makeFormatOutput(outputFlags, svc))
	case "tavily_map":
		return runTavilyMapLike(ctx, svc, tool, args)
	case "tavily_crawl":
		return runTavilyMapLike(ctx, svc, tool, args)
	default:
		return printProviderError("tavily", "unsupported tavily tool: "+tool, args, svc)
	}
}

func runTavilyMapLike(ctx context.Context, svc *service.Service, tool string, args []string) int {
	fs := flagSet("tavily " + strings.TrimPrefix(tool, "tavily_"))
	instructions := fs.String("instructions", "", "")
	maxDepth := fs.Int("max-depth", 1, "")
	if tool == "tavily_crawl" {
		*maxDepth = 2
	}
	maxBreadth := fs.Int("max-breadth", 20, "")
	limit := fs.Int("limit", 50, "")
	timeoutSeconds := fs.Int("timeout", 150, "")
	allowExternal := fs.Bool("allow-external", false, "")
	extractDepth := fs.String("extract-depth", "", "")
	contentFormat := fs.String("extract-format", "markdown", "")
	includeImages := fs.Bool("include-images", false, "")
	includeFavicon := fs.Bool("include-favicon", false, "")
	selectDomains := stringListFlag{}
	selectPaths := stringListFlag{}
	excludeDomains := stringListFlag{}
	excludePaths := stringListFlag{}
	fs.Var(&selectDomains, "select-domains", "")
	fs.Var(&selectPaths, "select-paths", "")
	fs.Var(&excludeDomains, "exclude-domains", "")
	fs.Var(&excludePaths, "exclude-paths", "")
	outputFlags := addOutputFlags(fs)
	if err := parse(fs, args); err != nil {
		return printProviderToolParameterError("tavily", tool, err.Error(), outputFlags, svc)
	}
	if fs.NArg() < 1 {
		return printProviderToolParameterError("tavily", tool, tool+" requires url", outputFlags, svc)
	}
	if tool == "tavily_map" {
		data := annotateProviderTool(svc.TavilyMap(ctx, fs.Arg(0), providers.TavilyMapOptions{Instructions: *instructions, MaxDepth: *maxDepth, MaxBreadth: *maxBreadth, Limit: *limit, TimeoutSeconds: *timeoutSeconds, AllowExternal: *allowExternal, SelectDomains: selectDomains.values, SelectPaths: selectPaths.values, ExcludeDomains: excludeDomains.values, ExcludePaths: excludePaths.values}), "tavily", tool)
		return printCommand("tavily", data, makeFormatOutput(outputFlags, svc))
	}
	data := annotateProviderTool(svc.TavilyCrawl(ctx, fs.Arg(0), providers.TavilyCrawlOptions{Instructions: *instructions, MaxDepth: *maxDepth, MaxBreadth: *maxBreadth, Limit: *limit, TimeoutSeconds: *timeoutSeconds, AllowExternal: *allowExternal, SelectDomains: selectDomains.values, SelectPaths: selectPaths.values, ExcludeDomains: excludeDomains.values, ExcludePaths: excludePaths.values, ExtractDepth: *extractDepth, Format: *contentFormat, IncludeImages: *includeImages, IncludeFavicon: *includeFavicon}), "tavily", tool)
	return printCommand("tavily", data, makeFormatOutput(outputFlags, svc))
}

func runFirecrawlGroup(ctx context.Context, svc *service.Service, tool string, args []string) int {
	switch tool {
	case "firecrawl_search":
		fs := flagSet("firecrawl search")
		limit := fs.Int("limit", 14, "")
		outputFlags := addOutputFlags(fs)
		if err := parse(fs, args); err != nil {
			return printProviderToolParameterError("firecrawl", tool, err.Error(), outputFlags, svc)
		}
		if fs.NArg() < 1 {
			return printProviderToolParameterError("firecrawl", tool, "firecrawl_search requires query", outputFlags, svc)
		}
		return printCommand("firecrawl", annotateProviderTool(svc.FirecrawlSearch(ctx, fs.Arg(0), *limit), "firecrawl", tool), makeFormatOutput(outputFlags, svc))
	case "firecrawl_scrape":
		fs := flagSet("firecrawl scrape")
		attempts := fs.Int("attempts", 0, "")
		outputFlags := addOutputFlags(fs)
		if err := parse(fs, args); err != nil {
			return printProviderToolParameterError("firecrawl", tool, err.Error(), outputFlags, svc)
		}
		if fs.NArg() < 1 {
			return printProviderToolParameterError("firecrawl", tool, "firecrawl_scrape requires url", outputFlags, svc)
		}
		return printCommand("firecrawl", annotateProviderTool(svc.FirecrawlScrape(ctx, fs.Arg(0), *attempts), "firecrawl", tool), makeFormatOutput(outputFlags, svc))
	case "firecrawl_map":
		fs := flagSet("firecrawl map")
		limit := fs.Int("limit", 50, "")
		outputFlags := addOutputFlags(fs)
		if err := parse(fs, args); err != nil {
			return printProviderToolParameterError("firecrawl", tool, err.Error(), outputFlags, svc)
		}
		if fs.NArg() < 1 {
			return printProviderToolParameterError("firecrawl", tool, "firecrawl_map requires url", outputFlags, svc)
		}
		return printCommand("firecrawl", annotateProviderTool(svc.FirecrawlMap(ctx, fs.Arg(0), *limit), "firecrawl", tool), makeFormatOutput(outputFlags, svc))
	case "firecrawl_crawl":
		fs := flagSet("firecrawl crawl")
		maxDepth := fs.Int("max-depth", 2, "")
		limit := fs.Int("limit", 20, "")
		timeoutSeconds := fs.Int("timeout", 180, "")
		outputFlags := addOutputFlags(fs)
		if err := parse(fs, args); err != nil {
			return printProviderToolParameterError("firecrawl", tool, err.Error(), outputFlags, svc)
		}
		if fs.NArg() < 1 {
			return printProviderToolParameterError("firecrawl", tool, "firecrawl_crawl requires url", outputFlags, svc)
		}
		return printCommand("firecrawl", annotateProviderTool(svc.FirecrawlCrawl(ctx, fs.Arg(0), service.CrawlOptions{MaxDepth: *maxDepth, Limit: *limit, Timeout: *timeoutSeconds}), "firecrawl", tool), makeFormatOutput(outputFlags, svc))
	default:
		return printProviderError("firecrawl", "unsupported firecrawl tool: "+tool, args, svc)
	}
}

func runContext7Group(ctx context.Context, svc *service.Service, tool string, args []string) int {
	switch tool {
	case "resolve_library_id":
		fs := flagSet("context7 resolve-library-id")
		outputFlags := addOutputFlags(fs)
		if err := parse(fs, args); err != nil {
			return printProviderToolParameterError("context7", tool, err.Error(), outputFlags, svc)
		}
		if fs.NArg() < 1 {
			return printProviderToolParameterError("context7", tool, "resolve_library_id requires name", outputFlags, svc)
		}
		query := ""
		if fs.NArg() > 1 {
			query = fs.Arg(1)
		}
		return printCommand("context7", annotateProviderTool(svc.Context7Library(ctx, fs.Arg(0), query), "context7", tool), makeFormatOutput(outputFlags, svc))
	case "query_docs":
		fs := flagSet("context7 query-docs")
		outputFlags := addOutputFlags(fs)
		if err := parse(fs, args); err != nil {
			return printProviderToolParameterError("context7", tool, err.Error(), outputFlags, svc)
		}
		if fs.NArg() < 2 {
			return printProviderToolParameterError("context7", tool, "query_docs requires library_id and query", outputFlags, svc)
		}
		return printCommand("context7", annotateProviderTool(svc.Context7Docs(ctx, fs.Arg(0), fs.Arg(1)), "context7", tool), makeFormatOutput(outputFlags, svc))
	default:
		return printProviderError("context7", "unsupported context7 tool: "+tool, args, svc)
	}
}

func runDeepWikiGroup(ctx context.Context, svc *service.Service, tool string, args []string) int {
	fs := flagSet("deepwiki " + strings.ReplaceAll(tool, "_", "-"))
	outputFlags := addOutputFlags(fs)
	if err := parse(fs, args); err != nil {
		return printProviderToolParameterError("deepwiki", tool, err.Error(), outputFlags, svc)
	}
	if fs.NArg() < 1 {
		return printProviderToolParameterError("deepwiki", tool, tool+" requires repo", outputFlags, svc)
	}
	switch tool {
	case "ask_question":
		if fs.NArg() < 2 {
			return printProviderToolParameterError("deepwiki", tool, "ask_question requires repo and question", outputFlags, svc)
		}
		return printCommand("deepwiki", annotateProviderTool(svc.RepoWiki(ctx, fs.Arg(0), fs.Arg(1), service.RepoWikiOptions{Mode: "ask", Provider: "deepwiki"}), "deepwiki", tool), makeFormatOutput(outputFlags, svc))
	case "read_wiki_structure":
		return printCommand("deepwiki", annotateProviderTool(svc.RepoWiki(ctx, fs.Arg(0), "", service.RepoWikiOptions{Mode: "structure", Provider: "deepwiki"}), "deepwiki", tool), makeFormatOutput(outputFlags, svc))
	case "read_wiki_contents":
		return printCommand("deepwiki", annotateProviderTool(svc.RepoWiki(ctx, fs.Arg(0), "", service.RepoWikiOptions{Mode: "contents", Provider: "deepwiki"}), "deepwiki", tool), makeFormatOutput(outputFlags, svc))
	default:
		return printProviderError("deepwiki", "unsupported deepwiki tool: "+tool, args, svc)
	}
}

func runAnySearchGroup(ctx context.Context, svc *service.Service, tool string, args []string) int {
	switch tool {
	case "domains":
		return runAnyDomains(ctx, svc, args)
	case "search":
		return runAnySearch(ctx, svc, args)
	case "extract":
		return runAnyExtract(ctx, svc, args)
	case "batch":
		return runAnyBatch(ctx, svc, args)
	default:
		return printProviderError("anysearch", "unsupported anysearch tool: "+tool, args, svc)
	}
}

func runZhipuGroup(ctx context.Context, svc *service.Service, tool string, args []string) int {
	switch tool {
	case "zhipu_search":
		fs := flagSet("zhipu search")
		count := fs.Int("count", 10, "")
		engine := fs.String("search-engine", "", "")
		recency := fs.String("search-recency-filter", "noLimit", "")
		domain := fs.String("search-domain-filter", "", "")
		contentSize := fs.String("content-size", "medium", "")
		outputFlags := addOutputFlags(fs)
		if err := parse(fs, args); err != nil {
			return printProviderToolParameterError("zhipu", tool, err.Error(), outputFlags, svc)
		}
		if fs.NArg() < 1 {
			return printProviderToolParameterError("zhipu", tool, "zhipu search requires query", outputFlags, svc)
		}
		data := annotateProviderTool(svc.ZhipuSearch(ctx, fs.Arg(0), providers.ZhipuOptions{Count: *count, SearchEngine: *engine, SearchRecencyFilter: *recency, SearchDomainFilter: *domain, ContentSize: *contentSize}), "zhipu", tool)
		return printCommand("zhipu", data, makeFormatOutput(outputFlags, svc))
	default:
		return printProviderError("zhipu", "unsupported zhipu tool: "+tool, args, svc)
	}
}

func canonicalProviderTool(provider, subcommand string) (string, bool) {
	aliases := providerToolAliases[provider]
	if aliases == nil {
		return "", false
	}
	tool, ok := aliases[subcommand]
	return tool, ok
}

func annotateProviderTool(data map[string]any, provider, tool string) map[string]any {
	if data == nil {
		data = map[string]any{}
	}
	data["provider"] = provider
	data["tool"] = tool
	return data
}

func annotateStatusCommands(data map[string]any) map[string]any {
	if data == nil {
		data = map[string]any{}
	}
	providers, _ := data["providers"].(map[string]any)
	direct := map[string]any{}
	names := make([]string, 0, len(providerToolAliases))
	for provider := range providerToolAliases {
		names = append(names, provider)
	}
	sort.Strings(names)
	for _, provider := range names {
		commands := providerCommands(provider)
		item := map[string]any{"commands": commands}
		if raw, ok := providers[provider]; ok {
			providerData, _ := raw.(map[string]any)
			if providerData != nil {
				providerData["direct"] = true
				providerData["commands"] = commands
				for _, key := range []string{"available", "enabled", "capabilities", "status", "base_url", "api_key_env", "has_api_key"} {
					if value, ok := providerData[key]; ok {
						item[key] = value
					}
				}
			}
		} else {
			item["available"] = false
			item["reason"] = "unknown_provider"
		}
		direct[provider] = item
	}
	data["direct_endpoints"] = direct
	return data
}

func providerCommands(provider string) []string {
	aliases := providerToolAliases[provider]
	commands := make([]string, 0, len(aliases))
	for alias := range aliases {
		commands = append(commands, "onesearch "+provider+" "+alias)
	}
	sort.Strings(commands)
	return commands
}

func printProviderError(provider, message string, args []string, svc *service.Service) int {
	return printCommand(provider, map[string]any{"ok": false, "provider": provider, "error_type": "parameter_error", "error": message}, parseFormatOutput(args, svc))
}

func printProviderToolParameterError(provider, tool, message string, flags outputFlags, svc *service.Service) int {
	return printCommand(provider, map[string]any{"ok": false, "provider": provider, "tool": tool, "error_type": "parameter_error", "error": message}, makeFormatOutput(flags, svc))
}

func printProviderHelp(provider string) {
	fmt.Println("onesearch " + provider + " <command> [args] [--format json|markdown|content] [--quiet|--verbose]")
	if aliases := providerToolAliases[provider]; len(aliases) > 0 {
		fmt.Println()
		fmt.Println("Commands:")
		names := make([]string, 0, len(aliases))
		for alias := range aliases {
			names = append(names, alias)
		}
		sort.Strings(names)
		for _, alias := range names {
			fmt.Println("  " + alias)
		}
	}
}

func isHelpToken(value string) bool {
	return value == "--help" || value == "-h"
}

func nonEmptyArgs(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if text := strings.TrimSpace(item); text != "" {
			out = append(out, text)
		}
	}
	return out
}
