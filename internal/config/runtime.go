package config

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

const (
	AdapterAnySearchMCP         = "anysearch_mcp"
	AdapterContext7             = "context7"
	AdapterExa                  = "exa"
	AdapterFirecrawl            = "firecrawl"
	AdapterMCPStdio             = "mcp_stdio"
	AdapterOpenAIChatCompletion = "openai_chat_completions"
	AdapterOpenAIResponses      = "openai_responses"
	AdapterTavily               = "tavily"
	AdapterXAIResponses         = "xai_responses"
	AdapterZhipuWebSearch       = "zhipu_web_search"
)

var supportedAdapters = map[string]struct{}{
	AdapterAnySearchMCP:         {},
	AdapterContext7:             {},
	AdapterExa:                  {},
	AdapterFirecrawl:            {},
	AdapterMCPStdio:             {},
	AdapterOpenAIChatCompletion: {},
	AdapterOpenAIResponses:      {},
	AdapterTavily:               {},
	AdapterXAIResponses:         {},
	AdapterZhipuWebSearch:       {},
	"deepwiki":                  {},
}

func IsSupportedAdapter(adapter string) bool {
	_, ok := supportedAdapters[strings.TrimSpace(adapter)]
	return ok
}

type ProviderDefinition struct {
	ID           string
	Adapter      string
	Capabilities []string
	BaseURL      string
	APIKey       string
	APIKeyEnv    string
	Enabled      any
	Settings     map[string]any
	Aliases      []string
}

type ProfileConfig struct {
	ID                   string
	RequiredCapabilities []string
	OptionalCapabilities []string
}

type RuntimeConfig struct {
	SchemaVersion int
	Source        string
	Defaults      map[string]any
	Providers     map[string]ProviderDefinition
	Routes        map[string][]string
	Profiles      map[string]ProfileConfig
	Pipelines     map[string][]string
	Raw           map[string]any
}

type ResolvedProvider struct {
	ID                string
	Adapter           string
	Capability        string
	BaseURL           string
	APIKeyEnv         string
	APIKey            string
	Settings          map[string]any
	Enabled           any
	Available         bool
	UnavailableReason string
	ConfigError       bool
	Aliases           []string
}

type ProviderCredentialState struct {
	DirectValue      string
	EnvironmentValue string
	Value            string
	Source           string
	DirectSet        bool
	EnvironmentSet   bool
}

func LoadRuntime(c *Config) RuntimeConfig {
	raw := map[string]any{}
	if c != nil {
		raw = c.LoadFile()
	}
	return runtimeFromRaw(raw)
}

func LoadRuntimeStrict(c *Config) (RuntimeConfig, error) {
	raw, err := c.LoadFileStrict()
	if err != nil {
		return RuntimeConfig{}, err
	}
	if !isRuntimeSchema(raw) {
		return RuntimeConfig{}, fmt.Errorf("配置文件不是可识别的 runtime schema")
	}
	return runtimeFromRaw(raw), nil
}

func runtimeFromRaw(raw map[string]any) RuntimeConfig {
	isSchema := isRuntimeSchema(raw)
	runtime := RuntimeConfig{
		SchemaVersion: 1,
		Source:        "builtin",
		Defaults:      defaultRuntimeDefaults(),
		Providers:     defaultProviders(),
		Routes:        defaultRuntimeRoutes(),
		Profiles:      defaultRuntimeProfiles(),
		Pipelines:     defaultRuntimePipelines(),
		Raw:           raw,
	}
	if isSchema {
		runtime.Source = "config-new"
		runtime.SchemaVersion = intFromAny(raw["schema_version"], 1)
		runtime.Defaults = mergeAnyMap(runtime.Defaults, mapValue(raw["defaults"]))
		runtime.Pipelines = mergeStringListMap(runtime.Pipelines, mapValue(raw["pipelines"]))
		runtime.Routes = mergeStringListMap(runtime.Routes, mapValue(raw["routes"]))
		runtime.Profiles = mergeProfiles(runtime.Profiles, mapValue(raw["profiles"]))
		for id, value := range mapValue(raw["providers"]) {
			data := mapValue(value)
			base, ok := runtime.Providers[id]
			if !ok {
				base = ProviderDefinition{ID: id, Settings: map[string]any{}}
			}
			runtime.Providers[id] = mergeProvider(id, base, data)
		}
	}
	runtime.Routes = routesWithProviderCapabilities(runtime.Routes, runtime.Providers)
	return runtime
}

