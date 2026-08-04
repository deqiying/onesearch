package commandcontract

func providerDefinitions() []CommandDefinition {
	definitions := []CommandDefinition{
		providerCommand("exa.web-search", "exa", []string{"exa", "web-search"}, "Search the web with Exa.", []string{"docs_search", "source_search"}, []PositionalDefinition{positional("query", "Search query.", true)}, []OptionDefinition{
			optionDefault("num_results", "num-results", TypeInteger, 5, "Number of results."),
			{Name: "max_results", Flag: "max-results", Type: TypeInteger, Default: 0, HasDefault: true, Description: "Compatibility flag that overrides num_results when greater than zero.", Deprecated: true, Overrides: "num_results", OverridesWhen: "positive"},
			optionDefault("search_type", "search-type", TypeString, "neural", "Exa search type."),
			optionDefault("include_text", "include-text", TypeBoolean, false, "Include page text."),
			optionDefault("include_highlights", "include-highlights", TypeBoolean, false, "Include highlights."),
			optionDefault("start_published_date", "start-published-date", TypeString, "", "Minimum publication date."),
			optionDefault("category", "category", TypeString, "", "Optional result category."),
			listOption("include_domains", "include-domains", "Domains to include; repeat the flag or provide comma-separated values."),
			listOption("exclude_domains", "exclude-domains", "Domains to exclude; repeat the flag or provide comma-separated values."),
		}),
		providerCommand("exa.web-fetch", "exa", []string{"exa", "web-fetch"}, "Fetch one or more URLs with Exa.", []string{"page_fetch"}, []PositionalDefinition{variadicPositional("url_args", "URLs supplied as positional arguments.", 0)}, []OptionDefinition{
			listOption("urls", "urls", "URLs to fetch; repeat the flag or provide comma-separated values."),
			optionDefault("max_characters", "max-characters", TypeInteger, 20000, "Maximum characters per fetched page."),
		}),
		providerCommand("exa.similar", "exa", []string{"exa", "similar"}, "Find pages similar to a URL with Exa.", []string{"source_search"}, []PositionalDefinition{positional("url", "Reference URL.", true)}, []OptionDefinition{
			optionDefault("num_results", "num-results", TypeInteger, 5, "Number of results."),
		}),

		providerCommand("tavily.search", "tavily", []string{"tavily", "search"}, "Search the web with Tavily.", []string{"source_search"}, []PositionalDefinition{positional("query", "Search query.", true)}, []OptionDefinition{
			optionDefault("max_results", "max-results", TypeInteger, 6, "Number of results."),
			optionDefault("search_depth", "search-depth", TypeString, "advanced", "Tavily search depth."),
			optionDefault("topic", "topic", TypeString, "", "Search topic."),
			optionDefault("time_range", "time-range", TypeString, "", "Relative time range."),
			optionDefault("start_date", "start-date", TypeString, "", "Start date."),
			optionDefault("end_date", "end-date", TypeString, "", "End date."),
			optionDefault("country", "country", TypeString, "", "Country filter."),
			optionDefault("include_raw_content", "include-raw-content", TypeBoolean, false, "Include raw page content."),
			optionDefault("include_images", "include-images", TypeBoolean, false, "Include images."),
			optionDefault("include_favicon", "include-favicon", TypeBoolean, false, "Include favicons."),
			listOption("include_domains", "include-domains", "Domains to include."),
			listOption("exclude_domains", "exclude-domains", "Domains to exclude."),
		}),
		providerCommand("tavily.extract", "tavily", []string{"tavily", "extract"}, "Extract content from one or more URLs with Tavily.", []string{"page_fetch"}, []PositionalDefinition{variadicPositional("url_args", "URLs supplied as positional arguments.", 0)}, []OptionDefinition{
			listOption("urls", "urls", "URLs to extract."),
			optionDefault("extract_format", "extract-format", TypeString, "markdown", "Extracted content format."),
			optionDefault("extract_depth", "extract-depth", TypeString, "", "Extraction depth."),
			optionDefault("query", "query", TypeString, "", "Extraction relevance query."),
			optionDefault("include_images", "include-images", TypeBoolean, false, "Include images."),
			optionDefault("include_favicon", "include-favicon", TypeBoolean, false, "Include favicons."),
			optionDefault("timeout", "timeout", TypeNumber, float64(0), "Timeout in seconds; zero uses provider defaults."),
		}),
		tavilyMapDefinition(false),
		tavilyMapDefinition(true),

		providerCommand("firecrawl.search", "firecrawl", []string{"firecrawl", "search"}, "Search the web with Firecrawl.", []string{"source_search"}, []PositionalDefinition{positional("query", "Search query.", true)}, []OptionDefinition{
			optionDefault("limit", "limit", TypeInteger, 14, "Number of results."),
		}),
		providerCommand("firecrawl.scrape", "firecrawl", []string{"firecrawl", "scrape"}, "Scrape a URL with Firecrawl.", []string{"page_fetch"}, []PositionalDefinition{positional("url", "Target URL.", true)}, []OptionDefinition{
			optionDefault("attempts", "attempts", TypeInteger, 0, "Retry attempts; zero uses provider defaults."),
		}),
		providerCommand("firecrawl.map", "firecrawl", []string{"firecrawl", "map"}, "Map links from a website with Firecrawl.", []string{"site_map"}, []PositionalDefinition{positional("url", "Website URL.", true)}, []OptionDefinition{
			optionDefault("limit", "limit", TypeInteger, 50, "Maximum returned links."),
		}),
		providerCommand("firecrawl.crawl", "firecrawl", []string{"firecrawl", "crawl"}, "Crawl a website with Firecrawl.", []string{"site_crawl"}, []PositionalDefinition{positional("url", "Website URL.", true)}, []OptionDefinition{
			optionDefault("max_depth", "max-depth", TypeInteger, 2, "Maximum traversal depth."),
			optionDefault("limit", "limit", TypeInteger, 20, "Maximum crawled pages."),
			optionDefault("timeout", "timeout", TypeInteger, 180, "Timeout in seconds."),
		}),

		providerCommand("context7.resolve-library-id", "context7", []string{"context7", "resolve-library-id"}, "Resolve a Context7 library identifier.", []string{"docs_search"}, []PositionalDefinition{
			positional("name", "Library name.", true), positional("query", "Optional documentation query.", false),
		}, nil),
		providerCommand("context7.query-docs", "context7", []string{"context7", "query-docs"}, "Query documentation from a Context7 library.", []string{"docs_search"}, []PositionalDefinition{
			positional("library_id", "Resolved Context7 library identifier.", true), positional("query", "Documentation query.", true),
		}, nil),

		providerCommand("deepwiki.ask-question", "deepwiki", []string{"deepwiki", "ask-question"}, "Ask a question about a repository wiki.", []string{"repo_wiki"}, []PositionalDefinition{
			positional("repo", "Repository in owner/name form.", true), positional("question", "Repository question.", true),
		}, nil),
		providerCommand("deepwiki.read-wiki-structure", "deepwiki", []string{"deepwiki", "read-wiki-structure"}, "Read a repository wiki structure.", []string{"repo_wiki"}, []PositionalDefinition{positional("repo", "Repository in owner/name form.", true)}, nil),
		providerCommand("deepwiki.read-wiki-contents", "deepwiki", []string{"deepwiki", "read-wiki-contents"}, "Read repository wiki contents.", []string{"repo_wiki"}, []PositionalDefinition{positional("repo", "Repository in owner/name form.", true)}, nil),

		providerCommand("anysearch.domains", "anysearch", []string{"anysearch", "domains"}, "List or inspect AnySearch domains.", []string{"vertical_search"}, []PositionalDefinition{positional("domain", "Optional domain identifier.", false)}, nil),
		providerCommand("anysearch.search", "anysearch", []string{"anysearch", "search"}, "Search an AnySearch vertical domain.", []string{"vertical_search"}, []PositionalDefinition{positional("query", "Search query.", true)}, []OptionDefinition{
			optionDefault("domain", "domain", TypeString, "", "Domain filter."),
			optionDefault("sub_domain", "sub-domain", TypeString, "", "Sub-domain filter."),
			optionDefault("max_results", "max-results", TypeInteger, 5, "Number of results."),
		}),
		providerCommand("anysearch.extract", "anysearch", []string{"anysearch", "extract"}, "Extract content from a URL with AnySearch.", []string{"page_fetch"}, []PositionalDefinition{positional("url", "Target URL.", true)}, []OptionDefinition{
			optionDefault("max_length", "max-length", TypeInteger, 20000, "Maximum content length."),
		}),
		providerCommand("anysearch.batch", "anysearch", []string{"anysearch", "batch"}, "Run multiple AnySearch queries.", []string{"vertical_search"}, []PositionalDefinition{variadicPositional("queries", "Queries to execute.", 1)}, []OptionDefinition{
			optionDefault("max_results", "max-results", TypeInteger, 3, "Number of results per query."),
		}),

		providerCommand("zhipu.search", "zhipu", []string{"zhipu", "search"}, "Search the web with Zhipu.", []string{"source_search"}, []PositionalDefinition{positional("query", "Search query.", true)}, []OptionDefinition{
			optionDefault("count", "count", TypeInteger, 10, "Number of results."),
			optionDefault("search_engine", "search-engine", TypeString, "", "Search engine override."),
			optionDefault("search_recency_filter", "search-recency-filter", TypeString, "noLimit", "Recency filter."),
			optionDefault("search_domain_filter", "search-domain-filter", TypeString, "", "Domain filter."),
			optionDefault("content_size", "content-size", TypeString, "medium", "Returned content size."),
		}),

		providerCommand("ddg.search", "ddg", []string{"ddg", "search"}, "Search the web with DuckDuckGo.", []string{"source_search"}, []PositionalDefinition{positional("query", "Search query.", true)}, []OptionDefinition{
			optionDefault("max_results", "max-results", TypeInteger, 10, "Number of results."),
			optionDefault("region", "region", TypeString, "", "DuckDuckGo region."),
		}),
		providerCommand("ddg.fetch-content", "ddg", []string{"ddg", "fetch-content"}, "Fetch page content with DuckDuckGo.", []string{"page_fetch"}, []PositionalDefinition{positional("url", "Target URL.", true)}, []OptionDefinition{
			optionDefault("start_index", "start-index", TypeInteger, 0, "Starting content index."),
			optionDefault("max_length", "max-length", TypeInteger, 8000, "Maximum content length."),
			optionDefault("backend", "backend", TypeString, "auto", "Fetch backend."),
		}),

		providerCommand("freecrawl.search", "freecrawl", []string{"freecrawl", "search"}, "Search the web with Freecrawl.", []string{"source_search"}, []PositionalDefinition{positional("query", "Search query.", true)}, []OptionDefinition{
			optionDefault("num_results", "num-results", TypeInteger, 5, "Number of results."),
			optionDefault("search_engine", "search-engine", TypeString, "", "Search engine override."),
			optionDefault("scrape_results", "scrape-results", TypeBoolean, false, "Scrape result pages."),
		}),
		providerCommand("freecrawl.scrape", "freecrawl", []string{"freecrawl", "scrape"}, "Scrape a URL with Freecrawl.", []string{"page_fetch"}, []PositionalDefinition{positional("url", "Target URL.", true)}, []OptionDefinition{
			optionDefault("formats", "formats", TypeString, "markdown", "Comma-separated output formats."),
			optionDefault("javascript", "javascript", TypeBoolean, false, "Enable JavaScript rendering."),
			optionDefault("anti_bot", "anti-bot", TypeBoolean, false, "Enable anti-bot handling."),
			optionDefault("cache", "cache", TypeBoolean, false, "Enable cache."),
			optionDefault("timeout", "timeout", TypeInteger, 60000, "Timeout in milliseconds."),
			optionDefault("wait_for", "wait-for", TypeInteger, 0, "Wait time in milliseconds."),
		}),
		providerCommand("freecrawl.crawl", "freecrawl", []string{"freecrawl", "crawl"}, "Crawl a website with Freecrawl.", []string{"site_crawl"}, []PositionalDefinition{positional("url", "Website URL.", true)}, []OptionDefinition{
			optionDefault("max_depth", "max-depth", TypeInteger, 2, "Maximum traversal depth."),
			optionDefault("max_pages", "max-pages", TypeInteger, 20, "Maximum crawled pages."),
			optionDefault("same_domain_only", "same-domain-only", TypeBoolean, false, "Restrict crawl to the starting domain."),
			listOption("include_patterns", "include-patterns", "URL patterns to include."),
			listOption("exclude_patterns", "exclude-patterns", "URL patterns to exclude."),
		}),
		providerCommand("freecrawl.deep-research", "freecrawl", []string{"freecrawl", "deep-research"}, "Run Freecrawl deep research.", []string{"source_search"}, []PositionalDefinition{positional("topic", "Research topic.", true)}, []OptionDefinition{
			optionDefault("num_sources", "num-sources", TypeInteger, 8, "Number of sources."),
			optionDefault("max_depth", "max-depth", TypeInteger, 3, "Maximum research depth."),
			optionDefault("include_academic", "include-academic", TypeBoolean, false, "Include academic sources."),
			listOption("search_queries", "search-queries", "Additional search queries."),
		}),
	}

	for i := range definitions {
		switch definitions[i].ID {
		case "exa.web-fetch", "tavily.extract":
			definitions[i].Constraints = normalConstraints(ConstraintDefinition{Kind: "at_least_one", Members: []string{"url_args", "urls"}})
		case "anysearch.search":
			definitions[i].PreferredFor = []string{"vertical_search"}
		}
	}
	return definitions
}

