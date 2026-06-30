package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/deqiying/onesearch/internal/app"
	"github.com/deqiying/onesearch/internal/config"
	"github.com/deqiying/onesearch/internal/output"
	"github.com/deqiying/onesearch/internal/providers"
	"github.com/deqiying/onesearch/internal/service"
	"github.com/deqiying/onesearch/internal/skills"
)

var aliases = map[string]string{
	"s": "search", "f": "fetch", "m": "map",
	"cr":  "crawl",
	"rw":  "repo-wiki",
	"exa": "exa-search", "x": "exa-search", "xs": "exa-similar",
	"z": "zhipu-search", "zp": "zhipu-search",
	"as-domains": "anysearch-domains", "as-search": "anysearch-search", "as": "anysearch-search", "as-extract": "anysearch-extract", "as-batch": "anysearch-batch",
	"c7": "context7-library", "ctx7": "context7-library", "c7d": "context7-docs", "c7docs": "context7-docs", "ctx7-docs": "context7-docs",
	"dr": "deep", "sm": "smoke", "d": "doctor", "mdl": "model", "cfg": "config", "skill": "skills", "load-skill": "load_skill", "reg": "regression",
}

func Execute(args []string) int {
	if len(args) == 0 {
		printHelp()
		return 2
	}
	if args[0] == "--version" || args[0] == "-v" || args[0] == "--v" {
		fmt.Printf("%s %s\n", app.Name, app.Version)
		return 0
	}
	if args[0] == "--help" || args[0] == "-h" {
		printHelp()
		return 0
	}
	cfg := config.Load()
	svc := service.New(cfg)
	ctx := context.Background()
	if shouldDispatchProviderCommand(args[0], args[1:]) {
		return runProviderCommand(ctx, svc, args[0], args[1:])
	}
	command := canonical(args[0])
	switch command {
	case "search":
		return runSearch(ctx, svc, args[1:])
	case "fetch":
		return runFetch(ctx, svc, args[1:])
	case "map":
		return runMap(ctx, svc, args[1:])
	case "crawl":
		return runCrawl(ctx, svc, args[1:])
	case "repo-wiki":
		return runRepoWiki(ctx, svc, args[1:])
	case "exa-search":
		return runExaSearch(ctx, svc, args[1:])
	case "exa-similar":
		return runExaSimilar(ctx, svc, args[1:])
	case "zhipu-search":
		return runZhipu(ctx, svc, args[1:])
	case "anysearch-domains":
		return runAnyDomains(ctx, svc, args[1:])
	case "anysearch-search":
		return runAnySearch(ctx, svc, args[1:])
	case "anysearch-extract":
		return runAnyExtract(ctx, svc, args[1:])
	case "anysearch-batch":
		return runAnyBatch(ctx, svc, args[1:])
	case "context7-library":
		return runContext7Library(ctx, svc, args[1:])
	case "context7-docs":
		return runContext7Docs(ctx, svc, args[1:])
	case "deep":
		return runDeep(svc, args[1:])
	case "doctor":
		return printCommand("doctor", svc.Doctor(ctx), parseFormatOutput(args[1:], svc))
	case "smoke":
		return runSmoke(ctx, svc, args[1:])
	case "model":
		return runModel(svc, args[1:])
	case "config":
		return runConfig(svc, args[1:])
	case "skills":
		return runSkills(svc, args[1:])
	case "load_skill":
		return runLoadSkill(svc, args[1:])
	case "regression":
		return printCommand("smoke", svc.Smoke(ctx, "mock"), formatOutput{format: "json", verbosity: "quiet"})
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", args[0])
		return 2
	}
}

