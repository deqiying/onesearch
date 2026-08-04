package commandcontract

import (
	"reflect"
	"testing"
)

func testArgvRegistry(t *testing.T) *Registry {
	t.Helper()
	command := CommandDefinition{
		ID: "demo", Path: []string{"demo"}, Category: CategoryUtility, Visibility: VisibilityPublic,
		Summary: "demo", Positionals: []PositionalDefinition{variadicPositional("queries", "queries", 1)},
		Options: []OptionDefinition{
			{Name: "mode", Flag: "mode", Type: TypeString, Enum: []string{"fast", "deep"}, HasDefault: true, Default: "fast"},
			{Name: "enabled", Flag: "enabled", Type: TypeBoolean, HasDefault: true, Default: false},
			{Name: "labels", Flag: "label", Type: TypeStringArray, Repeatable: true},
			{Name: "quiet", Flag: "quiet", Type: TypeBoolean},
			{Name: "verbose", Flag: "verbose", Type: TypeBoolean},
		},
		Constraints: []ConstraintDefinition{{Kind: "mutually_exclusive", Members: []string{"quiet", "verbose"}}},
		Output:      OutputDefinition{DefaultFormat: "json"},
	}
	registry, err := NewRegistry([]CommandDefinition{command}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func TestBuildArgvRepeatsArraysAndOnlyEmitsTrueBooleans(t *testing.T) {
	registry := testArgvRegistry(t)
	got, err := registry.BuildArgv("demo", map[string]any{
		"mode":    "deep",
		"enabled": false,
		"labels":  []string{"a", "b"},
		"queries": []string{"first", "-second"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"onesearch", "demo", "--mode", "deep", "--label", "a", "--label", "b", "--", "first", "-second"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("argv = %#v, want %#v", got, want)
	}
}

func TestBuildArgvPlacesNormalPositionalsBeforeFlags(t *testing.T) {
	registry := testArgvRegistry(t)
	got, err := registry.BuildArgv("demo", map[string]any{"mode": "deep", "queries": []string{"query"}})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"onesearch", "demo", "query", "--mode", "deep"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("argv = %#v, want %#v", got, want)
	}
}

func TestBuildArgvValidatesRequiredEnumAndMutualExclusion(t *testing.T) {
	registry := testArgvRegistry(t)
	if _, err := registry.BuildArgv("demo", map[string]any{"mode": "fast"}); err == nil {
		t.Fatal("missing variadic positional should fail")
	}
	if _, err := registry.BuildArgv("demo", map[string]any{"mode": "invalid", "queries": []string{"q"}}); err == nil {
		t.Fatal("invalid enum should fail")
	}
	if _, err := registry.BuildArgv("demo", map[string]any{"quiet": true, "verbose": true, "queries": []string{"q"}}); err == nil {
		t.Fatal("mutually exclusive flags should fail")
	}
	if _, err := registry.BuildArgv("demo", map[string]any{"unknown": "value", "queries": []string{"q"}}); err == nil {
		t.Fatal("unknown value should fail")
	}
}

func TestBuildArgvAcceptsFlagNamesAndDoesNotMutateInput(t *testing.T) {
	command := CommandDefinition{
		ID: "override", Path: []string{"override"}, Category: CategoryUtility, Visibility: VisibilityPublic,
		Summary: "override", Options: []OptionDefinition{
			{Name: "old", Flag: "old", Type: TypeString},
			{Name: "new", Flag: "new", Type: TypeString, Overrides: "old"},
		}, Output: OutputDefinition{DefaultFormat: "json"},
	}
	registry, err := NewRegistry([]CommandDefinition{command}, nil)
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]any{"old": "old-value", "new": "new-value"}
	got, err := registry.BuildArgv("override", values)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"onesearch", "override", "--new", "new-value"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("argv = %#v, want %#v", got, want)
	}
	if values["old"] != "old-value" {
		t.Fatalf("input values were mutated: %#v", values)
	}
}

func TestBuildArgvRejectsEmptyArrayItems(t *testing.T) {
	registry := testArgvRegistry(t)
	_, err := registry.BuildArgv("demo", map[string]any{"queries": []string{"query"}, "labels": []string{""}})
	if err == nil {
		t.Fatal("empty array item should fail because the parser drops empty list values")
	}
}

func TestBuildArgvAppliesConditionalCompatibilityOverride(t *testing.T) {
	registry := MustDefaultRegistry()
	zero, err := registry.BuildArgv("exa.web-search", map[string]any{"query": "query", "num_results": 9, "max_results": 0})
	if err != nil {
		t.Fatal(err)
	}
	if !containsTokenPair(zero, "--num-results", "9") || !containsTokenPair(zero, "--max-results", "0") {
		t.Fatalf("zero max_results unexpectedly overrode num_results: %#v", zero)
	}
	positive, err := registry.BuildArgv("exa.web-search", map[string]any{"query": "query", "num_results": 9, "max_results": 3})
	if err != nil {
		t.Fatal(err)
	}
	if containsTokenPair(positive, "--num-results", "9") || !containsTokenPair(positive, "--max-results", "3") {
		t.Fatalf("positive max_results did not override num_results: %#v", positive)
	}
}

func containsTokenPair(argv []string, flag, value string) bool {
	for index := 0; index+1 < len(argv); index++ {
		if argv[index] == flag && argv[index+1] == value {
			return true
		}
	}
	return false
}
