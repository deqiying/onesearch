package service

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/deqiying/onesearch/internal/commandcontract"
	"github.com/deqiying/onesearch/internal/config"
	"github.com/deqiying/onesearch/internal/providers"
	"github.com/deqiying/onesearch/internal/sources"
)

const sourceWarning = "extra_sources are retrieved in parallel and are not automatically used to verify generated content; use fetch on key URLs for claim-level evidence."

var serviceCommandRegistry = commandcontract.MustDefaultRegistry()

type Service struct {
	Config *config.Config
}

type SearchOptions struct {
	Platform        string
	Model           string
	ExtraSources    int
	FetchSources    int
	Validation      string
	Fallback        string
	Providers       string
	ProviderFilters map[string]string
	Stream          *bool
	RepoWiki        string
	RepoWikiMode    string
	RepoWikiQuery   string
}

func (o SearchOptions) providerFilter(capability string) string {
	capability = config.V2CapabilityName(capability)
	if o.ProviderFilters != nil {
		if value := strings.TrimSpace(o.ProviderFilters[capability]); value != "" {
			return value
		}
	}
	if !usesLegacySearchProviderFilter(capability) {
		return "auto"
	}
	return valueOr(o.Providers, "auto")
}

func usesLegacySearchProviderFilter(capability string) bool {
	switch capability {
	case "answer_search", "source_search", "docs_search":
		return true
	default:
		return false
	}
}

