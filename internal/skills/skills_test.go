package skills

import (
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/deqiying/onesearch/internal/commandcontract"
)

var canonicalSchemaPaths = map[string][][]string{
	"onesearch": {{"search"}},
	"search":    {{"search"}},
	"docs": {
		{"context7", "resolve-library-id"},
		{"context7", "query-docs"},
		{"exa", "web-search"},
	},
	"fetch":         {{"fetch"}, {"map"}, {"crawl"}},
	"deep-research": {{"deep"}},
	"exa":           {{"exa", "web-search"}, {"exa", "web-fetch"}, {"exa", "similar"}},
	"tavily":        {{"tavily", "search"}, {"tavily", "extract"}, {"tavily", "map"}, {"tavily", "crawl"}},
	"firecrawl":     {{"firecrawl", "search"}, {"firecrawl", "scrape"}, {"firecrawl", "map"}, {"firecrawl", "crawl"}},
	"context7":      {{"context7", "resolve-library-id"}, {"context7", "query-docs"}},
	"deepwiki":      {{"deepwiki", "ask-question"}, {"deepwiki", "read-wiki-structure"}, {"deepwiki", "read-wiki-contents"}},
	"anysearch":     {{"anysearch", "domains"}, {"anysearch", "search"}, {"anysearch", "extract"}, {"anysearch", "batch"}},
	"zhipu":         {{"zhipu", "search"}},
	"ddg":           {{"ddg", "search"}, {"ddg", "fetch-content"}},
	"freecrawl":     {{"freecrawl", "search"}, {"freecrawl", "scrape"}, {"freecrawl", "crawl"}, {"freecrawl", "deep-research"}},
}

func TestDefinitionsMatchBundledAssetFolders(t *testing.T) {
	defs := Definitions()
	if len(defs) != 14 {
		t.Fatalf("Definitions() returned %d definitions, want 14", len(defs))
	}

	entries, err := fs.ReadDir(assets, "assets")
	if err != nil {
		t.Fatal(err)
	}
	assetFolders := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			t.Fatalf("unexpected file directly under embedded assets: %s", entry.Name())
		}
		assetFolders[entry.Name()] = true
	}
	if len(assetFolders) != 14 {
		t.Fatalf("embedded assets contain %d skill folders, want 14", len(assetFolders))
	}

	definitionIDs := make(map[string]bool, len(defs))
	definitionFolders := make(map[string]bool, len(defs))
	for _, def := range defs {
		if definitionIDs[def.ID] {
			t.Fatalf("duplicate Definition.ID %q", def.ID)
		}
		definitionIDs[def.ID] = true
		if definitionFolders[def.Folder] {
			t.Fatalf("duplicate Definition.Folder %q", def.Folder)
		}
		definitionFolders[def.Folder] = true
		if !assetFolders[def.Folder] {
			t.Errorf("Definition %q refers to missing asset folder %q", def.ID, def.Folder)
		}
	}
	for folder := range assetFolders {
		if !definitionFolders[folder] {
			t.Errorf("asset folder %q has no Definition", folder)
		}
	}
}

func TestBundledSkillMetadataContracts(t *testing.T) {
	for _, def := range Definitions() {
		def := def
		t.Run(def.ID, func(t *testing.T) {
			skillFile, err := ReadFile(def.ID, "SKILL.md")
			if err != nil {
				t.Fatal(err)
			}
			frontmatter := parseFrontmatter(t, string(skillFile.Data))
			assertExactKeys(t, frontmatter, "name", "description")
			if frontmatter["name"] != def.Folder {
				t.Fatalf("frontmatter name = %q, want Definition.Folder %q", frontmatter["name"], def.Folder)
			}
			if frontmatter["description"] == "" {
				t.Fatal("frontmatter description is empty")
			}

			metadataFile, err := ReadFile(def.ID, "agents/openai.yaml")
			if err != nil {
				t.Fatal(err)
			}
			metadata := parseOpenAIInterface(t, string(metadataFile.Data))
			assertExactKeys(t, metadata, "display_name", "short_description", "default_prompt")
			for _, field := range []string{"display_name", "short_description", "default_prompt"} {
				if metadata[field] == "" {
					t.Fatalf("interface.%s is empty", field)
				}
			}
			if want := "$" + frontmatter["name"]; !strings.Contains(metadata["default_prompt"], want) {
				t.Fatalf("interface.default_prompt does not mention %q: %q", want, metadata["default_prompt"])
			}
		})
	}
}