func (r RuntimeConfig) RoutesForOutput() map[string][]string {
	return copyStringListMap(r.Routes)
}

func (r RuntimeConfig) ProfilesForOutput() map[string]map[string]any {
	out := map[string]map[string]any{}
	keys := make([]string, 0, len(r.Profiles))
	for key := range r.Profiles {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		profile := r.Profiles[key]
		out[key] = map[string]any{
			"required_capabilities": append([]string{}, profile.RequiredCapabilities...),
			"optional_capabilities": append([]string{}, profile.OptionalCapabilities...),
		}
	}
	return out
}

func (r RuntimeConfig) PipelinesForOutput() map[string][]string {
	return copyStringListMap(r.Pipelines)
}

func (r RuntimeConfig) DefaultsForOutput() map[string]any {
	return copyAnyMap(r.Defaults)
}

func (r RuntimeConfig) ProvidersForOutput(c *Config) map[string]any {
	out := map[string]any{}
	keys := make([]string, 0, len(r.Providers))
	for key := range r.Providers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		provider := r.Providers[key]
		credential := ResolveProviderCredential(c, provider)
		capabilityStatus := map[string]any{}
		available := false
		for _, capability := range provider.Capabilities {
			resolved := firstResolved(r.ResolveProviders(c, capability, "auto", true), key)
			if resolved.ID == "" {
				continue
			}
			available = available || resolved.Available
			capabilityStatus[capability] = map[string]any{
				"available": resolved.Available,
				"reason":    resolved.UnavailableReason,
			}
		}
		out[key] = map[string]any{
			"adapter":         provider.Adapter,
			"enabled":         normalizeEnabled(provider.Enabled),
			"capabilities":    append([]string{}, provider.Capabilities...),
			"base_url":        provider.BaseURL,
			"api_key":         maskSecret(provider.APIKey),
			"api_key_env":     provider.APIKeyEnv,
			"api_key_set":     credential.DirectSet,
			"api_key_env_set": credential.EnvironmentSet,
			"api_key_src":     credential.Source,
			"has_api_key":     credential.Value != "",
			"available":       available,
			"settings":        settingsForOutput(provider.Settings),
			"status":          capabilityStatus,
			"aliases":         append([]string{}, provider.Aliases...),
		}
	}
	return out
}

func (r RuntimeConfig) ResolveProviders(c *Config, capability, providerFilter string, includeUnavailable bool) []ResolvedProvider {
	capability = V2CapabilityName(capability)
	var out []ResolvedProvider
	for _, providerID := range r.Routes[capability] {
		provider, ok := r.Providers[providerID]
		if !ok {
			item := ResolvedProvider{ID: providerID, Capability: capability, Enabled: "auto", UnavailableReason: "unknown_provider", ConfigError: true}
			if providerMatches(item.ID, nil, providerFilter) && includeUnavailable {
				out = append(out, item)
			}
			continue
		}
		apiKey := providerAPIKey(c, provider)
		available, reason, configError := r.providerAvailability(provider, capability, apiKey)
		item := ResolvedProvider{
			ID:                provider.ID,
			Adapter:           provider.Adapter,
			Capability:        capability,
			BaseURL:           provider.BaseURL,
			APIKeyEnv:         provider.APIKeyEnv,
			APIKey:            apiKey,
			Settings:          copyAnyMap(provider.Settings),
			Enabled:           normalizeEnabled(provider.Enabled),
			Available:         available,
			UnavailableReason: reason,
			ConfigError:       configError,
			Aliases:           append([]string{}, provider.Aliases...),
		}
		if providerMatches(item.ID, item.Aliases, providerFilter) && (includeUnavailable || item.Available) {
			out = append(out, item)
		}
	}
	return out
}

