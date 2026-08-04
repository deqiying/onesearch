package cli

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deqiying/onesearch/internal/config"
	"github.com/deqiying/onesearch/internal/service"
)

func TestConfigSetupReadsKeyFromStdinWithoutPrintingIt(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(config.ConfigDirEnvName, configDir)
	t.Setenv("EXA_API_KEY", "")
	secret := "stdin-setup-secret"
	useConfigInput(t, false, secret+"\n", "")

	stdout := captureStdout(t, func() {
		if code := Execute([]string{"config", "setup", "exa", "--api-key-stdin", "--format", "json"}); code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
	})
	if strings.Contains(stdout, secret) {
		t.Fatalf("stdout leaked key: %q", stdout)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatal(err)
	}
	if result["provider"] != "exa" || result["api_key_src"] != "config" || result["has_api_key"] != true {
		t.Fatalf("setup result = %#v", result)
	}
	raw, err := os.ReadFile(filepath.Join(configDir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved map[string]any
	if err := json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	provider := saved["providers"].(map[string]any)["exa"].(map[string]any)
	if provider["api_key"] != secret || provider["enabled"] != "auto" {
		t.Fatalf("saved provider = %#v", provider)
	}
}

func TestConfigSetupInteractiveUsesHiddenKeyAndBlankBaseURLDefault(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(config.ConfigDirEnvName, configDir)
	t.Setenv("OPENAI_COMPATIBLE_API_KEY", "")
	secret := "interactive-secret"
	useConfigInput(t, true, "\n", secret)

	stderr := captureStderr(t, func() {
		stdout := captureStdout(t, func() {
			if code := Execute([]string{"config", "setup", "openai-compatible", "--format", "json"}); code != 0 {
				t.Fatalf("exit code = %d, want 0", code)
			}
		})
		if strings.Contains(stdout, secret) {
			t.Fatalf("stdout leaked key: %q", stdout)
		}
		var result map[string]any
		if err := json.Unmarshal([]byte(stdout), &result); err != nil {
			t.Fatal(err)
		}
		if result["provider"] != "openai_compatible" || result["base_url"] != "https://api.openai.com/v1" {
			t.Fatalf("setup result = %#v", result)
		}
	})
	if strings.Contains(stderr, secret) {
		t.Fatalf("stderr leaked key: %q", stderr)
	}
	if !strings.Contains(stderr, "API key") || !strings.Contains(stderr, "Base URL [https://api.openai.com/v1]") {
		t.Fatalf("interactive prompts = %q", stderr)
	}
}

func TestSafeBaseURLPromptOmitsCredentialBearingOrParameterizedURL(t *testing.T) {
	for _, value := range []string{
		"https://user:password@example.com/v1",
		"https://example.com/v1?key=secret",
		"https://example.com/v1#fragment",
	} {
		if got := safeBaseURLPrompt(value); got != "configured value omitted" {
			t.Fatalf("safeBaseURLPrompt(%q) = %q", value, got)
		}
	}
	if got := safeBaseURLPrompt("https://example.com/v1"); got != "https://example.com/v1" {
		t.Fatalf("safe base URL prompt = %q", got)
	}
}

func TestConfigSetupNonInteractiveRequiresExplicitStdin(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(config.ConfigDirEnvName, configDir)
	t.Setenv("EXA_API_KEY", "")
	useConfigInput(t, false, "", "")

	stdout := captureStdout(t, func() {
		if code := Execute([]string{"config", "setup", "exa", "--format", "json"}); code != 2 {
			t.Fatalf("exit code = %d, want 2", code)
		}
	})
	if !strings.Contains(stdout, "--api-key-stdin") {
		t.Fatalf("non-interactive error = %q", stdout)
	}
	loaded := (&config.Config{ConfigFile: filepath.Join(configDir, "config.json")}).LoadFile()
	provider := loaded["providers"].(map[string]any)["exa"].(map[string]any)
	if provider["enabled"] != false || provider["api_key"] != "" {
		t.Fatalf("failed setup changed provider: %#v", provider)
	}
}

func TestConfigSetupFailureRedactsTransientKeyInEveryFormat(t *testing.T) {
	for _, format := range []string{"json", "content", "markdown"} {
		configDir := t.TempDir()
		t.Setenv(config.ConfigDirEnvName, configDir)
		t.Setenv("EXA_API_KEY", "")
		secret := "transient-" + format + "-secret"
		useConfigInput(t, false, secret+"\n", "")
		stdout := captureStdout(t, func() {
			code := Execute([]string{"config", "setup", "exa", "--api-key-stdin", "--base-url", "https://user:password@example.com/v1?key=" + secret, "--format", format})
			if code != 2 {
				t.Fatalf("%s exit code = %d, want 2", format, code)
			}
		})
		if strings.Contains(stdout, secret) {
			t.Fatalf("%s output leaked transient key: %q", format, stdout)
		}
	}
}

func TestConfigSetupOutputFileReceivesSameRedactedContent(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(config.ConfigDirEnvName, configDir)
	t.Setenv("EXA_API_KEY", "")
	secret := "output-file-secret"
	useConfigInput(t, false, secret+"\n", "")
	outputPath := filepath.Join(t.TempDir(), "setup.json")

	stdout := captureStdout(t, func() {
		if code := Execute([]string{"config", "setup", "exa", "--api-key-stdin", "--output", outputPath, "--format", "json"}); code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
	})
	written, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if stdout != string(written) {
		t.Fatalf("stdout and --output differ\nstdout: %q\nfile: %q", stdout, written)
	}
	if strings.Contains(stdout, secret) || strings.Contains(string(written), secret) {
		t.Fatalf("output leaked key: stdout=%q file=%q", stdout, written)
	}
}

func TestConfigSetupRejectsAPIKeyArgument(t *testing.T) {
	t.Setenv(config.ConfigDirEnvName, t.TempDir())
	secret := "argv-secret"
	stdout := captureStdout(t, func() {
		if code := Execute([]string{"config", "setup", "exa", "--api-key", secret, "--format", "json"}); code != 2 {
			t.Fatalf("exit code = %d, want 2", code)
		}
	})
	if strings.Contains(stdout, secret) || !strings.Contains(stdout, "--api-key-stdin") {
		t.Fatalf("argv rejection output = %q", stdout)
	}
}

func TestConfigSetupMalformedSensitiveActivatorDoesNotLeakOrInitializeConfig(t *testing.T) {
	configDir := filepath.Join(t.TempDir(), "missing-config")
	t.Setenv(config.ConfigDirEnvName, configDir)
	secret := "malformed-inline-secret"
	outputPath := filepath.Join(t.TempDir(), "error.json")
	var stdout string
	stderr := captureStderr(t, func() {
		stdout = captureStdout(t, func() {
			code := Execute([]string{"config", "setup", "exa", "--output", outputPath, "--api-key-stdin=" + secret})
			if code != 2 {
				t.Fatalf("exit code = %d, want 2", code)
			}
		})
	})
	written, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{"stdout": stdout, "stderr": stderr, "output": string(written)} {
		if strings.Contains(value, secret) {
			t.Fatalf("%s leaked sensitive inline value: %q", name, value)
		}
	}
	if !strings.Contains(stdout, "expected boolean") || stdout != string(written) {
		t.Fatalf("unexpected safe error output: stdout=%q file=%q", stdout, written)
	}
	if _, err := os.Stat(filepath.Join(configDir, "config.json")); !os.IsNotExist(err) {
		t.Fatalf("parse error initialized config: %v", err)
	}
}

func TestPrintCommandRedactsConfiguredAndEnvironmentKeysFromErrors(t *testing.T) {
	cfg := &config.Config{ConfigFile: filepath.Join(t.TempDir(), "config.json"), ConfigDirSource: "test"}
	raw := config.InitialRuntimeSchema()
	provider := raw["providers"].(map[string]any)["exa"].(map[string]any)
	provider["api_key"] = "configured-secret"
	if err := cfg.SetFile(raw); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EXA_API_KEY", "overridden-environment-secret")
	svc := service.New(cfg)
	for _, format := range []string{"json", "content", "markdown"} {
		stdout := captureStdout(t, func() {
			code := printCommand(svc, "exa", map[string]any{
				"ok":         false,
				"error_type": "network_error",
				"error":      "remote echoed configured-secret and overridden-environment-secret",
			}, formatOutput{format: format, verbosity: "verbose"})
			if code != 4 {
				t.Fatalf("%s exit code = %d, want 4", format, code)
			}
		})
		if strings.Contains(stdout, "configured-secret") || strings.Contains(stdout, "overridden-environment-secret") {
			t.Fatalf("%s output leaked a key: %q", format, stdout)
		}
	}
}

func useConfigInput(t *testing.T, terminal bool, stdin, password string) {
	t.Helper()
	originalTerminal := configInputIsTerminal
	originalPassword := configReadPassword
	originalStdin := configStdin
	reader := strings.NewReader(stdin)
	configInputIsTerminal = func() bool { return terminal }
	configReadPassword = func() ([]byte, error) { return []byte(password), nil }
	configStdin = func() io.Reader { return reader }
	t.Cleanup(func() {
		configInputIsTerminal = originalTerminal
		configReadPassword = originalPassword
		configStdin = originalStdin
	})
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	original := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan []byte, 1)
	errs := make(chan error, 1)
	go func() {
		data, err := io.ReadAll(reader)
		if err != nil {
			errs <- err
			return
		}
		done <- data
	}()
	os.Stderr = writer
	defer func() { os.Stderr = original }()
	fn()
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-errs:
		t.Fatal(err)
	case data := <-done:
		return string(data)
	}
	return ""
}