func runSearch(ctx context.Context, svc *service.Service, args []string) int {
	fs := flagSet("search")
	platform := fs.String("platform", "", "")
	model := fs.String("model", "", "")
	extra := fs.Int("extra-sources", 0, "")
	fetchSources := fs.Int("fetch-sources", 0, "")
	validation := fs.String("validation", "", "")
	fallback := fs.String("fallback", "", "")
	providerFilter := fs.String("providers", "auto", "")
	answerProviders := fs.String("answer-providers", "", "")
	sourceProviders := fs.String("source-providers", "", "")
	docsProviders := fs.String("docs-providers", "", "")
	fetchProviders := fs.String("fetch-providers", "", "")
	repoProviders := fs.String("repo-providers", "", "")
	repoWiki := fs.String("repo-wiki", "", "")
	repoWikiMode := fs.String("repo-wiki-mode", "", "")
	repoWikiQuery := fs.String("repo-wiki-query", "", "")
	timeoutSeconds := fs.Float64("timeout", 90, "")
	streamFlag := boolFlag{}
	noStreamFlag := boolFlag{}
	fs.Var(&streamFlag, "stream", "")
	fs.Var(&noStreamFlag, "no-stream", "")
	outputFlags := addOutputFlags(fs)
	if err := parse(fs, args); err != nil {
		return printParameterError("search", err.Error(), makeFormatOutput(outputFlags, svc))
	}
	if fs.NArg() < 1 {
		return printParameterError("search", "search requires query", makeFormatOutput(outputFlags, svc))
	}
	providers, providerFilters, err := parseSearchProviderFilters(*providerFilter)
	if err != nil {
		return printParameterError("search", err.Error(), makeFormatOutput(outputFlags, svc))
	}
	providerFilters = overlayProviderFilter(providerFilters, "answer_search", *answerProviders)
	providerFilters = overlayProviderFilter(providerFilters, "source_search", *sourceProviders)
	providerFilters = overlayProviderFilter(providerFilters, "docs_search", *docsProviders)
	providerFilters = overlayProviderFilter(providerFilters, "page_fetch", *fetchProviders)
	providerFilters = overlayProviderFilter(providerFilters, "repo_wiki", *repoProviders)
	query := fs.Arg(0)
	var stream *bool
	if streamFlag.set {
		value := true
		stream = &value
	}
	if noStreamFlag.set {
		value := false
		stream = &value
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(*timeoutSeconds*float64(time.Second)))
	defer cancel()
	data := svc.Search(ctx, query, service.SearchOptions{Platform: *platform, Model: *model, ExtraSources: *extra, FetchSources: *fetchSources, Validation: *validation, Fallback: *fallback, Providers: providers, ProviderFilters: providerFilters, Stream: stream, RepoWiki: *repoWiki, RepoWikiMode: *repoWikiMode, RepoWikiQuery: *repoWikiQuery})
	if ctx.Err() == context.DeadlineExceeded {
		data = map[string]any{"ok": false, "error_type": "network_error", "error": fmt.Sprintf("Search timed out after %g seconds", *timeoutSeconds), "query": query, "content": "", "sources": []map[string]any{}, "sources_count": 0, "timeout_seconds": *timeoutSeconds}
	}
	return printCommand("search", data, makeFormatOutput(outputFlags, svc))
}

func runFetch(ctx context.Context, svc *service.Service, args []string) int {
	fs := flagSet("fetch")
	outputFlags := addOutputFlags(fs)
	if err := parse(fs, args); err != nil {
		return printParameterError("fetch", err.Error(), makeFormatOutput(outputFlags, svc))
	}
	if fs.NArg() < 1 {
		return printParameterError("fetch", "fetch requires url", makeFormatOutput(outputFlags, svc))
	}
	return printCommand("fetch", svc.Fetch(ctx, fs.Arg(0)), makeFormatOutput(outputFlags, svc))
}

func runMap(ctx context.Context, svc *service.Service, args []string) int {
	fs := flagSet("map")
	instructions := fs.String("instructions", "", "")
	maxDepth := fs.Int("max-depth", 1, "")
	maxBreadth := fs.Int("max-breadth", 20, "")
	limit := fs.Int("limit", 50, "")
	timeoutSeconds := fs.Int("timeout", 150, "")
	outputFlags := addOutputFlags(fs)
	if err := parse(fs, args); err != nil {
		return printParameterError("map", err.Error(), makeFormatOutput(outputFlags, svc))
	}
	if fs.NArg() < 1 {
		return printParameterError("map", "map requires url", makeFormatOutput(outputFlags, svc))
	}
	data := svc.Map(ctx, fs.Arg(0), service.MapOptions{Instructions: *instructions, MaxDepth: *maxDepth, MaxBreadth: *maxBreadth, Limit: *limit, Timeout: *timeoutSeconds})
	return printCommand("map", data, makeFormatOutput(outputFlags, svc))
}