func (o SearchOptions) providerFiltersForDiagnostics() map[string]any {
	out := map[string]any{}
	for key, value := range o.ProviderFilters {
		key = config.V2CapabilityName(key)
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		out[key] = strings.TrimSpace(value)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

type MapOptions struct {
	Instructions string
	MaxDepth     int
	MaxBreadth   int
	Limit        int
	Timeout      int
	Provider     string
}

type CrawlOptions struct {
	MaxDepth int
	Limit    int
	Timeout  int
	Provider string
}

type FetchOptions struct {
	Provider string
}

type RepoWikiOptions struct {
	Mode     string
	Provider string
}

func New(cfg *config.Config) *Service {
	return &Service{Config: cfg}
}

func (s *Service) Search(ctx context.Context, query string, options SearchOptions) map[string]any {
	start := time.Now()
	sessionID := sources.NewSessionID()
	validation, err := s.validation(options.Validation)
	if err != nil {
		return emptySearch(start, sessionID, query, "parameter_error", err.Error())
	}
	fallback, err := s.fallback(options.Fallback)
	if err != nil {
		return emptySearch(start, sessionID, query, "parameter_error", err.Error())
	}
	if strings.TrimSpace(options.RepoWiki) != "" {
		if _, err := normalizeRepoName(options.RepoWiki); err != nil {
			return emptySearch(start, sessionID, query, "parameter_error", err.Error())
		}
		if _, err := validateRepoWikiMode(options.RepoWikiMode); err != nil {
			return emptySearch(start, sessionID, query, "parameter_error", err.Error())
		}
	}
	minimum := s.ValidateMinimumProfile()
	if !truthy(minimum["ok"]) {
		out := emptySearch(start, sessionID, query, stringValue(minimum["error_type"]), stringValue(minimum["error"]))
		out["diagnostics"] = searchDiagnostics(minimum, nil, nil)
		out["validation_level"] = validation
		return out
	}
	mainConfigs, err := s.mainProviderConfigs(options.Model, options.providerFilter("answer_search"), options.Stream)
	if err != nil {
		out := emptySearch(start, sessionID, query, "parameter_error", err.Error())
		out["validation_level"] = validation
		return out
	}
	if len(mainConfigs) == 0 {
		out := emptySearch(start, sessionID, query, "config_error", "No configured answer_search provider matches --providers.")
		out["validation_level"] = validation
		out["diagnostics"] = searchDiagnostics(minimum, nil, nil)
		return out
	}
	selected := mainConfigs
	if fallback == "off" && len(selected) > 1 {
		selected = selected[:1]
	}
	docsIntent := isDocsIntent(query)
	zhCurrentIntent := isZHCurrentIntent(query)
	fetchIntent := isFetchIntent(query)
	supplemental := []string{}
	if docsIntent {
		supplemental = append(supplemental, "docs_search")
	}
	if zhCurrentIntent || validation == "strict" {
		supplemental = append(supplemental, "source_search")
	}
	if fetchIntent {
		supplemental = append(supplemental, "page_fetch")
	}
	if strings.TrimSpace(options.RepoWiki) != "" {
		supplemental = append(supplemental, "repo_wiki")
	}
	routing := map[string]any{
		"docs_intent":              docsIntent,
		"zh_current_intent":        zhCurrentIntent,
		"web_current_intent":       zhCurrentIntent,
		"fetch_intent":             fetchIntent,
		"supplemental_paths":       supplemental,
		"validation_level":         validation,
		"fallback_mode":            fallback,
		"providers":                valueOr(options.Providers, "auto"),
		"provider_filters":         options.providerFiltersForDiagnostics(),
		"answer_search_chain":      providerNames(selected),
		"openai_compatible_stream": openAIStream(selected),
	}

	var attempts []providers.Attempt
	var primaryResult string
	var success mainProviderConfig
	var answerAttempt providers.Attempt
	var lastError map[string]any
	for _, cfg := range selected {
		attemptStart := time.Now()
		text, err := s.callMainProvider(ctx, cfg, query, options.Platform)
		if err == nil && strings.TrimSpace(text) != "" {
			primaryResult = text
			success = cfg
			answerAttempt = attempt("answer_search", cfg.DisplayName, "ok", attemptStart, 1, "", "")
			attempts = append(attempts, answerAttempt)
			break
		}
		if err != nil {
			errType, msg := providers.ErrorPayload(err)
			lastError = primarySearchError(start, sessionID, query, cfg.Mode, errType, cfg.DisplayName+" "+msg)
			attempts = append(attempts, attempt("answer_search", cfg.DisplayName, "error", attemptStart, 0, errType, msg))
		} else {
			lastError = primarySearchError(start, sessionID, query, cfg.Mode, "network_error", cfg.DisplayName+" 返回空结果")
			attempts = append(attempts, attempt("answer_search", cfg.DisplayName, "empty", attemptStart, 0, "", ""))
		}
	}
	if strings.TrimSpace(primaryResult) == "" {
		if lastError == nil {
			lastError = primarySearchError(start, sessionID, query, mainConfigs[0].Mode, "network_error", "搜索失败或无结果")
		}
		lastError["provider_attempts"] = attemptsToMaps(attempts)
		lastError["providers_used"] = providerNamesFromAttempts(attempts)
		lastError["fallback_used"] = fallbackUsed(attempts)
		lastError["routing_decision"] = routing
		lastError["validation_level"] = validation
		lastError["diagnostics"] = searchDiagnostics(minimum, routing, attemptsToMaps(attempts))
		return lastError
	}

	answer, primarySources := sources.SplitAnswerAndSources(primaryResult)
	answerSources := tagResultSources(primarySources, "answer_search", success.Provider)
	used := []map[string]any{searchUsedCapability("answer_search", "primary_answer", providerResult(success.Provider, answerAttempt, map[string]any{
		"content":       answer,
		"sources":       answerSources,
		"sources_count": len(answerSources),
	}, map[string]any{
		"mode":  success.Mode,
		"model": success.Model,
	}))}
	extraSources, extraAttempts := s.extraSources(ctx, query, options.ExtraSources, options.providerFilter("source_search"))
	attempts = append(attempts, extraAttempts...)
	if len(extraSources) > 0 {
		used = append(used, usedCapabilitiesFromSourceAttempts("source_search", "extra_sources", extraSources, extraAttempts)...)
	}
	var supplementalSources []map[string]any
	var sourceSearchSources []map[string]any
	if validation == "balanced" || validation == "strict" {
		if docsIntent {
			found, docsAttempts := s.runDocsSearchFallback(ctx, query, options.providerFilter("docs_search"), fallback)
			attempts = append(attempts, docsAttempts...)
			supplementalSources = append(supplementalSources, found...)
			if len(found) > 0 {
				used = append(used, usedCapabilitiesFromSourceAttempts("docs_search", "documentation_sources", found, docsAttempts)...)
			}
		}
		if zhCurrentIntent || validation == "strict" {
			found, webAttempts := s.runWebSearchFallback(ctx, query, max(1, valueOrInt(options.ExtraSources, 3)), options.providerFilter("source_search"), fallback)
			attempts = append(attempts, webAttempts...)
			supplementalSources = append(supplementalSources, found...)
			sourceSearchSources = append(sourceSearchSources, found...)
			if len(found) > 0 {
				used = append(used, usedCapabilitiesFromSourceAttempts("source_search", "current_sources", found, webAttempts)...)
			}
		}
		if fetchIntent {
			found, fetchAttempts := s.runWebFetchFallback(ctx, strings.TrimSpace(query), options.providerFilter("page_fetch"), fallback)
			attempts = append(attempts, fetchAttempts...)
			if truthy(found["ok"]) {
				supplementalSources = append(supplementalSources, map[string]any{"url": found["url"], "provider": found["provider"], "description": truncate(stringValue(found["content"]), 300)})
				used = append(used, usedCapabilityFromFetch(found, fetchAttempts))
			}
		}
	}
	if options.FetchSources > 0 {
		fetched, fetchAttempts := s.fetchSourceCandidates(ctx, sources.Merge(sourceSearchSources, extraSources), options.FetchSources, options.providerFilter("page_fetch"), fallback)
		attempts = append(attempts, fetchAttempts...)
		if len(fetched) > 0 {
			used = append(used, usedCapabilityFromFetchedSources(fetched, fetchAttempts))
			for _, item := range fetched {
				supplementalSources = append(supplementalSources, map[string]any{"url": item["url"], "provider": item["provider"], "description": stringValue(item["content_preview"])})
			}
		}
	}
	if strings.TrimSpace(options.RepoWiki) != "" {
		repoQuery := firstNonEmpty(strings.TrimSpace(options.RepoWikiQuery), query)
		found, repoAttempts := s.runRepoWikiFallback(ctx, options.RepoWiki, repoQuery, options.RepoWikiMode, options.providerFilter("repo_wiki"), fallback)
		attempts = append(attempts, repoAttempts...)
		if truthy(found["ok"]) {
			used = append(used, usedCapabilityFromRepoWiki(found, repoAttempts))
		}
	}
	extraSources = sources.Merge(extraSources, supplementalSources)
	mergedSources := sources.Merge(primarySources, extraSources)
	ok := strings.TrimSpace(answer) != "" || len(mergedSources) > 0
	if validation == "strict" && len(mergedSources) == 0 {
		ok = false
	}
	elapsed := providers.Elapsed(start)
	meta := map[string]any{
		"session_id":       sessionID,
		"validation_level": validation,
		"elapsed_ms":       elapsed,
		"fallback_used":    fallbackUsed(attempts),
	}
	return map[string]any{
		"ok":                    ok,
		"error_type":            map[bool]string{true: "", false: map[bool]string{true: "evidence_error", false: "network_error"}[validation == "strict"]}[ok],
		"error":                 map[bool]string{true: "", false: map[bool]string{true: "strict 模式证据不足", false: "搜索失败或无结果"}[validation == "strict"]}[ok],
		"session_id":            sessionID,
		"query":                 query,
		"platform":              options.Platform,
		"model":                 success.Model,
		"content":               answer,
		"sources":               mergedSources,
		"sources_count":         len(mergedSources),
		"primary_sources":       primarySources,
		"primary_sources_count": len(primarySources),
		"extra_sources":         extraSources,
		"extra_sources_count":   len(extraSources),
		"source_warning":        map[bool]string{true: sourceWarning, false: ""}[len(extraSources) > 0],
		"routing_decision":      routing,
		"provider_mode":         success.Mode,
		"providers_used":        providerNamesFromAttempts(attempts),
		"provider_attempts":     attemptsToMaps(attempts),
		"fallback_used":         meta["fallback_used"],
		"validation_level":      validation,
		"diagnostics":           searchDiagnostics(minimum, routing, attemptsToMaps(attempts)),
		"used":                  used,
		"meta":                  meta,
		"elapsed_ms":            elapsed,
	}
}

func searchDiagnostics(minimum map[string]any, routing any, attempts any) map[string]any {
	out := map[string]any{
		"minimum_profile": minimum,
		"capabilities":    minimum["capabilities"],
	}
	if routing != nil {
		out["routing_decision"] = routing
	}
	if attempts != nil {
		out["provider_attempts"] = attempts
	}
	return out
}

func (s *Service) Fetch(ctx context.Context, targetURL string, options FetchOptions) map[string]any {
	start := time.Now()
	filter := valueOr(options.Provider, "auto")
	result, attempts := s.runWebFetchFallback(ctx, targetURL, filter, s.defaultString("fallback_mode", config.DefaultFallbackMode))
	if truthy(result["ok"]) {
		result["provider_attempts"] = attemptsToMaps(attempts)
		result["fallback_used"] = fallbackUsed(attempts)
		result["elapsed_ms"] = providers.Elapsed(start)
		return result
	}
	if len(attempts) == 0 {
		result["error_type"] = "config_error"
		result["error"] = capabilityProviderError("page_fetch", filter, "请检查 routes.page_fetch、provider capabilities 和 API key。")
	} else {
		result["error_type"] = "network_error"
		result["error"] = "所有提取服务均未能获取内容"
	}
	result["ok"] = false
	result["url"] = targetURL
	result["provider"] = ""
	result["content"] = ""
	result["provider_attempts"] = attemptsToMaps(attempts)
	result["fallback_used"] = fallbackUsed(attempts)
	result["elapsed_ms"] = providers.Elapsed(start)
	return result
}

func (s *Service) Map(ctx context.Context, targetURL string, options MapOptions) map[string]any {
	overallStart := time.Now()
	if options.MaxDepth <= 0 {
		options.MaxDepth = 1
	}
	if options.MaxBreadth <= 0 {
		options.MaxBreadth = 20
	}
	if options.Limit <= 0 {
		options.Limit = 50
	}
	if options.Timeout <= 0 {
		options.Timeout = 150
	}
	filter := valueOr(options.Provider, "auto")
	var attempts []providers.Attempt
	for _, provider := range s.runtime().ResolveProviders(s.Config, "site_map", filter, false) {
		start := time.Now()
		var data map[string]any
		switch provider.ID {
		case "tavily":
			data = providers.Tavily{
				APIURL:  provider.BaseURL,
				APIKey:  provider.APIKey,
				Timeout: time.Duration(options.Timeout+10) * time.Second,
			}.Map(ctx, targetURL, options.Instructions, options.MaxDepth, options.MaxBreadth, options.Limit, options.Timeout)
		case "firecrawl":
			data = providers.Firecrawl{APIURL: provider.BaseURL, APIKey: provider.APIKey}.Map(ctx, targetURL, options.Limit)
		default:
			data = map[string]any{"ok": false, "url": targetURL, "provider": provider.ID, "error_type": "provider_error", "error": "unsupported site_map provider: " + provider.ID}
		}
		if truthy(data["ok"]) {
			data["results"] = sameHostMapResults(targetURL, data["results"])
			resultCount := mapResultCount(data["results"])
			if resultCount == 0 {
				attempts = append(attempts, attempt("site_map", provider.ID, "empty", start, 0, "", ""))
				continue
			}
			data["provider"] = provider.ID
			attempts = append(attempts, attempt("site_map", provider.ID, "ok", start, resultCount, "", ""))
			data["provider_attempts"] = attemptsToMaps(attempts)
			data["fallback_used"] = fallbackUsed(attempts)
			data["elapsed_ms"] = providers.Elapsed(overallStart)
			return data
		}
		errType := firstNonEmpty(stringValue(data["error_type"]), "provider_error")
		msg := stringValue(data["error"])
		attempts = append(attempts, attempt("site_map", provider.ID, "error", start, 0, errType, msg))
	}
	if len(attempts) > 0 {
		return map[string]any{"ok": false, "url": targetURL, "error_type": "network_error", "error": "所有 site_map provider 均未返回同域结果", "provider_attempts": attemptsToMaps(attempts), "fallback_used": fallbackUsed(attempts), "elapsed_ms": providers.Elapsed(overallStart)}
	}
	return map[string]any{"ok": false, "url": targetURL, "error_type": "config_error", "error": capabilityProviderError("site_map", filter, "请检查 routes.site_map、provider capabilities 和 API key。"), "elapsed_ms": providers.Elapsed(overallStart)}
}

func (s *Service) Crawl(ctx context.Context, targetURL string, options CrawlOptions) map[string]any {
	if options.MaxDepth <= 0 {
		options.MaxDepth = 2
	}
	if options.Limit <= 0 {
		options.Limit = 20
	}
	if options.Timeout <= 0 {
		options.Timeout = 180
	}
	filter := valueOr(options.Provider, "auto")
	for _, provider := range s.runtime().ResolveProviders(s.Config, "site_crawl", filter, false) {
		switch provider.ID {
		case "tavily":
			return providers.Tavily{APIURL: provider.BaseURL, APIKey: provider.APIKey, Timeout: durationSeconds(provider.SettingFloat("timeout_seconds", 150))}.Crawl(ctx, targetURL, providers.TavilyCrawlOptions{MaxDepth: options.MaxDepth, Limit: options.Limit, TimeoutSeconds: options.Timeout})
		case "firecrawl":
			return providers.Firecrawl{APIURL: provider.BaseURL, APIKey: provider.APIKey}.Crawl(ctx, targetURL, options.MaxDepth, options.Limit)
		case "freecrawl":
			return providers.Freecrawl{MCP: providers.NewMCPStdio("freecrawl", provider.Settings)}.Crawl(ctx, targetURL, providers.FreecrawlCrawlOptions{MaxDepth: options.MaxDepth, MaxPages: options.Limit, SameDomainOnly: true})
		}
	}
	return map[string]any{"ok": false, "url": targetURL, "error_type": "config_error", "error": capabilityProviderError("site_crawl", filter, "请检查 routes.site_crawl、provider capabilities 和 API key。")}
}

func (s *Service) RepoWiki(ctx context.Context, repo, question string, options RepoWikiOptions) map[string]any {
	start := time.Now()
	mode := options.Mode
	if _, err := validateRepoWikiMode(mode); err != nil {
		return map[string]any{"ok": false, "repo": repo, "error_type": "parameter_error", "error": err.Error(), "elapsed_ms": providers.Elapsed(start)}
	}
	result, attempts := s.runRepoWikiFallback(ctx, repo, question, mode, valueOr(options.Provider, "auto"), s.defaultString("fallback_mode", config.DefaultFallbackMode))
	result["provider_attempts"] = attemptsToMaps(attempts)
	result["fallback_used"] = fallbackUsed(attempts)
	result["elapsed_ms"] = providers.Elapsed(start)
	return result
}

func (s *Service) ExaSearch(ctx context.Context, query string, options providers.ExaOptions) map[string]any {
	provider, ok := s.providerByID("exa")
	if !ok || provider.APIKey == "" {
		return s.providerConfigError("exa")
	}
	return providers.Exa{APIURL: provider.BaseURL, APIKey: provider.APIKey, Timeout: durationSeconds(provider.SettingFloat("timeout_seconds", 30))}.Search(ctx, query, options)
}

func (s *Service) ExaSimilar(ctx context.Context, targetURL string, numResults int) map[string]any {
	provider, ok := s.providerByID("exa")
	if !ok || provider.APIKey == "" {
		return s.providerConfigError("exa")
	}
	return providers.Exa{APIURL: provider.BaseURL, APIKey: provider.APIKey, Timeout: durationSeconds(provider.SettingFloat("timeout_seconds", 30))}.Similar(ctx, targetURL, numResults)
}

func (s *Service) ZhipuSearch(ctx context.Context, query string, options providers.ZhipuOptions) map[string]any {
	provider, ok := s.providerByID("zhipu")
	if !ok || provider.APIKey == "" {
		return s.providerConfigError("zhipu")
	}
	return providers.Zhipu{
		APIURL:          provider.BaseURL,
		APIKey:          provider.APIKey,
		SearchEngine:    provider.SettingString("search_engine", config.DefaultZhipuSearchEngine),
		ProtocolProfile: provider.SettingString("protocol_profile", "bigmodel_cn"),
		SearchIntent:    provider.SettingBool("search_intent", false),
		Timeout:         durationSeconds(provider.SettingFloat("timeout_seconds", 30)),
	}.Search(ctx, query, options)
}

func (s *Service) Context7Library(ctx context.Context, name, query string) map[string]any {
	provider, ok := s.providerByID("context7")
	if !ok || provider.APIKey == "" {
		return s.providerConfigError("context7")
	}
	return providers.Context7{APIURL: provider.BaseURL, APIKey: provider.APIKey, Timeout: durationSeconds(provider.SettingFloat("timeout_seconds", 30)), LegacySearchEndpoint: provider.SettingBool("legacy_search_endpoint", false)}.Library(ctx, name, query)
}

func (s *Service) Context7Docs(ctx context.Context, libraryID, query string) map[string]any {
	provider, ok := s.providerByID("context7")
	if !ok || provider.APIKey == "" {
		return s.providerConfigError("context7")
	}
	return providers.Context7{APIURL: provider.BaseURL, APIKey: provider.APIKey, Timeout: durationSeconds(provider.SettingFloat("timeout_seconds", 30))}.Docs(ctx, libraryID, query)
}

func (s *Service) AnySearch() providers.AnySearch {
	provider, _ := s.providerByID("anysearch")
	return providers.AnySearch{
		APIURL:      firstNonEmpty(provider.BaseURL, config.DefaultAnySearchAPIURL),
		APIKey:      provider.APIKey,
		Timeout:     durationSeconds(provider.SettingFloat("timeout_seconds", 30)),
		SessionMode: provider.SettingString("session_mode", provider.SettingString("mcp.session_mode", "")),
	}
}

func (s *Service) DeepWiki() providers.DeepWiki {
	provider, _ := s.providerByID("deepwiki")
	return providers.DeepWiki{
		APIURL:      firstNonEmpty(provider.BaseURL, config.DefaultDeepWikiAPIURL),
		APIKey:      provider.APIKey,
		Timeout:     durationSeconds(provider.SettingFloat("timeout_seconds", 30)),
		SessionMode: provider.SettingString("session_mode", provider.SettingString("mcp.session_mode", "")),
	}
}

func (s *Service) Doctor(ctx context.Context) map[string]any {
	start := time.Now()
	runtime := s.runtime()
	capability := s.CapabilityStatus()
	minimum := s.ValidateMinimumProfile()
	ok := truthy(minimum["ok"])
	info := map[string]any{
		"ok":     ok,
		"status": map[bool]string{true: "ok", false: "config_error"}[ok],
		"config": s.configDiagnostic(),
		"schema": map[string]any{
			"version": runtime.SchemaVersion,
			"source":  runtime.Source,
		},
		"minimum_profile":       compactMinimumProfile(minimum),
		"issues":                doctorIssues(minimum, capability),
		"effective_environment": s.effectiveEnvironment(runtime),
		"elapsed_ms":            providers.Elapsed(start),
	}
	if ok {
		info["error_type"] = ""
		info["error"] = ""
	} else {
		info["error_type"] = minimum["error_type"]
		info["error"] = minimum["error"]
	}
	if s.Config.InitializationError != "" {
		info["status"] = "initialization_error"
		info["ok"] = false
		info["error_type"] = "config_error"
		info["error"] = s.Config.InitializationError
		configInfo := asMap(info["config"])
		configInfo["initialization_error"] = s.Config.InitializationError
		info["config"] = configInfo
	}
	return info
}

func (s *Service) CurrentModel() map[string]any {
	runtime := s.runtime()
	xai := runtime.Providers["xai"]
	compatible := runtime.Providers["openai_compatible"]
	return map[string]any{
		"ok": true,
		"providers": map[string]any{
			"xai": map[string]any{
				"model": stringValue(xai.Settings["model"]),
			},
			"openai_compatible": map[string]any{
				"model": config.ApplyModelSuffixForURL(stringValue(compatible.Settings["model"]), compatible.BaseURL),
			},
		},
		"config_file": s.Config.ConfigFile,
	}
}

func (s *Service) ConfigPath() map[string]any {
	info := s.Config.PathInfo()
	return map[string]any{
		"ok":                                  info.OK,
		"config_file":                         info.ConfigFile,
		"config_dir":                          info.ConfigDir,
		"config_dir_source":                   info.ConfigDirSource,
		"default_config_file":                 info.DefaultConfigFile,
		"config_dir_override_value":           info.ConfigDirOverrideValue,
		"config_dir_override_matches_default": info.ConfigDirOverrideMatchesDefault,
		"exists":                              info.Exists,
	}
}

func (s *Service) ConfigList(_ bool) map[string]any {
	runtime := s.runtime()
	return map[string]any{
		"ok":        true,
		"metadata":  s.configListMetadata(),
		"schema":    map[string]any{"version": runtime.SchemaVersion, "source": runtime.Source},
		"defaults":  runtime.DefaultsForOutput(),
		"pipelines": runtime.PipelinesForOutput(),
		"routes":    runtime.RoutesForOutput(),
		"profiles":  runtime.ProfilesForOutput(),
		"providers": runtime.ProvidersForOutput(s.Config),
	}
}

func (s *Service) Status() map[string]any {
	start := time.Now()
	runtime := s.runtime()
	minimum := s.ValidateMinimumProfile()
	ready := truthy(minimum["ok"]) && s.Config.InitializationError == ""
	status := "ready"
	if !ready {
		status = "degraded"
	}
	info := map[string]any{
		"ok":     true,
		"ready":  ready,
		"status": status,
		"config": s.configDiagnostic(),
		"schema": map[string]any{
			"version": runtime.SchemaVersion,
			"source":  runtime.Source,
		},
		"minimum_profile":       compactMinimumProfile(minimum),
		"capabilities":          runtimeCapabilityStatus(runtime, s.Config),
		"providers":             runtime.ProvidersForOutput(s.Config),
		"effective_environment": s.effectiveEnvironment(runtime),
		"elapsed_ms":            providers.Elapsed(start),
	}
	if s.Config.InitializationError != "" {
		info["status"] = "initialization_error"
		info["error_type"] = "config_error"
		info["error"] = s.Config.InitializationError
		configInfo := asMap(info["config"])
		configInfo["initialization_error"] = s.Config.InitializationError
		info["config"] = configInfo
	}
	return info
}

func (s *Service) configListMetadata() map[string]any {
	info := s.Config.PathInfo()
	return map[string]any{
		"config_file":                         info.ConfigFile,
		"config_dir":                          info.ConfigDir,
		"config_dir_source":                   info.ConfigDirSource,
		"default_config_file":                 info.DefaultConfigFile,
		"config_dir_override_value":           info.ConfigDirOverrideValue,
		"config_dir_override_matches_default": info.ConfigDirOverrideMatchesDefault,
		"exists":                              info.Exists,
	}
}

func (s *Service) Smoke(ctx context.Context, mode string) map[string]any {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "mock"
	}
	if mode != "mock" && mode != "live" {
		return map[string]any{"ok": false, "error_type": "parameter_error", "error": "mode must be mock or live"}
	}
	if mode == "live" {
		doctor := s.Doctor(ctx)
		minimum := asMap(doctor["minimum_profile"])
		ok := truthy(minimum["ok"])
		failed := []string{}
		if !ok {
			failed = append(failed, "doctor minimum profile")
		}
		return map[string]any{"ok": ok, "mode": "live", "failed_cases": failed, "degraded_cases": []string{}, "cases": []map[string]any{{"name": "doctor minimum profile", "ok": ok, "error": doctor["error"]}}, "elapsed_ms": doctor["elapsed_ms"]}
	}
	return mockSmoke()
}

func (s *Service) ValidateMinimumProfile() map[string]any {
	profile, err := s.minimumProfile()
	if err != nil {
		return map[string]any{"ok": false, "error_type": "parameter_error", "error": err.Error(), "missing": []string{}}
	}
	runtime := s.runtime()
	status := s.CapabilityStatus()
	profileConfig, ok := runtime.Profiles[profile]
	if !ok {
		return map[string]any{"ok": false, "error_type": "parameter_error", "error": "Unknown profile: " + profile, "missing": []string{}, "capabilities": status}
	}
	required := append([]string{}, profileConfig.RequiredCapabilities...)
	missing := []string{}
	for _, capability := range required {
		if item, ok := status[capability].(map[string]any); !ok || !truthy(item["ok"]) {
			missing = append(missing, capability)
		}
	}
	return map[string]any{
		"ok":           len(missing) == 0,
		"error_type":   map[bool]string{true: "", false: "config_error"}[len(missing) == 0],
		"error":        map[bool]string{true: "", false: fmt.Sprintf("最低配置不满足：profile %s 要求能力 %s；缺失能力: %s", profile, strings.Join(required, ", "), strings.Join(missing, ", "))}[len(missing) == 0],
		"profile":      profile,
		"required":     required,
		"missing":      missing,
		"capabilities": status,
	}
}

func (s *Service) CapabilityStatus() map[string]any {
	out := s.runtime().CapabilityStatus(s.Config)
	if item, ok := out["vertical_search"].(map[string]any); ok {
		item["experimental"] = true
	}
	return out
}

func compactMinimumProfile(minimum map[string]any) map[string]any {
	out := map[string]any{
		"ok":       truthy(minimum["ok"]),
		"profile":  minimum["profile"],
		"required": minimum["required"],
		"missing":  minimum["missing"],
	}
	if !truthy(minimum["ok"]) {
		out["error_type"] = minimum["error_type"]
		out["error"] = minimum["error"]
	}
	return out
}

func runtimeCapabilityStatus(runtime config.RuntimeConfig, cfg *config.Config) map[string]any {
	out := map[string]any{}
	for _, capability := range sortedStringListMapKeys(runtime.Routes) {
		resolved := runtime.ResolveProviders(cfg, capability, "auto", true)
		available := []string{}
		providerItems := make([]map[string]any, 0, len(resolved))
		for _, provider := range resolved {
			if provider.Available {
				available = append(available, provider.ID)
			}
			providerItems = append(providerItems, runtimeProviderStatus(provider))
		}
		item := map[string]any{
			"ok":              len(available) > 0,
			"command":         capabilityCommand(capability),
			"available":       available,
			"fallback_chain":  append([]string{}, runtime.Routes[capability]...),
			"provider_status": providerItems,
		}
		if capability == "vertical_search" {
			item["experimental"] = true
		}
		out[capability] = item
	}
	return out
}

func runtimeProviderStatus(provider config.ResolvedProvider) map[string]any {
	return map[string]any{
		"provider":     provider.ID,
		"adapter":      provider.Adapter,
		"available":    provider.Available,
		"reason":       provider.UnavailableReason,
		"config_error": provider.ConfigError,
		"enabled":      provider.Enabled,
		"base_url":     provider.BaseURL,
		"api_key_env":  provider.APIKeyEnv,
		"has_api_key":  provider.APIKey != "",
		"aliases":      append([]string{}, provider.Aliases...),
	}
}

func capabilityCommand(capability string) string {
	definition, ok := serviceCommandRegistry.PreferredFor(capability)
	if !ok {
		return ""
	}
	return "onesearch " + strings.Join(definition.Path, " ")
}

func doctorIssues(minimum, capabilities map[string]any) []map[string]any {
	var issues []map[string]any
	missingRequired := map[string]struct{}{}
	for _, capability := range asStrings(minimum["missing"]) {
		missingRequired[capability] = struct{}{}
		item := asMap(capabilities[capability])
		issues = append(issues, map[string]any{
			"type":       "missing_required_capability",
			"capability": capability,
			"reason":     "no_available_provider",
			"message":    "required capability has no available provider",
			"providers":  compactSkippedProviders(item["skipped"]),
		})
	}

	for _, capability := range sortedMapKeys(capabilities) {
		if _, ok := missingRequired[capability]; ok {
			continue
		}
		item := asMap(capabilities[capability])
		for _, skipped := range asMapSlice(item["skipped"]) {
			if !truthy(skipped["config_error"]) {
				continue
			}
			issues = append(issues, map[string]any{
				"type":       "provider_config_error",
				"capability": capability,
				"provider":   skipped["provider"],
				"reason":     skipped["reason"],
				"message":    "provider is enabled but not usable",
			})
		}
	}
	return issues
}

func compactSkippedProviders(value any) []map[string]any {
	items := asMapSlice(value)
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]any{
			"provider":     item["provider"],
			"reason":       item["reason"],
			"config_error": truthy(item["config_error"]),
		})
	}
	return out
}

