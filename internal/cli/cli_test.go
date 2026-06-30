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

func TestProviderCommandAliasesCoverMCPToolNames(t *testing.T) {
	for provider, cases := range map[string]map[string]string{
		"exa": {
			"web-search":     "web_search_exa",
			"web_search_exa": "web_search_exa",
			"web-fetch":      "web_fetch_exa",
			"web_fetch_exa":  "web_fetch_exa",
		},
		"tavily": {
			"search":         "tavily_search",
			"tavily_search":  "tavily_search",
			"extract":        "tavily_extract",
			"tavily_extract": "tavily_extract",
			"map":            "tavily_map",
			"tavily_map":     "tavily_map",
			"crawl":          "tavily_crawl",
			"tavily_crawl":   "tavily_crawl",
		},
		"context7": {
			"resolve-library-id": "resolve_library_id",
			"resolve_library_id": "resolve_library_id",
			"query-docs":         "query_docs",
			"query_docs":         "query_docs",
		},
		"deepwiki": {
			"ask-question":        "ask_question",
			"ask_question":        "ask_question",
			"read-wiki-structure": "read_wiki_structure",
			"read_wiki_structure": "read_wiki_structure",
			"read-wiki-contents":  "read_wiki_contents",
			"read_wiki_contents":  "read_wiki_contents",
		},
	} {
		for alias, want := range cases {
			got, ok := canonicalProviderTool(provider, alias)
			if !ok || got != want {
				t.Fatalf("%s %s = (%q, %v), want %q", provider, alias, got, ok, want)
			}
		}
	}
}

func TestGlobalMCPRoutesOriginalToolNames(t *testing.T) {
	for tool, want := range map[string]providerToolRoute{
		"web_search_exa":      {provider: "exa", tool: "web_search_exa"},
		"web_fetch_exa":       {provider: "exa", tool: "web_fetch_exa"},
		"tavily_search":       {provider: "tavily", tool: "tavily_search"},
		"tavily_extract":      {provider: "tavily", tool: "tavily_extract"},
		"tavily_map":          {provider: "tavily", tool: "tavily_map"},
		"tavily_crawl":        {provider: "tavily", tool: "tavily_crawl"},
		"firecrawl_search":    {provider: "firecrawl", tool: "firecrawl_search"},
		"firecrawl_scrape":    {provider: "firecrawl", tool: "firecrawl_scrape"},
		"firecrawl_map":       {provider: "firecrawl", tool: "firecrawl_map"},
		"firecrawl_crawl":     {provider: "firecrawl", tool: "firecrawl_crawl"},
		"resolve_library_id":  {provider: "context7", tool: "resolve_library_id"},
		"query_docs":          {provider: "context7", tool: "query_docs"},
		"ask_question":        {provider: "deepwiki", tool: "ask_question"},
		"read_wiki_structure": {provider: "deepwiki", tool: "read_wiki_structure"},
		"read_wiki_contents":  {provider: "deepwiki", tool: "read_wiki_contents"},
	} {
		if got := mcpToolRoutes[tool]; got != want {
			t.Fatalf("mcp route %s = %#v, want %#v", tool, got, want)
		}
	}
}

func TestExaProviderCommandDoesNotStealLegacyExaAlias(t *testing.T) {
	if shouldDispatchProviderCommand("exa", []string{"golang"}) {
		t.Fatal("onesearch exa <query> should keep legacy exa-search alias behavior")
	}
	if shouldDispatchProviderCommand("exa", []string{"search", "golang"}) {
		t.Fatal("onesearch exa search <query> should keep legacy exa-search alias behavior")
	}
	if shouldDispatchProviderCommand("exa", []string{"fetch", "golang"}) {
		t.Fatal("onesearch exa fetch <query> should keep legacy exa-search alias behavior")
	}
	if !shouldDispatchProviderCommand("exa", []string{"web-search", "golang"}) {
		t.Fatal("onesearch exa web-search should dispatch provider group command")
	}
	if shouldDispatchProviderCommand("exa", []string{"--", "web-search"}) {
		t.Fatal("onesearch exa -- web-search should keep legacy exa-search alias behavior")
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
	for _, id := range []string{"onesearch-cli", "exa", "tavily", "firecrawl", "context7", "deepwiki", "anysearch", "zhipu", "mcp-tools"} {
		if !found[id] {
			t.Fatalf("skills list missing %s: %#v", id, got["skills"])
		}
	}
}

func TestSkillsShowCommandPrintsContent(t *testing.T) {
	t.Setenv("ONESEARCH_CONFIG_DIR", t.TempDir())
	output := captureStdout(t, func() {
		if code := Execute([]string{"skills", "show", "mcp", "--format", "content"}); code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
	})
	if !strings.Contains(output, "Onesearch MCP Tool Router") || !strings.Contains(output, "onesearch mcp web_search_exa") {
		t.Fatalf("skills show content = %q", output)
	}
}

func TestSkillsShowProviderSkillPrintsToolCommands(t *testing.T) {
	t.Setenv("ONESEARCH_CONFIG_DIR", t.TempDir())
	for _, tc := range []struct {
		name string
		want []string
	}{
		{name: "exa", want: []string{"Onesearch Exa", "web_search_exa", "onesearch exa web-fetch"}},
		{name: "tavily", want: []string{"Onesearch Tavily", "tavily_crawl", "onesearch tavily extract"}},
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
	if !found["tavily"] || !found["firecrawl"] || !found["mcp-tools"] {
		t.Fatalf("site_crawl skills missing provider entries: %#v", got["skills"])
	}
	if found["onesearch-cli"] {
		t.Fatalf("site_crawl filter should not include router-only skill: %#v", got["skills"])
	}
}

func TestLoadSkillListCompatibilityCommand(t *testing.T) {
	t.Setenv("ONESEARCH_CONFIG_DIR", t.TempDir())
	output := captureStdout(t, func() {
		if code := Execute([]string{"load_skill", "list", "--format", "json"}); code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
	})
	var got map[string]any
	if err := json.Unmarshal([]byte(output), &got); err != nil {
		t.Fatal(err)
	}
	if got["ok"] != true || got["total"].(float64) < 1 {
		t.Fatalf("load_skill list = %#v", got)
	}
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
