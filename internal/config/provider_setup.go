package config

import (
	"fmt"
	"sort"
	"strings"
)

type ProviderSetupSpec struct {
	Provider        ProviderDefinition
	Credential      ProviderCredentialState
	RequiresAPIKey  bool
	SupportsBaseURL bool
	RequiresBaseURL bool
}

type ProviderSetupPatch struct {
	APIKey  *string
	BaseURL *string
}

func ResolveProviderSetup(c *Config, identifier string) (ProviderSetupSpec, error) {
	runtime, err := LoadRuntimeStrict(c)
	if err != nil {
		return ProviderSetupSpec{}, err
	}
	return ResolveProviderSetupFromRuntime(c, runtime, identifier)
}

func ResolveProviderSetupFromRuntime(c *Config, runtime RuntimeConfig, identifier string) (ProviderSetupSpec, error) {
	provider, err := resolveProviderIdentifier(runtime.Providers, identifier)
	if err != nil {
		return ProviderSetupSpec{}, err
	}
	return ProviderSetupSpec{
		Provider:        provider,
		Credential:      ResolveProviderCredential(c, provider),
		RequiresAPIKey:  !boolSetting(provider.Settings, "anonymous_allowed", false),
		SupportsBaseURL: strings.TrimSpace(provider.BaseURL) != "" || boolSetting(provider.Settings, "requires_base_url", false),
		RequiresBaseURL: boolSetting(provider.Settings, "requires_base_url", false),
	}, nil
}

func (c *Config) PatchProviderSetup(providerID string, patch ProviderSetupPatch) ([]string, error) {
	raw, err := c.LoadFileStrict()
	if err != nil {
		return nil, err
	}
	if !isRuntimeSchema(raw) {
		return nil, fmt.Errorf("配置文件不是可识别的 runtime schema")
	}
	providers := raw["providers"].(map[string]any)
	providerData := map[string]any{}
	if existing, ok := providers[providerID]; ok {
		var valid bool
		providerData, valid = existing.(map[string]any)
		if !valid {
			return nil, fmt.Errorf("providers.%s 必须是 JSON object", providerID)
		}
	}

	changed := make([]string, 0, 3)
	if patch.APIKey != nil && stringFromAny(providerData["api_key"]) != *patch.APIKey {
		providerData["api_key"] = *patch.APIKey
		changed = append(changed, "api_key")
	}
	if patch.BaseURL != nil && stringFromAny(providerData["base_url"]) != *patch.BaseURL {
		providerData["base_url"] = *patch.BaseURL
		changed = append(changed, "base_url")
	}
	if enabled, ok := providerData["enabled"]; !ok || normalizeEnabled(enabled) != "auto" {
		providerData["enabled"] = "auto"
		changed = append(changed, "enabled")
	}
	if len(changed) == 0 {
		return changed, nil
	}
	providers[providerID] = providerData
	raw["providers"] = providers
	if err := c.SaveFile(raw); err != nil {
		return nil, err
	}
	return changed, nil
}

func resolveProviderIdentifier(providers map[string]ProviderDefinition, identifier string) (ProviderDefinition, error) {
	target := normalizeProviderIdentifier(identifier)
	if target == "" {
		return ProviderDefinition{}, fmt.Errorf("provider 不能为空")
	}
	ids := sortedProviderIDs(providers)
	idMatches := make([]string, 0, 1)
	for _, id := range ids {
		if normalizeProviderIdentifier(id) == target {
			idMatches = append(idMatches, id)
		}
	}
	if len(idMatches) == 1 {
		return providers[idMatches[0]], nil
	}
	if len(idMatches) > 1 {
		return ProviderDefinition{}, fmt.Errorf("provider 标识 %q 存在冲突", identifier)
	}

	aliasMatches := map[string]struct{}{}
	for _, id := range ids {
		for _, alias := range providers[id].Aliases {
			if normalizeProviderIdentifier(alias) == target {
				aliasMatches[id] = struct{}{}
			}
		}
	}
	if len(aliasMatches) == 1 {
		for id := range aliasMatches {
			return providers[id], nil
		}
	}
	if len(aliasMatches) > 1 {
		return ProviderDefinition{}, fmt.Errorf("provider alias %q 存在冲突", identifier)
	}
	return ProviderDefinition{}, fmt.Errorf("未知 provider: %s", identifier)
}

func sortedProviderIDs(providers map[string]ProviderDefinition) []string {
	ids := make([]string, 0, len(providers))
	for id := range providers {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func normalizeProviderIdentifier(value string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(value)), "-", "_")
}