type mainProviderConfig struct {
	Provider    string
	DisplayName string
	Mode        string
	APIURL      string
	APIKey      string
	Model       string
	Stream      bool
	Tools       []map[string]any
	ToolChoice  any
	Search      func(context.Context, string, string) (string, error)
}

type answerSearchBuilder func(config.ResolvedProvider, string, *bool) mainProviderConfig

var answerSearchAdapters = map[string]answerSearchBuilder{
	config.AdapterXAIResponses:         buildXAIResponsesRunner,
	config.AdapterOpenAIChatCompletion: buildOpenAICompatibleRunner,
	config.AdapterOpenAIResponses:      buildOpenAIResponsesRunner,
}

func (s *Service) mainProviderConfigs(modelOverride, filter string, streamOverride *bool) ([]mainProviderConfig, error) {
	var out []mainProviderConfig
	for _, provider := range s.runtime().ResolveProviders(s.Config, "answer_search", filter, false) {
		builder := answerSearchAdapters[provider.Adapter]
		if builder == nil {
			continue
		}
		out = append(out, builder(provider, modelOverride, streamOverride))
	}
	return out, nil
}

func buildXAIResponsesRunner(provider config.ResolvedProvider, modelOverride string, _ *bool) mainProviderConfig {
	model := valueOr(modelOverride, provider.SettingString("model", config.DefaultXAIModel))
	tools := toolsFromSetting(provider.Settings["tools"], toolPayloads("web_search", "x_search"))
	toolChoice := toolChoiceFromSetting(provider.Settings["tool_choice"], nil)
	return mainProviderConfig{
		Provider:    provider.ID,
		DisplayName: provider.ID,
		Mode:        "xai-responses",
		APIURL:      provider.BaseURL,
		APIKey:      provider.APIKey,
		Model:       model,
		Tools:       tools,
		ToolChoice:  toolChoice,
		Search: func(ctx context.Context, query, platform string) (string, error) {
			return providers.XAIResponses{APIURL: provider.BaseURL, APIKey: provider.APIKey, Model: model, Tools: tools, ToolChoice: toolChoice}.Search(ctx, query, platform)
		},
	}
}

