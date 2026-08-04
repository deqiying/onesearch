package output

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestJSONFormattingDefaultsCompactAndPrettyIsOptIn(t *testing.T) {
	data := map[string]any{
		"ok":     true,
		"html":   "<tag>",
		"nested": map[string]any{"value": "line one\nline two"},
	}
	compact := RenderWithOptions("custom", data, Options{Format: "json", Verbosity: "verbose"})
	pretty := RenderWithOptions("custom", data, Options{Format: "json", Pretty: true, Verbosity: "verbose"})

	if !strings.HasSuffix(compact, "\n") || strings.Count(compact, "\n") != 1 || strings.Contains(compact, "\n  ") {
		t.Fatalf("default JSON is not one compact line with a trailing LF: %q", compact)
	}
	if !strings.HasSuffix(pretty, "\n") || strings.Count(pretty, "\n") <= 1 || !strings.Contains(pretty, "\n  \"") {
		t.Fatalf("pretty JSON is not indented with a trailing LF: %q", pretty)
	}
	if strings.Contains(compact, `\u003c`) || strings.Contains(pretty, `\u003c`) {
		t.Fatalf("JSON formatting changed HTML escaping: compact=%q pretty=%q", compact, pretty)
	}

	var compactValue, prettyValue any
	if err := json.Unmarshal([]byte(compact), &compactValue); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(pretty), &prettyValue); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(compactValue, prettyValue) {
		t.Fatalf("compact and pretty JSON differ semantically:\ncompact=%#v\npretty=%#v", compactValue, prettyValue)
	}
}

func TestJSONFormattingDoesNotChangeQuietOrVerbosePayloads(t *testing.T) {
	data := map[string]any{
		"ok":       true,
		"provider": "exa",
		"tool":     "web_search_exa",
		"query":    "format contract",
		"content":  strings.Repeat("payload ", 200),
		"result":   map[string]any{"results": []map[string]any{{"title": "Result", "url": "https://example.com"}}},
	}
	for _, verbosity := range []string{"quiet", "verbose"} {
		compact := RenderWithOptions("exa", data, Options{Format: "json", Verbosity: verbosity})
		pretty := RenderWithOptions("exa", data, Options{Format: "json", Pretty: true, Verbosity: verbosity})

		var compactValue, prettyValue any
		if err := json.Unmarshal([]byte(compact), &compactValue); err != nil {
			t.Fatalf("decode compact %s JSON: %v", verbosity, err)
		}
		if err := json.Unmarshal([]byte(pretty), &prettyValue); err != nil {
			t.Fatalf("decode pretty %s JSON: %v", verbosity, err)
		}
		if !reflect.DeepEqual(compactValue, prettyValue) {
			t.Fatalf("%s compact and pretty payloads differ:\ncompact=%#v\npretty=%#v", verbosity, compactValue, prettyValue)
		}
	}
}

func TestPrettyDoesNotAffectNonJSONFormats(t *testing.T) {
	data := map[string]any{"ok": true, "content": "plain text"}
	for _, format := range []string{"content", "markdown"} {
		compact := RenderWithOptions("custom", data, Options{Format: format, Verbosity: "verbose"})
		pretty := RenderWithOptions("custom", data, Options{Format: format, Pretty: true, Verbosity: "verbose"})
		if compact != pretty {
			t.Fatalf("Pretty changed %s output:\nwithout=%q\nwith=%q", format, compact, pretty)
		}
	}
}

