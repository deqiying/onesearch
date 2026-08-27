package service

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/deqiying/onesearch/internal/config"
	"github.com/deqiying/onesearch/internal/output"
	"github.com/deqiying/onesearch/internal/providers"
)

func TestDoctorUsesRuntimeSchemaCapabilityNames(t *testing.T) {
	clearProviderEnv(t)
	svc := New(testConfig(t))

	data := svc.Doctor(t.Context())
	wantKeys := []string{"config", "effective_environment", "elapsed_ms", "error", "error_type", "issues", "minimum_profile", "ok", "schema", "status"}
	if got := sortedMapKeys(data); !reflect.DeepEqual(got, wantKeys) {
		t.Fatalf("doctor top-level keys = %#v, want %#v", got, wantKeys)
	}
	schema, ok := data["schema"].(map[string]any)
	if !ok {
		t.Fatalf("schema has type %T", data["schema"])
	}
	if schema["source"] != "builtin" {
		t.Fatalf("schema.source = %#v", schema["source"])
	}
	for _, legacy := range []string{
		"config_schema_source",
		"config_schema_version",
		"config_sources",
		"config_status",
		"provider_availability",
		"capability_status",
		"capability_status_v2",
		"minimum_profile_ok",
		"answer_search_connection_tests",
		"primary_api_mode",
		"defaults",
		"pipelines",
		"profiles",
		"providers",
		"routes",
		"capabilities",
	} {
		if _, ok := data[legacy]; ok {
			t.Fatalf("doctor should not expose legacy field %s", legacy)
		}
	}
	minimum, ok := data["minimum_profile"].(map[string]any)
	if !ok {
		t.Fatalf("minimum_profile has type %T", data["minimum_profile"])
	}
	if _, ok := minimum["capabilities"]; ok {
		t.Fatalf("minimum_profile should not duplicate full capability diagnostics: %#v", minimum)
	}
	if got := testStrings(minimum["missing"]); !reflect.DeepEqual(got, []string{"answer_search", "docs_search"}) {
		t.Fatalf("minimum missing = %#v", got)
	}
	issues, ok := data["issues"].([]map[string]any)
	if !ok {
		t.Fatalf("issues has type %T", data["issues"])
	}
	if len(issues) != 2 {
		t.Fatalf("issues len = %d, issues = %#v", len(issues), issues)
	}
	for _, issue := range issues {
		if issue["type"] != "missing_required_capability" {
			t.Fatalf("issue type = %#v, issue = %#v", issue["type"], issue)
		}
		if issue["reason"] != "no_available_provider" {
			t.Fatalf("issue reason = %#v, issue = %#v", issue["reason"], issue)
		}
		providers, ok := issue["providers"].([]map[string]any)
		if !ok {
			t.Fatalf("issue providers has type %T", issue["providers"])
		}
		for _, provider := range providers {
			for _, allowed := range []string{"provider", "reason", "config_error"} {
				if _, ok := provider[allowed]; !ok {
					t.Fatalf("provider diagnostic missing %s: %#v", allowed, provider)
				}
			}
			if len(provider) != 3 {
				t.Fatalf("provider diagnostic should stay compact: %#v", provider)
			}
		}
	}

	routes, ok := data["routes"].(map[string][]string)
	if ok {
		t.Fatalf("doctor should not include full routes: %#v", routes)
	}
}

func TestStatusReportsCapabilityAndProviderAvailability(t *testing.T) {
	clearProviderEnv(t)
	svc := New(testConfig(t))

	data := svc.Status()
	if data["ok"] != true {
		t.Fatalf("status ok = %#v", data["ok"])
	}
	if data["ready"] != false || data["status"] != "degraded" {
		t.Fatalf("status readiness = ready:%#v status:%#v", data["ready"], data["status"])
	}
	capabilities := data["capabilities"].(map[string]any)
	answer := capabilities["answer_search"].(map[string]any)
	if answer["ok"] != false || answer["command"] != "onesearch search" {
		t.Fatalf("answer_search status = %#v", answer)
	}
	providers := answer["provider_status"].([]map[string]any)
	if len(providers) == 0 || providers[0]["provider"] != "xai" || providers[0]["reason"] != "disabled" {
		t.Fatalf("answer_search provider status = %#v", providers)
	}
	repoWiki := capabilities["repo_wiki"].(map[string]any)
	if repoWiki["ok"] != true || !containsString(testStrings(repoWiki["available"]), "deepwiki") {
		t.Fatalf("repo_wiki status = %#v", repoWiki)
	}
	verticalSearch := capabilities["vertical_search"].(map[string]any)
	if verticalSearch["command"] != "onesearch anysearch search" {
		t.Fatalf("vertical_search command = %#v, want onesearch anysearch search", verticalSearch["command"])
	}
	allProviders := data["providers"].(map[string]any)
	zhipu := allProviders["zhipu"].(map[string]any)
	if zhipu["available"] != false || zhipu["enabled"] != false {
		t.Fatalf("zhipu provider output = %#v", zhipu)
	}
}

func TestSetupProviderWritesCanonicalTargetAndSafeResult(t *testing.T) {
	clearProviderEnv(t)
	cfg := testConfig(t)
	if err := cfg.SetFile(config.InitialRuntimeSchema()); err != nil {
		t.Fatal(err)
	}
	key := "setup-secret"
	baseURL := "https://gateway.example.com/v1/"
	data := New(cfg).SetupProvider(ProviderSetupRequest{
		Provider: "openai-compatible",
		APIKey:   &key,
		BaseURL:  &baseURL,
	})
	if data["ok"] != true || data["provider"] != "openai_compatible" || data["base_url"] != "https://gateway.example.com/v1" {
		t.Fatalf("setup result = %#v", data)
	}
	if data["api_key_src"] != "config" || data["has_api_key"] != true || data["api_key_env_set"] != false {
		t.Fatalf("setup credential status = %#v", data)
	}
	if strings.Contains(fmt.Sprint(data), key) {
		t.Fatalf("setup result leaked key: %#v", data)
	}
	provider := cfg.LoadFile()["providers"].(map[string]any)["openai_compatible"].(map[string]any)
	if provider["api_key"] != key || provider["enabled"] != "auto" || provider["base_url"] != "https://gateway.example.com/v1" {
		t.Fatalf("saved provider = %#v", provider)
	}
}

func TestSetupProviderKeepsExistingKeyOnBlankInput(t *testing.T) {
	clearProviderEnv(t)
	raw := config.InitialRuntimeSchema()
	provider := raw["providers"].(map[string]any)["exa"].(map[string]any)
	provider["api_key"] = "existing-secret"
	cfg := testConfig(t)
	if err := cfg.SetFile(raw); err != nil {
		t.Fatal(err)
	}
	blank := "  "
	data := New(cfg).SetupProvider(ProviderSetupRequest{Provider: "exa", APIKey: &blank})
	if data["ok"] != true || data["api_key_src"] != "config" {
		t.Fatalf("setup result = %#v", data)
	}
	saved := cfg.LoadFile()["providers"].(map[string]any)["exa"].(map[string]any)
	if saved["api_key"] != "existing-secret" {
		t.Fatalf("existing key was changed: %#v", saved)
	}
}

func TestSetupProviderAllowsAnonymousProviderWithoutKeyAndRejectsMCPStdio(t *testing.T) {
	clearProviderEnv(t)
	cfg := testConfig(t)
	if err := cfg.SetFile(config.InitialRuntimeSchema()); err != nil {
		t.Fatal(err)
	}
	anonymous := New(cfg).SetupProvider(ProviderSetupRequest{Provider: "anysearch"})
	if anonymous["ok"] != true || anonymous["has_api_key"] != false || anonymous["enabled"] != "auto" {
		t.Fatalf("anonymous setup = %#v", anonymous)
	}
	mcp := New(cfg).SetupProvider(ProviderSetupRequest{Provider: "ddg"})
	if mcp["ok"] != false || mcp["error_type"] != "parameter_error" {
		t.Fatalf("mcp_stdio setup = %#v", mcp)
	}
}

