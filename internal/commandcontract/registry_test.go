package commandcontract

import (
	"reflect"
	"testing"
)

func TestDefaultRegistryIndexesCanonicalAliasesAndCapabilities(t *testing.T) {
	registry, err := DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if got := len(registry.Commands()); got != 44 {
		t.Fatalf("public command count = %d, want 44 (43 existing routes plus schema)", got)
	}
	if got := len(registry.Namespaces()); got != 12 {
		t.Fatalf("namespace count = %d, want 12", got)
	}
	if command, ok := registry.LookupID("exa.similar"); !ok || pathKey(command.Path) != "exa\x00similar" {
		t.Fatalf("LookupID(exa.similar) = %#v, %v", command, ok)
	}
	if command, ok := registry.Lookup("s"); !ok || command.ID != "search" {
		t.Fatalf("Lookup(s) = %#v, %v", command, ok)
	}
	if got := len(registry.CommandsForProvider("exa")); got != 3 {
		t.Fatalf("exa command count = %d, want 3", got)
	}
	if command, ok := registry.PreferredFor("vertical_search"); !ok || command.ID != "anysearch.search" {
		t.Fatalf("PreferredFor(vertical_search) = %#v, %v", command, ok)
	}
}

func TestRegistryRejectsConflictingDefinitions(t *testing.T) {
	base := CommandDefinition{
		ID: "demo", Path: []string{"demo"}, Category: CategoryUtility, Visibility: VisibilityPublic,
		Summary: "demo", Output: OutputDefinition{DefaultFormat: "json"},
	}
	if _, err := NewRegistry([]CommandDefinition{base, base}, nil); err == nil {
		t.Fatal("duplicate command ID should fail")
	}
	base.ID = "other"
	if _, err := NewRegistry([]CommandDefinition{{ID: "", Path: []string{"demo"}, Category: CategoryUtility, Visibility: VisibilityPublic, Summary: "demo", Output: OutputDefinition{DefaultFormat: "json"}}}, nil); err == nil {
		t.Fatal("empty command ID should fail")
	}
}

func TestRegistryReturnsCopies(t *testing.T) {
	command := CommandDefinition{ID: "demo", Path: []string{"demo"}, Category: CategoryUtility, Visibility: VisibilityPublic, Summary: "demo", Output: OutputDefinition{DefaultFormat: "json"}}
	registry, err := NewRegistry([]CommandDefinition{command}, nil)
	if err != nil {
		t.Fatal(err)
	}
	commands := registry.Commands()
	commands[0].Path[0] = "changed"
	commands[0].Output.Formats = append(commands[0].Output.Formats, "changed")
	got, ok := registry.LookupID("demo")
	if !ok || got.Path[0] != "demo" || len(got.Output.Formats) != 0 {
		t.Fatalf("registry was mutated through returned copy: %#v", got)
	}
}

func TestRuntimeCommandsDeclareFirstRunConfigInitialization(t *testing.T) {
	registry := MustDefaultRegistry()
	for _, command := range registry.Commands() {
		if command.ID == "schema" {
			if contains(command.SideEffects, "config_initialize_when_missing") {
				t.Fatal("schema must remain independent from runtime config initialization")
			}
			continue
		}
		if !contains(command.SideEffects, "config_initialize_when_missing") {
			t.Fatalf("command %q does not declare first-run config initialization", command.ID)
		}
	}
}

func TestDefaultRegistryPreferredCommandsCoverRuntimeCapabilities(t *testing.T) {
	registry := MustDefaultRegistry()
	want := map[string]string{
		"answer_search": "search", "source_search": "search", "docs_search": "search", "page_fetch": "fetch",
		"site_map": "map", "site_crawl": "crawl", "repo_wiki": "repo-wiki", "vertical_search": "anysearch.search",
	}
	for capability, id := range want {
		definition, ok := registry.PreferredFor(capability)
		if !ok || definition.ID != id {
			t.Fatalf("PreferredFor(%s) = %#v, %v; want %s", capability, definition, ok, id)
		}
	}
}

func TestConfigSetupManifestKeepsSensitiveInputOffArgv(t *testing.T) {
	definition, ok := MustDefaultRegistry().LookupID("config.setup")
	if !ok {
		t.Fatal("config.setup is missing")
	}
	manifest := definition.Manifest()
	if len(manifest.InputChannels) != 1 {
		t.Fatalf("input_channels = %#v", manifest.InputChannels)
	}
	channel := manifest.InputChannels[0]
	if !channel.Sensitive || channel.ForbiddenBinding != "argv" || channel.RequiredWhenRuntime == "" {
		t.Fatalf("sensitive channel = %#v", channel)
	}
	properties := manifest.InputSchema["properties"].(map[string]any)
	if _, exists := properties["api_key"]; exists {
		t.Fatal("api_key must not have an argv property")
	}
	if _, exists := properties["api_key_stdin"]; !exists {
		t.Fatal("api_key_stdin activator is missing")
	}
}

func TestArraySchemaPublishesCanonicalListEncoding(t *testing.T) {
	definition, _ := MustDefaultRegistry().LookupID("freecrawl.deep-research")
	properties := definition.Manifest().InputSchema["properties"].(map[string]any)
	property := properties["search_queries"].(map[string]any)
	binding := property["x-cli-binding"].(map[string]any)
	if binding["list_encoding"] != ListEncoding {
		t.Fatalf("list encoding = %#v", binding["list_encoding"])
	}
	if got := property["items"].(map[string]any)["minLength"]; !reflect.DeepEqual(got, 1) {
		t.Fatalf("array item minLength = %#v, want 1", got)
	}
}