func TestQuietSearchOutputMergesFetchedPages(t *testing.T) {
	data := map[string]any{
		"ok":    true,
		"query": "rank",
		"used": []map[string]any{
			{
				"capability": "page_fetch",
				"role":       "page_evidence",
				"providers": []map[string]any{
					{
						"provider": "tavily",
						"status":   "ok",
						"result": map[string]any{
							"pages": []map[string]any{
								{"url": "https://example.com/a", "content_preview": "alpha", "content_length": 5},
							},
						},
					},
				},
			},
			{
				"capability": "page_fetch",
				"role":       "source_evidence",
				"providers": []map[string]any{
					{
						"provider": "tavily",
						"status":   "ok",
						"result": map[string]any{
							"pages": []map[string]any{
								{"url": "https://example.com/b", "content_preview": "beta", "content_length": 4},
							},
						},
					},
				},
			},
		},
	}

	rendered := RenderWithOptions("search", data, Options{Format: "json", Verbosity: "quiet"})
	var got map[string]any
	if err := json.Unmarshal([]byte(rendered), &got); err != nil {
		t.Fatal(err)
	}
	pageFetch := got["used"].(map[string]any)["page_fetch"].(map[string]any)
	roles := pageFetch["roles"].([]any)
	if len(roles) != 2 {
		t.Fatalf("roles = %#v, want both page_fetch roles", roles)
	}
	result := pageFetch["providers"].(map[string]any)["tavily"].(map[string]any)["result"].(map[string]any)
	pages := result["pages"].([]any)
	if len(pages) != 2 || result["pages_count"].(float64) != 2 {
		t.Fatalf("pages result = %#v", result)
	}
}

func TestQuietProviderToolOutputKeepsJobEnvelopeWithoutRawResult(t *testing.T) {
	rendered := RenderWithOptions("firecrawl", map[string]any{
		"ok":       true,
		"provider": "firecrawl",
		"tool":     "firecrawl_crawl",
		"url":      "https://example.com",
		"id":       "crawl-1",
		"status":   "submitted",
		"success":  true,
		"result": map[string]any{
			"success": true,
			"id":      "crawl-1",
			"url":     "https://example.com",
			"debug":   strings.Repeat("raw ", 100),
		},
	}, Options{Format: "json", Verbosity: "quiet"})

	var got map[string]any
	if err := json.Unmarshal([]byte(rendered), &got); err != nil {
		t.Fatal(err)
	}
	if got["provider"] != "firecrawl" || got["tool"] != "firecrawl_crawl" || got["id"] != "crawl-1" || got["status"] != "submitted" {
		t.Fatalf("quiet job envelope = %#v", got)
	}
	if _, ok := got["result"]; ok {
		t.Fatalf("quiet provider tool output should omit raw result: %s", rendered)
	}
}

func TestCompactErrorHintUsesTypedRecovery(t *testing.T) {
	tests := []struct {
		errorType  string
		want       string
		rejectText string
	}{
		{errorType: "parameter_error", want: "onesearch schema", rejectText: "onesearch doctor"},
		{errorType: "config_error", want: "onesearch doctor"},
		{errorType: "evidence_error", want: "evidence gap", rejectText: "onesearch doctor"},
		{errorType: "local_error", want: "local path", rejectText: "onesearch doctor"},
		{errorType: "provider_error", want: "--verbose", rejectText: "onesearch doctor"},
	}
	for _, test := range tests {
		t.Run(test.errorType, func(t *testing.T) {
			rendered := RenderWithOptions("skills", map[string]any{
				"ok":         false,
				"error_type": test.errorType,
				"error":      "failure",
			}, Options{Format: "json", Verbosity: "quiet"})
			var got map[string]any
			if err := json.Unmarshal([]byte(rendered), &got); err != nil {
				t.Fatal(err)
			}
			hint, _ := got["hint"].(string)
			if !strings.Contains(hint, test.want) {
				t.Fatalf("%s hint = %q, want %q", test.errorType, hint, test.want)
			}
			if test.rejectText != "" && strings.Contains(hint, test.rejectText) {
				t.Fatalf("%s hint = %q, must not contain %q", test.errorType, hint, test.rejectText)
			}
		})
	}
}

func TestRenderWithOptionsRedactsEveryFormatAndVerbosity(t *testing.T) {
	for _, format := range []string{"json", "content", "markdown"} {
		for _, verbosity := range []string{"quiet", "verbose"} {
			for _, pretty := range []bool{false, true} {
				data := map[string]any{
					"ok":         false,
					"error_type": "network_error",
					"error":      "provider echoed actual-secret in response",
					"api_key":    "field-secret",
					"nested":     []map[string]any{{"authorization": "Bearer field-secret"}},
				}
				rendered := RenderWithOptions("exa", data, Options{Format: format, Pretty: pretty, Verbosity: verbosity, SecretValues: []string{"actual-secret"}})
				if strings.Contains(rendered, "actual-secret") || strings.Contains(rendered, "field-secret") {
					t.Fatalf("%s/%s/pretty=%t leaked a secret: %q", format, verbosity, pretty, rendered)
				}
				if data["api_key"] != "field-secret" || !strings.Contains(data["error"].(string), "actual-secret") {
					t.Fatalf("renderer mutated input for %s/%s/pretty=%t: %#v", format, verbosity, pretty, data)
				}
			}
		}
	}
}

