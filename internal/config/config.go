package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	DefaultXAIURL            = "https://api.x.ai/v1"
	DefaultXAIModel          = "grok-4-fast"
	DefaultValidationLevel   = "balanced"
	DefaultFallbackMode      = "auto"
	DefaultMinimumProfile    = "standard"
	DefaultExaBaseURL        = "https://api.exa.ai"
	DefaultContext7BaseURL   = "https://context7.com"
	DefaultZhipuAPIURL       = "https://open.bigmodel.cn/api"
	DefaultZhipuSearchEngine = "search_std"
	DefaultTavilyAPIURL      = "https://api.tavily.com"
	DefaultFirecrawlAPIURL   = "https://api.firecrawl.dev/v2"
	DefaultAnySearchAPIURL   = "https://api.anysearch.com/mcp"
	DefaultDeepWikiAPIURL    = "https://mcp.deepwiki.com/mcp"
)

type Config struct {
	ConfigFile          string
	ConfigDirSource     string
	MissingConfigFile   bool
	CreatedConfigFile   bool
	InitializationError string
}

type PathInfo struct {
	OK                              bool   `json:"ok"`
	ConfigFile                      string `json:"config_file"`
	ConfigDir                       string `json:"config_dir"`
	ConfigDirSource                 string `json:"config_dir_source"`
	DefaultConfigFile               string `json:"default_config_file"`
	ConfigDirOverrideValue          string `json:"config_dir_override_value"`
	ConfigDirOverrideMatchesDefault bool   `json:"config_dir_override_matches_default"`
	Exists                          bool   `json:"exists"`
}

func Load() *Config {
	dir, source := resolveConfigDir()
	c := &Config{
		ConfigFile:      filepath.Join(dir, "config.json"),
		ConfigDirSource: source,
	}
	missing, created, err := c.EnsureInitialized()
	c.MissingConfigFile = missing
	c.CreatedConfigFile = created
	if err != nil {
		c.InitializationError = err.Error()
	}
	return c
}

func defaultConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".", ".onesearch")
	}
	return filepath.Join(home, ".config", "onesearch")
}

func overrideDir() string {
	if value := strings.TrimSpace(os.Getenv("ONESEARCH_CONFIG_DIR")); value != "" {
		return value
	}
	return ""
}

func resolveConfigDir() (string, string) {
	if value := overrideDir(); value != "" {
		return filepath.Clean(value), "environment"
	}
	def := defaultConfigDir()
	return def, "default"
}

func (c *Config) PathInfo() PathInfo {
	def := filepath.Join(defaultConfigDir(), "config.json")
	return PathInfo{
		OK:                              true,
		ConfigFile:                      c.ConfigFile,
		ConfigDir:                       filepath.Dir(c.ConfigFile),
		ConfigDirSource:                 c.ConfigDirSource,
		DefaultConfigFile:               def,
		ConfigDirOverrideValue:          overrideDir(),
		ConfigDirOverrideMatchesDefault: overrideMatchesDefault(),
		Exists:                          fileExists(c.ConfigFile),
	}
}

func (c *Config) LoadFile() map[string]any {
	if c == nil {
		return map[string]any{}
	}
	data, err := os.ReadFile(c.ConfigFile)
	if err != nil {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return map[string]any{}
	}
	if out == nil {
		return map[string]any{}
	}
	return out
}

func (c *Config) SaveFile(data map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(c.ConfigFile), 0o755); err != nil {
		return fmt.Errorf("无法创建配置目录: %w", err)
	}
	body, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	if err := os.WriteFile(c.ConfigFile, body, 0o600); err != nil {
		return fmt.Errorf("无法保存配置文件: %w", err)
	}
	return nil
}

func (c *Config) SetFile(data map[string]any) error {
	if data == nil {
		data = map[string]any{}
	}
	return c.SaveFile(data)
}

func (c *Config) EnsureInitialized() (bool, bool, error) {
	if c == nil {
		return false, false, errors.New("nil config")
	}
	if fileExists(c.ConfigFile) {
		return false, false, nil
	}
	if err := c.SaveFile(InitialRuntimeSchema()); err != nil {
		return true, false, err
	}
	return true, true, nil
}

func (c *Config) Get(key, fallback string) string {
	key = strings.ToUpper(strings.TrimSpace(key))
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func (c *Config) Metadata() map[string]any {
	pathInfo := c.PathInfo()
	return map[string]any{
		"config_file":                         c.ConfigFile,
		"config_dir":                          filepath.Dir(c.ConfigFile),
		"config_dir_source":                   c.ConfigDirSource,
		"config_missing":                      c.MissingConfigFile,
		"config_created":                      c.CreatedConfigFile,
		"config_initialization_error":         c.InitializationError,
		"default_config_file":                 pathInfo.DefaultConfigFile,
		"config_dir_override_value":           pathInfo.ConfigDirOverrideValue,
		"config_dir_override_matches_default": pathInfo.ConfigDirOverrideMatchesDefault,
	}
}

func ApplyModelSuffixForURL(model, apiURL string) string {
	if strings.Contains(strings.ToLower(apiURL), "openrouter") && !strings.Contains(model, ":online") {
		return model + ":online"
	}
	return model
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func samePath(left, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if runtime.GOOS == "windows" {
		left = strings.ToLower(left)
		right = strings.ToLower(right)
	}
	return left == right
}

func overrideMatchesDefault() bool {
	value := overrideDir()
	if value == "" {
		return false
	}
	return samePath(value, defaultConfigDir())
}

func ValidateWriteable(c *Config) error {
	if c == nil {
		return errors.New("nil config")
	}
	return os.MkdirAll(filepath.Dir(c.ConfigFile), 0o755)
}
