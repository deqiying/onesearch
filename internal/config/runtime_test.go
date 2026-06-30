package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDefaultRuntimeRoutesMatchConfigNewOrder(t *testing.T) {
	runtime := LoadRuntime(testConfig(t, nil))

	want := []string{"exa", "zhipu", "tavily", "firecrawl"}
	if got := runtime.Routes["source_search"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("source_search route = %#v, want %#v", got, want)
	}
}

func TestRuntimeSchemaRoutesDefineProviderOrder(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("CONTEXT7_API_KEY", "context7-secret")
	t.Setenv("EXA_API_KEY", "exa-secret")

	cfg := testConfig(t, map[string]any{
		"schema_version": 1,
		"routes": map[string]any{
			"docs_search": []any{"context7", "exa"},
		},
		"providers": map[string]any{
			"context7": map[string]any{
				"enabled":      "auto",
				"adapter":      "context7",
				"capabilities": []any{"docs_search"},
				"api_key_env":  "CONTEXT7_API_KEY",
			},
			"exa": map[string]any{
				"enabled":      "auto",
				"adapter":      "exa",
				"capabilities": []any{"docs_search"},
				"api_key_env":  "EXA_API_KEY",
			},
		},
	})

	providers := LoadRuntime(cfg).ResolveProviders(cfg, "docs_search", "auto", false)
	if got := providerIDs(providers); !reflect.DeepEqual(got, []string{"context7", "exa"}) {
		t.Fatalf("provider order = %#v", got)
	}
}

func TestProviderCapabilitiesAutoRegisterRoutes(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("OPENAI_API_KEY", "openai-secret")
	cfg := testConfig(t, map[string]any{
		"schema_version": 1,
		"routes": map[string]any{
			"answer_search": []any{},
		},
		"providers": map[string]any{
			"custom_responses": map[string]any{
				"enabled":      true,
				"adapter":      "openai_responses",
				"capabilities": []any{"answer_search"},
				"api_key_env":  "OPENAI_API_KEY",
			},
		},
	})

	providers := LoadRuntime(cfg).ResolveProviders(cfg, "answer_search", "auto", false)
	if got := providerIDs(providers); !reflect.DeepEqual(got, []string{"custom_responses"}) {
		t.Fatalf("provider order = %#v, want custom capability provider", got)
	}
}

func TestDefaultCompatibilityCapabilitiesIncludeExaFetchAndTavilyCrawl(t *testing.T) {
	clearProviderEnv(t)
	cfg := testConfig(t, nil)
	runtime := LoadRuntime(cfg)

	exa := runtime.Providers["exa"]
	if !containsCapability(exa.Capabilities, "page_fetch") {
		t.Fatalf("exa capabilities = %#v, want page_fetch for web_fetch_exa", exa.Capabilities)
	}
	tavily := runtime.Providers["tavily"]
	if !containsCapability(tavily.Capabilities, "site_crawl") {
		t.Fatalf("tavily capabilities = %#v, want site_crawl for tavily_crawl", tavily.Capabilities)
	}
	if got := runtime.Routes["site_crawl"]; !reflect.DeepEqual(got, []string{"tavily", "firecrawl"}) {
		t.Fatalf("site_crawl route = %#v, want tavily then firecrawl", got)
	}
}

func TestOpenAIResponsesAdapterIsSupported(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("OPENAI_API_KEY", "openai-secret")
	cfg := testConfig(t, map[string]any{
		"schema_version": 1,
		"routes": map[string]any{
			"answer_search": []any{"openai_responses"},
		},
		"providers": map[string]any{
			"openai_responses": map[string]any{
				"enabled":      true,
				"adapter":      "openai_responses",
				"capabilities": []any{"answer_search"},
				"api_key_env":  "OPENAI_API_KEY",
			},
		},
	})

	provider := LoadRuntime(cfg).ResolveProviders(cfg, "answer_search", "auto", true)[0]
	if !provider.Available || provider.UnavailableReason == "unsupported_adapter" {
		t.Fatalf("openai_responses provider = %#v", provider)
	}
}

func TestProviderSettingsEmptyValuesKeepDefaults(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("OPENAI_API_KEY", "openai-secret")
	cfg := testConfig(t, map[string]any{
		"schema_version": 1,
		"routes": map[string]any{
			"answer_search": []any{"openai_responses"},
		},
		"providers": map[string]any{
			"openai_responses": map[string]any{
				"enabled":      true,
				"adapter":      "openai_responses",
				"capabilities": []any{"answer_search"},
				"api_key_env":  "OPENAI_API_KEY",
				"settings": map[string]any{
					"stream":      "",
					"tools":       []any{},
					"tool_choice": "",
				},
			},
		},
	})

	provider := LoadRuntime(cfg).ResolveProviders(cfg, "answer_search", "auto", true)[0]
	if provider.Settings["stream"] != false {
		t.Fatalf("stream setting = %#v, want default false", provider.Settings["stream"])
	}
	if !reflect.DeepEqual(provider.Settings["tools"], []string{"web_search"}) {
		t.Fatalf("tools setting = %#v, want default web_search", provider.Settings["tools"])
	}
	if provider.Settings["tool_choice"] != "required" {
		t.Fatalf("tool_choice setting = %#v, want required", provider.Settings["tool_choice"])
	}
}

