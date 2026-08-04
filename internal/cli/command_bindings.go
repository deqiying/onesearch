package cli

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/deqiying/onesearch/internal/providers"
	"github.com/deqiying/onesearch/internal/redact"
	"github.com/deqiying/onesearch/internal/service"
)

type commandBinding struct {
	ID         string
	HandlerKey string
}

var commandBindings = map[string]commandBinding{
	"search":                       {ID: "search", HandlerKey: "search"},
	"fetch":                        {ID: "fetch", HandlerKey: "fetch"},
	"map":                          {ID: "map", HandlerKey: "map"},
	"crawl":                        {ID: "crawl", HandlerKey: "crawl"},
	"repo-wiki":                    {ID: "repo-wiki", HandlerKey: "repo-wiki"},
	"deep":                         {ID: "deep", HandlerKey: "deep"},
	"exa.web-search":               {ID: "exa.web-search", HandlerKey: "web_search_exa"},
	"exa.web-fetch":                {ID: "exa.web-fetch", HandlerKey: "web_fetch_exa"},
	"exa.similar":                  {ID: "exa.similar", HandlerKey: "find_similar"},
	"tavily.search":                {ID: "tavily.search", HandlerKey: "tavily_search"},
	"tavily.extract":               {ID: "tavily.extract", HandlerKey: "tavily_extract"},
	"tavily.map":                   {ID: "tavily.map", HandlerKey: "tavily_map"},
	"tavily.crawl":                 {ID: "tavily.crawl", HandlerKey: "tavily_crawl"},
	"firecrawl.search":             {ID: "firecrawl.search", HandlerKey: "firecrawl_search"},
	"firecrawl.scrape":             {ID: "firecrawl.scrape", HandlerKey: "firecrawl_scrape"},
	"firecrawl.map":                {ID: "firecrawl.map", HandlerKey: "firecrawl_map"},
	"firecrawl.crawl":              {ID: "firecrawl.crawl", HandlerKey: "firecrawl_crawl"},
	"context7.resolve-library-id":  {ID: "context7.resolve-library-id", HandlerKey: "resolve_library_id"},
	"context7.query-docs":          {ID: "context7.query-docs", HandlerKey: "query_docs"},
	"deepwiki.ask-question":        {ID: "deepwiki.ask-question", HandlerKey: "ask_question"},
	"deepwiki.read-wiki-structure": {ID: "deepwiki.read-wiki-structure", HandlerKey: "read_wiki_structure"},
	"deepwiki.read-wiki-contents":  {ID: "deepwiki.read-wiki-contents", HandlerKey: "read_wiki_contents"},
	"anysearch.domains":            {ID: "anysearch.domains", HandlerKey: "domains"},
	"anysearch.search":             {ID: "anysearch.search", HandlerKey: "search"},
	"anysearch.extract":            {ID: "anysearch.extract", HandlerKey: "extract"},
	"anysearch.batch":              {ID: "anysearch.batch", HandlerKey: "batch"},
	"zhipu.search":                 {ID: "zhipu.search", HandlerKey: "zhipu_search"},
	"ddg.search":                   {ID: "ddg.search", HandlerKey: "search"},
	"ddg.fetch-content":            {ID: "ddg.fetch-content", HandlerKey: "fetch_content"},
	"freecrawl.search":             {ID: "freecrawl.search", HandlerKey: "search"},
	"freecrawl.scrape":             {ID: "freecrawl.scrape", HandlerKey: "scrape"},
	"freecrawl.crawl":              {ID: "freecrawl.crawl", HandlerKey: "crawl"},
	"freecrawl.deep-research":      {ID: "freecrawl.deep-research", HandlerKey: "deep_research"},
	"doctor":                       {ID: "doctor", HandlerKey: "doctor"},
	"status":                       {ID: "status", HandlerKey: "status"},
	"smoke":                        {ID: "smoke", HandlerKey: "smoke"},
	"model.current":                {ID: "model.current", HandlerKey: "model.current"},
	"config.path":                  {ID: "config.path", HandlerKey: "config.path"},
	"config.list":                  {ID: "config.list", HandlerKey: "config.list"},
	"config.setup":                 {ID: "config.setup", HandlerKey: "config.setup"},
	"skills.list":                  {ID: "skills.list", HandlerKey: "skills.list"},
	"skills.show":                  {ID: "skills.show", HandlerKey: "skills.show"},
	"regression":                   {ID: "regression", HandlerKey: "regression"},
	"schema":                       {ID: "schema", HandlerKey: "schema"},
}