func runCrawl(ctx context.Context, svc *service.Service, args []string) int {
	fs := flagSet("crawl")
	maxDepth := fs.Int("max-depth", 2, "")
	limit := fs.Int("limit", 20, "")
	timeoutSeconds := fs.Int("timeout", 180, "")
	outputFlags := addOutputFlags(fs)
	if err := parse(fs, args); err != nil {
		return printParameterError("crawl", err.Error(), makeFormatOutput(outputFlags, svc))
	}
	if fs.NArg() < 1 {
		return printParameterError("crawl", "crawl requires url", makeFormatOutput(outputFlags, svc))
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(*timeoutSeconds)*time.Second)
	defer cancel()
	data := svc.Crawl(ctx, fs.Arg(0), service.CrawlOptions{MaxDepth: *maxDepth, Limit: *limit, Timeout: *timeoutSeconds})
	if ctx.Err() == context.DeadlineExceeded {
		data = map[string]any{"ok": false, "error_type": "network_error", "error": fmt.Sprintf("Crawl timed out after %d seconds", *timeoutSeconds), "url": fs.Arg(0), "timeout_seconds": *timeoutSeconds}
	}
	return printCommand("crawl", data, makeFormatOutput(outputFlags, svc))
}

func runRepoWiki(ctx context.Context, svc *service.Service, args []string) int {
	fs := flagSet("repo-wiki")
	mode := fs.String("mode", "", "")
	timeoutSeconds := fs.Float64("timeout", 60, "")
	outputFlags := addOutputFlags(fs)
	if err := parse(fs, args); err != nil {
		return printParameterError("repo-wiki", err.Error(), makeFormatOutput(outputFlags, svc))
	}
	if fs.NArg() < 1 {
		return printParameterError("repo-wiki", "repo-wiki requires repo", makeFormatOutput(outputFlags, svc))
	}
	question := ""
	if fs.NArg() > 1 {
		question = fs.Arg(1)
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(*timeoutSeconds*float64(time.Second)))
	defer cancel()
	data := svc.RepoWiki(ctx, fs.Arg(0), question, *mode)
	if ctx.Err() == context.DeadlineExceeded {
		data = map[string]any{"ok": false, "error_type": "network_error", "error": fmt.Sprintf("Repo wiki timed out after %g seconds", *timeoutSeconds), "repo": fs.Arg(0), "timeout_seconds": *timeoutSeconds}
	}
	return printCommand("repo-wiki", data, makeFormatOutput(outputFlags, svc))
}

func runExaSearch(ctx context.Context, svc *service.Service, args []string) int {
	fs := flagSet("exa-search")
	numResults := fs.Int("num-results", 5, "")
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
		return printParameterError("exa-search", err.Error(), makeFormatOutput(outputFlags, svc))
	}
	if fs.NArg() < 1 {
		return printParameterError("exa-search", "exa-search requires query", makeFormatOutput(outputFlags, svc))
	}
	data := svc.ExaSearch(ctx, fs.Arg(0), providers.ExaOptions{NumResults: *numResults, SearchType: *searchType, IncludeText: *includeText, IncludeHighlights: *includeHighlights, StartPublishedDate: *startDate, IncludeDomains: includeDomains.values, ExcludeDomains: excludeDomains.values, Category: *category})
	return printCommand("exa-search", data, makeFormatOutput(outputFlags, svc))
}

func runExaSimilar(ctx context.Context, svc *service.Service, args []string) int {
	fs := flagSet("exa-similar")
	numResults := fs.Int("num-results", 5, "")
	outputFlags := addOutputFlags(fs)
	if err := parse(fs, args); err != nil {
		return printParameterError("exa-similar", err.Error(), makeFormatOutput(outputFlags, svc))
	}
	if fs.NArg() < 1 {
		return printParameterError("exa-similar", "exa-similar requires url", makeFormatOutput(outputFlags, svc))
	}
	return printCommand("exa-similar", svc.ExaSimilar(ctx, fs.Arg(0), *numResults), makeFormatOutput(outputFlags, svc))
}