func TestSetupProviderRejectsUnsupportedAdapterAndMissingRequiredBaseURL(t *testing.T) {
	clearProviderEnv(t)
	for _, tc := range []struct {
		name     string
		provider map[string]any
	}{
		{
			name: "unsupported_adapter",
			provider: map[string]any{
				"enabled":      false,
				"adapter":      "custom_unknown_adapter",
				"capabilities": []any{"source_search"},
			},
		},
		{
			name: "missing_required_base_url",
			provider: map[string]any{
				"enabled":      false,
				"adapter":      "exa",
				"capabilities": []any{"source_search"},
				"settings":     map[string]any{"requires_base_url": true},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testConfig(t)
			if err := cfg.SetFile(map[string]any{
				"schema_version": 1,
				"routes":         map[string]any{"source_search": []any{"custom"}},
				"providers":      map[string]any{"custom": tc.provider},
			}); err != nil {
				t.Fatal(err)
			}
			key := "custom-secret"
			data := New(cfg).SetupProvider(ProviderSetupRequest{Provider: "custom", APIKey: &key})
			if data["ok"] != false || data["error_type"] != "parameter_error" {
				t.Fatalf("setup result = %#v", data)
			}
		})
	}
}

func TestSetupProviderRejectsUnsafeBaseURLWithoutWriting(t *testing.T) {
	clearProviderEnv(t)
	cfg := testConfig(t)
	if err := cfg.SetFile(config.InitialRuntimeSchema()); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(cfg.ConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	key := "transient-secret"
	baseURL := "https://user:password@example.com/v1?key=secret"
	data := New(cfg).SetupProvider(ProviderSetupRequest{Provider: "exa", APIKey: &key, BaseURL: &baseURL})
	if data["ok"] != false || data["error_type"] != "parameter_error" {
		t.Fatalf("unsafe URL result = %#v", data)
	}
	after, err := os.ReadFile(cfg.ConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("unsafe URL changed config file")
	}
}

func TestNormalizeEndpointBaseURLContract(t *testing.T) {
	for _, value := range []string{
		"ftp://example.com",
		"https://user@example.com",
		"https://example.com?",
		"https://example.com?key=value",
		"https://example.com#",
		"https://example.com#fragment",
		"example.com",
	} {
		if _, err := normalizeEndpointBaseURL(value); err == nil {
			t.Fatalf("unsafe base URL %q should fail", value)
		}
	}
	if got, err := normalizeEndpointBaseURL("HTTPS://example.com/v1/"); err != nil || got != "https://example.com/v1" {
		t.Fatalf("normalized base URL = %q, err = %v", got, err)
	}
}

func TestStatusAndDoctorReportOnlyEffectiveEnvironmentNames(t *testing.T) {
	clearProviderEnv(t)
	raw := config.InitialRuntimeSchema()
	exa := raw["providers"].(map[string]any)["exa"].(map[string]any)
	exa["enabled"] = "auto"
	cfg := testConfig(t)
	cfg.ConfigDirSource = "environment"
	if err := cfg.SetFile(raw); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EXA_API_KEY", "environment-secret")

	svc := New(cfg)
	status := svc.Status()
	configInfo := status["config"].(map[string]any)
	if configInfo["file"] != cfg.ConfigFile || configInfo["dir_source"] != "environment" || configInfo["dir_env"] != config.ConfigDirEnvName {
		t.Fatalf("config diagnostics = %#v", configInfo)
	}
	provider := status["providers"].(map[string]any)["exa"].(map[string]any)
	if provider["api_key_env_set"] != true || provider["api_key_src"] != "env" || provider["has_api_key"] != true {
		t.Fatalf("provider environment status = %#v", provider)
	}
	if !hasEffectiveEnvironment(status["effective_environment"], config.ConfigDirEnvName, "config_dir", "") || !hasEffectiveEnvironment(status["effective_environment"], "EXA_API_KEY", "provider_api_key", "exa") {
		t.Fatalf("effective environment = %#v", status["effective_environment"])
	}
	if strings.Contains(fmt.Sprint(status), "environment-secret") {
		t.Fatalf("status leaked environment value: %#v", status)
	}
	doctor := svc.Doctor(t.Context())
	if _, ok := doctor["providers"]; ok {
		t.Fatal("doctor should remain compact")
	}
	if !hasEffectiveEnvironment(doctor["effective_environment"], "EXA_API_KEY", "provider_api_key", "exa") || strings.Contains(fmt.Sprint(doctor), "environment-secret") {
		t.Fatalf("doctor environment diagnostics = %#v", doctor)
	}
}

func TestEffectiveEnvironmentExcludesOverriddenAndDisabledProviderKeys(t *testing.T) {
	clearProviderEnv(t)
	raw := config.InitialRuntimeSchema()
	providers := raw["providers"].(map[string]any)
	exa := providers["exa"].(map[string]any)
	exa["enabled"] = "auto"
	exa["api_key"] = "direct-secret"
	tavily := providers["tavily"].(map[string]any)
	tavily["enabled"] = false
	cfg := testConfig(t)
	if err := cfg.SetFile(raw); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EXA_API_KEY", "overridden-env-secret")
	t.Setenv("TAVILY_API_KEY", "disabled-env-secret")

	status := New(cfg).Status()
	if hasEffectiveEnvironment(status["effective_environment"], "EXA_API_KEY", "provider_api_key", "exa") || hasEffectiveEnvironment(status["effective_environment"], "TAVILY_API_KEY", "provider_api_key", "tavily") {
		t.Fatalf("inactive provider variables should be excluded: %#v", status["effective_environment"])
	}
	exaStatus := status["providers"].(map[string]any)["exa"].(map[string]any)
	if exaStatus["api_key_env_set"] != true || exaStatus["api_key_src"] != "config" {
		t.Fatalf("override diagnostics = %#v", exaStatus)
	}
}

func TestOutputSecretValuesIncludesOverriddenEnvironmentAndSensitiveSettings(t *testing.T) {
	clearProviderEnv(t)
	raw := config.InitialRuntimeSchema()
	exa := raw["providers"].(map[string]any)["exa"].(map[string]any)
	exa["api_key"] = "direct-secret"
	ddg := raw["providers"].(map[string]any)["ddg"].(map[string]any)
	settings := ddg["settings"].(map[string]any)
	settings["env"] = map[string]any{"ACCESS_TOKEN": "stdio-secret", "KEY": "generic-key-secret", "MODE": "true"}
	cfg := testConfig(t)
	if err := cfg.SetFile(raw); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EXA_API_KEY", "overridden-env-secret")
	values := New(cfg).OutputSecretValues()
	for _, want := range []string{"direct-secret", "overridden-env-secret", "stdio-secret", "generic-key-secret"} {
		if !containsString(values, want) {
			t.Fatalf("secret inventory missing %q: %#v", want, values)
		}
	}
	if containsString(values, "true") {
		t.Fatalf("generic settings env value should not enter literal replacement inventory: %#v", values)
	}
}

func TestOutputSecretValuesFollowsAPIKeyEnvInUnrecognizedSchema(t *testing.T) {
	cfg := testConfig(t)
	if err := cfg.SetFile(map[string]any{
		"providers": map[string]any{
			"custom": map[string]any{"api_key_env": "CUSTOM_API_KEY"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CUSTOM_API_KEY", "custom-environment-secret")
	if values := New(cfg).OutputSecretValues(); !containsString(values, "custom-environment-secret") {
		t.Fatalf("secret inventory = %#v", values)
	}
}

func hasEffectiveEnvironment(value any, name, purpose, provider string) bool {
	items, ok := value.([]map[string]any)
	if !ok {
		return false
	}
	for _, item := range items {
		itemProvider := ""
		if value, ok := item["provider"]; ok {
			itemProvider = fmt.Sprint(value)
		}
		if item["name"] == name && item["purpose"] == purpose && itemProvider == provider {
			return true
		}
	}
	return false
}

func TestDDGDirectUsesMCPStdioProvider(t *testing.T) {
	clearProviderEnv(t)
	cfg := testConfig(t)
	if err := cfg.SetFile(map[string]any{
		"schema_version": 1,
		"routes":         map[string]any{},
		"providers": map[string]any{
			"ddg": mcpStdioTestProvider("ddg", []any{"source_search", "page_fetch"}, map[string]any{
				"search":        "search",
				"fetch_content": "fetch_content",
			}),
		},
	}); err != nil {
		t.Fatal(err)
	}

	got := New(cfg).DDGSearch(t.Context(), "golang", providers.DDGSearchOptions{MaxResults: 2})
	if got["ok"] != true || got["provider"] != "ddg" || got["query"] != "golang" {
		t.Fatalf("ddg search = %#v", got)
	}
	results := got["results"].([]map[string]any)
	if len(results) != 1 || results[0]["url"] != "https://example.com/golang" {
		t.Fatalf("ddg results = %#v", results)
	}
}

func TestExplicitRouteUsesDDGMCPStdioSourceProvider(t *testing.T) {
	clearProviderEnv(t)
	cfg := testConfig(t)
	if err := cfg.SetFile(map[string]any{
		"schema_version": 1,
		"routes": map[string]any{
			"source_search": []any{"ddg"},
		},
		"providers": map[string]any{
			"ddg": mcpStdioTestProvider("ddg", []any{"source_search"}, map[string]any{"search": "search"}),
		},
	}); err != nil {
		t.Fatal(err)
	}

	provider := config.LoadRuntime(cfg).ResolveProviders(cfg, "source_search", "auto", false)[0]
	results, err := New(cfg).searchWithSourceProvider(t.Context(), provider, "golang", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0]["provider"] != "ddg" || results[0]["url"] != "https://example.com/golang" {
		t.Fatalf("results = %#v", results)
	}
}

func TestDeepPlanCurrentChineseUsesStatusGatedSearchRoute(t *testing.T) {
	clearProviderEnv(t)
	svc := New(testConfig(t))

	data := svc.DeepPlan("总结今天微博前十的热搜", "standard", t.TempDir())
	rawSteps, err := json.Marshal(data["steps"])
	if err != nil {
		t.Fatal(err)
	}
	steps := string(rawSteps)
	if strings.Contains(steps, "onesearch zhipu search") {
		t.Fatalf("deep plan should not hardcode zhipu direct command: %s", steps)
	}
	if !strings.Contains(steps, "onesearch search") {
		t.Fatalf("deep plan should use runtime search routing: %s", steps)
	}
	if containsString(testStrings(data["allowed_tools"]), "zhipu search") {
		t.Fatalf("deep plan allowed_tools should not advertise ungated zhipu direct command: %#v", data["allowed_tools"])
	}
}

func TestDeepPlanCommandsExposeStructuredArgvAndPresentationString(t *testing.T) {
	clearProviderEnv(t)
	svc := New(testConfig(t))
	query := `PowerShell's $HOME; "quoted" & <tag> --literal`
	data := svc.DeepPlan(query, "deep", filepath.Join(t.TempDir(), "evidence dir's $bucket"))

	groups := []struct {
		name  string
		items []map[string]any
	}{
		{name: "preflight", items: data["preflight"].([]map[string]any)},
		{name: "steps", items: data["steps"].([]map[string]any)},
	}
	sawSpecialToken := false
	for _, group := range groups {
		if len(group.items) == 0 {
			t.Fatalf("%s must not be empty", group.name)
		}
		for _, item := range group.items {
			argv, ok := item["command_argv"].([]string)
			if !ok || len(argv) < 2 || argv[0] != "onesearch" {
				t.Fatalf("%s command_argv = %#v", group.name, item["command_argv"])
			}
			command, ok := item["command"].(string)
			if !ok || command != renderPowerShellCommand(argv) {
				t.Fatalf("%s command = %#v, argv = %#v", group.name, item["command"], argv)
			}
			for _, token := range argv {
				if token != query {
					continue
				}
				sawSpecialToken = true
				if command == strings.Join(argv, " ") || !strings.Contains(command, `'PowerShell''s $HOME; "quoted" & <tag> --literal'`) {
					t.Fatalf("special token was not safely separated from presentation string: argv=%#v command=%q", argv, command)
				}
			}
		}
	}
	if !sawSpecialToken {
		t.Fatalf("query token %q missing from deep command argv", query)
	}
}

func TestAnswerSearchUsesConfiguredCapabilityProviderViaAdapterRegistry(t *testing.T) {
	clearProviderEnv(t)
	cfg := testConfig(t)
	if err := cfg.SetFile(map[string]any{
		"schema_version": 1,
		"routes": map[string]any{
			"answer_search": []any{"openai_responses"},
		},
		"profiles": map[string]any{
			"standard": map[string]any{
				"required_capabilities": []any{"answer_search"},
			},
		},
		"providers": map[string]any{
			"openai_responses": map[string]any{
				"enabled":      true,
				"adapter":      "openai_responses",
				"capabilities": []any{"answer_search"},
				"api_key":      "test-key",
				"settings": map[string]any{
					"model": "gpt-test",
				},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	runners, err := New(cfg).mainProviderConfigs("", "auto", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(runners) != 1 {
		t.Fatalf("runner len = %d, runners = %#v", len(runners), runners)
	}
	if runners[0].Provider != "openai_responses" || runners[0].Mode != "openai-responses" || runners[0].Search == nil {
		t.Fatalf("unexpected runner = %#v", runners[0])
	}
}

func TestOpenAIResponsesRunnerIgnoresLegacySettingsAndStreamOverride(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/alpha/search" {
			t.Fatalf("path = %q, want /v1/alpha/search", r.URL.Path)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["model"] != "gpt-test" {
			t.Fatalf("model = %#v", payload["model"])
		}
		for _, field := range []string{"stream", "tools", "tool_choice", "instructions", "input"} {
			if _, ok := payload[field]; ok {
				t.Fatalf("legacy field %q found in Alpha request: %#v", field, payload)
			}
		}
		_, _ = w.Write([]byte(`{"output":"answer"}`))
	}))
	defer server.Close()

	clearProviderEnv(t)
	cfg := testConfig(t)
	if err := cfg.SetFile(map[string]any{
		"schema_version": 1,
		"routes": map[string]any{
			"answer_search": []any{"openai_responses"},
		},
		"profiles": map[string]any{
			"standard": map[string]any{
				"required_capabilities": []any{"answer_search"},
			},
		},
		"providers": map[string]any{
			"openai_responses": map[string]any{
				"enabled":      true,
				"adapter":      "openai_responses",
				"capabilities": []any{"answer_search"},
				"base_url":     server.URL,
				"api_key":      "test-key",
				"settings": map[string]any{
					"model":       "gpt-test",
					"stream":      true,
					"tools":       []any{},
					"tool_choice": "",
				},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	override := true
	runners, err := New(cfg).mainProviderConfigs("", "auto", &override)
	if err != nil {
		t.Fatal(err)
	}
	if len(runners) != 1 {
		t.Fatalf("runners = %#v", runners)
	}
	if runners[0].Stream || len(runners[0].Tools) != 0 || runners[0].ToolChoice != nil {
		t.Fatalf("legacy settings affected runner = %#v", runners[0])
	}
	got, err := runners[0].Search(t.Context(), "query", "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "answer" {
		t.Fatalf("answer = %q", got)
	}
}

func TestSearchUsesOpenAIAlphaOutputSourcesAndModelOverride(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/alpha/search" {
			t.Fatalf("path = %q, want /v1/alpha/search", r.URL.Path)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["model"] != "gpt-override" {
			t.Fatalf("model = %#v, want gpt-override", payload["model"])
		}
		if _, ok := payload["stream"]; ok {
			t.Fatalf("stream must not be sent: %#v", payload)
		}
		_, _ = w.Write([]byte(`{"output":"alpha answer","results":[{"url":"https://example.com/source","title":"Example"}]}`))
	}))
	defer server.Close()

	clearProviderEnv(t)
	cfg := testConfig(t)
	if err := cfg.SetFile(map[string]any{
		"schema_version": 1,
		"routes": map[string]any{
			"answer_search": []any{"openai_responses"},
		},
		"profiles": map[string]any{
			"standard": map[string]any{
				"required_capabilities": []any{"answer_search"},
			},
		},
		"providers": map[string]any{
			"openai_responses": map[string]any{
				"enabled":      true,
				"adapter":      "openai_responses",
				"capabilities": []any{"answer_search"},
				"base_url":     server.URL,
				"api_key":      "test-key",
				"settings": map[string]any{
					"model":       "gpt-config",
					"stream":      true,
					"tools":       []any{"web_search"},
					"tool_choice": "required",
				},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	stream := true
	got := New(cfg).Search(t.Context(), "plain question", SearchOptions{Model: "gpt-override", Stream: &stream})
	if got["ok"] != true || got["content"] != "alpha answer" || got["model"] != "gpt-override" {
		t.Fatalf("search result = %#v", got)
	}
	wantSources := []map[string]any{{"url": "https://example.com/source", "title": "Example"}}
	if !reflect.DeepEqual(got["sources"], wantSources) {
		t.Fatalf("sources = %#v, want %#v", got["sources"], wantSources)
	}
}

func TestOpenAIAlphaHTTPFailureUsesServiceFallbackWithoutResponsesRetry(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		fallback     string
		wantOK       bool
		wantChatCall int
	}{
		{name: "404_auto", status: http.StatusNotFound, fallback: "auto", wantOK: true, wantChatCall: 1},
		{name: "405_auto", status: http.StatusMethodNotAllowed, fallback: "auto", wantOK: true, wantChatCall: 1},
		{name: "404_off", status: http.StatusNotFound, fallback: "off", wantOK: false, wantChatCall: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alphaCalls := 0
			alpha := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				alphaCalls++
				if r.URL.Path != "/v1/alpha/search" {
					t.Fatalf("unexpected OpenAI path %q", r.URL.Path)
				}
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(`{"error":{"message":"alpha unavailable"}}`))
			}))
			defer alpha.Close()

			chatCalls := 0
			chat := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				chatCalls++
				if r.URL.Path != "/v1/chat/completions" {
					t.Fatalf("chat path = %q", r.URL.Path)
				}
				_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"fallback answer"}}]}`))
			}))
			defer chat.Close()

			clearProviderEnv(t)
			cfg := testConfig(t)
			if err := cfg.SetFile(map[string]any{
				"schema_version": 1,
				"routes": map[string]any{
					"answer_search": []any{"openai_responses", "openai_compatible"},
				},
				"profiles": map[string]any{
					"standard": map[string]any{
						"required_capabilities": []any{"answer_search"},
					},
				},
				"providers": map[string]any{
					"openai_responses": map[string]any{
						"enabled":      true,
						"adapter":      "openai_responses",
						"capabilities": []any{"answer_search"},
						"base_url":     alpha.URL,
						"api_key":      "alpha-key",
						"settings":     map[string]any{"model": "gpt-alpha"},
					},
					"openai_compatible": map[string]any{
						"enabled":      true,
						"adapter":      "openai_chat_completions",
						"capabilities": []any{"answer_search"},
						"base_url":     chat.URL,
						"api_key":      "chat-key",
						"settings":     map[string]any{"model": "gpt-chat"},
					},
				},
			}); err != nil {
				t.Fatal(err)
			}

			got := New(cfg).Search(t.Context(), "plain question", SearchOptions{Fallback: tt.fallback})
			if got["ok"] != tt.wantOK {
				t.Fatalf("ok = %#v, want %v; result=%#v", got["ok"], tt.wantOK, got)
			}
			if tt.wantOK && (got["content"] != "fallback answer" || got["fallback_used"] != true) {
				t.Fatalf("fallback result = %#v", got)
			}
			if alphaCalls != 1 || chatCalls != tt.wantChatCall {
				t.Fatalf("calls: alpha=%d chat=%d; want alpha=1 chat=%d", alphaCalls, chatCalls, tt.wantChatCall)
			}
		})
	}
}

func TestOpenAICompatibleRunnerSupportsOptionalToolsAndToolChoice(t *testing.T) {
	clearProviderEnv(t)
	cfg := testConfig(t)
	if err := cfg.SetFile(map[string]any{
		"schema_version": 1,
		"routes": map[string]any{
			"answer_search": []any{"openai_compatible"},
		},
		"profiles": map[string]any{
			"standard": map[string]any{
				"required_capabilities": []any{"answer_search"},
			},
		},
		"providers": map[string]any{
			"openai_compatible": map[string]any{
				"enabled":      true,
				"adapter":      "openai_chat_completions",
				"capabilities": []any{"answer_search"},
				"api_key":      "test-key",
				"settings": map[string]any{
					"model":       "gpt-test",
					"stream":      true,
					"tools":       []any{map[string]any{"type": "web_search"}},
					"tool_choice": "auto",
				},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	runners, err := New(cfg).mainProviderConfigs("", "auto", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(runners) != 1 || !runners[0].Stream {
		t.Fatalf("runner stream = %#v, want true", runners)
	}
	if !reflect.DeepEqual(runners[0].Tools, []map[string]any{{"type": "web_search"}}) {
		t.Fatalf("runner tools = %#v", runners[0].Tools)
	}
	if runners[0].ToolChoice != "auto" {
		t.Fatalf("runner tool_choice = %#v, want auto", runners[0].ToolChoice)
	}
}

func TestNormalizeRepoNameAcceptsOwnerRepoAndGitHubURL(t *testing.T) {
	for _, item := range map[string]string{
		"microsoft/playwright":                         "microsoft/playwright",
		"https://github.com/microsoft/playwright":      "microsoft/playwright",
		"https://github.com/microsoft/playwright.git":  "microsoft/playwright",
		"https://github.com/microsoft/playwright/tree": "microsoft/playwright",
	} {
		got, err := normalizeRepoName(item)
		if err != nil {
			t.Fatalf("normalizeRepoName(%q) error: %v", item, err)
		}
		if got != "microsoft/playwright" {
			t.Fatalf("normalizeRepoName(%q) = %q, want microsoft/playwright", item, got)
		}
	}
	if _, err := normalizeRepoName("playwright"); err == nil {
		t.Fatal("bare repo name should be rejected")
	}
}

func TestRepoWikiCommandUsesDeepWikiProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"repo answer"}]}}`))
	}))
	defer server.Close()

	clearProviderEnv(t)
	cfg := testConfig(t)
	if err := cfg.SetFile(map[string]any{
		"schema_version": 1,
		"routes": map[string]any{
			"repo_wiki": []any{"deepwiki"},
		},
		"providers": map[string]any{
			"deepwiki": map[string]any{
				"enabled":      true,
				"adapter":      "deepwiki",
				"capabilities": []any{"repo_wiki"},
				"base_url":     server.URL,
				"settings": map[string]any{
					"anonymous_allowed": true,
				},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	got := New(cfg).RepoWiki(t.Context(), "https://github.com/microsoft/playwright", "architecture?", RepoWikiOptions{})
	if got["ok"] != true || got["repo"] != "microsoft/playwright" || got["content"] != "repo answer" {
		t.Fatalf("repo wiki result = %#v", got)
	}
}

func TestSearchCanForceRepoWiki(t *testing.T) {
	deepwiki := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"repo answer"}]}}`))
	}))
	defer deepwiki.Close()
	openai := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/alpha/search" {
			t.Fatalf("path = %q, want /v1/alpha/search", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"output":"answer","results":[]}`))
	}))
	defer openai.Close()

	clearProviderEnv(t)
	cfg := testConfig(t)
	if err := cfg.SetFile(map[string]any{
		"schema_version": 1,
		"routes": map[string]any{
			"answer_search": []any{"openai_responses"},
			"repo_wiki":     []any{"deepwiki"},
		},
		"profiles": map[string]any{
			"standard": map[string]any{
				"required_capabilities": []any{"answer_search"},
			},
		},
		"providers": map[string]any{
			"openai_responses": map[string]any{
				"enabled":      true,
				"adapter":      "openai_responses",
				"capabilities": []any{"answer_search"},
				"base_url":     openai.URL,
				"api_key":      "test-key",
				"settings": map[string]any{
					"model": "gpt-test",
				},
			},
			"deepwiki": map[string]any{
				"enabled":      true,
				"adapter":      "deepwiki",
				"capabilities": []any{"repo_wiki"},
				"base_url":     deepwiki.URL,
				"settings": map[string]any{
					"anonymous_allowed": true,
				},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	got := New(cfg).Search(t.Context(), "explain repo", SearchOptions{RepoWiki: "microsoft/playwright"})
	used := got["used"].([]map[string]any)
	var found bool
	for _, item := range used {
		if item["capability"] == "repo_wiki" {
			found = true
		}
	}
	if !found {
		t.Fatalf("search used missing repo_wiki: %#v", used)
	}
}

func TestSearchForcedRepoWikiIgnoresAnswerProviderFilter(t *testing.T) {
	deepwiki := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"repo answer"}]}}`))
	}))
	defer deepwiki.Close()
	openai := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/alpha/search" {
			t.Fatalf("path = %q, want /v1/alpha/search", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"output":"answer","results":[]}`))
	}))
	defer openai.Close()

	clearProviderEnv(t)
	cfg := testConfig(t)
	if err := cfg.SetFile(map[string]any{
		"schema_version": 1,
		"routes": map[string]any{
			"answer_search": []any{"openai_responses"},
			"repo_wiki":     []any{"deepwiki"},
		},
		"profiles": map[string]any{
			"standard": map[string]any{
				"required_capabilities": []any{"answer_search"},
			},
		},
		"providers": map[string]any{
			"openai_responses": map[string]any{
				"enabled":      true,
				"adapter":      "openai_responses",
				"capabilities": []any{"answer_search"},
				"base_url":     openai.URL,
				"api_key":      "test-key",
				"settings": map[string]any{
					"model": "gpt-test",
				},
			},
			"deepwiki": map[string]any{
				"enabled":      true,
				"adapter":      "deepwiki",
				"capabilities": []any{"repo_wiki"},
				"base_url":     deepwiki.URL,
				"settings": map[string]any{
					"anonymous_allowed": true,
				},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	got := New(cfg).Search(t.Context(), "explain repo", SearchOptions{Providers: "openai_responses", RepoWiki: "microsoft/playwright"})
	used := got["used"].([]map[string]any)
	var found bool
	for _, item := range used {
		if item["capability"] == "repo_wiki" {
			found = true
		}
	}
	if !found {
		t.Fatalf("forced repo_wiki should not be filtered by answer --providers: %#v", used)
	}
}

func TestSearchCapabilityProviderFilterUsesSourceProviderOnly(t *testing.T) {
	var exaCalls int
	exa := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		exaCalls++
		_, _ = w.Write([]byte(`{"results":[{"title":"Exa","url":"https://exa.example/source"}]}`))
	}))
	defer exa.Close()
	tavily := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"results":[{"title":"Official","url":"https://official.example/rank","content":"official source"}]}`))
	}))
	defer tavily.Close()
	openai := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/alpha/search" {
			t.Fatalf("path = %q, want /v1/alpha/search", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"output":"answer","results":[]}`))
	}))
	defer openai.Close()

	clearProviderEnv(t)
	cfg := testConfig(t)
	if err := cfg.SetFile(map[string]any{
		"schema_version": 1,
		"routes": map[string]any{
			"answer_search": []any{"openai_responses"},
			"source_search": []any{"exa", "tavily"},
		},
		"profiles": map[string]any{
			"standard": map[string]any{
				"required_capabilities": []any{"answer_search"},
			},
		},
		"providers": map[string]any{
			"openai_responses": map[string]any{
				"enabled":      true,
				"adapter":      "openai_responses",
				"capabilities": []any{"answer_search"},
				"base_url":     openai.URL,
				"api_key":      "test-key",
				"settings":     map[string]any{"model": "gpt-test"},
			},
			"exa": map[string]any{
				"enabled":      true,
				"adapter":      "exa",
				"capabilities": []any{"source_search"},
				"base_url":     exa.URL,
				"api_key":      "test-key",
			},
			"tavily": map[string]any{
				"enabled":      true,
				"adapter":      "tavily",
				"capabilities": []any{"source_search"},
				"base_url":     tavily.URL,
				"api_key":      "test-key",
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	got := New(cfg).Search(t.Context(), "今天榜单", SearchOptions{
		Validation: "strict",
		Providers:  "openai_responses",
		ProviderFilters: map[string]string{
			"source_search": "tavily",
		},
	})
	if got["ok"] != true {
		t.Fatalf("search failed: %#v", got)
	}
	if exaCalls != 0 {
		t.Fatalf("capability-level source filter should skip exa, calls = %d", exaCalls)
	}
	attempts := got["provider_attempts"].([]map[string]any)
	var sourceProvider string
	for _, attempt := range attempts {
		if attempt["capability"] == "source_search" && attempt["status"] == "ok" {
			sourceProvider = attempt["provider"].(string)
		}
	}
	if sourceProvider != "tavily" {
		t.Fatalf("source provider = %q, attempts = %#v", sourceProvider, attempts)
	}
}

func TestSearchFetchSourcesAddsPageEvidence(t *testing.T) {
	tavily := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			_, _ = w.Write([]byte(`{"results":[{"title":"Official rank","url":"https://official.example/rank","content":"official rank page"}]}`))
		case "/extract":
			_, _ = w.Write([]byte(`{"results":[{"raw_content":"# Official rank\n\n1. alpha\n2. beta"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer tavily.Close()
	openai := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/alpha/search" {
			t.Fatalf("path = %q, want /v1/alpha/search", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"output":"answer","results":[]}`))
	}))
	defer openai.Close()

	clearProviderEnv(t)
	cfg := testConfig(t)
	if err := cfg.SetFile(map[string]any{
		"schema_version": 1,
		"routes": map[string]any{
			"answer_search": []any{"openai_responses"},
			"source_search": []any{"tavily"},
			"page_fetch":    []any{"tavily"},
		},
		"profiles": map[string]any{
			"standard": map[string]any{
				"required_capabilities": []any{"answer_search"},
			},
		},
		"providers": map[string]any{
			"openai_responses": map[string]any{
				"enabled":      true,
				"adapter":      "openai_responses",
				"capabilities": []any{"answer_search"},
				"base_url":     openai.URL,
				"api_key":      "test-key",
				"settings":     map[string]any{"model": "gpt-test"},
			},
			"tavily": map[string]any{
				"enabled":      true,
				"adapter":      "tavily",
				"capabilities": []any{"source_search", "page_fetch"},
				"base_url":     tavily.URL,
				"api_key":      "test-key",
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	got := New(cfg).Search(t.Context(), "今天榜单", SearchOptions{
		Validation:   "strict",
		Providers:    "openai_responses",
		FetchSources: 1,
		ProviderFilters: map[string]string{
			"source_search": "tavily",
		},
	})
	if got["ok"] != true {
		t.Fatalf("search failed: %#v", got)
	}
	used := got["used"].([]map[string]any)
	var pageEvidence map[string]any
	for _, item := range used {
		if item["capability"] == "page_fetch" && item["role"] == "source_evidence" {
			pageEvidence = item
		}
	}
	if pageEvidence == nil {
		t.Fatalf("used missing page_fetch source_evidence: %#v", used)
	}
	provider := pageEvidence["providers"].([]map[string]any)[0]
	result := provider["result"].(map[string]any)
	pages := result["pages"].([]map[string]any)
	if len(pages) != 1 || pages[0]["url"] != "https://official.example/rank" || !strings.Contains(pages[0]["content_preview"].(string), "Official rank") {
		t.Fatalf("fetched pages = %#v", pages)
	}
}

func TestQuietRepoWikiOutputOmitsFullContent(t *testing.T) {
	fullContent := strings.Repeat("repo detail ", 130)
	rendered := output.RenderWithOptions("repo-wiki", map[string]any{
		"ok":       true,
		"repo":     "microsoft/playwright",
		"provider": "deepwiki",
		"tool":     "ask_question",
		"content":  fullContent,
	}, output.Options{Format: "json", Verbosity: "quiet"})

	var got map[string]any
	if err := json.Unmarshal([]byte(rendered), &got); err != nil {
		t.Fatal(err)
	}
	if _, ok := got["content"]; ok {
		t.Fatalf("quiet repo-wiki output should omit full content: %s", rendered)
	}
	if stringValue(got["content_preview"]) == "" || got["content_length"] != float64(len(fullContent)) {
		t.Fatalf("quiet repo-wiki output should include preview and length: %#v", got)
	}
}

func TestRepoWikiContentOutputKeepsFullContent(t *testing.T) {
	rendered := output.RenderWithOptions("repo-wiki", map[string]any{
		"ok":      true,
		"repo":    "microsoft/playwright",
		"content": "complete repo wiki",
	}, output.Options{Format: "content", Verbosity: "quiet"})
	if rendered != "complete repo wiki\n" {
		t.Fatalf("repo-wiki content output = %q", rendered)
	}
}

func TestMapFallsBackToFirecrawl(t *testing.T) {
	tavily := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"tavily unavailable"}}`, http.StatusBadGateway)
	}))
	defer tavily.Close()
	firecrawl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/map" {
			t.Fatalf("firecrawl path = %s, want /v2/map", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"links":["https://example.com/docs"]}`))
	}))
	defer firecrawl.Close()

	clearProviderEnv(t)
	cfg := testConfig(t)
	if err := cfg.SetFile(map[string]any{
		"schema_version": 1,
		"routes": map[string]any{
			"site_map": []any{"tavily", "firecrawl"},
		},
		"providers": map[string]any{
			"tavily": map[string]any{
				"enabled":      true,
				"adapter":      "tavily",
				"capabilities": []any{"site_map"},
				"base_url":     tavily.URL,
				"api_key":      "tavily-key",
			},
			"firecrawl": map[string]any{
				"enabled":      true,
				"adapter":      "firecrawl",
				"capabilities": []any{"site_map"},
				"base_url":     firecrawl.URL,
				"api_key":      "firecrawl-key",
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	got := New(cfg).Map(t.Context(), "https://example.com", MapOptions{Limit: 5, Timeout: 2})
	if got["ok"] != true || got["provider"] != "firecrawl" || got["fallback_used"] != true {
		t.Fatalf("map result = %#v", got)
	}
	if results := testStrings(got["results"]); !reflect.DeepEqual(results, []string{"https://example.com/docs"}) {
		t.Fatalf("map results = %#v", got["results"])
	}
}

func TestMapEmptyResultsAreNotSuccess(t *testing.T) {
	tavily := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"base_url":"https://playwright.dev/mcp","results":[]}`))
	}))
	defer tavily.Close()

	clearProviderEnv(t)
	cfg := testConfig(t)
	if err := cfg.SetFile(map[string]any{
		"schema_version": 1,
		"routes": map[string]any{
			"site_map": []any{"tavily"},
		},
		"providers": map[string]any{
			"tavily": map[string]any{
				"enabled":      true,
				"adapter":      "tavily",
				"capabilities": []any{"site_map"},
				"base_url":     tavily.URL,
				"api_key":      "tavily-key",
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	got := New(cfg).Map(t.Context(), "https://playwright.dev/mcp", MapOptions{Limit: 20, Timeout: 2})
	if got["ok"] == true {
		t.Fatalf("empty map results should not be success: %#v", got)
	}
	attempts := got["provider_attempts"].([]map[string]any)
	if attempts[0]["status"] != "empty" || attempts[0]["result_count"] != 0 {
		t.Fatalf("empty map attempt should be marked empty: %#v", attempts)
	}
}

func TestMapFiltersExternalResultsAndFallsBack(t *testing.T) {
	tavily := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"base_url":"https://playwright.dev","results":["https://code.visualstudio.com/"]}`))
	}))
	defer tavily.Close()
	firecrawl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"links":["https://playwright.dev/mcp/introduction"]}`))
	}))
	defer firecrawl.Close()

	clearProviderEnv(t)
	cfg := testConfig(t)
	if err := cfg.SetFile(map[string]any{
		"schema_version": 1,
		"routes": map[string]any{
			"site_map": []any{"tavily", "firecrawl"},
		},
		"providers": map[string]any{
			"tavily": map[string]any{
				"enabled":      true,
				"adapter":      "tavily",
				"capabilities": []any{"site_map"},
				"base_url":     tavily.URL,
				"api_key":      "tavily-key",
			},
			"firecrawl": map[string]any{
				"enabled":      true,
				"adapter":      "firecrawl",
				"capabilities": []any{"site_map"},
				"base_url":     firecrawl.URL,
				"api_key":      "firecrawl-key",
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	got := New(cfg).Map(t.Context(), "https://playwright.dev", MapOptions{Limit: 20, Timeout: 2})
	if got["ok"] != true || got["provider"] != "firecrawl" || got["fallback_used"] != true {
		t.Fatalf("map should fall back after external-only results: %#v", got)
	}
	if results := testStrings(got["results"]); !reflect.DeepEqual(results, []string{"https://playwright.dev/mcp/introduction"}) {
		t.Fatalf("map results = %#v", got["results"])
	}
}

func TestCrawlUsesFirecrawlProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/crawl" {
			t.Fatalf("firecrawl path = %s, want /v2/crawl", r.URL.Path)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["maxDiscoveryDepth"] != float64(3) || payload["limit"] != float64(7) {
			t.Fatalf("crawl payload = %#v", payload)
		}
		_, _ = w.Write([]byte(`{"success":true,"id":"crawl-1","url":"https://example.com"}`))
	}))
	defer server.Close()

	clearProviderEnv(t)
	cfg := testConfig(t)
	if err := cfg.SetFile(map[string]any{
		"schema_version": 1,
		"routes": map[string]any{
			"site_crawl": []any{"firecrawl"},
		},
		"providers": map[string]any{
			"firecrawl": map[string]any{
				"enabled":      true,
				"adapter":      "firecrawl",
				"capabilities": []any{"site_crawl"},
				"base_url":     server.URL,
				"api_key":      "firecrawl-key",
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	got := New(cfg).Crawl(t.Context(), "https://example.com", CrawlOptions{MaxDepth: 3, Limit: 7})
	if got["ok"] != true || got["provider"] != "firecrawl" || got["tool"] != "firecrawl_crawl" || got["id"] != "crawl-1" || got["status"] != "submitted" {
		t.Fatalf("crawl result = %#v", got)
	}
	result := got["result"].(map[string]any)
	if result["id"] != "crawl-1" {
		t.Fatalf("crawl result payload = %#v", result)
	}
}

func TestCrawlUsesTavilyProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/crawl" {
			t.Fatalf("tavily path = %s, want /crawl", r.URL.Path)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["max_depth"] != float64(3) || payload["limit"] != float64(7) {
			t.Fatalf("crawl payload = %#v", payload)
		}
		_, _ = w.Write([]byte(`{"results":[{"url":"https://example.com/docs","raw_content":"docs"}]}`))
	}))
	defer server.Close()

	clearProviderEnv(t)
	cfg := testConfig(t)
	if err := cfg.SetFile(map[string]any{
		"schema_version": 1,
		"routes": map[string]any{
			"site_crawl": []any{"tavily"},
		},
		"providers": map[string]any{
			"tavily": map[string]any{
				"enabled":      true,
				"adapter":      "tavily",
				"capabilities": []any{"site_crawl"},
				"base_url":     server.URL,
				"api_key":      "tavily-key",
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	got := New(cfg).Crawl(t.Context(), "https://example.com", CrawlOptions{MaxDepth: 3, Limit: 7})
	if got["ok"] != true || got["provider"] != "tavily" || got["tool"] != "tavily_crawl" {
		t.Fatalf("crawl result = %#v", got)
	}
}

func TestFetchProviderFilterUsesExa(t *testing.T) {
	var tavilyCalls int
	tavily := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tavilyCalls++
		http.Error(w, "should be skipped", http.StatusInternalServerError)
	}))
	defer tavily.Close()
	exa := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/contents" {
			t.Fatalf("exa path = %s, want /contents", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"results":[{"url":"https://example.com","title":"Example","text":"exa page body"}]}`))
	}))
	defer exa.Close()

	clearProviderEnv(t)
	cfg := testConfig(t)
	if err := cfg.SetFile(map[string]any{
		"schema_version": 1,
		"routes": map[string]any{
			"page_fetch": []any{"tavily", "exa"},
		},
		"providers": map[string]any{
			"tavily": map[string]any{
				"enabled":      true,
				"adapter":      "tavily",
				"capabilities": []any{"page_fetch"},
				"base_url":     tavily.URL,
				"api_key":      "tavily-key",
			},
			"exa": map[string]any{
				"enabled":      true,
				"adapter":      "exa",
				"capabilities": []any{"page_fetch"},
				"base_url":     exa.URL,
				"api_key":      "exa-key",
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	got := New(cfg).Fetch(t.Context(), "https://example.com", FetchOptions{Provider: "exa"})
	if got["ok"] != true || got["provider"] != "exa" || got["content"] != "exa page body" {
		t.Fatalf("fetch result = %#v", got)
	}
	if tavilyCalls != 0 {
		t.Fatalf("fetch --provider exa should skip tavily, calls = %d", tavilyCalls)
	}
}

func TestMapProviderFilterSkipsUnmatchedProviders(t *testing.T) {
	var tavilyCalls int
	tavily := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tavilyCalls++
		http.Error(w, "should be skipped", http.StatusInternalServerError)
	}))
	defer tavily.Close()
	firecrawl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"links":["https://example.com/docs"]}`))
	}))
	defer firecrawl.Close()

	clearProviderEnv(t)
	cfg := testConfig(t)
	if err := cfg.SetFile(map[string]any{
		"schema_version": 1,
		"routes": map[string]any{
			"site_map": []any{"tavily", "firecrawl"},
		},
		"providers": map[string]any{
			"tavily": map[string]any{
				"enabled":      true,
				"adapter":      "tavily",
				"capabilities": []any{"site_map"},
				"base_url":     tavily.URL,
				"api_key":      "tavily-key",
			},
			"firecrawl": map[string]any{
				"enabled":      true,
				"adapter":      "firecrawl",
				"capabilities": []any{"site_map"},
				"base_url":     firecrawl.URL,
				"api_key":      "firecrawl-key",
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	got := New(cfg).Map(t.Context(), "https://example.com", MapOptions{Provider: "firecrawl", Limit: 5, Timeout: 2})
	if got["ok"] != true || got["provider"] != "firecrawl" {
		t.Fatalf("map result = %#v", got)
	}
	if tavilyCalls != 0 {
		t.Fatalf("map --provider firecrawl should skip tavily, calls = %d", tavilyCalls)
	}
}

func TestCrawlProviderFilterSkipsUnmatchedProviders(t *testing.T) {
	var firecrawlCalls int
	firecrawl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		firecrawlCalls++
		http.Error(w, "should be skipped", http.StatusInternalServerError)
	}))
	defer firecrawl.Close()
	tavily := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"results":[{"url":"https://example.com/docs","raw_content":"docs"}]}`))
	}))
	defer tavily.Close()

	clearProviderEnv(t)
	cfg := testConfig(t)
	if err := cfg.SetFile(map[string]any{
		"schema_version": 1,
		"routes": map[string]any{
			"site_crawl": []any{"firecrawl", "tavily"},
		},
		"providers": map[string]any{
			"firecrawl": map[string]any{
				"enabled":      true,
				"adapter":      "firecrawl",
				"capabilities": []any{"site_crawl"},
				"base_url":     firecrawl.URL,
				"api_key":      "firecrawl-key",
			},
			"tavily": map[string]any{
				"enabled":      true,
				"adapter":      "tavily",
				"capabilities": []any{"site_crawl"},
				"base_url":     tavily.URL,
				"api_key":      "tavily-key",
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	got := New(cfg).Crawl(t.Context(), "https://example.com", CrawlOptions{Provider: "tavily", MaxDepth: 2, Limit: 5})
	if got["ok"] != true || got["provider"] != "tavily" {
		t.Fatalf("crawl result = %#v", got)
	}
	if firecrawlCalls != 0 {
		t.Fatalf("crawl --provider tavily should skip firecrawl, calls = %d", firecrawlCalls)
	}
}

func TestRepoWikiProviderFilterNoMatchReturnsConfigError(t *testing.T) {
	clearProviderEnv(t)
	cfg := testConfig(t)
	if err := cfg.SetFile(map[string]any{
		"schema_version": 1,
		"routes": map[string]any{
			"repo_wiki": []any{"deepwiki"},
		},
		"providers": map[string]any{
			"deepwiki": map[string]any{
				"enabled":      true,
				"adapter":      "deepwiki",
				"capabilities": []any{"repo_wiki"},
				"base_url":     "https://example.com",
				"settings": map[string]any{
					"anonymous_allowed": true,
				},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	got := New(cfg).RepoWiki(t.Context(), "microsoft/playwright", "architecture?", RepoWikiOptions{Provider: "exa"})
	if got["ok"] != false || got["error_type"] != "config_error" || !strings.Contains(stringValue(got["error"]), "--provider exa") {
		t.Fatalf("repo-wiki provider filter error = %#v", got)
	}
}

func TestPageFetchIncludesAutoRegisteredAnySearch(t *testing.T) {
	clearProviderEnv(t)
	cfg := testConfig(t)
	svc := New(cfg)

	providers := svc.pageFetchProviders()
	if !containsString(providers, "anysearch") {
		t.Fatalf("page_fetch providers = %#v, want auto-registered anysearch", providers)
	}
}

func TestQuietErrorOutputOmitsDiagnostics(t *testing.T) {
	longError := `openai_responses Post "http://127.0.0.1:1/v1/alpha/search": dial tcp 127.0.0.1:1: connectex: No connection could be made because the target machine actively refused it.`
	data := map[string]any{
		"ok":         false,
		"error_type": "config_error",
		"error":      longError,
		"session_id": "sid",
		"query":      "q",
		"diagnostics": map[string]any{
			"capabilities": map[string]any{"answer_search": map[string]any{"ok": false}},
		},
	}

	rendered := output.RenderWithOptions("search", data, output.Options{Format: "json", Verbosity: "quiet"})
	var got map[string]any
	if err := json.Unmarshal([]byte(rendered), &got); err != nil {
		t.Fatal(err)
	}
	if _, ok := got["diagnostics"]; ok {
		t.Fatalf("quiet output should omit diagnostics: %s", rendered)
	}
	if stringValue(got["hint"]) == "" {
		t.Fatalf("quiet output should include hint: %s", rendered)
	}
	if strings.Contains(stringValue(got["error"]), "dial tcp") || strings.Contains(stringValue(got["error"]), "connectex") {
		t.Fatalf("quiet output should compact low-level network details: %s", rendered)
	}
}

func TestVerboseErrorOutputKeepsDiagnostics(t *testing.T) {
	longError := `openai_responses Post "http://127.0.0.1:1/v1/alpha/search": dial tcp 127.0.0.1:1: connectex: No connection could be made because the target machine actively refused it.`
	data := map[string]any{
		"ok":          false,
		"error_type":  "config_error",
		"error":       longError,
		"diagnostics": map[string]any{"capabilities": map[string]any{}},
	}

	rendered := output.RenderWithOptions("search", data, output.Options{Format: "json", Verbosity: "verbose"})
	if !strings.Contains(rendered, "diagnostics") {
		t.Fatalf("verbose output should keep diagnostics: %s", rendered)
	}
	if !strings.Contains(rendered, "dial tcp") {
		t.Fatalf("verbose output should keep full error detail: %s", rendered)
	}
}

func TestQuietSearchOutputUsesUnifiedResultShape(t *testing.T) {
	longAnswer := strings.Repeat("answer detail ", 130)
	data := map[string]any{
		"ok":               true,
		"query":            "q",
		"content":          "answer",
		"answer":           "duplicate answer",
		"session_id":       "sid",
		"elapsed_ms":       12.3,
		"validation_level": "balanced",
		"sources": []map[string]any{{
			"provider": "exa",
			"title":    "Doc",
			"url":      "https://example.com",
			"extra":    "debug",
		}},
		"extra_sources":     []map[string]any{{"provider": "exa"}},
		"primary_sources":   []map[string]any{},
		"provider_attempts": []map[string]any{{"provider": "openai_responses", "status": "ok"}},
		"routing_decision":  map[string]any{"providers": "auto"},
		"diagnostics":       map[string]any{"capabilities": map[string]any{}},
		"providers_used":    []string{"openai_responses", "exa"},
		"used": []map[string]any{{
			"capability": "answer_search",
			"role":       "primary_answer",
			"providers": []map[string]any{{
				"provider":   "openai_responses",
				"status":     "ok",
				"mode":       "openai-responses",
				"model":      "gpt-test",
				"elapsed_ms": 10.1,
				"result": map[string]any{
					"content": longAnswer,
					"sources": []map[string]any{{
						"capability": "answer_search",
						"provider":   "openai_responses",
						"title":      "Primary",
						"url":        "https://primary.example.com",
						"debug":      "hidden",
					}},
				},
			}},
		}, {
			"capability": "docs_search",
			"role":       "documentation_sources",
			"providers": []map[string]any{{
				"provider":   "exa",
				"status":     "ok",
				"elapsed_ms": 2.2,
				"result": map[string]any{
					"sources": []map[string]any{{
						"capability": "docs_search",
						"provider":   "exa",
						"title":      "Doc",
						"url":        "https://example.com",
						"extra":      "debug",
					}},
				},
			}},
		}, {
			"capability": "source_search",
			"role":       "extra_sources",
			"providers": []map[string]any{{
				"provider":   "exa",
				"status":     "ok",
				"elapsed_ms": 1.1,
				"result": map[string]any{
					"sources": []map[string]any{{
						"capability": "source_search",
						"provider":   "exa",
						"title":      "Extra",
						"url":        "https://source.example.com/extra",
					}},
				},
			}},
		}, {
			"capability": "source_search",
			"role":       "current_sources",
			"providers": []map[string]any{{
				"provider":   "exa",
				"status":     "ok",
				"elapsed_ms": 3.3,
				"result": map[string]any{
					"sources": []map[string]any{{
						"capability": "source_search",
						"provider":   "exa",
						"title":      "Current",
						"url":        "https://source.example.com/current",
					}, {
						"capability": "source_search",
						"provider":   "exa",
						"title":      "Extra duplicate",
						"url":        "https://source.example.com/extra",
					}},
				},
			}},
		}},
	}

	rendered := output.RenderWithOptions("search", data, output.Options{Format: "json", Verbosity: "quiet"})
	var got map[string]any
	if err := json.Unmarshal([]byte(rendered), &got); err != nil {
		t.Fatal(err)
	}
	wantKeys := []string{"meta", "ok", "query", "used"}
	if keys := sortedMapKeys(got); !reflect.DeepEqual(keys, wantKeys) {
		t.Fatalf("quiet search keys = %#v, want %#v\n%s", keys, wantKeys, rendered)
	}
	for _, key := range []string{"answer", "content", "diagnostics", "provider_attempts", "routing_decision", "extra_sources", "primary_sources", "providers_used", "sources", "sources_count"} {
		if _, ok := got[key]; ok {
			t.Fatalf("quiet search output should omit %s: %s", key, rendered)
		}
	}
	meta, ok := got["meta"].(map[string]any)
	if !ok {
		t.Fatalf("meta = %#v", got["meta"])
	}
	if meta["session_id"] != "sid" || meta["validation_level"] != "balanced" {
		t.Fatalf("meta should keep compact run metadata: %#v", meta)
	}
	used, ok := got["used"].(map[string]any)
	if !ok || len(used) != 3 {
		t.Fatalf("used = %#v", got["used"])
	}
	answerCapability, ok := used["answer_search"].(map[string]any)
	if !ok {
		t.Fatalf("used.answer_search type = %T", used["answer_search"])
	}
	if answerCapability["role"] != "primary_answer" {
		t.Fatalf("answer capability = %#v", answerCapability)
	}
	answerProviders := answerCapability["providers"].(map[string]any)
	answerProvider := answerProviders["openai_responses"].(map[string]any)
	if _, ok := answerProvider["provider"]; ok {
		t.Fatalf("provider object should not repeat provider id: %#v", answerProvider)
	}
	answerResult := answerProvider["result"].(map[string]any)
	if _, ok := answerResult["content"]; ok {
		t.Fatalf("quiet answer result should omit full content: %#v", answerResult)
	}
	preview, _ := answerResult["content_preview"].(string)
	if len(preview) >= len(longAnswer) || !strings.HasSuffix(preview, "...") {
		t.Fatalf("quiet answer result should keep truncated content preview: %#v", answerResult)
	}
	if answerResult["content_length"] != float64(len(longAnswer)) {
		t.Fatalf("quiet answer result should keep content length: %#v", answerResult)
	}
	docsCapability := used["docs_search"].(map[string]any)
	docsProvider := docsCapability["providers"].(map[string]any)["exa"].(map[string]any)
	docsResult := docsProvider["result"].(map[string]any)
	docsSources := docsResult["sources"].([]any)
	source := docsSources[0].(map[string]any)
	if source["extra"] != nil || source["debug"] != nil {
		t.Fatalf("quiet source should omit debug fields: %#v", source)
	}
	if source["capability"] != "docs_search" || source["provider"] != "exa" || source["title"] != "Doc" || source["url"] != "https://example.com" {
		t.Fatalf("quiet source should keep provenance and quality fields: %#v", source)
	}
	sourceCapability := used["source_search"].(map[string]any)
	if _, ok := sourceCapability["role"]; ok {
		t.Fatalf("merged capability with multiple roles should not keep singular role: %#v", sourceCapability)
	}
	if roles := testStrings(sourceCapability["roles"]); !reflect.DeepEqual(roles, []string{"extra_sources", "current_sources"}) {
		t.Fatalf("merged source_search roles = %#v", sourceCapability["roles"])
	}
	sourceProvider := sourceCapability["providers"].(map[string]any)["exa"].(map[string]any)
	sourceResult := sourceProvider["result"].(map[string]any)
	sourceSources := sourceResult["sources"].([]any)
	if len(sourceSources) != 2 {
		t.Fatalf("merged source_search should dedupe provider sources by URL: %#v", sourceSources)
	}
}

func TestVerboseSearchOutputKeepsDiagnostics(t *testing.T) {
	fullAnswer := "answer with complete detail"
	data := map[string]any{
		"ok":                true,
		"query":             "q",
		"content":           fullAnswer,
		"sources":           []map[string]any{},
		"provider_attempts": []map[string]any{{"provider": "openai_responses", "status": "ok"}},
		"routing_decision":  map[string]any{"providers": "auto"},
		"diagnostics":       map[string]any{"capabilities": map[string]any{}},
	}

	rendered := output.RenderWithOptions("search", data, output.Options{Format: "json", Verbosity: "verbose"})
	for _, key := range []string{"diagnostics", "provider_attempts", "routing_decision", fullAnswer} {
		if !strings.Contains(rendered, key) {
			t.Fatalf("verbose search output should keep %s: %s", key, rendered)
		}
	}
}

func mcpStdioTestProvider(provider string, capabilities []any, tools map[string]any) map[string]any {
	return map[string]any{
		"enabled":      true,
		"adapter":      "mcp_stdio",
		"capabilities": capabilities,
		"settings": map[string]any{
			"direct_only":       true,
			"anonymous_allowed": true,
			"command":           os.Args[0],
			"args":              []any{"-test.run=TestServiceMCPStdioHelperProcess"},
			"timeout_seconds":   2,
			"env": map[string]any{
				"GO_WANT_SERVICE_MCPSTDIO_HELPER": "1",
				"SERVICE_MCPSTDIO_PROVIDER":       provider,
			},
			"tools": tools,
		},
	}
}

func TestServiceMCPStdioHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_SERVICE_MCPSTDIO_HELPER") != "1" {
		return
	}
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var request map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &request); err != nil {
			os.Exit(2)
		}
		id, hasID := request["id"]
		if !hasID {
			continue
		}
		switch request["method"] {
		case "initialize":
			serviceMCPRespond(id, map[string]any{"protocolVersion": "2025-03-26", "capabilities": map[string]any{}})
		case "tools/list":
			serviceMCPRespond(id, map[string]any{"tools": []map[string]any{
				{"name": "search"},
				{"name": "fetch_content"},
				{"name": "scrape"},
				{"name": "crawl"},
			}})
		case "tools/call":
			params := request["params"].(map[string]any)
			args := params["arguments"].(map[string]any)
			switch params["name"] {
			case "search":
				query := args["query"].(string)
				serviceMCPRespond(id, map[string]any{"content": []map[string]any{{"type": "text", "text": `{"results":[{"title":"Result","url":"https://example.com/` + query + `","snippet":"summary"}]}`}}})
			case "fetch_content", "scrape":
				serviceMCPRespond(id, map[string]any{"content": []map[string]any{{"type": "text", "text": "page body"}}})
			case "crawl":
				serviceMCPRespond(id, map[string]any{"content": []map[string]any{{"type": "text", "text": `{"pages":[{"url":"https://example.com/docs","markdown":"docs"}]}`}}})
			default:
				serviceMCPRespondError(id, -32601, "unknown tool")
			}
		default:
			serviceMCPRespondError(id, -32601, "unknown method")
		}
	}
	os.Exit(0)
}

func serviceMCPRespond(id any, result any) {
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
	os.Stdout.Write(append(body, '\n'))
}

func serviceMCPRespondError(id any, code int, message string) {
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": code, "message": message}})
	os.Stdout.Write(append(body, '\n'))
}

func testStrings(value any) []string {
	switch items := value.(type) {
	case []string:
		return items
	case []any:
		out := make([]string, 0, len(items))
		for _, item := range items {
			if text, ok := item.(string); ok {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}

func testConfig(t *testing.T) *config.Config {
	t.Helper()
	dir := t.TempDir()
	return &config.Config{ConfigFile: filepath.Join(dir, "config.json"), ConfigDirSource: "test"}
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
	} {
		t.Setenv(key, "")
	}
}