func init() {
	if err := validateCommandBindings(); err != nil {
		panic(err)
	}
}

func validateCommandBindings() error {
	definitions := commandRegistry.Commands()
	if len(commandBindings) != len(definitions) {
		return fmt.Errorf("command binding count %d does not match public command count %d", len(commandBindings), len(definitions))
	}
	for _, definition := range definitions {
		binding, ok := commandBindings[definition.ID]
		if !ok || binding.ID != definition.ID || binding.HandlerKey == "" {
			return fmt.Errorf("missing command binding for %q", definition.ID)
		}
	}
	return nil
}

func bindingFor(id string) (commandBinding, bool) {
	binding, ok := commandBindings[id]
	return binding, ok
}

func runParsedCommand(ctx context.Context, svc *service.Service, parsed *parsedCommand) int {
	if parsed.Definition.Category == "provider" {
		return runParsedProvider(ctx, svc, parsed)
	}
	fo := formatOutputFromParsed(parsed, svc)
	switch parsed.Definition.ID {
	case "search":
		return runParsedSearch(ctx, svc, parsed, fo)
	case "fetch":
		return printCommand(svc, "fetch", svc.Fetch(ctx, parsed.String("url"), service.FetchOptions{Provider: parsed.String("provider")}), fo)
	case "map":
		data := svc.Map(ctx, parsed.String("url"), service.MapOptions{Instructions: parsed.String("instructions"), MaxDepth: parsed.Int("max_depth"), MaxBreadth: parsed.Int("max_breadth"), Limit: parsed.Int("limit"), Timeout: parsed.Int("timeout"), Provider: parsed.String("provider")})
		return printCommand(svc, "map", data, fo)
	case "crawl":
		timeout := parsed.Int("timeout")
		timedCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()
		data := svc.Crawl(timedCtx, parsed.String("url"), service.CrawlOptions{MaxDepth: parsed.Int("max_depth"), Limit: parsed.Int("limit"), Timeout: timeout, Provider: parsed.String("provider")})
		if timedCtx.Err() == context.DeadlineExceeded {
			data = map[string]any{"ok": false, "error_type": "network_error", "error": fmt.Sprintf("Crawl timed out after %d seconds", timeout), "url": parsed.String("url"), "timeout_seconds": timeout}
		}
		return printCommand(svc, "crawl", data, fo)
	case "repo-wiki":
		timeout := parsed.Number("timeout")
		timedCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout*float64(time.Second)))
		defer cancel()
		data := svc.RepoWiki(timedCtx, parsed.String("repo"), parsed.String("question"), service.RepoWikiOptions{Mode: parsed.String("mode"), Provider: parsed.String("provider")})
		if timedCtx.Err() == context.DeadlineExceeded {
			data = map[string]any{"ok": false, "error_type": "network_error", "error": fmt.Sprintf("Repo wiki timed out after %g seconds", timeout), "repo": parsed.String("repo"), "timeout_seconds": timeout}
		}
		return printCommand(svc, "repo-wiki", data, fo)
	case "deep":
		return printCommand(svc, "deep", svc.DeepPlan(parsed.String("query"), parsed.String("budget"), parsed.String("evidence_dir")), fo)
	case "doctor":
		return printCommand(svc, "doctor", svc.Doctor(ctx), fo)
	case "status":
		return printCommand(svc, "status", annotateStatusCommands(svc.Status()), fo)
	case "smoke":
		mode := parsed.String("mode")
		if parsed.Bool("mock") {
			mode = "mock"
		}
		if parsed.Bool("live") {
			mode = "live"
		}
		return printCommand(svc, "smoke", svc.Smoke(ctx, mode), fo)
	case "model.current":
		return printCommand(svc, "model", svc.CurrentModel(), fo)
	case "config.path":
		return printCommand(svc, "config", svc.ConfigPath(), fo)
	case "config.list":
		return printCommand(svc, "config", svc.ConfigList(false), fo)
	case "config.setup":
		return runParsedConfigSetup(svc, parsed, fo)
	case "skills.list":
		return printCommand(svc, "skills", skillsListData(parsed.String("capability")), fo)
	case "skills.show":
		return printCommand(svc, "skills", skillShowData(parsed.String("name")), fo)
	case "regression":
		return printCommand(svc, "smoke", svc.Smoke(ctx, "mock"), formatOutput{format: "json", verbosity: "quiet"})
	default:
		return printParameterError(svc, outputCommand(parsed.Definition.ID), "unsupported command binding: "+parsed.Definition.ID, fo)
	}
}

