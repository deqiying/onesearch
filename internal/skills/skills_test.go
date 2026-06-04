package skills

import (
	"strings"
	"testing"
)

func TestReadMarkdownSupportsCapabilityAliases(t *testing.T) {
	for _, name := range []string{"search", "docs", "fetch", "deep-research"} {
		text, err := ReadMarkdown(name)
		if err != nil {
			t.Fatalf("ReadMarkdown(%q) error: %v", name, err)
		}
		if !strings.HasPrefix(text, "---\nname: onesearch-") {
			t.Fatalf("ReadMarkdown(%q) returned unexpected content: %.40q", name, text)
		}
	}
}

func TestReadMarkdownIncludesFullSkillGuidance(t *testing.T) {
	cases := map[string][]string{
		"search":        {"Routing guidance:", "Do not use AnySearch as the default `source_search` route"},
		"docs":          {"Workflow:", "For Context7, resolve the library first"},
		"fetch":         {"Workflow:", "If `fetch` returns a config error"},
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
	for _, name := range []string{"search", "docs", "fetch", "deep-research"} {
		files, err := LoadFiles(name)
		if err != nil {
			t.Fatalf("LoadFiles(%q) error: %v", name, err)
		}
		if !hasFile(files, "SKILL.md") || !hasFile(files, "agents/openai.yaml") {
			t.Fatalf("LoadFiles(%q) missing required files: %#v", name, fileNames(files))
		}
	}

	files, err := LoadFiles("base")
	if err != nil {
		t.Fatalf("LoadFiles(base) error: %v", err)
	}
	if !hasFile(files, "references/cli-contract.md") {
		t.Fatalf("base skill missing cli contract reference: %#v", fileNames(files))
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
