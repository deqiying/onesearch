package service

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/deqiying/onesearch/internal/config"
	"github.com/deqiying/onesearch/internal/redact"
)

type ProviderSetupSpec struct {
	Provider           string
	Adapter            string
	RequiresAPIKey     bool
	HasEffectiveAPIKey bool
	APIKeySource       string
	SupportsBaseURL    bool
	RequiresBaseURL    bool
	EffectiveBaseURL   string
}

type ProviderSetupRequest struct {
	Provider string
	APIKey   *string
	BaseURL  *string
}

type ProviderSetupError struct {
	ErrorType string
	Message   string
}

func (e *ProviderSetupError) Error() string { return e.Message }

func (s *Service) ProviderSetupSpec(identifier string) (ProviderSetupSpec, error) {
	runtime, err := config.LoadRuntimeStrict(s.Config)
	if err != nil {
		return ProviderSetupSpec{}, setupError("config_error", err.Error())
	}
	spec, err := config.ResolveProviderSetupFromRuntime(s.Config, runtime, identifier)
	if err != nil {
		return ProviderSetupSpec{}, setupError("parameter_error", err.Error())
	}
	if spec.Provider.Adapter == config.AdapterMCPStdio {
		return ProviderSetupSpec{}, setupError("parameter_error", "mcp_stdio provider 不支持通用 API key/base URL setup")
	}
	if !config.IsSupportedAdapter(spec.Provider.Adapter) {
		return ProviderSetupSpec{}, setupError("parameter_error", "该 provider 使用了不支持的 adapter")
	}
	return ProviderSetupSpec{
		Provider:           spec.Provider.ID,
		Adapter:            spec.Provider.Adapter,
		RequiresAPIKey:     spec.RequiresAPIKey,
		HasEffectiveAPIKey: spec.Credential.Value != "",
		APIKeySource:       spec.Credential.Source,
		SupportsBaseURL:    spec.SupportsBaseURL,
		RequiresBaseURL:    spec.RequiresBaseURL,
		EffectiveBaseURL:   spec.Provider.BaseURL,
	}, nil
}

func (s *Service) SetupProvider(request ProviderSetupRequest) map[string]any {
	spec, err := s.ProviderSetupSpec(request.Provider)
	if err != nil {
		return providerSetupFailure(err, request.Provider, s.Config.ConfigFile)
	}

	var apiKey *string
	if request.APIKey != nil {
		value := strings.TrimSpace(*request.APIKey)
		if value != "" {
			apiKey = &value
		}
	}
	if spec.RequiresAPIKey && apiKey == nil && !spec.HasEffectiveAPIKey {
		return providerSetupFailure(setupError("parameter_error", "该 provider 需要非空 API key"), spec.Provider, s.Config.ConfigFile)
	}

	var baseURL *string
	if request.BaseURL != nil {
		value := strings.TrimSpace(*request.BaseURL)
		if value != "" {
			if !spec.SupportsBaseURL {
				return providerSetupFailure(setupError("parameter_error", "该 provider 不支持 base_url 配置"), spec.Provider, s.Config.ConfigFile)
			}
			normalized, validateErr := normalizeEndpointBaseURL(value)
			if validateErr != nil {
				return providerSetupFailure(setupError("parameter_error", validateErr.Error()), spec.Provider, s.Config.ConfigFile)
			}
			baseURL = &normalized
		}
	}
	effectiveBaseURL := strings.TrimSpace(spec.EffectiveBaseURL)
	if baseURL != nil {
		effectiveBaseURL = *baseURL
	}
	if spec.RequiresBaseURL && effectiveBaseURL == "" {
		return providerSetupFailure(setupError("parameter_error", "该 provider 需要非空 base_url"), spec.Provider, s.Config.ConfigFile)
	}
	if spec.SupportsBaseURL && effectiveBaseURL != "" && baseURL == nil {
		if _, validateErr := normalizeEndpointBaseURL(effectiveBaseURL); validateErr != nil {
			return providerSetupFailure(setupError("parameter_error", "当前 base_url 无效；请提供新的安全 HTTP(S) URL"), spec.Provider, s.Config.ConfigFile)
		}
	}

	changed, err := s.Config.PatchProviderSetup(spec.Provider, config.ProviderSetupPatch{APIKey: apiKey, BaseURL: baseURL})
	if err != nil {
		return providerSetupFailure(setupError("config_error", err.Error()), spec.Provider, s.Config.ConfigFile)
	}
	runtime, err := config.LoadRuntimeStrict(s.Config)
	if err != nil {
		return providerSetupFailure(setupError("config_error", err.Error()), spec.Provider, s.Config.ConfigFile)
	}
	finalSpec, err := config.ResolveProviderSetupFromRuntime(s.Config, runtime, spec.Provider)
	if err != nil {
		return providerSetupFailure(setupError("config_error", err.Error()), spec.Provider, s.Config.ConfigFile)
	}
	credential := finalSpec.Credential
	return map[string]any{
		"ok":              true,
		"provider":        finalSpec.Provider.ID,
		"adapter":         finalSpec.Provider.Adapter,
		"config_file":     s.Config.ConfigFile,
		"enabled":         "auto",
		"api_key_set":     credential.DirectSet,
		"api_key_env":     finalSpec.Provider.APIKeyEnv,
		"api_key_env_set": credential.EnvironmentSet,
		"api_key_src":     credential.Source,
		"has_api_key":     credential.Value != "",
		"base_url":        finalSpec.Provider.BaseURL,
		"changed_fields":  changed,
	}
}