func TestCanonicalSchemaPathsResolveAndAreDocumented(t *testing.T) {
	registry := commandcontract.MustDefaultRegistry()
	seen := make(map[string]bool, len(canonicalSchemaPaths))
	for _, def := range Definitions() {
		paths, ok := canonicalSchemaPaths[def.ID]
		if !ok {
			t.Errorf("Definition %q has no canonical schema path contract", def.ID)
			continue
		}
		seen[def.ID] = true
		markdown, err := ReadMarkdown(def.ID)
		if err != nil {
			t.Fatalf("ReadMarkdown(%q): %v", def.ID, err)
		}
		for _, commandPath := range paths {
			if _, ok := registry.LookupCanonical(commandPath...); !ok {
				t.Errorf("%s declares unknown canonical command path %q", def.ID, strings.Join(commandPath, " "))
			}
			want := "onesearch schema " + strings.Join(commandPath, " ") + " --format json"
			if !strings.Contains(markdown, want) {
				t.Errorf("%s does not document targeted schema command %q", def.ID, want)
			}
		}
	}
	for id := range canonicalSchemaPaths {
		if !seen[id] {
			t.Errorf("canonical schema path contract refers to unknown Definition.ID %q", id)
		}
	}
}

func TestSharedReferenceMatchesRegistryAvailabilityPointers(t *testing.T) {
	registry := commandcontract.MustDefaultRegistry()
	workflow, ok := registry.LookupCanonical("search")
	if !ok {
		t.Fatal("search command is missing from the registry")
	}
	direct, ok := registry.LookupCanonical("exa", "web-search")
	if !ok {
		t.Fatal("exa web-search command is missing from the registry")
	}
	if !strings.HasSuffix(workflow.Availability.JSONPointer, "/ok") {
		t.Fatalf("workflow availability pointer = %q, want /ok readiness", workflow.Availability.JSONPointer)
	}
	if !strings.HasSuffix(direct.Availability.JSONPointer, "/available") {
		t.Fatalf("provider availability pointer = %q, want /available readiness", direct.Availability.JSONPointer)
	}

	referenceFile, err := ReadFile("onesearch", "references/agent-execution-contract.md")
	if err != nil {
		t.Fatal(err)
	}
	reference := string(referenceFile.Data)
	for _, want := range []string{
		"capabilities.<capability>.ok == true",
		"`available` lists the provider IDs",
		"direct_endpoints.<provider>.available",
	} {
		if !strings.Contains(reference, want) {
			t.Errorf("shared reference does not document registry availability semantics %q", want)
		}
	}
}

func TestReadMarkdownSupportsDefinitionIDsFoldersAndAliases(t *testing.T) {
	for _, def := range Definitions() {
		want, err := ReadMarkdown(def.ID)
		if err != nil {
			t.Fatalf("ReadMarkdown(%q): %v", def.ID, err)
		}
		identifiers := append([]string{def.Folder}, def.Aliases...)
		for _, identifier := range identifiers {
			got, err := ReadMarkdown(identifier)
			if err != nil {
				t.Errorf("ReadMarkdown(%q): %v", identifier, err)
				continue
			}
			if got != want {
				t.Errorf("ReadMarkdown(%q) did not resolve to Definition %q", identifier, def.ID)
			}
		}
	}
}