func runZhipu(ctx context.Context, svc *service.Service, args []string) int {
	fs := flagSet("zhipu-search")
	count := fs.Int("count", 10, "")
	engine := fs.String("search-engine", "", "")
	recency := fs.String("search-recency-filter", "noLimit", "")
	domain := fs.String("search-domain-filter", "", "")
	contentSize := fs.String("content-size", "medium", "")
	outputFlags := addOutputFlags(fs)
	if err := parse(fs, args); err != nil {
		return printParameterError("zhipu-search", err.Error(), makeFormatOutput(outputFlags, svc))
	}
	if fs.NArg() < 1 {
		return printParameterError("zhipu-search", "zhipu-search requires query", makeFormatOutput(outputFlags, svc))
	}
	data := svc.ZhipuSearch(ctx, fs.Arg(0), providers.ZhipuOptions{Count: *count, SearchEngine: *engine, SearchRecencyFilter: *recency, SearchDomainFilter: *domain, ContentSize: *contentSize})
	return printCommand("zhipu-search", data, makeFormatOutput(outputFlags, svc))
}

func runAnyDomains(ctx context.Context, svc *service.Service, args []string) int {
	fs := flagSet("anysearch-domains")
	outputFlags := addOutputFlags(fs)
	if err := parse(fs, args); err != nil {
		return printParameterError("anysearch-domains", err.Error(), makeFormatOutput(outputFlags, svc))
	}
	domain := ""
	if fs.NArg() > 0 {
		domain = fs.Arg(0)
	}
	return printCommand("anysearch-domains", svc.AnySearch().Domains(ctx, domain), makeFormatOutput(outputFlags, svc))
}

func runAnySearch(ctx context.Context, svc *service.Service, args []string) int {
	fs := flagSet("anysearch-search")
	domain := fs.String("domain", "", "")
	subDomain := fs.String("sub-domain", "", "")
	maxResults := fs.Int("max-results", 5, "")
	outputFlags := addOutputFlags(fs)
	if err := parse(fs, args); err != nil {
		return printParameterError("anysearch-search", err.Error(), makeFormatOutput(outputFlags, svc))
	}
	if fs.NArg() < 1 {
		return printParameterError("anysearch-search", "anysearch-search requires query", makeFormatOutput(outputFlags, svc))
	}
	return printCommand("anysearch-search", svc.AnySearch().Search(ctx, fs.Arg(0), *domain, *subDomain, *maxResults), makeFormatOutput(outputFlags, svc))
}

func runAnyExtract(ctx context.Context, svc *service.Service, args []string) int {
	fs := flagSet("anysearch-extract")
	maxLength := fs.Int("max-length", 20000, "")
	outputFlags := addOutputFlags(fs)
	if err := parse(fs, args); err != nil {
		return printParameterError("anysearch-extract", err.Error(), makeFormatOutput(outputFlags, svc))
	}
	if fs.NArg() < 1 {
		return printParameterError("anysearch-extract", "anysearch-extract requires url", makeFormatOutput(outputFlags, svc))
	}
	return printCommand("anysearch-extract", svc.AnySearch().Extract(ctx, fs.Arg(0), *maxLength), makeFormatOutput(outputFlags, svc))
}

func runAnyBatch(ctx context.Context, svc *service.Service, args []string) int {
	fs := flagSet("anysearch-batch")
	maxResults := fs.Int("max-results", 3, "")
	outputFlags := addOutputFlags(fs)
	if err := parse(fs, args); err != nil {
		return printParameterError("anysearch-batch", err.Error(), makeFormatOutput(outputFlags, svc))
	}
	if fs.NArg() < 1 {
		return printParameterError("anysearch-batch", "anysearch-batch requires at least one query", makeFormatOutput(outputFlags, svc))
	}
	return printCommand("anysearch-batch", svc.AnySearch().Batch(ctx, fs.Args(), *maxResults), makeFormatOutput(outputFlags, svc))
}