func tavilyMapDefinition(crawl bool) CommandDefinition {
	name := "map"
	capability := "site_map"
	maxDepth := 1
	if crawl {
		name = "crawl"
		capability = "site_crawl"
		maxDepth = 2
	}
	options := []OptionDefinition{
		optionDefault("instructions", "instructions", TypeString, "", "Traversal instructions."),
		optionDefault("max_depth", "max-depth", TypeInteger, maxDepth, "Maximum traversal depth."),
		optionDefault("max_breadth", "max-breadth", TypeInteger, 20, "Maximum links per level."),
		optionDefault("limit", "limit", TypeInteger, 50, "Maximum results."),
		optionDefault("timeout", "timeout", TypeInteger, 150, "Timeout in seconds."),
		optionDefault("allow_external", "allow-external", TypeBoolean, false, "Allow external domains."),
		listOption("select_domains", "select-domains", "Domains to include."),
		listOption("select_paths", "select-paths", "Paths to include."),
		listOption("exclude_domains", "exclude-domains", "Domains to exclude."),
		listOption("exclude_paths", "exclude-paths", "Paths to exclude."),
	}
	extractOptions := []OptionDefinition{
		optionDefault("extract_depth", "extract-depth", TypeString, "", "Extraction depth."),
		optionDefault("extract_format", "extract-format", TypeString, "markdown", "Extracted content format."),
		optionDefault("include_images", "include-images", TypeBoolean, false, "Include images."),
		optionDefault("include_favicon", "include-favicon", TypeBoolean, false, "Include favicons."),
	}
	if !crawl {
		for index := range extractOptions {
			extractOptions[index].Deprecated = true
			extractOptions[index].Description = "Accepted for compatibility; Tavily map ignores this option."
		}
	}
	options = append(options, extractOptions...)
	return providerCommand("tavily."+name, "tavily", []string{"tavily", name}, "Run Tavily "+name+" on a website.", []string{capability}, []PositionalDefinition{positional("url", "Website URL.", true)}, options)
}

func providerNamespaces() []NamespaceDefinition {
	providers := []string{"exa", "tavily", "firecrawl", "context7", "deepwiki", "anysearch", "zhipu", "ddg", "freecrawl"}
	out := make([]NamespaceDefinition, 0, len(providers))
	for _, provider := range providers {
		out = append(out, NamespaceDefinition{Path: []string{provider}, Category: CategoryProvider, Visibility: VisibilityPublic, Summary: "Direct " + provider + " provider commands."})
	}
	return out
}