func (r RuntimeConfig) CapabilityStatus(c *Config) map[string]any {
	out := map[string]any{}
	for _, capability := range sortedRouteKeys(r.Routes) {
		providers := r.ResolveProviders(c, capability, "auto", true)
		available := []string{}
		var skipped []map[string]any
		for _, provider := range providers {
			if provider.Available {
				available = append(available, provider.ID)
			} else {
				skipped = append(skipped, map[string]any{
					"provider":     provider.ID,
					"reason":       provider.UnavailableReason,
					"config_error": provider.ConfigError,
				})
			}
		}
		out[capability] = map[string]any{
			"ok":             len(available) > 0,
			"configured":     available,
			"available":      available,
			"fallback_chain": append([]string{}, r.Routes[capability]...),
			"skipped":        skipped,
		}
	}
	return out
}

func (r RuntimeConfig) providerAvailability(provider ProviderDefinition, capability, apiKey string) (bool, string, bool) {
	enabled := normalizeEnabled(provider.Enabled)
	if b, ok := enabled.(bool); ok && !b {
		return false, "disabled", false
	}
	if _, ok := supportedAdapters[provider.Adapter]; !ok {
		return false, "unsupported_adapter", enabled == true
	}
	if !containsCapability(provider.Capabilities, capability) {
		return false, "unsupported_capability", enabled == true
	}
	if boolSetting(provider.Settings, "requires_base_url", false) && strings.TrimSpace(provider.BaseURL) == "" {
		return false, "missing_base_url", enabled == true
	}
	if provider.Adapter == AdapterMCPStdio {
		if reason := mcpStdioAvailabilityReason(provider, capability); reason != "" {
			return false, reason, enabled == true
		}
		return true, "", false
	}
	if provider.Adapter == AdapterZhipuWebSearch {
		profile := strings.ToLower(strings.TrimSpace(fmt.Sprint(provider.Settings["protocol_profile"])))
		if profile != "" && profile != "bigmodel_cn" && profile != "zai_global" {
			return false, "invalid_protocol_profile", true
		}
	}
	if strings.TrimSpace(apiKey) == "" && !boolSetting(provider.Settings, "anonymous_allowed", false) {
		return false, "missing_api_key", enabled == true
	}
	return true, "", false
}

func providerAPIKey(c *Config, provider ProviderDefinition) string {
	return ResolveProviderCredential(c, provider).Value
}

func ResolveProviderCredential(c *Config, provider ProviderDefinition) ProviderCredentialState {
	direct := strings.TrimSpace(provider.APIKey)
	environment := ""
	if c != nil && strings.TrimSpace(provider.APIKeyEnv) != "" {
		environment = strings.TrimSpace(c.Get(provider.APIKeyEnv, ""))
	}
	state := ProviderCredentialState{
		DirectValue:      direct,
		EnvironmentValue: environment,
		DirectSet:        direct != "",
		EnvironmentSet:   environment != "",
	}
	if state.DirectSet {
		state.Value = direct
		state.Source = "config"
	} else if state.EnvironmentSet {
		state.Value = environment
		state.Source = "env"
	}
	return state
}

func maskSecret(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return "********"
}

func DefaultRuntimeRoutes() map[string][]string {
	return defaultRuntimeRoutes()
}

func DefaultRuntimeProfiles() map[string]ProfileConfig {
	return defaultRuntimeProfiles()
}

func InitialRuntimeSchema() map[string]any {
	return map[string]any{
		"schema_version": 1,
		"defaults":       defaultRuntimeDefaults(),
		"pipelines":      defaultRuntimePipelines(),
		"routes":         defaultRuntimeRoutes(),
		"profiles":       profileSchema(defaultRuntimeProfiles()),
		"providers":      providerSchema(defaultProviders()),
	}
}