func TestReadFileDefaultsToMarkdownAndLoadsOneBundledAsset(t *testing.T) {
	file, err := ReadFile("router", "")
	if err != nil {
		t.Fatal(err)
	}
	if file.Path != "SKILL.md" || !strings.HasPrefix(string(file.Data), "---\n") {
		t.Fatalf("ReadFile(router, empty path) = %#v", file)
	}

	file, err = ReadFile("router", "references/agent-execution-contract.md")
	if err != nil {
		t.Fatal(err)
	}
	if file.Path != "references/agent-execution-contract.md" || len(file.Data) == 0 {
		t.Fatalf("ReadFile(router, agent execution contract) = %#v", file)
	}
}

func TestReadFileRejectsUnsafePaths(t *testing.T) {
	tests := map[string]string{
		"missing":                  "missing.md",
		"parent traversal":         "../onesearch-exa/SKILL.md",
		"embedded traversal":       "references/../SKILL.md",
		"absolute":                 "/SKILL.md",
		"UNC forward slash":        "//server/share/SKILL.md",
		"UNC backslash":            `\\server\share\SKILL.md`,
		"drive absolute slash":     "C:/SKILL.md",
		"drive relative":           "C:SKILL.md",
		"drive absolute backslash": `C:\SKILL.md`,
		"backslash separator":      `agents\openai.yaml`,
	}
	for name, filePath := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := ReadFile("router", filePath); err == nil {
				t.Fatalf("ReadFile(router, %q) unexpectedly succeeded", filePath)
			}
		})
	}
}

func parseFrontmatter(t *testing.T, markdown string) map[string]string {
	t.Helper()
	if !strings.HasPrefix(markdown, "---\n") {
		t.Fatal("SKILL.md does not start with YAML frontmatter")
	}
	rest := strings.TrimPrefix(markdown, "---\n")
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		t.Fatal("SKILL.md frontmatter has no closing delimiter")
	}
	return parseFlatYAMLMapping(t, rest[:end], "")
}

func parseOpenAIInterface(t *testing.T, yaml string) map[string]string {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(yaml), "\n")
	if len(lines) == 0 || lines[0] != "interface:" {
		t.Fatalf("agents/openai.yaml must have interface as its only root key: %q", yaml)
	}
	return parseFlatYAMLMapping(t, strings.Join(lines[1:], "\n"), "  ")
}

func parseFlatYAMLMapping(t *testing.T, text, indent string) map[string]string {
	t.Helper()
	values := map[string]string{}
	for lineNumber, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.HasPrefix(line, indent) {
			t.Fatalf("line %d has unexpected indentation: %q", lineNumber+1, line)
		}
		line = strings.TrimPrefix(line, indent)
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			t.Fatalf("line %d contains an unexpected nested value: %q", lineNumber+1, line)
		}
		key, rawValue, ok := strings.Cut(line, ":")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			t.Fatalf("line %d is not a scalar mapping entry: %q", lineNumber+1, line)
		}
		if _, duplicate := values[key]; duplicate {
			t.Fatalf("line %d duplicates key %q", lineNumber+1, key)
		}
		values[key] = parseYAMLScalar(t, strings.TrimSpace(rawValue))
	}
	return values
}

func parseYAMLScalar(t *testing.T, raw string) string {
	t.Helper()
	if len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"' {
		value, err := strconv.Unquote(raw)
		if err != nil {
			t.Fatalf("invalid quoted YAML scalar %q: %v", raw, err)
		}
		return value
	}
	if len(raw) >= 2 && raw[0] == '\'' && raw[len(raw)-1] == '\'' {
		return strings.ReplaceAll(raw[1:len(raw)-1], "''", "'")
	}
	return raw
}

func assertExactKeys(t *testing.T, values map[string]string, keys ...string) {
	t.Helper()
	if len(values) != len(keys) {
		t.Fatalf("mapping keys = %v, want exactly %v", mapKeys(values), keys)
	}
	for _, key := range keys {
		if _, ok := values[key]; !ok {
			t.Fatalf("mapping keys = %v, missing %q", mapKeys(values), key)
		}
	}
}

func mapKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