func runContext7Library(ctx context.Context, svc *service.Service, args []string) int {
	fs := flagSet("context7-library")
	outputFlags := addOutputFlags(fs)
	if err := parse(fs, args); err != nil {
		return printParameterError("context7-library", err.Error(), makeFormatOutput(outputFlags, svc))
	}
	if fs.NArg() < 1 {
		return printParameterError("context7-library", "context7-library requires name", makeFormatOutput(outputFlags, svc))
	}
	query := ""
	if fs.NArg() > 1 {
		query = fs.Arg(1)
	}
	return printCommand("context7-library", svc.Context7Library(ctx, fs.Arg(0), query), makeFormatOutput(outputFlags, svc))
}

func runContext7Docs(ctx context.Context, svc *service.Service, args []string) int {
	fs := flagSet("context7-docs")
	outputFlags := addOutputFlags(fs)
	if err := parse(fs, args); err != nil {
		return printParameterError("context7-docs", err.Error(), makeFormatOutput(outputFlags, svc))
	}
	if fs.NArg() < 2 {
		return printParameterError("context7-docs", "context7-docs requires library_id and query", makeFormatOutput(outputFlags, svc))
	}
	return printCommand("context7-docs", svc.Context7Docs(ctx, fs.Arg(0), fs.Arg(1)), makeFormatOutput(outputFlags, svc))
}

func runDeep(svc *service.Service, args []string) int {
	fs := flagSet("deep")
	budget := fs.String("budget", "standard", "")
	evidenceDir := fs.String("evidence-dir", "", "")
	outputFlags := addOutputFlags(fs)
	if err := parse(fs, args); err != nil {
		return printParameterError("deep", err.Error(), makeFormatOutput(outputFlags, svc))
	}
	if fs.NArg() < 1 {
		return printParameterError("deep", "deep requires query", makeFormatOutput(outputFlags, svc))
	}
	return printCommand("deep", svc.DeepPlan(fs.Arg(0), *budget, *evidenceDir), makeFormatOutput(outputFlags, svc))
}

func runSmoke(ctx context.Context, svc *service.Service, args []string) int {
	fs := flagSet("smoke")
	mode := fs.String("mode", "mock", "")
	mock := fs.Bool("mock", false, "")
	live := fs.Bool("live", false, "")
	outputFlags := addOutputFlags(fs)
	if err := parse(fs, args); err != nil {
		return printParameterError("smoke", err.Error(), makeFormatOutput(outputFlags, svc))
	}
	if *mock {
		*mode = "mock"
	}
	if *live {
		*mode = "live"
	}
	return printCommand("smoke", svc.Smoke(ctx, *mode), makeFormatOutput(outputFlags, svc))
}

func runModel(svc *service.Service, args []string) int {
	if len(args) == 0 {
		return parameterError("model requires subcommand")
	}
	sub := canonicalModel(args[0])
	fs := flagSet("model")
	outputFlags := addOutputFlags(fs)
	if err := parse(fs, args[1:]); err != nil {
		return printParameterError("model", err.Error(), makeFormatOutput(outputFlags, svc))
	}
	if sub == "current" {
		return printCommand("model", svc.CurrentModel(), makeFormatOutput(outputFlags, svc))
	}
	return parameterError("unknown model subcommand")
}

func runConfig(svc *service.Service, args []string) int {
	if len(args) == 0 {
		return parameterError("config requires subcommand")
	}
	sub := canonicalConfig(args[0])
	fs := flagSet("config")
	outputFlags := addOutputFlags(fs)
	if err := parse(fs, args[1:]); err != nil {
		return printParameterError("config", err.Error(), makeFormatOutput(outputFlags, svc))
	}
	var data map[string]any
	switch sub {
	case "path":
		data = svc.ConfigPath()
	case "list":
		data = svc.ConfigList(false)
	default:
		return parameterError("unknown config subcommand")
	}
	return printCommand("config", data, makeFormatOutput(outputFlags, svc))
}