func runParsedSearch(ctx context.Context, svc *service.Service, parsed *parsedCommand, fo formatOutput) int {
	providersFilter, providerFilters, err := parseSearchProviderFilters(parsed.String("providers"))
	if err != nil {
		return printParameterError(svc, "search", err.Error(), fo)
	}
	providerFilters = overlayProviderFilter(providerFilters, "answer_search", parsed.String("answer_providers"))
	providerFilters = overlayProviderFilter(providerFilters, "source_search", parsed.String("source_providers"))
	providerFilters = overlayProviderFilter(providerFilters, "docs_search", parsed.String("docs_providers"))
	providerFilters = overlayProviderFilter(providerFilters, "page_fetch", parsed.String("fetch_providers"))
	providerFilters = overlayProviderFilter(providerFilters, "repo_wiki", parsed.String("repo_providers"))
	var stream *bool
	if parsed.Bool("stream") {
		value := true
		stream = &value
	}
	if parsed.Bool("no_stream") {
		value := false
		stream = &value
	}
	timeout := parsed.Number("timeout")
	timedCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout*float64(time.Second)))
	defer cancel()
	query := parsed.String("query")
	data := svc.Search(timedCtx, query, service.SearchOptions{
		Platform: parsed.String("platform"), Model: parsed.String("model"), ExtraSources: parsed.Int("extra_sources"), FetchSources: parsed.Int("fetch_sources"),
		Validation: parsed.String("validation"), Fallback: parsed.String("fallback"), Providers: providersFilter, ProviderFilters: providerFilters, Stream: stream,
		RepoWiki: parsed.String("repo_wiki"), RepoWikiMode: parsed.String("repo_wiki_mode"), RepoWikiQuery: parsed.String("repo_wiki_query"),
	})
	if timedCtx.Err() == context.DeadlineExceeded {
		data = map[string]any{"ok": false, "error_type": "network_error", "error": fmt.Sprintf("Search timed out after %g seconds", timeout), "query": query, "content": "", "sources": []map[string]any{}, "sources_count": 0, "timeout_seconds": timeout}
	}
	return printCommand(svc, "search", data, fo)
}