func buildOpenAICompatibleRunner(provider config.ResolvedProvider, modelOverride string, streamOverride *bool) mainProviderConfig {
	rawModel := valueOr(modelOverride, provider.SettingString("model", "gpt-4.1"))
	model := config.ApplyModelSuffixForURL(rawModel, provider.BaseURL)
	stream := provider.SettingBool("stream", false)
	tools := toolsFromSetting(provider.Settings["tools"], nil)
	toolChoice := toolChoiceFromSetting(provider.Settings["tool_choice"], nil)
	if streamOverride != nil {
		stream = *streamOverride
	}
	return mainProviderConfig{
		Provider:    provider.ID,
		DisplayName: provider.ID,
		Mode:        "chat-completions",
		APIURL:      provider.BaseURL,
		APIKey:      provider.APIKey,
		Model:       model,
		Stream:      stream,
		Tools:       tools,
		ToolChoice:  toolChoice,
		Search: func(ctx context.Context, query, platform string) (string, error) {
			return providers.OpenAICompatible{APIURL: provider.BaseURL, APIKey: provider.APIKey, Model: model, Stream: stream, Tools: tools, ToolChoice: toolChoice}.Search(ctx, query, platform)
		},
	}
}

func buildOpenAIResponsesRunner(provider config.ResolvedProvider, modelOverride string, _ *bool) mainProviderConfig {
	model := valueOr(modelOverride, provider.SettingString("model", "gpt-4.1"))
	return mainProviderConfig{
		Provider:    provider.ID,
		DisplayName: provider.ID,
		Mode:        "openai-responses",
		APIURL:      provider.BaseURL,
		APIKey:      provider.APIKey,
		Model:       model,
		Search: func(ctx context.Context, query, platform string) (string, error) {
			return providers.OpenAIResponses{APIURL: provider.BaseURL, APIKey: provider.APIKey, Model: model}.Search(ctx, query, platform)
		},
	}
}