func V2CapabilityName(capability string) string {
	switch capability {
	case "main_search":
		return "answer_search"
	case "web_search":
		return "source_search"
	case "web_fetch":
		return "page_fetch"
	default:
		return capability
	}
}

func (p ResolvedProvider) SettingString(key, fallback string) string {
	if value, ok := p.Settings[key]; ok && value != nil {
		text := strings.TrimSpace(fmt.Sprint(value))
		if text != "" {
			return text
		}
	}
	return fallback
}

func (p ResolvedProvider) SettingBool(key string, fallback bool) bool {
	if value, ok := p.Settings[key]; ok {
		return boolFromAny(value, fallback)
	}
	return fallback
}

func (p ResolvedProvider) SettingFloat(key string, fallback float64) float64 {
	if value, ok := p.Settings[key]; ok {
		return floatFromAny(value, fallback)
	}
	return fallback
}

func isRuntimeSchema(raw map[string]any) bool {
	if raw == nil {
		return false
	}
	_, hasProviders := raw["providers"].(map[string]any)
	_, hasRoutes := raw["routes"].(map[string]any)
	return raw["schema_version"] != nil && hasProviders && hasRoutes
}

func defaultRuntimeDefaults() map[string]any {
	return map[string]any{
		"pipeline":         "default",
		"fallback_mode":    DefaultFallbackMode,
		"validation_level": DefaultValidationLevel,
		"minimum_profile":  DefaultMinimumProfile,
		"timeout_seconds":  30,
		"ssl_verify":       true,
		"log_level":        "INFO",
		"log_to_file":      false,
		"log_dir":          "logs",
		"output_cleanup":   true,
		"retry":            map[string]any{"max_attempts": 3, "multiplier": 1, "max_wait_seconds": 10},
	}
}

func defaultRuntimePipelines() map[string][]string {
	return map[string][]string{
		"default":  {"answer_search", "source_search", "docs_search", "page_fetch"},
		"research": {"source_search", "docs_search", "page_fetch", "site_map", "repo_wiki"},
		"docs":     {"docs_search", "page_fetch"},
		"crawl":    {"site_map", "site_crawl", "page_fetch"},
	}
}

func defaultRuntimeRoutes() map[string][]string {
	return map[string][]string{
		"answer_search":   {"xai", "openai_compatible", "openai_responses"},
		"source_search":   {"exa", "zhipu", "tavily", "firecrawl"},
		"docs_search":     {"exa", "context7"},
		"page_fetch":      {"tavily", "firecrawl"},
		"site_map":        {"tavily", "firecrawl"},
		"site_crawl":      {"tavily", "firecrawl"},
		"repo_wiki":       {"deepwiki"},
		"vertical_search": {"anysearch"},
	}
}

func defaultRuntimeProfiles() map[string]ProfileConfig {
	return map[string]ProfileConfig{
		"standard": {ID: "standard", RequiredCapabilities: []string{"answer_search", "docs_search", "page_fetch"}, OptionalCapabilities: []string{"source_search", "site_map", "site_crawl", "repo_wiki", "vertical_search"}},
		"minimal":  {ID: "minimal", RequiredCapabilities: []string{"answer_search"}, OptionalCapabilities: []string{"source_search", "docs_search", "page_fetch"}},
		"off":      {ID: "off", RequiredCapabilities: []string{}, OptionalCapabilities: []string{}},
	}
}