func runParsedProvider(ctx context.Context, svc *service.Service, parsed *parsedCommand) int {
	binding := commandBindings[parsed.Definition.ID]
	provider := parsed.Definition.Provider
	tool := binding.HandlerKey
	fo := formatOutputFromParsed(parsed, svc)
	var data map[string]any
	switch parsed.Definition.ID {
	case "exa.web-search":
		numResults := parsed.Int("num_results")
		if parsed.Int("max_results") > 0 {
			numResults = parsed.Int("max_results")
		}
		data = svc.ExaSearch(ctx, parsed.String("query"), providers.ExaOptions{NumResults: numResults, SearchType: parsed.String("search_type"), IncludeText: parsed.Bool("include_text"), IncludeHighlights: parsed.Bool("include_highlights"), StartPublishedDate: parsed.String("start_published_date"), IncludeDomains: parsed.Strings("include_domains"), ExcludeDomains: parsed.Strings("exclude_domains"), Category: parsed.String("category")})
	case "exa.web-fetch":
		urls := append(parsed.Strings("urls"), parsed.Strings("url_args")...)
		data = svc.ExaFetch(ctx, urls, providers.ExaFetchOptions{MaxCharacters: parsed.Int("max_characters")})
	case "exa.similar":
		data = svc.ExaSimilar(ctx, parsed.String("url"), parsed.Int("num_results"))
	case "tavily.search":
		data = svc.TavilySearch(ctx, parsed.String("query"), providers.TavilySearchOptions{MaxResults: parsed.Int("max_results"), SearchDepth: parsed.String("search_depth"), Topic: parsed.String("topic"), TimeRange: parsed.String("time_range"), StartDate: parsed.String("start_date"), EndDate: parsed.String("end_date"), Country: parsed.String("country"), IncludeRawContent: parsed.Bool("include_raw_content"), IncludeImages: parsed.Bool("include_images"), IncludeFavicon: parsed.Bool("include_favicon"), IncludeDomains: parsed.Strings("include_domains"), ExcludeDomains: parsed.Strings("exclude_domains")})
	case "tavily.extract":
		urls := append(parsed.Strings("urls"), parsed.Strings("url_args")...)
		data = svc.TavilyExtract(ctx, urls, providers.TavilyExtractOptions{Format: parsed.String("extract_format"), ExtractDepth: parsed.String("extract_depth"), Query: parsed.String("query"), IncludeImages: parsed.Bool("include_images"), IncludeFavicon: parsed.Bool("include_favicon"), TimeoutSeconds: parsed.Number("timeout")})
	case "tavily.map":
		data = svc.TavilyMap(ctx, parsed.String("url"), providers.TavilyMapOptions{Instructions: parsed.String("instructions"), MaxDepth: parsed.Int("max_depth"), MaxBreadth: parsed.Int("max_breadth"), Limit: parsed.Int("limit"), TimeoutSeconds: parsed.Int("timeout"), AllowExternal: parsed.Bool("allow_external"), SelectDomains: parsed.Strings("select_domains"), SelectPaths: parsed.Strings("select_paths"), ExcludeDomains: parsed.Strings("exclude_domains"), ExcludePaths: parsed.Strings("exclude_paths")})
	case "tavily.crawl":
		data = svc.TavilyCrawl(ctx, parsed.String("url"), providers.TavilyCrawlOptions{Instructions: parsed.String("instructions"), MaxDepth: parsed.Int("max_depth"), MaxBreadth: parsed.Int("max_breadth"), Limit: parsed.Int("limit"), TimeoutSeconds: parsed.Int("timeout"), AllowExternal: parsed.Bool("allow_external"), SelectDomains: parsed.Strings("select_domains"), SelectPaths: parsed.Strings("select_paths"), ExcludeDomains: parsed.Strings("exclude_domains"), ExcludePaths: parsed.Strings("exclude_paths"), ExtractDepth: parsed.String("extract_depth"), Format: parsed.String("extract_format"), IncludeImages: parsed.Bool("include_images"), IncludeFavicon: parsed.Bool("include_favicon")})
	case "firecrawl.search":
		data = svc.FirecrawlSearch(ctx, parsed.String("query"), parsed.Int("limit"))
	case "firecrawl.scrape":
		data = svc.FirecrawlScrape(ctx, parsed.String("url"), parsed.Int("attempts"))
	case "firecrawl.map":
		data = svc.FirecrawlMap(ctx, parsed.String("url"), parsed.Int("limit"))
	case "firecrawl.crawl":
		data = svc.FirecrawlCrawl(ctx, parsed.String("url"), service.CrawlOptions{MaxDepth: parsed.Int("max_depth"), Limit: parsed.Int("limit"), Timeout: parsed.Int("timeout")})
	case "context7.resolve-library-id":
		data = svc.Context7Library(ctx, parsed.String("name"), parsed.String("query"))
	case "context7.query-docs":
		data = svc.Context7Docs(ctx, parsed.String("library_id"), parsed.String("query"))
	case "deepwiki.ask-question":
		data = svc.RepoWiki(ctx, parsed.String("repo"), parsed.String("question"), service.RepoWikiOptions{Mode: "ask", Provider: "deepwiki"})
	case "deepwiki.read-wiki-structure":
		data = svc.RepoWiki(ctx, parsed.String("repo"), "", service.RepoWikiOptions{Mode: "structure", Provider: "deepwiki"})
	case "deepwiki.read-wiki-contents":
		data = svc.RepoWiki(ctx, parsed.String("repo"), "", service.RepoWikiOptions{Mode: "contents", Provider: "deepwiki"})
	case "anysearch.domains":
		data = svc.AnySearch().Domains(ctx, parsed.String("domain"))
	case "anysearch.search":
		data = svc.AnySearch().Search(ctx, parsed.String("query"), parsed.String("domain"), parsed.String("sub_domain"), parsed.Int("max_results"))
	case "anysearch.extract":
		data = svc.AnySearch().Extract(ctx, parsed.String("url"), parsed.Int("max_length"))
	case "anysearch.batch":
		data = svc.AnySearch().Batch(ctx, parsed.Strings("queries"), parsed.Int("max_results"))
	case "zhipu.search":
		data = svc.ZhipuSearch(ctx, parsed.String("query"), providers.ZhipuOptions{Count: parsed.Int("count"), SearchEngine: parsed.String("search_engine"), SearchRecencyFilter: parsed.String("search_recency_filter"), SearchDomainFilter: parsed.String("search_domain_filter"), ContentSize: parsed.String("content_size")})
	case "ddg.search":
		data = svc.DDGSearch(ctx, parsed.String("query"), providers.DDGSearchOptions{MaxResults: parsed.Int("max_results"), Region: parsed.String("region")})
	case "ddg.fetch-content":
		data = svc.DDGFetchContent(ctx, parsed.String("url"), providers.DDGFetchOptions{StartIndex: parsed.Int("start_index"), MaxLength: parsed.Int("max_length"), Backend: parsed.String("backend")})
	case "freecrawl.search":
		data = svc.FreecrawlSearch(ctx, parsed.String("query"), providers.FreecrawlSearchOptions{NumResults: parsed.Int("num_results"), SearchEngine: parsed.String("search_engine"), ScrapeResults: parsed.Bool("scrape_results")})
	case "freecrawl.scrape":
		data = svc.FreecrawlScrape(ctx, parsed.String("url"), providers.FreecrawlScrapeOptions{Formats: splitCSV(parsed.String("formats")), Javascript: parsed.Bool("javascript"), AntiBot: parsed.Bool("anti_bot"), Cache: parsed.Bool("cache"), Timeout: parsed.Int("timeout"), WaitFor: parsed.Int("wait_for")})
	case "freecrawl.crawl":
		data = svc.FreecrawlCrawl(ctx, parsed.String("url"), providers.FreecrawlCrawlOptions{MaxDepth: parsed.Int("max_depth"), MaxPages: parsed.Int("max_pages"), SameDomainOnly: parsed.Bool("same_domain_only"), IncludePatterns: parsed.Strings("include_patterns"), ExcludePatterns: parsed.Strings("exclude_patterns")})
	case "freecrawl.deep-research":
		data = svc.FreecrawlDeepResearch(ctx, parsed.String("topic"), providers.FreecrawlDeepResearchOptions{NumSources: parsed.Int("num_sources"), MaxDepth: parsed.Int("max_depth"), IncludeAcademic: parsed.Bool("include_academic"), SearchQueries: parsed.Strings("search_queries")})
	default:
		return printProviderToolParameterErrorParsed(parsed, "unsupported provider command: "+parsed.Definition.ID, svc)
	}
	return printCommand(svc, provider, annotateProviderTool(data, provider, tool), fo)
}

