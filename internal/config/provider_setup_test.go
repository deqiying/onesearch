package config

import (
	"os"
	"reflect"
	"testing"
)

func TestResolveProviderSetupAcceptsKebabCaseAndAlias(t *testing.T) {
	cfg := testConfig(t, InitialRuntimeSchema())

	compatible, err := ResolveProviderSetup(cfg, "openai-compatible")
	if err != nil {
		t.Fatal(err)
	}
	if compatible.Provider.ID != "openai_compatible" || !compatible.RequiresAPIKey || !compatible.SupportsBaseURL {
		t.Fatalf("compatible spec = %#v", compatible)
	}

	xai, err := ResolveProviderSetup(cfg, "grok")
	if err != nil {
		t.Fatal(err)
	}
	if xai.Provider.ID != "xai" {
		t.Fatalf("alias resolved to %q", xai.Provider.ID)
	}
}

func TestPatchProviderSetupPreservesUnrelatedAndUnknownFields(t *testing.T) {
	raw := InitialRuntimeSchema()
	raw["custom_top"] = map[string]any{"keep": true}
	providers := raw["providers"].(map[string]any)
	exa := providers["exa"].(map[string]any)
	exa["custom_provider_field"] = "keep"
	originalEnv := exa["api_key_env"]
	cfg := testConfig(t, raw)

	key := "new-secret"
	baseURL := "https://gateway.example.com/v1"
	changed, err := cfg.PatchProviderSetup("exa", ProviderSetupPatch{APIKey: &key, BaseURL: &baseURL})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(changed, []string{"api_key", "base_url", "enabled"}) {
		t.Fatalf("changed fields = %#v", changed)
	}

	updated, err := cfg.LoadFileStrict()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(updated["custom_top"], map[string]any{"keep": true}) {
		t.Fatalf("top-level custom field changed: %#v", updated["custom_top"])
	}
	updatedExa := updated["providers"].(map[string]any)["exa"].(map[string]any)
	if updatedExa["api_key"] != key || updatedExa["base_url"] != baseURL || updatedExa["enabled"] != "auto" {
		t.Fatalf("updated provider = %#v", updatedExa)
	}
	settings := updatedExa["settings"].(map[string]any)
	if updatedExa["custom_provider_field"] != "keep" || updatedExa["api_key_env"] != originalEnv || settings["timeout_seconds"] != float64(30) {
		t.Fatalf("unrelated provider fields changed: %#v", updatedExa)
	}
}

func TestPatchProviderSetupDoesNotOverwriteInvalidJSON(t *testing.T) {
	cfg := testConfig(t, nil)
	original := []byte(`{"schema_version":1,"providers":`)
	if err := os.WriteFile(cfg.ConfigFile, original, 0o600); err != nil {
		t.Fatal(err)
	}
	key := "new-secret"
	if _, err := cfg.PatchProviderSetup("exa", ProviderSetupPatch{APIKey: &key}); err == nil {
		t.Fatal("invalid JSON should fail")
	}
	after, err := os.ReadFile(cfg.ConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(original) {
		t.Fatalf("invalid config was overwritten: %q", after)
	}
}

func TestPatchProviderSetupCreatesMinimalBuiltinOverride(t *testing.T) {
	cfg := testConfig(t, map[string]any{
		"schema_version": 1,
		"providers":      map[string]any{},
		"routes":         map[string]any{},
	})
	key := "new-secret"
	changed, err := cfg.PatchProviderSetup("exa", ProviderSetupPatch{APIKey: &key})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(changed, []string{"api_key", "enabled"}) {
		t.Fatalf("changed = %#v", changed)
	}
	provider := cfg.LoadFile()["providers"].(map[string]any)["exa"].(map[string]any)
	if !reflect.DeepEqual(provider, map[string]any{"api_key": key, "enabled": "auto"}) {
		t.Fatalf("minimal override = %#v", provider)
	}
}

func TestProviderCredentialStateTrimsBothSourcesAndReportsOverride(t *testing.T) {
	cfg := testConfig(t, InitialRuntimeSchema())
	t.Setenv("EXA_API_KEY", "  env-secret  ")
	provider := LoadRuntime(cfg).Providers["exa"]
	provider.APIKey = "  direct-secret  "
	state := ResolveProviderCredential(cfg, provider)
	if state.Value != "direct-secret" || state.Source != "config" || !state.DirectSet || !state.EnvironmentSet {
		t.Fatalf("credential state = %#v", state)
	}

	t.Setenv("EXA_API_KEY", "   ")
	provider.APIKey = "  "
	state = ResolveProviderCredential(cfg, provider)
	if state.Value != "" || state.Source != "" || state.DirectSet || state.EnvironmentSet {
		t.Fatalf("whitespace credential state = %#v", state)
	}
}

func TestResolveProviderSetupRejectsAliasCollision(t *testing.T) {
	raw := map[string]any{
		"schema_version": 1,
		"routes":         map[string]any{},
		"providers": map[string]any{
			"custom_one": map[string]any{"adapter": "exa", "aliases": []any{"same-alias"}},
			"custom_two": map[string]any{"adapter": "exa", "aliases": []any{"same_alias"}},
		},
	}
	cfg := testConfig(t, raw)
	if _, err := ResolveProviderSetup(cfg, "same-alias"); err == nil {
		t.Fatal("alias collision should fail")
	}
}
