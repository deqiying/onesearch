package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/deqiying/onesearch/internal/config"
	"github.com/deqiying/onesearch/internal/service"
)

func TestSharedExecutionBindingsPreserveErrorsAndProviderAnnotations(t *testing.T) {
	t.Setenv(config.ConfigDirEnvName, t.TempDir())
	svc := service.New(config.Load())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, id := range []string{"search", "fetch", "map", "crawl", "repo-wiki", "exa.web-search", "context7.resolve-library-id", "context7.query-docs"} {
		t.Run(id, func(t *testing.T) {
			definition, ok := commandRegistry.LookupID(id)
			if !ok {
				t.Fatal("missing command")
			}
			parsed := &parsedCommand{Definition: definition, Values: map[string]any{"format": "json", "verbose": true, "pretty": true}}
			stdout := captureStdout(t, func() {
				if code := runParsedCommand(ctx, svc, parsed); code != 4 {
					t.Fatalf("exit code=%d want 4", code)
				}
			})
			assertPrettyJSONText(t, stdout)
			var data map[string]any
			if err := json.Unmarshal([]byte(stdout), &data); err != nil {
				t.Fatal(err)
			}
			if data["ok"] != false || data["error_type"] != "network_error" || data["error"] != "context canceled" {
				t.Fatalf("result=%v", data)
			}
			if definition.Provider != "" && (data["provider"] != definition.Provider || data["tool"] != commandBindings[id].HandlerKey) {
				t.Fatalf("missing provider annotation: %v", data)
			}
		})
	}
}

func TestSharedSearchFilterErrorKeepsParameterExitCode(t *testing.T) {
	t.Setenv(config.ConfigDirEnvName, t.TempDir())
	stdout := captureStdout(t, func() {
		if code := Execute([]string{"search", "query", "--providers", "answer=exa;broken", "--format", "json"}); code != 2 {
			t.Fatalf("exit code=%d want 2", code)
		}
	})
	if !strings.Contains(stdout, "parameter_error") || !strings.Contains(stdout, `provider filter \"broken\" must use capability=providers`) {
		t.Fatalf("result=%s", stdout)
	}
}