func runParsedConfigSetup(svc *service.Service, parsed *parsedCommand, fo formatOutput) int {
	requestedProvider := parsed.String("provider")
	spec, err := svc.ProviderSetupSpec(requestedProvider)
	if err != nil {
		return printCommand(svc, "config", setupErrorData(svc, requestedProvider, err), fo)
	}
	interactive := configInputIsTerminal() && !parsed.Bool("api_key_stdin")
	var apiKey *string
	var transientSecret string
	if parsed.Bool("api_key_stdin") {
		value, readErr := readConfigLine(configStdin())
		transientSecret = value
		if readErr != nil {
			return printCommand(svc, "config", configInputErrorData(svc, spec.Provider, readErr), fo, transientSecret)
		}
		apiKey = &value
	} else if interactive {
		prompt := "API key"
		if spec.HasEffectiveAPIKey {
			prompt += " [已配置，留空保留]"
		}
		writeConfigPrompt(svc, prompt+": ")
		value, readErr := configReadPassword()
		fmt.Fprintln(os.Stderr)
		text := strings.TrimSpace(string(value))
		transientSecret = text
		if readErr != nil {
			return printCommand(svc, "config", configInputErrorData(svc, spec.Provider, readErr), fo, transientSecret)
		}
		apiKey = &text
	} else if spec.RequiresAPIKey && !spec.HasEffectiveAPIKey {
		return printParameterError(svc, "config", "non-interactive setup requires --api-key-stdin when no effective API key exists", fo)
	}
	var requestedBaseURL *string
	if parsed.IsSet("base_url") {
		value := parsed.String("base_url")
		requestedBaseURL = &value
	} else if interactive && spec.SupportsBaseURL {
		prompt := "Base URL"
		if value := strings.TrimSpace(spec.EffectiveBaseURL); value != "" {
			prompt += " [" + safeBaseURLPrompt(value) + "]"
		}
		writeConfigPrompt(svc, redact.Text(prompt+": ", []string{transientSecret}))
		value, readErr := readConfigLine(configStdin())
		if readErr != nil {
			return printCommand(svc, "config", configInputErrorData(svc, spec.Provider, readErr), fo, transientSecret)
		}
		requestedBaseURL = &value
	}
	preSaveSecrets := svc.OutputSecretValues()
	data := svc.SetupProvider(service.ProviderSetupRequest{Provider: spec.Provider, APIKey: apiKey, BaseURL: requestedBaseURL})
	return printCommand(svc, "config", data, fo, append(preSaveSecrets, transientSecret)...)
}

