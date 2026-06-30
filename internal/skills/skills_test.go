package skills

import (
	"strings"
	"testing"
)

func TestReadMarkdownSupportsCapabilityAliases(t *testing.T) {
	for _, name := range []string{"base", "router", "search", "docs", "fetch", "exa", "tavily", "firecrawl", "context7", "deepwiki", "anysearch", "zhipu", "mcp-tools", "deep-research"} {
		text, err := ReadMarkdown(name)
		if err != nil {
			t.Fatalf("ReadMarkdown(%q) error: %v", name, err)
		}
		text = strings.ReplaceAll(text, "\r\n", "\n")
		if !strings.HasPrefix(text, "---\nname: onesearch-") {
			t.Fatalf("ReadMarkdown(%q) returned unexpected content: %.40q", name, text)
		}
	}
}

func TestReadMarkdownIncludesFullSkillGuidance(t *testing.T) {
	cases := map[string][]string{
		"base":          {"Onesearch CLI Router", "Route By User Intent", "provider skills own provider-specific commands"},
		"search":        {"Routing guidance:", "Do not use AnySearch as the default `source_search` route"},
		"docs":          {"Workflow:", "For Context7, resolve the library first"},
		"fetch":         {"Workflow:", "If `fetch` returns a config error"},
		"exa":           {"web_search_exa", "onesearch exa web-fetch"},
		"tavily":        {"tavily_search", "onesearch tavily crawl"},
		"firecrawl":     {"firecrawl_scrape", "onesearch firecrawl crawl"},
		"context7":      {"resolve_library_id", "onesearch context7 query-docs"},
		"deepwiki":      {"ask_question", "onesearch deepwiki read-wiki-contents"},
		"anysearch":     {"AnySearch", "onesearch anysearch search"},
		"zhipu":         {"Zhipu", "onesearch zhipu-search"},
		"mcp-tools":     {"Onesearch MCP Tool Router", "onesearch mcp web_search_exa"},
		"deep-research": {"Workflow:", "Deep planning does not change the default route order"},
	}
	for name, wants := range cases {
		text, err := ReadMarkdown(name)
		if err != nil {
			t.Fatalf("ReadMarkdown(%q) error: %v", name, err)
		}
		for _, want := range wants {
			if !strings.Contains(text, want) {
				t.Fatalf("ReadMarkdown(%q) missing %q", name, want)
			}
		}
	}
}

func TestLoadFilesIncludesAgentAndReferenceAssets(t *testing.T) {
	for _, name := range []string{"base", "search", "docs", "fetch", "exa", "tavily", "firecrawl", "context7", "deepwiki", "anysearch", "zhipu", "mcp-tools", "deep-research"} {
		files, err := LoadFiles(name)
		if err != nil {
			t.Fatalf("LoadFiles(%q) error: %v", name, err)
		}
		if !hasFile(files, "SKILL.md") || !hasFile(files, "agents/openai.yaml") {
			t.Fatalf("LoadFiles(%q) missing required files: %#v", name, fileNames(files))
		}
	}

	files, err := LoadFiles("router")
	if err != nil {
		t.Fatalf("LoadFiles(router) error: %v", err)
	}
	if !hasFile(files, "references/cli-contract.md") {
		t.Fatalf("router skill missing cli contract reference: %#v", fileNames(files))
	}
}

func TestDefinitionsAndDescribeExposeSkillMetadata(t *testing.T) {
	found := map[string]bool{}
	for _, def := range Definitions() {
		found[def.ID] = true
		switch def.ID {
		case "onesearch-cli":
			if !contains(def.Capabilities, "routing") || !contains(def.Aliases, "router") {
				t.Fatalf("onesearch-cli metadata = %#v", def)
			}
		case "exa":
			if !contains(def.Capabilities, "page_fetch") || !contains(def.Aliases, "web_search_exa") {
				t.Fatalf("exa metadata = %#v", def)
			}
		case "tavily":
			if !contains(def.Capabilities, "site_crawl") || !contains(def.Aliases, "tavily_crawl") {
				t.Fatalf("tavily metadata = %#v", def)
			}
		case "mcp-tools":
			if !contains(def.Capabilities, "page_fetch") || !contains(def.Aliases, "mcp") {
				t.Fatalf("mcp-tools metadata = %#v", def)
			}
		}
	}
	for _, id := range []string{"onesearch-cli", "exa", "tavily", "firecrawl", "context7", "deepwiki", "anysearch", "zhipu", "mcp-tools"} {
		if !found[id] {
			t.Fatalf("Definitions missing %s", id)
		}
	}
	def, err := Describe("web_search_exa")
	if err != nil {
		t.Fatal(err)
	}
	if def.ID != "exa" {
		t.Fatalf("Describe(web_search_exa) = %#v, want exa", def)
	}
	def, err = Describe("tavily_crawl")
	if err != nil {
		t.Fatal(err)
	}
	if def.ID != "tavily" {
		t.Fatalf("Describe(tavily_crawl) = %#v, want tavily", def)
	}
	def, err = Describe("mcp")
	if err != nil {
		t.Fatal(err)
	}
	if def.ID != "mcp-tools" {
		t.Fatalf("Describe(mcp) = %#v, want mcp-tools", def)
	}
}

func hasFile(files []File, name string) bool {
	for _, file := range files {
		if file.Path == name {
			return true
		}
	}
	return false
}

func fileNames(files []File) []string {
	out := make([]string, 0, len(files))
	for _, file := range files {
		out = append(out, file.Path)
	}
	return out
}

func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