func defaultProviders() map[string]ProviderDefinition {
	return map[string]ProviderDefinition{
		"xai": {
			ID:           "xai",
			Adapter:      AdapterXAIResponses,
			Capabilities: []string{"answer_search"},
			BaseURL:      DefaultXAIURL,
			APIKeyEnv:    "XAI_API_KEY",
			Enabled:      false,
			Settings:     map[string]any{"model": DefaultXAIModel, "tools": []string{"web_search", "x_search"}},
			Aliases:      []string{"xai-responses", "grok", "grok-web-tools"},
		},
		"openai_compatible": {
			ID:           "openai_compatible",
			Adapter:      AdapterOpenAIChatCompletion,
			Capabilities: []string{"answer_search"},
			BaseURL:      "https://api.openai.com/v1",
			APIKeyEnv:    "OPENAI_COMPATIBLE_API_KEY",
			Enabled:      false,
			Settings:     map[string]any{"model": "gpt-4.1", "stream": false, "tools": []string{}, "tool_choice": ""},
			Aliases:      []string{"openai-compatible", "openai", "chat-completions", "chat_completions", "primary"},
		},
		"openai_responses": {
			ID:           "openai_responses",
			Adapter:      AdapterOpenAIResponses,
			Capabilities: []string{"answer_search"},
			BaseURL:      "https://api.openai.com/v1",
			APIKeyEnv:    "OPENAI_API_KEY",
			Enabled:      false,
			Settings:     map[string]any{"model": "gpt-4.1"},
			Aliases:      []string{"openai-responses", "responses"},
		},
		"exa": {
			ID:           "exa",
			Adapter:      AdapterExa,
			Capabilities: []string{"source_search", "docs_search", "page_fetch"},
			BaseURL:      DefaultExaBaseURL,
			APIKeyEnv:    "EXA_API_KEY",
			Enabled:      false,
			Settings:     map[string]any{"timeout_seconds": 30},
		},
		"context7": {
			ID:           "context7",
			Adapter:      AdapterContext7,
			Capabilities: []string{"docs_search"},
			BaseURL:      DefaultContext7BaseURL,
			APIKeyEnv:    "CONTEXT7_API_KEY",
			Enabled:      false,
			Settings:     map[string]any{"timeout_seconds": 30},
			Aliases:      []string{"ctx7"},
		},
		"zhipu": {
			ID:           "zhipu",
			Adapter:      AdapterZhipuWebSearch,
			Capabilities: []string{"source_search"},
			BaseURL:      DefaultZhipuAPIURL,
			APIKeyEnv:    "ZHIPU_API_KEY",
			Enabled:      false,
			Settings:     map[string]any{"protocol_profile": "bigmodel_cn", "search_engine": DefaultZhipuSearchEngine, "search_intent": false, "timeout_seconds": 30},
			Aliases:      []string{"zp", "zai"},
		},
		"tavily": {
			ID:           "tavily",
			Adapter:      AdapterTavily,
			Capabilities: []string{"source_search", "page_fetch", "site_map", "site_crawl"},
			BaseURL:      DefaultTavilyAPIURL,
			APIKeyEnv:    "TAVILY_API_KEY",
			Enabled:      false,
			Settings:     map[string]any{"timeout_seconds": 30},
		},
		"firecrawl": {
			ID:           "firecrawl",
			Adapter:      AdapterFirecrawl,
			Capabilities: []string{"source_search", "page_fetch", "site_map", "site_crawl"},
			BaseURL:      DefaultFirecrawlAPIURL,
			APIKeyEnv:    "FIRECRAWL_API_KEY",
			Enabled:      false,
			Settings:     map[string]any{"timeout_seconds": 30},
		},
		"ddg": {
			ID:           "ddg",
			Adapter:      AdapterMCPStdio,
			Capabilities: []string{"source_search", "page_fetch"},
			Enabled:      false,
			Settings: map[string]any{
				"direct_only":       true,
				"anonymous_allowed": true,
				"timeout_seconds":   60,
				"command":           "uvx",
				"args":              []string{"duckduckgo-mcp-server", "--transport", "stdio"},
				"env": map[string]string{
					"DDG_SAFE_SEARCH": "MODERATE",
					"DDG_REGION":      "cn-zh",
				},
				"tools": map[string]string{
					"search":        "search",
					"fetch_content": "fetch_content",
				},
			},
			Aliases: []string{"ddg-search", "duckduckgo", "duckduckgo-mcp"},
		},
		"freecrawl": {
			ID:           "freecrawl",
			Adapter:      AdapterMCPStdio,
			Capabilities: []string{"source_search", "page_fetch", "site_crawl"},
			Enabled:      false,
			Settings: map[string]any{
				"direct_only":       true,
				"anonymous_allowed": true,
				"upstream_ref":      "pypi:freecrawl-mcp==0.1.2",
				"timeout_seconds":   160,
				"command":           "uvx",
				"args":              []string{"freecrawl-mcp==0.1.2"},
				"env": map[string]string{
					"FREECRAWL_TRANSPORT": "stdio",
					"FREECRAWL_HEADLESS":  "true",
					"FREECRAWL_TIMEOUT":   "160",
					"PYTHONIOENCODING":    "utf-8",
				},
				"tools": map[string]string{
					"scrape": "freecrawl_scrape",
				},
			},
			Aliases: []string{"freecrawl-mcp"},
		},
		"deepwiki": {
			ID:           "deepwiki",
			Adapter:      "deepwiki",
			Capabilities: []string{"repo_wiki"},
			BaseURL:      DefaultDeepWikiAPIURL,
			APIKeyEnv:    "DEEPWIKI_API_KEY",
			Enabled:      true,
			Settings:     map[string]any{"timeout_seconds": 30, "anonymous_allowed": true},
		},
		"anysearch": {
			ID:           "anysearch",
			Adapter:      AdapterAnySearchMCP,
			Capabilities: []string{"vertical_search", "page_fetch"},
			BaseURL:      DefaultAnySearchAPIURL,
			APIKeyEnv:    "ANYSEARCH_API_KEY",
			Enabled:      true,
			Settings:     map[string]any{"timeout_seconds": 30, "anonymous_allowed": true},
			Aliases:      []string{"anysearch-mcp"},
		},
	}
}

