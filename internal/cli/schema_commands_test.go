package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/deqiying/onesearch/internal/commandcontract"
	"github.com/deqiying/onesearch/internal/config"
)

func TestSchemaFullManifestContainsAllCommandsAndMatchesTarget(t *testing.T) {
	fullBytes := runSchemaForTest(t, []string{"schema"}, 0)
	var full commandcontract.Manifest
	if err := json.Unmarshal([]byte(fullBytes), &full); err != nil {
		t.Fatal(err)
	}
	if got := len(full.Commands); got != 44 {
		t.Fatalf("full schema command count = %d, want 44", got)
	}

	targetBytes := runSchemaForTest(t, []string{"schema", "exa", "web-search"}, 0)
	var target commandcontract.Manifest
	if err := json.Unmarshal([]byte(targetBytes), &target); err != nil {
		t.Fatal(err)
	}
	if target.Scope.Mode != "command" || !reflect.DeepEqual(target.Scope.Path, []string{"exa", "web-search"}) {
		t.Fatalf("target scope = %#v", target.Scope)
	}
	if len(target.Commands) != 1 {
		t.Fatalf("target commands = %d, want 1", len(target.Commands))
	}
	var fullEntry commandcontract.ManifestCommand
	for _, command := range full.Commands {
		if command.ID == "exa.web-search" {
			fullEntry = command
			break
		}
	}
	if fullEntry.ID == "" || !reflect.DeepEqual(target.Commands[0], fullEntry) {
		t.Fatalf("target entry differs from full schema entry\ntarget: %#v\nfull: %#v", target.Commands[0], fullEntry)
	}
}

func TestSchemaRejectsAliasTargetsAndNonJSONFormatAsJSONErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "alias target", args: []string{"schema", "s"}, wantErr: "unknown canonical command path: s"},
		{name: "non-JSON format", args: []string{"schema", "--format", "markdown"}, wantErr: "expected one of json"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout := runSchemaForTest(t, test.args, 2)
			var result map[string]any
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatalf("schema error is not JSON: %v\n%s", err, stdout)
			}
			if result["ok"] != false || result["error_type"] != "parameter_error" || !strings.Contains(result["error"].(string), test.wantErr) {
				t.Fatalf("schema error = %#v", result)
			}
		})
	}
}

func TestSchemaOutputIsDeterministicAndPreservesStaticBindings(t *testing.T) {
	first := runSchemaForTest(t, []string{"schema"}, 0)
	second := runSchemaForTest(t, []string{"schema"}, 0)
	if first != second {
		t.Fatal("schema output is not deterministic")
	}

	var manifest commandcontract.Manifest
	if err := json.Unmarshal([]byte(first), &manifest); err != nil {
		t.Fatal(err)
	}
	for _, command := range manifest.Commands {
		if command.ID != "search" {
			continue
		}
		properties := command.InputSchema["properties"].(map[string]any)
		outputProperty := properties["output"].(map[string]any)
		binding := outputProperty["x-cli-binding"].(map[string]any)
		if binding["token"] != "--output" {
			t.Fatalf("x-cli-binding.token = %#v, want --output", binding["token"])
		}
		return
	}
	t.Fatal("search command missing from manifest")
}

func TestSchemaOutputFileMatchesStdoutBytes(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "schema.json")
	stdout := runSchemaForTest(t, []string{"schema", "--output", outputPath}, 0)
	written, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if stdout != string(written) {
		t.Fatalf("stdout and --output differ\nstdout bytes: %d\nfile bytes: %d", len(stdout), len(written))
	}
}

func TestSchemaAndHelpDoNotCreateConfig(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "schema", args: []string{"schema"}},
		{name: "root help", args: []string{"--help"}},
		{name: "command help", args: []string{"search", "--help"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			configDir := filepath.Join(t.TempDir(), "missing-config-dir")
			t.Setenv(config.ConfigDirEnvName, configDir)
			captureStdout(t, func() {
				if code := Execute(test.args); code != 0 {
					t.Fatalf("Execute(%#v) = %d, want 0", test.args, code)
				}
			})
			if _, err := os.Stat(filepath.Join(configDir, "config.json")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("config was created or stat failed unexpectedly: %v", err)
			}
		})
	}
}

func TestStrictParseErrorsDoNotInitializeConfig(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "unknown flag", args: []string{"search", "query", "--typo"}},
		{name: "extra positional", args: []string{"fetch", "https://example.com", "extra"}},
		{name: "mutually exclusive", args: []string{"search", "query", "--quiet", "--verbose"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			configDir := filepath.Join(t.TempDir(), "missing-config")
			t.Setenv(config.ConfigDirEnvName, configDir)
			captureStdout(t, func() {
				if code := Execute(test.args); code != 2 {
					t.Fatalf("Execute(%#v) = %d, want 2", test.args, code)
				}
			})
			if _, err := os.Stat(filepath.Join(configDir, "config.json")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("parse error initialized config: %v", err)
			}
		})
	}
}

func TestEveryPublicHelpPathIsStaticAndRegistryBacked(t *testing.T) {
	configDir := filepath.Join(t.TempDir(), "missing-config-dir")
	t.Setenv(config.ConfigDirEnvName, configDir)

	for _, definition := range commandRegistry.Commands() {
		paths := append([][]string{definition.Path}, definition.Aliases...)
		for _, path := range paths {
			args := append(append([]string{}, path...), "--help")
			stdout := captureStdout(t, func() {
				if code := Execute(args); code != 0 {
					t.Fatalf("Execute(%#v) = %d, want 0", args, code)
				}
			})
			if !strings.Contains(stdout, definition.Summary) || !strings.Contains(stdout, "onesearch "+strings.Join(definition.Path, " ")) {
				t.Fatalf("help for %#v is not registry-backed:\n%s", args, stdout)
			}
		}
	}
	for _, namespace := range commandRegistry.Namespaces() {
		paths := append([][]string{namespace.Path}, namespace.Aliases...)
		for _, path := range paths {
			args := append(append([]string{}, path...), "--help")
			stdout := captureStdout(t, func() {
				if code := Execute(args); code != 0 {
					t.Fatalf("Execute(%#v) = %d, want 0", args, code)
				}
			})
			if !strings.Contains(stdout, namespace.Summary) {
				t.Fatalf("namespace help for %#v is not registry-backed:\n%s", args, stdout)
			}
		}
	}
	if _, err := os.Stat(filepath.Join(configDir, "config.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("help created config or stat failed unexpectedly: %v", err)
	}
}

func TestSchemaMatchesGolden(t *testing.T) {
	got := runSchemaForTest(t, []string{"schema"}, 0)
	path := filepath.Join("testdata", "cli-command-manifest-v1.golden.json")
	if os.Getenv("UPDATE_CLI_COMMAND_MANIFEST_GOLDEN") == "1" {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Fatal("CLI command manifest differs from golden; review the contract diff and rerun with UPDATE_CLI_COMMAND_MANIFEST_GOLDEN=1")
	}
}

func runSchemaForTest(t *testing.T, args []string, wantCode int) string {
	t.Helper()
	return captureStdout(t, func() {
		if code := Execute(args); code != wantCode {
			t.Fatalf("Execute(%#v) = %d, want %d", args, code, wantCode)
		}
	})
}
