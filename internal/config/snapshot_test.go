package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestReadOnlySnapshotMissingDoesNotInitialize(t *testing.T) {
	for _, nested := range []bool{false, true} {
		t.Run(map[bool]string{false: "readonly-directory", true: "missing-directory"}[nested], func(t *testing.T) {
			dir := t.TempDir()
			if nested {
				dir = filepath.Join(dir, "not-created")
			} else {
				if err := os.Chmod(dir, 0o555); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
			}
			t.Setenv(ConfigDirEnvName, dir)
			c, err := LoadReadOnlySnapshot()
			if err != nil || c == nil {
				t.Fatalf("snapshot = %v, %v", c, err)
			}
			if !c.MissingConfigFile || c.CreatedConfigFile || c.PathInfo().Exists {
				t.Fatalf("unexpected metadata: %#v", c.Metadata())
			}
			raw, err := c.LoadFileStrict()
			if err != nil || len(raw) != 0 {
				t.Fatalf("raw = %#v, %v", raw, err)
			}
			raw["changed"] = true
			if len(c.LoadFile()) != 0 {
				t.Fatal("empty snapshot was mutated")
			}
			r, err := LoadRuntimeStrict(c)
			if err != nil || r.Source != "builtin" || len(r.Providers) == 0 {
				t.Fatalf("builtin runtime = %#v, %v", r, err)
			}
			assertSnapshotWritesRejected(t, c)
			if _, err := os.Stat(c.ConfigFile); !os.IsNotExist(err) {
				t.Fatalf("config unexpectedly exists: %v", err)
			}
			if nested {
				if _, err := os.Stat(dir); !os.IsNotExist(err) {
					t.Fatalf("directory unexpectedly exists: %v", err)
				}
			}
		})
	}
}

func TestReadOnlySnapshotRejectsInvalidAndOversizedConfig(t *testing.T) {
	for name, body := range map[string]string{
		"json": `{"api_key":"sensitive-value",`,
		"null": `null`, "array": `[]`, "empty": `{}`,
		"providers": `{"schema_version":1,"providers":[],"routes":{}}`,
		"version":   `{"schema_version":2,"providers":{},"routes":{}}`,
		"oversize":  strings.Repeat(" ", maxSnapshotConfigBytes+1),
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv(ConfigDirEnvName, dir)
			path := filepath.Join(dir, "config.json")
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			c, err := LoadReadOnlySnapshot()
			assertSnapshotSafeError(t, c, err, dir)
			// 修复文件不会使失败快照变成成功或丢失原始安全错误。
			writeSnapshotTestConfig(t, path, InitialRuntimeSchema())
			if _, strictErr := c.LoadFileStrict(); strictErr != err {
				t.Fatalf("error not preserved: %v, want %v", strictErr, err)
			}
			if _, runtimeErr := LoadRuntimeStrict(c); runtimeErr != err {
				t.Fatalf("runtime error not preserved: %v", runtimeErr)
			}
			fresh, freshErr := LoadReadOnlySnapshot()
			if freshErr != nil || fresh == nil {
				t.Fatalf("new snapshot did not see repair: %v", freshErr)
			}
		})
	}
}

func TestReadOnlySnapshotRejectsUnreadableConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(ConfigDirEnvName, dir)
	// 目录在包括 Windows 和高权限测试进程的所有平台均不可作为配置读取。
	if err := os.Mkdir(filepath.Join(dir, "config.json"), 0o700); err != nil {
		t.Fatal(err)
	}
	c, err := LoadReadOnlySnapshot()
	assertSnapshotSafeError(t, c, err, dir)
}