func TestRuntimeProviderAvailabilityReasons(t *testing.T) {
	clearProviderEnv(t)
	cfg := testConfig(t, map[string]any{
		"schema_version": 1,
		"routes": map[string]any{
			"page_fetch": []any{"tavily", "firecrawl"},
		},
		"providers": map[string]any{
			"tavily": map[string]any{
				"enabled":      false,
				"adapter":      "tavily",
				"capabilities": []any{"page_fetch"},
				"api_key_env":  "TAVILY_API_KEY",
			},
			"firecrawl": map[string]any{
				"enabled":      "auto",
				"adapter":      "firecrawl",
				"capabilities": []any{"page_fetch"},
				"api_key_env":  "FIRECRAWL_API_KEY",
			},
		},
	})

	providers := LoadRuntime(cfg).ResolveProviders(cfg, "page_fetch", "auto", true)
	got := []string{providers[0].UnavailableReason, providers[1].UnavailableReason}
	if !reflect.DeepEqual(got, []string{"disabled", "missing_api_key"}) {
		t.Fatalf("unavailable reasons = %#v", got)
	}
	if providers[0].ConfigError || providers[1].ConfigError {
		t.Fatalf("disabled or auto missing key should not be config errors: %#v", providers)
	}
}

func TestEnabledTrueMissingKeyReportsConfigError(t *testing.T) {
	clearProviderEnv(t)
	cfg := testConfig(t, map[string]any{
		"schema_version": 1,
		"routes": map[string]any{
			"docs_search": []any{"exa"},
		},
		"providers": map[string]any{
			"exa": map[string]any{
				"enabled":      true,
				"adapter":      "exa",
				"capabilities": []any{"docs_search"},
				"api_key_env":  "EXA_API_KEY",
			},
		},
	})

	provider := LoadRuntime(cfg).ResolveProviders(cfg, "docs_search", "auto", true)[0]
	if provider.Available || provider.UnavailableReason != "missing_api_key" || !provider.ConfigError {
		t.Fatalf("provider = %#v", provider)
	}
}

func TestProviderAPIKeyCanComeFromConfig(t *testing.T) {
	clearProviderEnv(t)
	cfg := testConfig(t, map[string]any{
		"schema_version": 1,
		"routes": map[string]any{
			"docs_search": []any{"context7"},
		},
		"providers": map[string]any{
			"context7": map[string]any{
				"enabled":      true,
				"adapter":      "context7",
				"capabilities": []any{"docs_search"},
				"api_key":      "config-secret",
				"api_key_env":  "CONTEXT7_API_KEY",
			},
		},
	})

	provider := LoadRuntime(cfg).ResolveProviders(cfg, "docs_search", "auto", true)[0]
	if !provider.Available || provider.APIKey != "config-secret" {
		t.Fatalf("provider should use config api_key: %#v", provider)
	}
}

func TestProviderAPIKeyConfigBeatsEnvironment(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("CONTEXT7_API_KEY", "env-secret")
	cfg := testConfig(t, map[string]any{
		"schema_version": 1,
		"routes": map[string]any{
			"docs_search": []any{"context7"},
		},
		"providers": map[string]any{
			"context7": map[string]any{
				"enabled":      true,
				"adapter":      "context7",
				"capabilities": []any{"docs_search"},
				"api_key":      "config-secret",
				"api_key_env":  "CONTEXT7_API_KEY",
			},
		},
	})

	provider := LoadRuntime(cfg).ResolveProviders(cfg, "docs_search", "auto", true)[0]
	if provider.APIKey != "config-secret" {
		t.Fatalf("api_key should beat api_key_env, provider = %#v", provider)
	}
}

func TestProviderOutputMasksConfigAPIKey(t *testing.T) {
	clearProviderEnv(t)
	cfg := testConfig(t, map[string]any{
		"schema_version": 1,
		"routes": map[string]any{
			"docs_search": []any{"context7"},
		},
		"providers": map[string]any{
			"context7": map[string]any{
				"enabled":      true,
				"adapter":      "context7",
				"capabilities": []any{"docs_search"},
				"api_key":      "config-secret",
				"api_key_env":  "CONTEXT7_API_KEY",
			},
		},
	})

	out := LoadRuntime(cfg).ProvidersForOutput(cfg)
	provider := out["context7"].(map[string]any)
	if provider["api_key"] == "config-secret" {
		t.Fatalf("provider output leaked api_key: %#v", provider)
	}
	if provider["api_key"] == "" || provider["api_key_set"] != true || provider["api_key_src"] != "config" || provider["has_api_key"] != true {
		t.Fatalf("provider output should show masked config key status: %#v", provider)
	}
}

