package cli

import (
	"encoding/json"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestParseSearchProviderFiltersSupportsScopedExpression(t *testing.T) {
	providers, filters, err := parseSearchProviderFilters("answer=openai_responses;source:tavily;page=firecrawl;repo=deepwiki")
	if err != nil {
		t.Fatal(err)
	}
	if providers != "auto" {
		t.Fatalf("providers = %q, want auto", providers)
	}
	want := map[string]string{
		"answer_search": "openai_responses",
		"source_search": "tavily",
		"page_fetch":    "firecrawl",
		"repo_wiki":     "deepwiki",
	}
	if !reflect.DeepEqual(filters, want) {
		t.Fatalf("filters = %#v, want %#v", filters, want)
	}
}

func TestParseSearchProviderFiltersKeepsLegacyUnscopedProvider(t *testing.T) {
	providers, filters, err := parseSearchProviderFilters("openai_responses")
	if err != nil {
		t.Fatal(err)
	}
	if providers != "openai_responses" {
		t.Fatalf("providers = %q, want openai_responses", providers)
	}
	if filters != nil {
		t.Fatalf("filters = %#v, want nil", filters)
	}
}

func TestOverlayProviderFilterOverridesScopedExpression(t *testing.T) {
	_, filters, err := parseSearchProviderFilters("source_search=exa")
	if err != nil {
		t.Fatal(err)
	}
	filters = overlayProviderFilter(filters, "source_search", "tavily")
	if got := filters["source_search"]; got != "tavily" {
		t.Fatalf("source_search filter = %q, want tavily", got)
	}
}

func TestProviderCommandsExposeOnlyHumanReadableSubcommands(t *testing.T) {
	for provider, cases := range map[string]map[string]string{
		"exa": {
			"web-search": "web_search_exa",
			"web-fetch":  "web_fetch_exa",
			"similar":    "find_similar",
		},
		"tavily": {
			"search":  "tavily_search",
			"extract": "tavily_extract",
			"map":     "tavily_map",
			"crawl":   "tavily_crawl",
		},
		"context7": {
			"resolve-library-id": "resolve_library_id",
			"query-docs":         "query_docs",
		},
		"deepwiki": {
			"ask-question":        "ask_question",
			"read-wiki-structure": "read_wiki_structure",
			"read-wiki-contents":  "read_wiki_contents",
		},
		"anysearch": {
			"domains": "domains",
			"search":  "search",
			"extract": "extract",
			"batch":   "batch",
		},
		"zhipu": {
			"search": "zhipu_search",
		},
		"ddg": {
			"search":        "search",
			"fetch-content": "fetch_content",
		},
		"freecrawl": {
			"search":        "search",
			"scrape":        "scrape",
			"crawl":         "crawl",
			"deep-research": "deep_research",
		},
	} {
		for alias, want := range cases {
			got, ok := canonicalProviderTool(provider, alias)
			if !ok || got != want {
				t.Fatalf("%s %s = (%q, %v), want %q", provider, alias, got, ok, want)
			}
		}
	}
	for provider, aliases := range map[string][]string{
		"exa":       {"web_search_exa", "web_fetch_exa"},
		"tavily":    {"tavily_search", "tavily_extract", "tavily_map", "tavily_crawl"},
		"firecrawl": {"firecrawl_search", "firecrawl_scrape", "firecrawl_map", "firecrawl_crawl"},
		"context7":  {"resolve_library_id", "query_docs", "library", "docs"},
		"deepwiki":  {"ask_question", "read_wiki_structure", "read_wiki_contents"},
		"ddg":       {"fetch_content"},
		"freecrawl": {"deep_research"},
	} {
		for _, alias := range aliases {
			if got, ok := canonicalProviderTool(provider, alias); ok {
				t.Fatalf("%s %s unexpectedly maps to %q", provider, alias, got)
			}
		}
	}
}

func TestGlobalMCPCommandNoLongerDispatches(t *testing.T) {
	if shouldDispatchProviderCommand("mcp", []string{"web_search_exa"}) {
		t.Fatal("mcp compatibility router should not dispatch")
	}
	if code := Execute([]string{"mcp", "web_search_exa"}); code != 2 {
		t.Fatalf("mcp command exit code = %d, want 2", code)
	}
}

func TestExaProviderCommandDispatchesProviderGroupOnly(t *testing.T) {
	if !shouldDispatchProviderCommand("exa", nil) {
		t.Fatal("onesearch exa should dispatch provider group help/error")
	}
	if !shouldDispatchProviderCommand("exa", []string{"golang"}) {
		t.Fatal("onesearch exa <query> should dispatch provider group and return parameter error")
	}
	if !shouldDispatchProviderCommand("exa", []string{"web-search", "golang"}) {
		t.Fatal("onesearch exa web-search should dispatch provider group command")
	}
	if !shouldDispatchProviderCommand("exa", []string{"similar", "https://example.com"}) {
		t.Fatal("onesearch exa similar should dispatch provider group command")
	}
}

func TestProviderCommandParameterErrorKeepsProviderAndTool(t *testing.T) {
	t.Setenv("ONESEARCH_CONFIG_DIR", t.TempDir())
	output := captureStdout(t, func() {
		if code := Execute([]string{"tavily", "search", "--format", "json"}); code != 2 {
			t.Fatalf("exit code = %d, want 2", code)
		}
	})
	var got map[string]any
	if err := json.Unmarshal([]byte(output), &got); err != nil {
		t.Fatal(err)
	}
	if got["provider"] != "tavily" || got["tool"] != "tavily_search" || got["error_type"] != "parameter_error" {
		t.Fatalf("parameter error envelope = %#v", got)
	}
}

func TestSkillsListCommandIncludesProviderSkills(t *testing.T) {
	t.Setenv("ONESEARCH_CONFIG_DIR", t.TempDir())
	output := captureStdout(t, func() {
		if code := Execute([]string{"skills", "list", "--format", "json"}); code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
	})
	var got map[string]any
	if err := json.Unmarshal([]byte(output), &got); err != nil {
		t.Fatal(err)
	}
	if got["ok"] != true {
		t.Fatalf("skills list = %#v", got)
	}
	found := map[string]bool{}
	for _, raw := range got["skills"].([]any) {
		item := raw.(map[string]any)
		found[item["id"].(string)] = true
	}
	for _, id := range []string{"onesearch-cli", "exa", "tavily", "firecrawl", "context7", "deepwiki", "anysearch", "zhipu", "ddg", "freecrawl"} {
		if !found[id] {
			t.Fatalf("skills list missing %s: %#v", id, got["skills"])
		}
	}
	if found["mcp-tools"] {
		t.Fatalf("skills list should not include mcp-tools: %#v", got["skills"])
	}
}

func TestSkillsShowCommandPrintsContent(t *testing.T) {
	t.Setenv("ONESEARCH_CONFIG_DIR", t.TempDir())
	output := captureStdout(t, func() {
		if code := Execute([]string{"skills", "show", "onesearch-cli", "--format", "content"}); code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
	})
	if !strings.Contains(output, "Onesearch CLI Router") || !strings.Contains(output, "provider direct") {
		t.Fatalf("skills show content = %q", output)
	}
}

func TestSkillsShowProviderSkillPrintsToolCommands(t *testing.T) {
	t.Setenv("ONESEARCH_CONFIG_DIR", t.TempDir())
	for _, tc := range []struct {
		name string
		want []string
	}{
		{name: "exa", want: []string{"Onesearch Exa", "onesearch exa web-search", "onesearch exa web-fetch", "onesearch exa similar"}},
		{name: "tavily", want: []string{"Onesearch Tavily", "onesearch tavily search", "onesearch tavily extract"}},
		{name: "ddg", want: []string{"Onesearch DDG", "onesearch ddg search", "onesearch ddg fetch-content"}},
		{name: "freecrawl", want: []string{"Onesearch Freecrawl", "onesearch freecrawl scrape", "onesearch freecrawl deep-research"}},
	} {
		output := captureStdout(t, func() {
			if code := Execute([]string{"skills", "show", tc.name, "--format", "content"}); code != 0 {
				t.Fatalf("%s exit code = %d, want 0", tc.name, code)
			}
		})
		for _, want := range tc.want {
			if !strings.Contains(output, want) {
				t.Fatalf("skills show %s missing %q: %q", tc.name, want, output)
			}
		}
	}
}

func TestSkillsListCapabilityFilterIncludesProviderSkills(t *testing.T) {
	t.Setenv("ONESEARCH_CONFIG_DIR", t.TempDir())
	output := captureStdout(t, func() {
		if code := Execute([]string{"skills", "list", "--capability", "site_crawl", "--format", "json"}); code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
	})
	var got map[string]any
	if err := json.Unmarshal([]byte(output), &got); err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, raw := range got["skills"].([]any) {
		item := raw.(map[string]any)
		found[item["id"].(string)] = true
	}
	if !found["tavily"] || !found["firecrawl"] {
		t.Fatalf("site_crawl skills missing provider entries: %#v", got["skills"])
	}
	if found["mcp-tools"] {
		t.Fatalf("site_crawl skills should not include mcp-tools: %#v", got["skills"])
	}
	if found["onesearch-cli"] {
		t.Fatalf("site_crawl filter should not include router-only skill: %#v", got["skills"])
	}
}

func TestStatusCommandReportsDirectEndpointAvailability(t *testing.T) {
	t.Setenv("ONESEARCH_CONFIG_DIR", t.TempDir())
	output := captureStdout(t, func() {
		if code := Execute([]string{"status", "--format", "json"}); code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
	})
	var got map[string]any
	if err := json.Unmarshal([]byte(output), &got); err != nil {
		t.Fatal(err)
	}
	if got["ok"] != true || got["ready"] != false {
		t.Fatalf("status = %#v", got)
	}
	capabilities := got["capabilities"].(map[string]any)
	answer := capabilities["answer_search"].(map[string]any)
	if answer["ok"] != false || answer["command"] != "onesearch search" {
		t.Fatalf("answer_search status = %#v", answer)
	}
	direct := got["direct_endpoints"].(map[string]any)
	zhipu := direct["zhipu"].(map[string]any)
	if zhipu["available"] != false {
		t.Fatalf("zhipu direct endpoint should be unavailable by default: %#v", zhipu)
	}
	if !containsTestString(testStrings(zhipu["commands"]), "onesearch zhipu search") {
		t.Fatalf("zhipu commands = %#v", zhipu["commands"])
	}
	ddg := direct["ddg"].(map[string]any)
	if ddg["available"] != false || ddg["reason"] != "disabled" {
		t.Fatalf("ddg direct endpoint should be disabled by default: %#v", ddg)
	}
	if !containsTestString(testStrings(ddg["commands"]), "onesearch ddg fetch-content") {
		t.Fatalf("ddg commands = %#v", ddg["commands"])
	}
}

func containsTestString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func testStrings(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if text, ok := item.(string); ok {
			out = append(out, text)
		}
	}
	return out
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	original := os.Stdout
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
	os.Stdout = writer
	defer func() {
		os.Stdout = original
	}()
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