func TestReadOnlySnapshotFreezesFileEnvironmentAndMetadata(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(ConfigDirEnvName, dir)
	t.Setenv("SNAPSHOT_CUSTOM_KEY", "env-before")
	t.Setenv("SNAPSHOT_EMPTY_KEY", "")
	path := filepath.Join(dir, "config.json")
	raw := InitialRuntimeSchema()
	provider := raw["providers"].(map[string]any)["exa"].(map[string]any)
	provider["api_key"] = "direct-before"
	provider["api_key_env"] = "SNAPSHOT_CUSTOM_KEY"
	raw["nested"] = []any{map[string]any{"secret": "nested-before"}}
	writeSnapshotTestConfig(t, path, raw)
	c, err := LoadReadOnlySnapshot()
	if err != nil {
		t.Fatal(err)
	}
	metadata, pathInfo := c.Metadata(), c.PathInfo()
	// 普通读取和严格读取返回的嵌套 map、array 均可独立修改。
	for _, strict := range []bool{false, true} {
		copy := c.LoadFile()
		if strict {
			copy, err = c.LoadFileStrict()
			if err != nil {
				t.Fatal(err)
			}
		}
		copy["nested"].([]any)[0].(map[string]any)["secret"] = "mutated"
		copy["providers"].(map[string]any)["exa"].(map[string]any)["api_key"] = "mutated"
		delete(copy, "routes")
	}
	if got := c.LoadFile()["nested"].([]any)[0].(map[string]any)["secret"]; got != "nested-before" {
		t.Fatalf("deep copy failed: %v", got)
	}
	provider["api_key"] = "direct-after"
	writeSnapshotTestConfig(t, path, raw)
	t.Setenv("SNAPSHOT_CUSTOM_KEY", "env-after")
	t.Setenv("SNAPSHOT_EMPTY_KEY", "not-empty")
	t.Setenv("SNAPSHOT_LATE_KEY", "late")
	fresh, err := LoadReadOnlySnapshot()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		c           *Config
		direct, env string
	}{{c, "direct-before", "env-before"}, {fresh, "direct-after", "env-after"}} {
		r := LoadRuntime(test.c)
		state := ResolveProviderCredential(test.c, r.Providers["exa"])
		if state.DirectValue != test.direct || state.EnvironmentValue != test.env || state.Value != test.direct || state.Source != "config" {
			t.Fatalf("credential precedence/freeze failed: %#v", state)
		}
		p := r.Providers["exa"]
		p.APIKey = ""
		if got := ResolveProviderCredential(test.c, p).Value; got != test.env {
			t.Fatalf("environment fallback = %q, want %q", got, test.env)
		}
	}
	if got := c.Get(" snapshot_custom_key ", "fallback"); got != "env-before" {
		t.Fatalf("Get normalization/freeze = %q", got)
	}
	if c.Get("SNAPSHOT_EMPTY_KEY", "fallback") != "" || c.Get("SNAPSHOT_LATE_KEY", "fallback") != "fallback" {
		t.Fatal("empty or missing environment value not frozen")
	}
	t.Setenv(ConfigDirEnvName, t.TempDir())
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c.PathInfo(), pathInfo) || !reflect.DeepEqual(c.Metadata(), metadata) {
		t.Fatal("metadata changed after environment/file rotation")
	}
	if got := LoadRuntime(c).Providers["exa"].APIKey; got != "direct-before" {
		t.Fatalf("snapshot reread deleted file: %q", got)
	}
	assertSnapshotWritesRejected(t, c)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("write recreated deleted config: %v", err)
	}
}

func TestReadOnlySnapshotMissingMetadataRemainsFrozen(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(ConfigDirEnvName, dir)
	c, err := LoadReadOnlySnapshot()
	if err != nil {
		t.Fatal(err)
	}
	before := c.Metadata()
	writeSnapshotTestConfig(t, c.ConfigFile, InitialRuntimeSchema())
	if c.PathInfo().Exists || !reflect.DeepEqual(c.Metadata(), before) || len(c.LoadFile()) != 0 {
		t.Fatal("missing snapshot observed subsequently created file")
	}
	fresh, err := LoadReadOnlySnapshot()
	if err != nil || !fresh.PathInfo().Exists || fresh.MissingConfigFile {
		t.Fatalf("fresh snapshot did not see created file: %v", err)
	}
	body, err := os.ReadFile(c.ConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	assertSnapshotWritesRejected(t, fresh)
	after, err := os.ReadFile(c.ConfigFile)
	if err != nil || string(after) != string(body) {
		t.Fatalf("write changed existing file: %v", err)
	}
}

func assertSnapshotSafeError(t *testing.T, c *Config, err error, path string) {
	t.Helper()
	if c == nil || err == nil {
		t.Fatalf("expected nonnil config and error, got %v, %v", c, err)
	}
	if c.MissingConfigFile || c.CreatedConfigFile || !c.PathInfo().Exists || c.InitializationError != err.Error() {
		t.Fatalf("invalid error metadata: %#v", c.Metadata())
	}
	if strings.Contains(err.Error(), path) || strings.Contains(err.Error(), "sensitive-value") {
		t.Fatalf("error leaks sensitive data: %v", err)
	}
	assertSnapshotWritesRejected(t, c)
}

func assertSnapshotWritesRejected(t *testing.T, c *Config) {
	t.Helper()
	for name, write := range map[string]func() error{
		"SaveFile":           func() error { return c.SaveFile(InitialRuntimeSchema()) },
		"SetFile":            func() error { return c.SetFile(nil) },
		"EnsureInitialized":  func() error { _, _, err := c.EnsureInitialized(); return err },
		"PatchProviderSetup": func() error { _, err := c.PatchProviderSetup("exa", ProviderSetupPatch{}); return err },
		"ValidateWriteable":  func() error { return ValidateWriteable(c) },
	} {
		if err := write(); !errors.Is(err, errReadOnlySnapshot) {
			t.Errorf("%s error = %v, want read-only", name, err)
		}
	}
}

func writeSnapshotTestConfig(t *testing.T, path string, raw map[string]any) {
	t.Helper()
	body, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
}