func TestAnonymousProviderCanBeAvailableWithoutKey(t *testing.T) {
	clearProviderEnv(t)
	cfg := testConfig(t, map[string]any{
		"schema_version": 1,
		"routes": map[string]any{
			"vertical_search": []any{"anon"},
		},
		"providers": map[string]any{
			"anon": map[string]any{
				"enabled":      "auto",
				"adapter":      "anysearch_mcp",
				"capabilities": []any{"vertical_search"},
				"api_key_env":  "ANYSEARCH_API_KEY",
				"settings": map[string]any{
					"anonymous_allowed": true,
				},
			},
		},
	})

	provider := LoadRuntime(cfg).ResolveProviders(cfg, "vertical_search", "auto", true)[0]
	if !provider.Available {
		t.Fatalf("anonymous provider should be available: %#v", provider)
	}
}

func TestDefaultDeepWikiAllowsPublicReposWithoutKey(t *testing.T) {
	clearProviderEnv(t)
	cfg := testConfig(t, nil)
	runtime := LoadRuntime(cfg)

	provider := runtime.ResolveProviders(cfg, "repo_wiki", "auto", true)[0]
	if !provider.Available {
		t.Fatalf("deepwiki should be available for public repos without key: %#v", provider)
	}
	if provider.BaseURL != DefaultDeepWikiAPIURL {
		t.Fatalf("deepwiki base_url = %#v, want %#v", provider.BaseURL, DefaultDeepWikiAPIURL)
	}
	if provider.APIKey != "" {
		t.Fatalf("deepwiki api key should be empty when env is unset")
	}
}

func TestDefaultDeepWikiUsesOptionalAPIKeyWhenConfigured(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("DEEPWIKI_API_KEY", "deepwiki-secret")
	cfg := testConfig(t, nil)

	provider := LoadRuntime(cfg).ResolveProviders(cfg, "repo_wiki", "auto", true)[0]
	if !provider.Available || provider.APIKey != "deepwiki-secret" {
		t.Fatalf("deepwiki should keep optional API key for private docs: %#v", provider)
	}
}

func TestEnsureInitializedCreatesRuntimeSchema(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{ConfigFile: filepath.Join(dir, "config.json"), ConfigDirSource: "test"}

	missing, created, err := cfg.EnsureInitialized()
	if err != nil {
		t.Fatal(err)
	}
	if !missing {
		t.Fatal("config should start missing")
	}
	if !created {
		t.Fatal("config should be created")
	}

	data := cfg.LoadFile()
	if !isRuntimeSchema(data) {
		t.Fatalf("generated config is not runtime schema: %#v", data)
	}
	if data["schema_version"] != float64(1) {
		t.Fatalf("schema_version = %#v", data["schema_version"])
	}
	if _, ok := data["providers"].(map[string]any)["xai"]; !ok {
		t.Fatalf("generated config missing xai provider: %#v", data["providers"])
	}
	providers := data["providers"].(map[string]any)
	for _, id := range []string{"xai", "openai_compatible", "exa", "context7", "zhipu", "tavily", "firecrawl"} {
		provider := providers[id].(map[string]any)
		if provider["enabled"] != false {
			t.Fatalf("%s enabled = %#v, want false", id, provider["enabled"])
		}
	}
	deepwiki := providers["deepwiki"].(map[string]any)
	if deepwiki["enabled"] != true {
		t.Fatalf("deepwiki enabled = %#v, want true", deepwiki["enabled"])
	}
	if deepwiki["base_url"] != DefaultDeepWikiAPIURL {
		t.Fatalf("deepwiki base_url = %#v, want %#v", deepwiki["base_url"], DefaultDeepWikiAPIURL)
	}
	settings := deepwiki["settings"].(map[string]any)
	if settings["anonymous_allowed"] != true {
		t.Fatalf("deepwiki anonymous_allowed = %#v, want true", settings["anonymous_allowed"])
	}
	anysearch := providers["anysearch"].(map[string]any)
	if anysearch["enabled"] != true {
		t.Fatalf("anysearch enabled = %#v, want true", anysearch["enabled"])
	}
}

func TestEnsureInitializedDoesNotOverwriteExistingConfig(t *testing.T) {
	cfg := testConfig(t, map[string]any{"custom": "value"})

	missing, created, err := cfg.EnsureInitialized()
	if err != nil {
		t.Fatal(err)
	}
	if missing {
		t.Fatal("existing config should not be missing")
	}
	if created {
		t.Fatal("existing config should not be recreated")
	}

	data := cfg.LoadFile()
	if data["custom"] != "value" {
		t.Fatalf("existing config was overwritten: %#v", data)
	}
}

func testConfig(t *testing.T, data map[string]any) *Config {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if data != nil {
		body, err := json.Marshal(data)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return &Config{ConfigFile: path, ConfigDirSource: "test"}
}

func clearProviderEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"XAI_API_KEY",
		"OPENAI_COMPATIBLE_API_KEY",
		"OPENAI_API_KEY",
		"EXA_API_KEY",
		"CONTEXT7_API_KEY",
		"ZHIPU_API_KEY",
		"TAVILY_API_KEY",
		"FIRECRAWL_API_KEY",
		"ANYSEARCH_API_KEY",
		"DEEPWIKI_API_KEY",
	} {
		t.Setenv(key, "")
	}
}

func providerIDs(items []ResolvedProvider) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.ID)
	}
	return out
}