func providerSchema(providers map[string]ProviderDefinition) map[string]any {
	out := map[string]any{}
	for id, provider := range providers {
		out[id] = map[string]any{
			"enabled":      provider.Enabled,
			"adapter":      provider.Adapter,
			"capabilities": append([]string{}, provider.Capabilities...),
			"base_url":     provider.BaseURL,
			"api_key":      provider.APIKey,
			"api_key_env":  provider.APIKeyEnv,
			"settings":     copyAnyMap(provider.Settings),
			"aliases":      append([]string{}, provider.Aliases...),
		}
	}
	return out
}

func routesWithProviderCapabilities(routes map[string][]string, providers map[string]ProviderDefinition) map[string][]string {
	out := copyStringListMap(routes)
	providerIDs := make([]string, 0, len(providers))
	for id := range providers {
		providerIDs = append(providerIDs, id)
	}
	sort.Strings(providerIDs)
	for _, id := range providerIDs {
		provider := providers[id]
		directOnly := boolSetting(provider.Settings, "direct_only", false)
		for _, capability := range provider.Capabilities {
			capability = V2CapabilityName(strings.TrimSpace(capability))
			if capability == "" || containsString(out[capability], id) {
				continue
			}
			if directOnly {
				continue
			}
			out[capability] = append(out[capability], id)
		}
	}
	return out
}

func profileSchema(profiles map[string]ProfileConfig) map[string]any {
	out := map[string]any{}
	for id, profile := range profiles {
		out[id] = map[string]any{
			"required_capabilities": append([]string{}, profile.RequiredCapabilities...),
			"optional_capabilities": append([]string{}, profile.OptionalCapabilities...),
		}
	}
	return out
}