func runLoadSkill(svc *service.Service, args []string) int {
	if len(args) < 1 {
		return parameterError("load_skill requires skill name")
	}
	if canonicalSkillsSubcommand(args[0]) == "list" {
		return runSkills(svc, args)
	}
	text, err := skills.ReadMarkdown(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	fmt.Print(text)
	if !strings.HasSuffix(text, "\n") {
		fmt.Println()
	}
	return 0
}

func parseSearchProviderFilters(raw string) (string, map[string]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.EqualFold(raw, "auto") {
		return "auto", nil, nil
	}
	if !strings.Contains(raw, "=") && !strings.Contains(raw, ":") {
		return raw, nil, nil
	}
	filters := map[string]string{}
	for _, part := range strings.Split(raw, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		idx := strings.Index(part, "=")
		if idx < 0 {
			idx = strings.Index(part, ":")
		}
		if idx < 0 {
			return "", nil, fmt.Errorf("provider filter %q must use capability=providers", part)
		}
		capability := normalizeCapabilityFilterKey(strings.TrimSpace(part[:idx]))
		value := strings.TrimSpace(part[idx+1:])
		if capability == "" || value == "" {
			return "", nil, fmt.Errorf("provider filter %q must include capability and provider list", part)
		}
		filters[capability] = value
	}
	if len(filters) == 0 {
		return "auto", nil, nil
	}
	return "auto", filters, nil
}

func overlayProviderFilter(filters map[string]string, capability, value string) map[string]string {
	value = strings.TrimSpace(value)
	if value == "" {
		return filters
	}
	if filters == nil {
		filters = map[string]string{}
	}
	filters[config.V2CapabilityName(capability)] = value
	return filters
}

func normalizeCapabilityFilterKey(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "answer", "answer_search", "search":
		return "answer_search"
	case "source", "sources", "source_search":
		return "source_search"
	case "docs", "doc", "documentation", "docs_search":
		return "docs_search"
	case "fetch", "page", "page_fetch":
		return "page_fetch"
	case "repo", "repo_wiki", "repository", "repository_wiki":
		return "repo_wiki"
	case "site_map", "map":
		return "site_map"
	case "site_crawl", "crawl":
		return "site_crawl"
	case "vertical", "vertical_search":
		return "vertical_search"
	default:
		return config.V2CapabilityName(value)
	}
}

type formatOutput struct {
	format    string
	output    string
	verbosity string
}

type outputFlags struct {
	format  *string
	output  *string
	verbose *bool
	quiet   *bool
}

func parseFormatOutput(args []string, svc *service.Service) formatOutput {
	fs := flagSet("format")
	flags := addOutputFlags(fs)
	_ = parse(fs, args)
	return makeFormatOutput(flags, svc)
}

func printCommand(command string, data map[string]any, fo formatOutput) int {
	if fo.format == "" {
		fo.format = "json"
	}
	rendered := output.RenderWithOptions(command, data, output.Options{Format: fo.format, Verbosity: fo.verbosity})
	if err := output.Write(fo.output, rendered); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 5
	}
	fmt.Print(rendered)
	return output.ExitCode(data)
}

func printParameterError(command, message string, fo formatOutput) int {
	return printCommand(command, map[string]any{
		"ok":         false,
		"error_type": "parameter_error",
		"error":      message,
	}, fo)
}

func addFormatFlags(fs *flag.FlagSet) (*string, *string) {
	flags := addOutputFlagsWithDefault(fs, "json")
	return flags.format, flags.output
}

func addFormatFlagsWithDefault(fs *flag.FlagSet, defaultFormat string) (*string, *string) {
	flags := addOutputFlagsWithDefault(fs, defaultFormat)
	return flags.format, flags.output
}

func addOutputFlags(fs *flag.FlagSet) outputFlags {
	return addOutputFlagsWithDefault(fs, "json")
}