func formatOutputFromParsed(parsed *parsedCommand, svc *service.Service) formatOutput {
	verbosity := defaultVerbosity(svc)
	if parsed.Bool("verbose") {
		verbosity = "verbose"
	}
	if parsed.Bool("quiet") {
		verbosity = "quiet"
	}
	format := parsed.String("format")
	if format == "" {
		format = "json"
	}
	return formatOutput{format: format, output: parsed.String("output"), verbosity: verbosity}
}

func printParsedParameterError(parsed *parsedCommand, message string, svc *service.Service) int {
	if parsed.Definition.Category == "provider" {
		return printProviderToolParameterErrorParsed(parsed, message, svc)
	}
	return printParameterError(svc, outputCommand(parsed.Definition.ID), message, formatOutputFromParsed(parsed, svc))
}

func printProviderToolParameterErrorParsed(parsed *parsedCommand, message string, svc *service.Service) int {
	binding := commandBindings[parsed.Definition.ID]
	data := map[string]any{"ok": false, "provider": parsed.Definition.Provider, "tool": binding.HandlerKey, "error_type": "parameter_error", "error": message}
	return printCommand(svc, parsed.Definition.Provider, data, formatOutputFromParsed(parsed, svc))
}

func outputCommand(id string) string {
	if index := strings.IndexByte(id, '.'); index >= 0 {
		return id[:index]
	}
	return id
}

func providerBindingCommands(provider string) []string {
	commands := []string{}
	for _, definition := range commandRegistry.CommandsForProvider(provider) {
		commands = append(commands, "onesearch "+strings.Join(definition.Path, " "))
	}
	sort.Strings(commands)
	return commands
}