func (s *Service) callMainProvider(ctx context.Context, cfg mainProviderConfig, query, platform string) (string, error) {
	if cfg.Search == nil {
		return "", fmt.Errorf("missing answer_search runner for provider %s", cfg.Provider)
	}
	return cfg.Search(ctx, query, platform)
}

func (s *Service) extraSources(ctx context.Context, query string, count int, filter string) ([]map[string]any, []providers.Attempt) {
	var attempts []providers.Attempt
	var merged []map[string]any
	if count <= 0 {
		return nil, nil
	}
	selected := s.resolvedProviders("source_search", filter)
	if len(selected) == 0 {
		return nil, nil
	}
	remaining := count
	for index, provider := range selected {
		if remaining <= 0 {
			break
		}
		providerCount := max(1, remaining/(len(selected)-index))
		start := time.Now()
		results, err := s.searchWithSourceProvider(ctx, provider, query, providerCount)
		if err == nil && len(results) > 0 {
			attempts = append(attempts, attempt("source_search", provider.ID, "ok", start, len(results), "", ""))
			merged = append(merged, extraResultsToSources(results, provider.ID)...)
			remaining -= len(results)
			continue
		}
		status := "empty"
		errType, msg := "", ""
		if err != nil {
			status = "error"
			errType, msg = providers.ErrorPayload(err)
		}
		attempts = append(attempts, attempt("source_search", provider.ID, status, start, 0, errType, msg))
	}
	return sources.Merge(merged), attempts
}

func (s *Service) runWebFetchFallback(ctx context.Context, targetURL, filter, fallback string) (map[string]any, []providers.Attempt) {
	var attempts []providers.Attempt
	for _, provider := range limitFallback(s.resolvedProviders("page_fetch", filter), fallback) {
		start := time.Now()
		var content string
		var err error
		if provider.ID == "tavily" {
			content, err = providers.Tavily{APIURL: provider.BaseURL, APIKey: provider.APIKey, Timeout: durationSeconds(provider.SettingFloat("timeout_seconds", 60))}.Extract(ctx, targetURL)
		} else if provider.ID == "firecrawl" {
			content, err = providers.Firecrawl{APIURL: provider.BaseURL, APIKey: provider.APIKey}.Scrape(ctx, targetURL, s.retryMaxAttempts())
		} else if provider.ID == "exa" {
			data := providers.Exa{APIURL: provider.BaseURL, APIKey: provider.APIKey, Timeout: durationSeconds(provider.SettingFloat("timeout_seconds", 30))}.Fetch(ctx, []string{targetURL}, providers.ExaFetchOptions{MaxCharacters: 20000})
			if truthy(data["ok"]) {
				content = stringValue(data["content"])
			} else {
				err = fmt.Errorf("%s", data["error"])
			}
		} else if provider.ID == "anysearch" {
			data := providers.AnySearch{APIURL: provider.BaseURL, APIKey: provider.APIKey, Timeout: durationSeconds(provider.SettingFloat("timeout_seconds", 60))}.Extract(ctx, targetURL, 20000)
			if truthy(data["ok"]) {
				content = stringValue(data["content"])
			} else {
				err = fmt.Errorf("%s", data["error"])
			}
		} else if provider.ID == "ddg" {
			data := providers.DDG{MCP: providers.NewMCPStdio("ddg", provider.Settings)}.FetchContent(ctx, targetURL, providers.DDGFetchOptions{MaxLength: 20000})
			if truthy(data["ok"]) {
				content = stringValue(data["content"])
			} else {
				err = fmt.Errorf("%s", data["error"])
			}
		} else if provider.ID == "freecrawl" {
			data := providers.Freecrawl{MCP: providers.NewMCPStdio("freecrawl", provider.Settings)}.Scrape(ctx, targetURL, providers.FreecrawlScrapeOptions{Formats: []string{"markdown"}, Timeout: 60000})
			if truthy(data["ok"]) {
				content = stringValue(data["content"])
			} else {
				err = fmt.Errorf("%s", data["error"])
			}
		} else {
			err = fmt.Errorf("unsupported page_fetch provider: %s", provider.ID)
		}
		if err == nil && strings.TrimSpace(content) != "" {
			attempts = append(attempts, attempt("page_fetch", provider.ID, "ok", start, 1, "", ""))
			return map[string]any{"ok": true, "url": targetURL, "provider": provider.ID, "content": content}, attempts
		}
		status := "empty"
		errType, msg := "", ""
		if err != nil {
			status = "error"
			errType, msg = providers.ErrorPayload(err)
		}
		attempts = append(attempts, attempt("page_fetch", provider.ID, status, start, 0, errType, msg))
	}
	return map[string]any{"ok": false}, attempts
}

func (s *Service) fetchSourceCandidates(ctx context.Context, candidates []map[string]any, limit int, filter, fallback string) ([]map[string]any, []providers.Attempt) {
	if limit <= 0 {
		return nil, nil
	}
	var fetched []map[string]any
	var attempts []providers.Attempt
	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		if len(fetched) >= limit {
			break
		}
		targetURL := strings.TrimSpace(stringValue(candidate["url"]))
		if !strings.HasPrefix(strings.ToLower(targetURL), "http://") && !strings.HasPrefix(strings.ToLower(targetURL), "https://") {
			continue
		}
		if _, ok := seen[targetURL]; ok {
			continue
		}
		seen[targetURL] = struct{}{}
		found, fetchAttempts := s.runWebFetchFallback(ctx, targetURL, filter, fallback)
		attempts = append(attempts, fetchAttempts...)
		if !truthy(found["ok"]) {
			continue
		}
		content := stringValue(found["content"])
		item := map[string]any{
			"url":             targetURL,
			"provider":        stringValue(found["provider"]),
			"content_preview": truncate(content, 500),
			"content_length":  len(content),
		}
		if title := strings.TrimSpace(stringValue(candidate["title"])); title != "" {
			item["source_title"] = title
		}
		if provider := strings.TrimSpace(stringValue(candidate["provider"])); provider != "" {
			item["source_provider"] = provider
		}
		fetched = append(fetched, item)
	}
	return fetched, attempts
}

func (s *Service) runWebSearchFallback(ctx context.Context, query string, count int, filter, fallback string) ([]map[string]any, []providers.Attempt) {
	var attempts []providers.Attempt
	for _, provider := range limitFallback(s.resolvedProviders("source_search", filter), fallback) {
		start := time.Now()
		results, err := s.searchWithSourceProvider(ctx, provider, query, count)
		if err == nil && len(results) > 0 {
			attempts = append(attempts, attempt("source_search", provider.ID, "ok", start, len(results), "", ""))
			return results, attempts
		}
		status := "empty"
		errType, msg := "", ""
		if err != nil {
			status = "error"
			errType, msg = providers.ErrorPayload(err)
		}
		attempts = append(attempts, attempt("source_search", provider.ID, status, start, 0, errType, msg))
	}
	return nil, attempts
}