func addOutputFlagsWithDefault(fs *flag.FlagSet, defaultFormat string) outputFlags {
	return outputFlags{
		format:  fs.String("format", defaultFormat, ""),
		output:  fs.String("output", "", ""),
		verbose: fs.Bool("verbose", false, ""),
		quiet:   fs.Bool("quiet", false, ""),
	}
}

func makeFormatOutput(flags outputFlags, svc *service.Service) formatOutput {
	verbosity := defaultVerbosity(svc)
	if flags.verbose != nil && *flags.verbose {
		verbosity = "verbose"
	}
	if flags.quiet != nil && *flags.quiet {
		verbosity = "quiet"
	}
	return formatOutput{format: *flags.format, output: *flags.output, verbosity: verbosity}
}

func defaultVerbosity(svc *service.Service) string {
	if svc != nil {
		data := svc.ConfigList(false)
		defaults, _ := data["defaults"].(map[string]any)
		if strings.EqualFold(strings.TrimSpace(fmt.Sprint(defaults["log_level"])), "debug") {
			return "verbose"
		}
	}
	return "quiet"
}

func flagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

func parse(fs *flag.FlagSet, args []string) error {
	return fs.Parse(reorderFlags(fs, args))
}

func reorderFlags(fs *flag.FlagSet, args []string) []string {
	var flags []string
	var positionals []string
	for i := 0; i < len(args); i++ {
		token := args[i]
		if token == "--" {
			positionals = append(positionals, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(token, "-") || token == "-" {
			positionals = append(positionals, token)
			continue
		}
		name := strings.TrimLeft(token, "-")
		if idx := strings.IndexByte(name, '='); idx >= 0 {
			if fs.Lookup(name[:idx]) != nil {
				flags = append(flags, token)
				continue
			}
		}
		f := fs.Lookup(name)
		if f == nil {
			positionals = append(positionals, token)
			continue
		}
		flags = append(flags, token)
		if isBool, ok := f.Value.(interface{ IsBoolFlag() bool }); ok && isBool.IsBoolFlag() {
			continue
		}
		if _, ok := f.Value.(*stringListFlag); ok {
			for i+1 < len(args) && !looksLikeKnownFlag(fs, args[i+1]) {
				i++
				flags = append(flags, args[i])
			}
			continue
		}
		if i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}
	return append(flags, positionals...)
}

func looksLikeKnownFlag(fs *flag.FlagSet, token string) bool {
	if !strings.HasPrefix(token, "-") || token == "-" {
		return false
	}
	name := strings.TrimLeft(token, "-")
	if idx := strings.IndexByte(name, '='); idx >= 0 {
		name = name[:idx]
	}
	return fs.Lookup(name) != nil
}

type boolFlag struct {
	set bool
}

func (b *boolFlag) String() string { return fmt.Sprint(b.set) }
func (b *boolFlag) Set(_ string) error {
	b.set = true
	return nil
}
func (b *boolFlag) IsBoolFlag() bool { return true }

type stringListFlag struct {
	values []string
}

func (s *stringListFlag) String() string { return strings.Join(s.values, ",") }
func (s *stringListFlag) Set(value string) error {
	for _, part := range strings.Split(value, ",") {
		for _, token := range strings.Fields(part) {
			if token != "" {
				s.values = append(s.values, token)
			}
		}
	}
	return nil
}

func canonical(command string) string {
	if value, ok := aliases[command]; ok {
		return value
	}
	return command
}

func canonicalConfig(command string) string {
	switch command {
	case "p":
		return "path"
	case "ls", "l":
		return "list"
	default:
		return command
	}
}

func canonicalModel(command string) string {
	switch command {
	case "cur", "c":
		return "current"
	default:
		return command
	}
}

func parameterError(message string) int {
	fmt.Fprintln(os.Stderr, message)
	return 2
}

func printHelp() {
	fmt.Println(app.Description)
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  onesearch <command> [args] [--format json|markdown|content] [--quiet|--verbose]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  search, fetch, map, crawl, repo-wiki, exa-search, exa-similar, zhipu-search")
	fmt.Println("  context7-library, context7-docs, anysearch-*, deep, doctor, smoke")
	fmt.Println("  config, model, skills, load_skill, regression")
}