func TestFinalTextRedactionHandlesJSONEscapingInUnknownTypes(t *testing.T) {
	secret := "key-with-\"quote\\slash"
	data := map[string]any{
		"ok":      true,
		"unknown": struct{ Value string }{Value: secret},
	}
	rendered := RenderWithOptions("custom", data, Options{Format: "json", Verbosity: "verbose", SecretValues: []string{secret}})
	if strings.Contains(rendered, secret) || strings.Contains(rendered, `key-with-\"quote\\slash`) {
		t.Fatalf("JSON-escaped secret leaked: %s", rendered)
	}
}

func TestRenderMasksSettingsEnvironmentValuesButKeepsNames(t *testing.T) {
	rendered := RenderWithOptions("config", map[string]any{
		"ok": true,
		"settings": map[string]any{
			"env": map[string]string{"ACCESS_TOKEN": "env-secret", "MODE": "true"},
		},
	}, Options{Format: "json", Verbosity: "verbose"})
	if strings.Contains(rendered, "env-secret") || strings.Contains(rendered, `"MODE": "true"`) {
		t.Fatalf("settings environment value leaked: %s", rendered)
	}
	if !strings.Contains(rendered, "ACCESS_TOKEN") || !strings.Contains(rendered, "MODE") {
		t.Fatalf("settings environment names missing: %s", rendered)
	}
}

func TestDiagnosticContentAndMarkdownIncludePathAndEnvironmentNames(t *testing.T) {
	data := map[string]any{
		"ok":     true,
		"ready":  true,
		"status": "ready",
		"config": map[string]any{
			"file":       "<config-dir>/config.json",
			"dir_source": "environment",
			"dir_env":    "ONESEARCH_CONFIG_DIR",
		},
		"effective_environment": []map[string]any{{
			"name":     "EXA_API_KEY",
			"purpose":  "provider_api_key",
			"provider": "exa",
		}},
		"minimum_profile": map[string]any{"ok": true, "profile": "minimal", "missing": []string{}},
		"capabilities":    map[string]any{},
	}
	for _, format := range []string{"content", "markdown"} {
		rendered := RenderWithOptions("status", data, Options{Format: format, SecretValues: []string{"environment-secret"}})
		for _, want := range []string{"<config-dir>/config.json", "ONESEARCH_CONFIG_DIR", "EXA_API_KEY", "provider_api_key"} {
			if !strings.Contains(rendered, want) {
				t.Fatalf("%s status missing %q: %s", format, want, rendered)
			}
		}
		if strings.Contains(rendered, "environment-secret") {
			t.Fatalf("%s status leaked environment value: %s", format, rendered)
		}
	}
}

func TestConfigSetupContentAndMarkdownContainOnlySafeState(t *testing.T) {
	data := map[string]any{
		"ok":              true,
		"provider":        "exa",
		"enabled":         "auto",
		"api_key_set":     true,
		"api_key_env":     "EXA_API_KEY",
		"api_key_env_set": false,
		"api_key_src":     "config",
		"has_api_key":     true,
		"base_url":        "https://api.exa.ai",
		"config_file":     "<config-dir>/config.json",
		"changed_fields":  []string{"api_key", "enabled"},
	}
	for _, format := range []string{"content", "markdown"} {
		rendered := RenderWithOptions("config", data, Options{Format: format})
		for _, want := range []string{"exa", "auto", "config", "https://api.exa.ai", "api_key"} {
			if !strings.Contains(rendered, want) {
				t.Fatalf("%s config output missing %q: %s", format, want, rendered)
			}
		}
	}
}