func (s *Service) searchWithSourceProvider(ctx context.Context, provider config.ResolvedProvider, query string, count int) ([]map[string]any, error) {
	switch provider.ID {
	case "exa":
		data := s.ExaSearch(ctx, query, providers.ExaOptions{NumResults: count, IncludeHighlights: true})
		if truthy(data["ok"]) {
			return providers.NormalizeSourceResults(asMapSlice(data["results"]), "exa"), nil
		}
		return nil, fmt.Errorf("%s", data["error"])
	case "zhipu":
		data := s.ZhipuSearch(ctx, query, providers.ZhipuOptions{Count: count})
		if truthy(data["ok"]) {
			return providers.NormalizeSourceResults(asMapSlice(data["results"]), "zhipu"), nil
		}
		return nil, fmt.Errorf("%s", data["error"])
	case "tavily":
		raw, err := providers.Tavily{APIURL: provider.BaseURL, APIKey: provider.APIKey, Timeout: durationSeconds(provider.SettingFloat("timeout_seconds", 90))}.Search(ctx, query, count)
		return providers.NormalizeSourceResults(raw, "tavily"), err
	case "firecrawl":
		raw, err := providers.Firecrawl{APIURL: provider.BaseURL, APIKey: provider.APIKey}.Search(ctx, query, count)
		return providers.NormalizeSourceResults(raw, "firecrawl"), err
	case "ddg":
		data := providers.DDG{MCP: providers.NewMCPStdio("ddg", provider.Settings)}.Search(ctx, query, providers.DDGSearchOptions{MaxResults: count})
		if truthy(data["ok"]) {
			return providers.NormalizeSourceResults(asMapSlice(data["results"]), "ddg"), nil
		}
		return nil, fmt.Errorf("%s", data["error"])
	case "freecrawl":
		data := providers.Freecrawl{MCP: providers.NewMCPStdio("freecrawl", provider.Settings)}.Search(ctx, query, providers.FreecrawlSearchOptions{NumResults: count})
		if truthy(data["ok"]) {
			return providers.NormalizeSourceResults(asMapSlice(data["results"]), "freecrawl"), nil
		}
		return nil, fmt.Errorf("%s", data["error"])
	default:
		return nil, fmt.Errorf("unsupported source_search provider: %s", provider.ID)
	}
}

func (s *Service) runDocsSearchFallback(ctx context.Context, query, filter, fallback string) ([]map[string]any, []providers.Attempt) {
	var attempts []providers.Attempt
	for _, provider := range limitFallback(s.resolvedProviders("docs_search", filter), fallback) {
		start := time.Now()
		var results []map[string]any
		var err error
		if provider.ID == "exa" {
			data := s.ExaSearch(ctx, query, providers.ExaOptions{NumResults: 5, IncludeHighlights: true})
			if truthy(data["ok"]) {
				results = providers.NormalizeSourceResults(asMapSlice(data["results"]), "exa")
			} else {
				err = fmt.Errorf("%s", data["error"])
			}
		} else if provider.ID == "context7" {
			data := s.Context7Library(ctx, query, query)
			if truthy(data["ok"]) {
				for _, item := range asMapSlice(data["results"]) {
					if id := stringValue(item["id"]); id != "" {
						results = append(results, map[string]any{"url": "context7:" + id, "title": firstNonEmpty(stringValue(item["title"]), id), "description": stringValue(item["description"]), "provider": "context7"})
					}
				}
			} else {
				err = fmt.Errorf("%s", data["error"])
			}
		}
		if err == nil && len(results) > 0 {
			attempts = append(attempts, attempt("docs_search", provider.ID, "ok", start, len(results), "", ""))
			return results, attempts
		}
		status := "empty"
		errType, msg := "", ""
		if err != nil {
			status = "error"
			errType, msg = providers.ErrorPayload(err)
		}
		attempts = append(attempts, attempt("docs_search", provider.ID, status, start, 0, errType, msg))
	}
	return nil, attempts
}

func (s *Service) runRepoWikiFallback(ctx context.Context, repo, question, mode, filter, fallback string) (map[string]any, []providers.Attempt) {
	normalized, err := normalizeRepoName(repo)
	if err != nil {
		return map[string]any{"ok": false, "repo": repo, "error_type": "parameter_error", "error": err.Error()}, nil
	}
	var attempts []providers.Attempt
	for _, provider := range limitFallback(s.resolvedProviders("repo_wiki", filter), fallback) {
		start := time.Now()
		var data map[string]any
		var callErr error
		switch provider.ID {
		case "deepwiki":
			client := providers.DeepWiki{APIURL: provider.BaseURL, APIKey: provider.APIKey, Timeout: durationSeconds(provider.SettingFloat("timeout_seconds", 30))}
			switch repoWikiMode(mode, question) {
			case "contents":
				data = client.Contents(ctx, normalized)
			case "structure":
				data = client.Structure(ctx, normalized)
			default:
				if strings.TrimSpace(question) == "" {
					callErr = fmt.Errorf("repo_wiki ask mode requires question")
				} else {
					data = client.Ask(ctx, normalized, question)
				}
			}
		default:
			callErr = fmt.Errorf("unsupported repo_wiki provider: %s", provider.ID)
		}
		if callErr == nil && truthy(data["ok"]) {
			attempts = append(attempts, attempt("repo_wiki", provider.ID, "ok", start, 1, "", ""))
			data["repo"] = normalized
			return data, attempts
		}
		status := "empty"
		errType, msg := "", ""
		if callErr != nil {
			status = "error"
			errType, msg = providers.ErrorPayload(callErr)
		} else if data != nil {
			status = "error"
			errType = firstNonEmpty(stringValue(data["error_type"]), "provider_error")
			msg = stringValue(data["error"])
		}
		attempts = append(attempts, attempt("repo_wiki", provider.ID, status, start, 0, errType, msg))
	}
	if len(attempts) == 0 {
		return map[string]any{"ok": false, "repo": normalized, "error_type": "config_error", "error": capabilityProviderError("repo_wiki", filter, "请检查 routes.repo_wiki、provider capabilities 和 API key。")}, attempts
	}
	return map[string]any{"ok": false, "repo": normalized, "error_type": "network_error", "error": "所有 repo_wiki provider 均未返回内容"}, attempts
}

func (s *Service) pageFetchProviders() []string {
	return providerIDs(s.resolvedProviders("page_fetch", "auto"))
}

func (s *Service) sourceSearchProviders(filter string) []string {
	return providerIDs(s.resolvedProviders("source_search", filter))
}

func (s *Service) runtime() config.RuntimeConfig {
	return config.LoadRuntime(s.Config)
}

func (s *Service) resolvedProviders(capability, filter string) []config.ResolvedProvider {
	return s.runtime().ResolveProviders(s.Config, capability, filter, false)
}

func (s *Service) providerByID(providerID string) (config.ResolvedProvider, bool) {
	runtime := s.runtime()
	provider, ok := runtime.Providers[providerID]
	if !ok {
		return config.ResolvedProvider{}, false
	}
	credential := config.ResolveProviderCredential(s.Config, provider)
	return config.ResolvedProvider{
		ID:        provider.ID,
		Adapter:   provider.Adapter,
		BaseURL:   provider.BaseURL,
		APIKeyEnv: provider.APIKeyEnv,
		APIKey:    credential.Value,
		Settings:  provider.Settings,
		Enabled:   provider.Enabled,
		Aliases:   provider.Aliases,
	}, true
}

func (s *Service) validation(value string) (string, error) {
	if strings.TrimSpace(value) != "" {
		return enumLocal(value, "validation level", []string{"fast", "balanced", "strict"})
	}
	return enumLocal(s.defaultString("validation_level", config.DefaultValidationLevel), "validation level", []string{"fast", "balanced", "strict"})
}

func (s *Service) fallback(value string) (string, error) {
	if strings.TrimSpace(value) != "" {
		return enumLocal(value, "fallback mode", []string{"auto", "off"})
	}
	return enumLocal(s.defaultString("fallback_mode", config.DefaultFallbackMode), "fallback mode", []string{"auto", "off"})
}

func (s *Service) minimumProfile() (string, error) {
	return enumLocal(s.defaultString("minimum_profile", config.DefaultMinimumProfile), "minimum profile", []string{"standard", "minimal", "off"})
}