func mergeProvider(id string, base ProviderDefinition, data map[string]any) ProviderDefinition {
	base.ID = id
	if value := stringFromAny(data["adapter"]); value != "" {
		base.Adapter = value
	}
	if items := stringListFromAny(data["capabilities"]); len(items) > 0 {
		base.Capabilities = items
	}
	if value := stringFromAny(data["base_url"]); value != "" {
		base.BaseURL = value
	}
	if _, ok := data["api_key"]; ok {
		base.APIKey = stringFromAny(data["api_key"])
	}
	if value := stringFromAny(data["api_key_env"]); value != "" {
		base.APIKeyEnv = value
	}
	if value, ok := data["enabled"]; ok {
		base.Enabled = normalizeEnabled(value)
	}
	if settings, ok := data["settings"].(map[string]any); ok {
		base.Settings = mergeSettingsMap(base.Settings, settings)
	}
	if items := stringListFromAny(data["aliases"]); len(items) > 0 {
		base.Aliases = items
	}
	return base
}

func mergeProfiles(base map[string]ProfileConfig, raw map[string]any) map[string]ProfileConfig {
	out := copyProfiles(base)
	for id, value := range raw {
		data := mapValue(value)
		profile := out[id]
		profile.ID = id
		if items := stringListFromAny(data["required_capabilities"]); items != nil {
			profile.RequiredCapabilities = items
		}
		if items := stringListFromAny(data["optional_capabilities"]); items != nil {
			profile.OptionalCapabilities = items
		}
		out[id] = profile
	}
	return out
}

func mergeStringListMap(base map[string][]string, raw map[string]any) map[string][]string {
	out := copyStringListMap(base)
	for key, value := range raw {
		out[key] = stringListFromAny(value)
	}
	return out
}

func copyStringListMap(input map[string][]string) map[string][]string {
	out := map[string][]string{}
	for key, value := range input {
		out[key] = append([]string{}, value...)
	}
	return out
}

func copyProfiles(input map[string]ProfileConfig) map[string]ProfileConfig {
	out := map[string]ProfileConfig{}
	for key, value := range input {
		value.RequiredCapabilities = append([]string{}, value.RequiredCapabilities...)
		value.OptionalCapabilities = append([]string{}, value.OptionalCapabilities...)
		out[key] = value
	}
	return out
}

func mergeAnyMap(base map[string]any, override map[string]any) map[string]any {
	out := copyAnyMap(base)
	for key, value := range override {
		out[key] = value
	}
	return out
}

func mergeSettingsMap(base map[string]any, override map[string]any) map[string]any {
	out := copyAnyMap(base)
	for key, value := range override {
		if isEmptySetting(value) {
			continue
		}
		out[key] = value
	}
	return out
}

func isEmptySetting(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(typed) == ""
	case []string:
		return len(nonEmptyStringList(typed)) == 0
	case []any:
		return len(stringListFromAny(typed)) == 0
	case map[string]any:
		return len(typed) == 0
	default:
		return false
	}
}

func nonEmptyStringList(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if text := strings.TrimSpace(item); text != "" {
			out = append(out, text)
		}
	}
	return out
}

func copyAnyMap(input map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range input {
		out[key] = value
	}
	return out
}

func settingsForOutput(input map[string]any) map[string]any {
	out := copyAnyMap(input)
	if env := stringMapFromAny(out["env"]); len(env) > 0 {
		masked := map[string]any{}
		for key := range env {
			masked[key] = "********"
		}
		out["env"] = masked
	}
	return out
}

func mapValue(value any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return map[string]any{}
}

