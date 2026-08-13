package commandcontract

func workflowDefinitions() []CommandDefinition {
	return []CommandDefinition{
		{
			ID: "search", Path: []string{"search"}, Aliases: [][]string{{"s"}}, Category: CategoryWorkflow, Visibility: VisibilityPublic,
			Description:  "Search the web through configured capability routes.",
			Capabilities: []string{"answer_search", "docs_search", "source_search"}, PreferredFor: []string{"answer_search", "docs_search", "source_search"},
			Positionals: []PositionalDefinition{positional("query", "Search query.", true)},
			Options: withOutput(
				optionDefault("platform", "platform", TypeString, "", "Optional platform hint."),
				optionDefault("model", "model", TypeString, "", "Optional model override."),
				optionDefault("extra_sources", "extra-sources", TypeInteger, 0, "Additional source-discovery passes."),
				optionDefault("fetch_sources", "fetch-sources", TypeInteger, 0, "Number of discovered sources to fetch."),
				enumOption("validation", "validation", "", []string{"", "fast", "balanced", "strict"}, "Evidence validation level."),
				enumOption("fallback", "fallback", "", []string{"", "auto", "off"}, "Provider fallback policy."),
				optionDefault("providers", "providers", TypeString, "auto", "Global provider filter or capability-specific filters."),
				optionDefault("answer_providers", "answer-providers", TypeString, "", "Answer-search provider filter."),
				optionDefault("source_providers", "source-providers", TypeString, "", "Source-search provider filter."),
				optionDefault("docs_providers", "docs-providers", TypeString, "", "Documentation-search provider filter."),
				optionDefault("fetch_providers", "fetch-providers", TypeString, "", "Page-fetch provider filter."),
				optionDefault("repo_providers", "repo-providers", TypeString, "", "Repository-wiki provider filter."),
				optionDefault("repo_wiki", "repo-wiki", TypeString, "", "Repository identifier to enrich the search."),
				optionDefault("repo_wiki_mode", "repo-wiki-mode", TypeString, "", "Repository-wiki mode."),
				optionDefault("repo_wiki_query", "repo-wiki-query", TypeString, "", "Repository-wiki question override."),
				optionDefault("timeout", "timeout", TypeNumber, float64(90), "Timeout in seconds."),
				optionDefault("stream", "stream", TypeBoolean, false, "Enable answer streaming."),
				optionDefault("no_stream", "no-stream", TypeBoolean, false, "Disable answer streaming."),
			),
			Constraints:  normalConstraints(ConstraintDefinition{Kind: "mutually_exclusive", Members: []string{"stream", "no_stream"}}),
			Availability: capabilityAvailability("answer_search"), SideEffects: runtimeSideEffects("filesystem_write_when_output_is_set", "network"), Output: normalOutput("search_result"),
		},
		workflowSingleURL("fetch", "f", "Fetch a URL through the page-fetch route.", "page_fetch", []OptionDefinition{optionDefault("provider", "provider", TypeString, "auto", "Provider filter.")}, "fetch_result"),
		{
			ID: "map", Path: []string{"map"}, Aliases: [][]string{{"m"}}, Category: CategoryWorkflow, Visibility: VisibilityPublic,
			Description: "Map links from a website.", Capabilities: []string{"site_map"}, PreferredFor: []string{"site_map"},
			Positionals: []PositionalDefinition{positional("url", "Website URL.", true)},
			Options: withOutput(
				optionDefault("instructions", "instructions", TypeString, "", "Mapping instructions."),
				optionDefault("max_depth", "max-depth", TypeInteger, 1, "Maximum traversal depth."),
				optionDefault("max_breadth", "max-breadth", TypeInteger, 20, "Maximum links per level."),
				optionDefault("limit", "limit", TypeInteger, 50, "Maximum returned links."),
				optionDefault("timeout", "timeout", TypeInteger, 150, "Timeout in seconds."),
				optionDefault("provider", "provider", TypeString, "auto", "Provider filter."),
			),
			Constraints: normalConstraints(), Availability: capabilityAvailability("site_map"), SideEffects: runtimeSideEffects("filesystem_write_when_output_is_set", "network"), Output: normalOutput("map_result"),
		},
		{
			ID: "crawl", Path: []string{"crawl"}, Aliases: [][]string{{"cr"}}, Category: CategoryWorkflow, Visibility: VisibilityPublic,
			Description: "Crawl pages from a website.", Capabilities: []string{"site_crawl"}, PreferredFor: []string{"site_crawl"},
			Positionals: []PositionalDefinition{positional("url", "Website URL.", true)},
			Options: withOutput(
				optionDefault("max_depth", "max-depth", TypeInteger, 2, "Maximum traversal depth."),
				optionDefault("limit", "limit", TypeInteger, 20, "Maximum crawled pages."),
				optionDefault("timeout", "timeout", TypeInteger, 180, "Timeout in seconds."),
				optionDefault("provider", "provider", TypeString, "auto", "Provider filter."),
			),
			Constraints: normalConstraints(), Availability: capabilityAvailability("site_crawl"), SideEffects: runtimeSideEffects("filesystem_write_when_output_is_set", "network"), Output: normalOutput("crawl_result"),
		},
		{
			ID: "repo-wiki", Path: []string{"repo-wiki"}, Aliases: [][]string{{"rw"}}, Category: CategoryWorkflow, Visibility: VisibilityPublic,
			Description: "Read or query a repository wiki.", Capabilities: []string{"repo_wiki"}, PreferredFor: []string{"repo_wiki"},
			Positionals: []PositionalDefinition{positional("repo", "Repository in owner/name form.", true), positional("question", "Optional repository question.", false)},
			Options: withOutput(
				enumOption("mode", "mode", "", []string{"", "ask", "structure", "contents"}, "Repository-wiki operation."),
				optionDefault("provider", "provider", TypeString, "auto", "Provider filter."),
				optionDefault("timeout", "timeout", TypeNumber, float64(60), "Timeout in seconds."),
			),
			Constraints: normalConstraints(), Availability: capabilityAvailability("repo_wiki"), SideEffects: runtimeSideEffects("filesystem_write_when_output_is_set", "network"), Output: normalOutput("repo_wiki_result"),
		},
		{
			ID: "deep", Path: []string{"deep"}, Aliases: [][]string{{"dr"}}, Category: CategoryWorkflow, Visibility: VisibilityPublic,
			Description: "Create an offline deep-research execution plan.",
			Positionals: []PositionalDefinition{positional("query", "Research question.", true)},
			Options: withOutput(
				enumOption("budget", "budget", "standard", []string{"quick", "standard", "deep"}, "Research breadth and depth budget."),
				optionDefault("evidence_dir", "evidence-dir", TypeString, "", "Evidence output directory embedded in the plan."),
			),
			Constraints: normalConstraints(), Availability: localAvailability(), SideEffects: runtimeSideEffects("filesystem_write_when_output_is_set"), Output: normalOutput("deep_plan"),
		},
	}
}

func workflowSingleURL(id, alias, description, capability string, options []OptionDefinition, contract string) CommandDefinition {
	return CommandDefinition{
		ID: id, Path: []string{id}, Aliases: [][]string{{alias}}, Category: CategoryWorkflow, Visibility: VisibilityPublic,
		Description: description, Capabilities: []string{capability}, PreferredFor: []string{capability},
		Positionals: []PositionalDefinition{positional("url", "Target URL.", true)}, Options: withOutput(options...), Constraints: normalConstraints(),
		Availability: capabilityAvailability(capability), SideEffects: runtimeSideEffects("filesystem_write_when_output_is_set", "network"), Output: normalOutput(contract),
	}
}