func (s *Service) defaultString(key, fallback string) string {
	if value := stringValue(s.runtime().Defaults[key]); strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func (s *Service) retryMaxAttempts() int {
	retry := asMap(s.runtime().Defaults["retry"])
	if value := intFromAny(retry["max_attempts"], 3); value > 0 {
		return value
	}
	return 3
}

func (s *Service) providerConfigError(providerID string) map[string]any {
	provider, ok := s.runtime().Providers[providerID]
	if !ok {
		return configError("provider " + providerID + " 不存在；请检查 config.json 的 providers 和 routes。")
	}
	return configError(fmt.Sprintf("provider %s 不可用；请在 config.json 的 providers.%s 中启用，并配置 api_key 或通过 api_key_env 指向的环境变量 %s 提供密钥。", providerID, providerID, provider.APIKeyEnv))
}

func enumLocal(value, label string, allowed []string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	for _, item := range allowed {
		if value == item {
			return value, nil
		}
	}
	return value, fmt.Errorf("Invalid %s: %s", label, value)
}

func configError(message string) map[string]any {
	return map[string]any{"ok": false, "error_type": "config_error", "error": message}
}

func capabilityProviderError(capability, filter, hint string) string {
	filter = strings.TrimSpace(filter)
	if filter == "" || strings.EqualFold(filter, "auto") {
		return fmt.Sprintf("%s 没有可用 provider；%s", capability, hint)
	}
	return fmt.Sprintf("%s 没有可用 provider 匹配 --provider %s；%s", capability, filter, hint)
}

func emptySearch(start time.Time, sessionID, query, errorType, message string) map[string]any {
	return map[string]any{
		"ok":                    false,
		"error_type":            errorType,
		"error":                 message,
		"session_id":            sessionID,
		"query":                 query,
		"content":               "",
		"sources":               []map[string]any{},
		"sources_count":         0,
		"primary_sources":       []map[string]any{},
		"primary_sources_count": 0,
		"extra_sources":         []map[string]any{},
		"extra_sources_count":   0,
		"source_warning":        "",
		"routing_decision":      map[string]any{},
		"providers_used":        []string{},
		"provider_attempts":     []map[string]any{},
		"fallback_used":         false,
		"validation_level":      "",
		"elapsed_ms":            providers.Elapsed(start),
	}
}

func primarySearchError(start time.Time, sessionID, query, mode, errorType, message string) map[string]any {
	out := emptySearch(start, sessionID, query, errorType, message)
	out["provider_mode"] = mode
	return out
}

func attempt(capability, provider, status string, start time.Time, resultCount int, errorType, err string) providers.Attempt {
	capability = v2Capability(capability)
	return providers.Attempt{
		Capability:  capability,
		Provider:    provider,
		Status:      status,
		ErrorType:   errorType,
		Error:       err,
		ElapsedMS:   providers.Elapsed(start),
		ResultCount: resultCount,
	}
}

func attemptsToMaps(items []providers.Attempt) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]any{"capability": item.Capability, "provider": item.Provider, "status": item.Status, "error_type": item.ErrorType, "error": item.Error, "elapsed_ms": item.ElapsedMS, "result_count": item.ResultCount})
	}
	return out
}

func searchUsedCapability(capability, role string, providers ...map[string]any) map[string]any {
	item := map[string]any{
		"capability": capability,
		"providers":  providers,
	}
	if strings.TrimSpace(role) != "" {
		item["role"] = role
	}
	return item
}

func providerResult(provider string, attempt providers.Attempt, result map[string]any, extras map[string]any) map[string]any {
	item := map[string]any{
		"provider":   provider,
		"status":     attempt.Status,
		"elapsed_ms": attempt.ElapsedMS,
		"result":     result,
	}
	for key, value := range extras {
		if stringValue(value) != "" {
			item[key] = value
		}
	}
	return item
}

func usedCapabilitiesFromSourceAttempts(capability, role string, found []map[string]any, attempts []providers.Attempt) []map[string]any {
	providerBuckets := map[string][]map[string]any{}
	for _, item := range found {
		provider := strings.TrimSpace(stringValue(item["provider"]))
		if provider == "" {
			provider = okAttemptProvider(attempts, capability)
		}
		if provider == "" {
			provider = "unknown"
		}
		providerBuckets[provider] = append(providerBuckets[provider], item)
	}
	if len(providerBuckets) == 0 {
		return nil
	}
	attemptByProvider := map[string]providers.Attempt{}
	for _, attempt := range attempts {
		if attempt.Capability == capability && attempt.Status == "ok" {
			attemptByProvider[attempt.Provider] = attempt
		}
	}
	providerIDs := make([]string, 0, len(providerBuckets))
	for provider := range providerBuckets {
		providerIDs = append(providerIDs, provider)
	}
	sort.Strings(providerIDs)
	providerResults := make([]map[string]any, 0, len(providerIDs))
	for _, provider := range providerIDs {
		resultSources := tagResultSources(providerBuckets[provider], capability, provider)
		attempt := attemptByProvider[provider]
		if attempt.Provider == "" {
			attempt = providers.Attempt{Provider: provider, Status: "ok", ResultCount: len(resultSources)}
		}
		providerResults = append(providerResults, providerResult(provider, attempt, map[string]any{
			"sources":       resultSources,
			"sources_count": len(resultSources),
		}, nil))
	}
	return []map[string]any{searchUsedCapability(capability, role, providerResults...)}
}

func usedCapabilityFromFetch(found map[string]any, attempts []providers.Attempt) map[string]any {
	provider := strings.TrimSpace(stringValue(found["provider"]))
	attempt := okAttempt(attempts, "page_fetch", provider)
	content := stringValue(found["content"])
	result := map[string]any{
		"url":             found["url"],
		"content_preview": truncate(content, 500),
	}
	if content != "" {
		result["content_length"] = len(content)
	}
	if provider == "" {
		provider = okAttemptProvider(attempts, "page_fetch")
	}
	if provider == "" {
		provider = "unknown"
	}
	if attempt.Provider == "" {
		attempt = providers.Attempt{Provider: provider, Status: "ok", ResultCount: 1}
	}
	return searchUsedCapability("page_fetch", "page_evidence", providerResult(provider, attempt, result, nil))
}

func usedCapabilityFromFetchedSources(fetched []map[string]any, attempts []providers.Attempt) map[string]any {
	providerBuckets := map[string][]map[string]any{}
	for _, item := range fetched {
		provider := strings.TrimSpace(stringValue(item["provider"]))
		if provider == "" {
			provider = okAttemptProvider(attempts, "page_fetch")
		}
		if provider == "" {
			provider = "unknown"
		}
		providerBuckets[provider] = append(providerBuckets[provider], item)
	}
	providerIDs := make([]string, 0, len(providerBuckets))
	for provider := range providerBuckets {
		providerIDs = append(providerIDs, provider)
	}
	sort.Strings(providerIDs)
	var providerResults []map[string]any
	for _, provider := range providerIDs {
		attempt := okAttempt(attempts, "page_fetch", provider)
		if attempt.Provider == "" {
			attempt = providers.Attempt{Provider: provider, Status: "ok", ResultCount: len(providerBuckets[provider])}
		}
		result := map[string]any{
			"pages":       providerBuckets[provider],
			"pages_count": len(providerBuckets[provider]),
		}
		if len(providerBuckets[provider]) == 1 {
			for _, key := range []string{"url", "content_preview", "content_length"} {
				result[key] = providerBuckets[provider][0][key]
			}
		}
		providerResults = append(providerResults, providerResult(provider, attempt, result, nil))
	}
	return searchUsedCapability("page_fetch", "source_evidence", providerResults...)
}

func usedCapabilityFromRepoWiki(found map[string]any, attempts []providers.Attempt) map[string]any {
	provider := strings.TrimSpace(stringValue(found["provider"]))
	attempt := okAttempt(attempts, "repo_wiki", provider)
	content := stringValue(found["content"])
	result := map[string]any{
		"repo":    found["repo"],
		"tool":    found["tool"],
		"content": content,
	}
	if query := stringValue(found["query"]); query != "" {
		result["query"] = query
	}
	if provider == "" {
		provider = okAttemptProvider(attempts, "repo_wiki")
	}
	if provider == "" {
		provider = "unknown"
	}
	if attempt.Provider == "" {
		attempt = providers.Attempt{Provider: provider, Status: "ok", ResultCount: 1}
	}
	return searchUsedCapability("repo_wiki", "repository_wiki", providerResult(provider, attempt, result, nil))
}

func tagResultSources(items []map[string]any, capability, provider string) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if len(item) == 0 {
			continue
		}
		tagged := map[string]any{}
		for key, value := range item {
			tagged[key] = value
		}
		if strings.TrimSpace(stringValue(tagged["provider"])) == "" && provider != "" {
			tagged["provider"] = provider
		}
		if capability != "" {
			tagged["capability"] = capability
		}
		out = append(out, tagged)
	}
	return out
}

func sameHostMapResults(targetURL string, value any) []any {
	targetHost := normalizedURLHost(targetURL)
	items := mapResultItems(value)
	if targetHost == "" {
		return items
	}
	var out []any
	for _, item := range items {
		itemURL := mapResultURL(item)
		if sameOrSubHost(normalizedURLHost(itemURL), targetHost) {
			out = append(out, item)
		}
	}
	return out
}

func mapResultCount(value any) int {
	return len(mapResultItems(value))
}

func mapResultItems(value any) []any {
	switch items := value.(type) {
	case []any:
		return items
	case []string:
		out := make([]any, 0, len(items))
		for _, item := range items {
			out = append(out, item)
		}
		return out
	case []map[string]any:
		out := make([]any, 0, len(items))
		for _, item := range items {
			out = append(out, item)
		}
		return out
	default:
		return nil
	}
}

func mapResultURL(value any) string {
	switch item := value.(type) {
	case string:
		return item
	case map[string]any:
		return firstNonEmpty(stringValue(item["url"]), stringValue(item["link"]), stringValue(item["href"]))
	default:
		return ""
	}
}