func stringListFromAny(value any) []string {
	switch items := value.(type) {
	case nil:
		return nil
	case []string:
		return append([]string{}, items...)
	case []any:
		out := make([]string, 0, len(items))
		for _, item := range items {
			if text := strings.TrimSpace(fmt.Sprint(item)); text != "" {
				out = append(out, text)
			}
		}
		return out
	case string:
		var out []string
		for _, item := range strings.Split(items, ",") {
			if text := strings.TrimSpace(item); text != "" {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}

func stringFromAny(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func intFromAny(value any, fallback int) int {
	switch typed := value.(type) {
	case int:
		return typed
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

func floatFromAny(value any, fallback float64) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case string:
		var parsed float64
		if _, err := fmt.Sscanf(strings.TrimSpace(typed), "%f", &parsed); err == nil {
			return parsed
		}
	}
	return fallback
}

func boolFromAny(value any, fallback bool) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "true", "1", "yes", "on":
			return true
		case "false", "0", "no", "off":
			return false
		}
	}
	return fallback
}

func normalizeEnabled(value any) any {
	if value == nil {
		return "auto"
	}
	if b, ok := value.(bool); ok {
		return b
	}
	switch strings.ToLower(strings.TrimSpace(fmt.Sprint(value))) {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	default:
		return "auto"
	}
}

func providerMatches(id string, aliases []string, filter string) bool {
	filter = strings.TrimSpace(filter)
	if filter == "" || strings.EqualFold(filter, "auto") {
		return true
	}
	names := map[string]struct{}{
		strings.ToLower(id): {},
		strings.ReplaceAll(strings.ToLower(id), "_", "-"): {},
	}
	for _, alias := range aliases {
		alias = strings.ToLower(strings.TrimSpace(alias))
		if alias == "" {
			continue
		}
		names[alias] = struct{}{}
		names[strings.ReplaceAll(alias, "-", "_")] = struct{}{}
	}
	for _, token := range strings.Split(filter, ",") {
		token = strings.ToLower(strings.TrimSpace(token))
		if token == "" {
			continue
		}
		if _, ok := names[token]; ok {
			return true
		}
		if _, ok := names[strings.ReplaceAll(token, "-", "_")]; ok {
			return true
		}
	}
	return false
}

func sortedRouteKeys(routes map[string][]string) []string {
	keys := make([]string, 0, len(routes))
	for key := range routes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func firstResolved(items []ResolvedProvider, id string) ResolvedProvider {
	for _, item := range items {
		if item.ID == id {
			return item
		}
	}
	return ResolvedProvider{}
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func containsCapability(items []string, target string) bool {
	target = V2CapabilityName(strings.TrimSpace(target))
	for _, item := range items {
		if V2CapabilityName(strings.TrimSpace(item)) == target {
			return true
		}
	}
	return false
}

func boolSetting(settings map[string]any, key string, fallback bool) bool {
	if value, ok := settings[key]; ok {
		return boolFromAny(value, fallback)
	}
	return fallback
}

func mcpStdioAvailabilityReason(provider ProviderDefinition, capability string) string {
	command := strings.TrimSpace(stringFromAny(provider.Settings["command"]))
	if command == "" {
		return "missing_command"
	}
	if _, err := exec.LookPath(command); err != nil {
		return "missing_command"
	}
	tools := stringMapFromAny(provider.Settings["tools"])
	if len(tools) == 0 {
		return "missing_tool_mapping"
	}
	for _, key := range mcpStdioCapabilityToolKeys(capability) {
		if strings.TrimSpace(tools[key]) != "" {
			return ""
		}
	}
	if len(mcpStdioCapabilityToolKeys(capability)) == 0 {
		return ""
	}
	return "missing_tool_mapping"
}

func mcpStdioCapabilityToolKeys(capability string) []string {
	switch V2CapabilityName(capability) {
	case "source_search":
		return []string{"search"}
	case "page_fetch":
		return []string{"fetch_content", "scrape", "extract"}
	case "site_crawl":
		return []string{"crawl"}
	case "site_map":
		return []string{"map"}
	default:
		return nil
	}
}

func stringMapFromAny(value any) map[string]string {
	switch items := value.(type) {
	case map[string]string:
		out := map[string]string{}
		for key, value := range items {
			if strings.TrimSpace(key) != "" && strings.TrimSpace(value) != "" {
				out[key] = value
			}
		}
		return out
	case map[string]any:
		out := map[string]string{}
		for key, value := range items {
			if strings.TrimSpace(key) != "" && strings.TrimSpace(stringFromAny(value)) != "" {
				out[key] = stringFromAny(value)
			}
		}
		return out
	default:
		return nil
	}
}