func (s *Service) OutputSecretValues() []string {
	if s == nil || s.Config == nil {
		return nil
	}
	runtime := s.runtime()
	values := redact.CollectSensitiveValues(runtime.Raw)
	for _, name := range redact.CollectAPIKeyEnvironmentNames(runtime.Raw) {
		if value := s.Config.Get(name, ""); strings.TrimSpace(value) != "" {
			values = append(values, value)
		}
	}
	for _, provider := range runtime.Providers {
		if value := strings.TrimSpace(provider.APIKey); value != "" {
			values = append(values, value)
		}
		if strings.TrimSpace(provider.APIKeyEnv) != "" {
			if value := s.Config.Get(provider.APIKeyEnv, ""); strings.TrimSpace(value) != "" {
				values = append(values, value)
			}
		}
		for name, value := range stringMapForService(provider.Settings["env"]) {
			if redact.IsSensitiveEnvironmentName(name) && strings.TrimSpace(value) != "" {
				values = append(values, value)
			}
		}
	}
	return redact.NormalizeSecrets(values)
}

func (s *Service) configDiagnostic() map[string]any {
	info := map[string]any{
		"file":                 s.Config.ConfigFile,
		"dir_source":           s.Config.ConfigDirSource,
		"created":              s.Config.CreatedConfigFile,
		"missing_before_start": s.Config.MissingConfigFile,
	}
	if s.Config.ConfigDirSource == "environment" {
		info["dir_env"] = config.ConfigDirEnvName
	}
	return info
}

func (s *Service) effectiveEnvironment(runtime config.RuntimeConfig) []map[string]any {
	items := make([]map[string]any, 0)
	if s.Config.ConfigDirSource == "environment" {
		items = append(items, map[string]any{"name": config.ConfigDirEnvName, "purpose": "config_dir"})
	}
	providerIDs := make([]string, 0, len(runtime.Providers))
	for id := range runtime.Providers {
		providerIDs = append(providerIDs, id)
	}
	sort.Strings(providerIDs)
	for _, id := range providerIDs {
		provider := runtime.Providers[id]
		if providerDisabled(provider.Enabled) {
			continue
		}
		credential := config.ResolveProviderCredential(s.Config, provider)
		if credential.Source == "env" {
			items = append(items, map[string]any{
				"name":     provider.APIKeyEnv,
				"purpose":  "provider_api_key",
				"provider": id,
			})
		}
		if provider.Adapter != config.AdapterMCPStdio {
			continue
		}
		env := stringMapForService(provider.Settings["env"])
		names := make([]string, 0, len(env))
		for name := range env {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			items = append(items, map[string]any{
				"name":     name,
				"purpose":  "mcp_stdio_env",
				"provider": id,
			})
		}
	}
	return items
}

func normalizeEndpointBaseURL(value string) (string, error) {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.Opaque != "" {
		return "", fmt.Errorf("base_url 必须是包含 scheme 和 host 的有效 HTTP(S) URL")
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("base_url 只支持 http 或 https")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || strings.Contains(value, "#") {
		return "", fmt.Errorf("base_url 不允许包含 userinfo、query 或 fragment")
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}

func providerSetupFailure(err error, provider, configFile string) map[string]any {
	errorType := "config_error"
	message := err.Error()
	if typed, ok := err.(*ProviderSetupError); ok {
		errorType = typed.ErrorType
		message = typed.Message
	}
	return map[string]any{
		"ok":          false,
		"error_type":  errorType,
		"error":       message,
		"provider":    provider,
		"config_file": configFile,
	}
}

func setupError(errorType, message string) error {
	return &ProviderSetupError{ErrorType: errorType, Message: message}
}

func providerDisabled(value any) bool {
	if disabled, ok := value.(bool); ok {
		return !disabled
	}
	switch strings.ToLower(strings.TrimSpace(fmt.Sprint(value))) {
	case "false", "0", "no", "off":
		return true
	default:
		return false
	}
}

func stringMapForService(value any) map[string]string {
	out := map[string]string{}
	switch typed := value.(type) {
	case map[string]string:
		for key, item := range typed {
			out[key] = item
		}
	case map[string]any:
		for key, item := range typed {
			out[key] = fmt.Sprint(item)
		}
	}
	return out
}