func normalizedURLHost(value string) string {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Host == "" {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
}

func sameOrSubHost(host, target string) bool {
	return host == target || strings.HasSuffix(host, "."+target)
}

func normalizeRepoName(input string) (string, error) {
	text := strings.TrimSpace(input)
	if text == "" {
		return "", fmt.Errorf("repo_wiki requires repo, expected owner/repo or GitHub URL")
	}
	text = strings.TrimSuffix(text, ".git")
	if strings.HasPrefix(strings.ToLower(text), "http://") || strings.HasPrefix(strings.ToLower(text), "https://") {
		parsed, err := url.Parse(text)
		if err != nil || parsed.Host == "" {
			return "", fmt.Errorf("invalid GitHub repo URL: %s", input)
		}
		host := strings.ToLower(strings.TrimPrefix(parsed.Host, "www."))
		if host != "github.com" {
			return "", fmt.Errorf("repo_wiki only accepts GitHub repo URLs or owner/repo names")
		}
		parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		if len(parts) < 2 {
			return "", fmt.Errorf("GitHub repo URL must include owner and repo")
		}
		return cleanRepoPair(parts[0], strings.TrimSuffix(parts[1], ".git"))
	}
	parts := strings.Split(strings.Trim(text, "/"), "/")
	if len(parts) != 2 {
		return "", fmt.Errorf("repo_wiki repo must be owner/repo, not %q", input)
	}
	return cleanRepoPair(parts[0], strings.TrimSuffix(parts[1], ".git"))
}

func cleanRepoPair(owner, repo string) (string, error) {
	owner = strings.TrimSpace(owner)
	repo = strings.TrimSpace(repo)
	if owner == "" || repo == "" {
		return "", fmt.Errorf("repo_wiki repo must include owner and repo")
	}
	return owner + "/" + repo, nil
}

func repoWikiMode(mode, question string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	switch mode {
	case "ask", "structure", "contents":
		return mode
	case "":
		if strings.TrimSpace(question) == "" {
			return "structure"
		}
		return "ask"
	default:
		return mode
	}
}

func validateRepoWikiMode(mode string) (string, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	switch mode {
	case "", "ask", "structure", "contents":
		return mode, nil
	default:
		return mode, fmt.Errorf("Invalid repo_wiki mode: %s", mode)
	}
}

func okAttemptProvider(attempts []providers.Attempt, capability string) string {
	attempt := okAttempt(attempts, capability, "")
	return attempt.Provider
}

func okAttempt(attempts []providers.Attempt, capability, provider string) providers.Attempt {
	for _, attempt := range attempts {
		if attempt.Capability != capability || attempt.Status != "ok" {
			continue
		}
		if provider == "" || attempt.Provider == provider {
			return attempt
		}
	}
	return providers.Attempt{}
}

func providerNamesFromAttempts(items []providers.Attempt) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, item := range items {
		if item.Status == "ok" && item.Provider != "" {
			if _, ok := seen[item.Provider]; ok {
				continue
			}
			seen[item.Provider] = struct{}{}
			out = append(out, item.Provider)
		}
	}
	return out
}

func fallbackUsed(items []providers.Attempt) bool {
	counts := map[string]int{}
	for _, item := range items {
		if item.Status == "ok" || item.Status == "empty" || item.Status == "error" {
			counts[item.Capability]++
		}
	}
	for _, count := range counts {
		if count > 1 {
			return true
		}
	}
	return false
}

func providerNames(configs []mainProviderConfig) []string {
	out := make([]string, 0, len(configs))
	for _, cfg := range configs {
		out = append(out, cfg.Provider)
	}
	return out
}

func openAIStream(configs []mainProviderConfig) bool {
	for _, cfg := range configs {
		if cfg.Provider == "openai-compatible" {
			return cfg.Stream
		}
	}
	return false
}

func v2Capability(capability string) string {
	return config.V2CapabilityName(capability)
}

func limitFallback[T any](items []T, fallback string) []T {
	if fallback == "off" && len(items) > 1 {
		return items[:1]
	}
	return items
}

func durationSeconds(value float64) time.Duration {
	if value <= 0 {
		value = 30
	}
	return time.Duration(value * float64(time.Second))
}

func extraResultsToSources(results []map[string]any, provider string) []map[string]any {
	var out []map[string]any
	for _, item := range results {
		url := strings.TrimSpace(stringValue(item["url"]))
		if url == "" {
			continue
		}
		source := map[string]any{"url": url, "provider": provider}
		if title := strings.TrimSpace(stringValue(item["title"])); title != "" {
			source["title"] = title
		}
		if desc := strings.TrimSpace(firstNonEmpty(stringValue(item["description"]), stringValue(item["content"]))); desc != "" {
			source["description"] = desc
		}
		out = append(out, source)
	}
	return sources.Merge(out)
}

func asMapSlice(value any) []map[string]any {
	switch items := value.(type) {
	case []map[string]any:
		return items
	case []any:
		out := make([]map[string]any, 0, len(items))
		for _, item := range items {
			if m, ok := item.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	default:
		return nil
	}
}

func asMap(value any) map[string]any {
	if item, ok := value.(map[string]any); ok {
		return item
	}
	return map[string]any{}
}

func asStrings(value any) []string {
	if items, ok := value.([]string); ok {
		return items
	}
	if items, ok := value.([]any); ok {
		out := make([]string, 0, len(items))
		for _, item := range items {
			out = append(out, stringValue(item))
		}
		return out
	}
	return nil
}

func sortedMapKeys(items map[string]any) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedStringListMapKeys(items map[string][]string) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func intFromAny(value any, fallback int) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		var parsed int
		if _, err := fmt.Sscanf(strings.TrimSpace(typed), "%d", &parsed); err == nil {
			return parsed
		}
	}
	return fallback
}

func providerIDs(items []config.ResolvedProvider) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.ID)
	}
	return out
}

func toolsFromSetting(value any, fallback []map[string]any) []map[string]any {
	switch items := value.(type) {
	case []string:
		if tools := toolPayloads(items...); len(tools) > 0 {
			return tools
		}
	case []any:
		out := make([]map[string]any, 0, len(items))
		for _, item := range items {
			switch typed := item.(type) {
			case map[string]any:
				tool := compactToolPayload(typed)
				if len(tool) > 0 {
					out = append(out, tool)
				}
			default:
				if text := strings.TrimSpace(stringValue(item)); text != "" {
					out = append(out, map[string]any{"type": text})
				}
			}
		}
		if len(out) > 0 {
			return out
		}
	case string:
		var names []string
		for _, item := range strings.Split(items, ",") {
			if text := strings.TrimSpace(item); text != "" {
				names = append(names, text)
			}
		}
		if tools := toolPayloads(names...); len(tools) > 0 {
			return tools
		}
	}
	return cloneToolPayloads(fallback)
}

func toolChoiceFromSetting(value any, fallback any) any {
	switch typed := value.(type) {
	case nil:
		return fallback
	case string:
		if text := strings.TrimSpace(typed); text != "" {
			return text
		}
	case map[string]any:
		choice := compactToolPayload(typed)
		if len(choice) > 0 {
			return choice
		}
	}
	return fallback
}

func toolPayloads(names ...string) []map[string]any {
	out := make([]map[string]any, 0, len(names))
	for _, name := range names {
		if text := strings.TrimSpace(name); text != "" {
			out = append(out, map[string]any{"type": text})
		}
	}
	return out
}

func cloneToolPayloads(items []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, compactToolPayload(item))
	}
	return out
}

func compactToolPayload(input map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range input {
		if strings.TrimSpace(key) == "" || stringValue(value) == "" {
			continue
		}
		out[key] = value
	}
	return out
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func truthy(value any) bool {
	if b, ok := value.(bool); ok {
		return b
	}
	return false
}

func valueOr(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func valueOrInt(value, fallback int) int {
	if value != 0 {
		return value
	}
	return fallback
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func truncate(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	return text[:limit]
}

func max(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func isDocsIntent(query string) bool {
	q := strings.ToLower(query)
	keywords := []string{"api", "sdk", "library", "framework", "docs", "documentation", "reference", "react", "next.js", "vue", "python", "prisma", "langchain", "openai", "context7", "接口", "文档", "库", "框架", "函数", "参数", "配置"}
	for _, keyword := range keywords {
		if strings.Contains(q, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func isZHCurrentIntent(query string) bool {
	q := strings.ToLower(query)
	keywords := []string{"今天", "最新", "国内", "中国", "政策", "新闻", "实时", "刚刚", "本周", "本月", "战报", "比分", "赛程", "赛果", "季后赛", "比赛", "nba", "足球", "篮球"}
	for _, keyword := range keywords {
		if strings.Contains(q, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func isFetchIntent(query string) bool {
	return strings.Contains(strings.ToLower(query), "http://") || strings.Contains(strings.ToLower(query), "https://")
}
