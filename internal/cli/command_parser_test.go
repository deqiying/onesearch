package cli

import (
	"reflect"
	"strings"
	"testing"

	"github.com/deqiying/onesearch/internal/commandcontract"
	"github.com/deqiying/onesearch/internal/service"
)

func TestParseCommandRejectsUnknownFlagsAndExtraPositionals(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		args    []string
		wantErr string
	}{
		{name: "unknown flag", id: "search", args: []string{"golang", "--not-a-real-flag"}, wantErr: "unknown flag --not-a-real-flag"},
		{name: "extra positional", id: "fetch", args: []string{"https://example.com", "extra"}, wantErr: "accepts at most 1 positional argument(s)"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			definition := mustCLICommandDefinition(t, test.id)
			if _, err := parseCommand(definition, test.args); err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("parseCommand() error = %v, want substring %q", err, test.wantErr)
			}
		})
	}
}

func TestParseCommandEndOfOptionsTreatsFlagLikeTokenAsPositional(t *testing.T) {
	parsed, err := parseCommand(mustCLICommandDefinition(t, "search"), []string{"--", "--literal-query"})
	if err != nil {
		t.Fatal(err)
	}
	if got := parsed.String("query"); got != "--literal-query" {
		t.Fatalf("query = %q, want --literal-query", got)
	}
}

func TestEndOfOptionsPreventsHelpInterception(t *testing.T) {
	definition, namespace, requested := helpTarget([]string{"search", "--", "--help"})
	if requested || definition.ID != "" || namespace != nil {
		t.Fatalf("help after -- was intercepted: definition=%#v namespace=%#v", definition, namespace)
	}
	parsed, err := parseCommand(mustCLICommandDefinition(t, "search"), []string{"--", "--help"})
	if err != nil {
		t.Fatal(err)
	}
	if parsed.String("query") != "--help" {
		t.Fatalf("query = %q, want --help", parsed.String("query"))
	}
}

func TestParseCommandCombinesGreedyRepeatedAndCSVArrayValues(t *testing.T) {
	parsed, err := parseCommand(mustCLICommandDefinition(t, "exa.web-search"), []string{
		"golang",
		"--include-domains", "one.example,two.example", "three.example",
		"--include-domains", "four.example, five.example",
		"--exclude-domains=six.example,seven.example",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := parsed.Strings("include_domains"), []string{"one.example", "two.example", "three.example", "four.example", "five.example"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("include_domains = %#v, want %#v", got, want)
	}
	if got, want := parsed.Strings("exclude_domains"), []string{"six.example", "seven.example"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("exclude_domains = %#v, want %#v", got, want)
	}
}

func TestBuildArgvRoundTripsArrayValuesWithoutChangingTokenBoundaries(t *testing.T) {
	want := []string{"exact query with spaces", "a,b", "-leading", "--", "same", "same", `C:\docs`}
	argv, err := commandRegistry.BuildArgv("freecrawl.deep-research", map[string]any{"topic": "topic", "search_queries": want})
	if err != nil {
		t.Fatal(err)
	}
	invocation, ok := resolveInvocation(argv[1:])
	if !ok {
		t.Fatalf("argv did not resolve: %#v", argv)
	}
	parsed, err := parseCommand(invocation.Definition, argv[1+invocation.Consumed:])
	if err != nil {
		t.Fatal(err)
	}
	if got := parsed.Strings("search_queries"); !reflect.DeepEqual(got, want) {
		t.Fatalf("search_queries = %#v, want %#v; argv=%#v", got, want, argv)
	}
}

func TestTavilyMapKeepsPreviouslyAcceptedCompatibilityFlags(t *testing.T) {
	_, err := parseCommand(mustCLICommandDefinition(t, "tavily.map"), []string{
		"https://example.com", "--extract-depth", "advanced", "--extract-format", "text", "--include-images", "--include-favicon",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestParseCommandEnforcesAllMutuallyExclusiveGroups(t *testing.T) {
	tests := []struct {
		name string
		id   string
		args []string
	}{
		{name: "quiet and verbose", id: "search", args: []string{"golang", "--quiet", "--verbose"}},
		{name: "stream and no-stream", id: "search", args: []string{"golang", "--stream", "--no-stream"}},
		{name: "mock and live", id: "smoke", args: []string{"--mock", "--live"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := parseCommand(mustCLICommandDefinition(t, test.id), test.args); err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
				t.Fatalf("parseCommand() error = %v, want mutually exclusive error", err)
			}
		})
	}
}

func TestInlineFalseDoesNotActivatePresenceBoolean(t *testing.T) {
	parsed, err := parseCommand(mustCLICommandDefinition(t, "search"), []string{"query", "--stream=false", "--no-stream=false"})
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Bool("stream") || parsed.Bool("no_stream") || parsed.ValueIsSet("stream") || parsed.ValueIsSet("no_stream") {
		t.Fatalf("inline false activated presence booleans: %#v", parsed.Values)
	}
}

func TestPrettyBooleanParsingAndOutputCombinations(t *testing.T) {
	definition := mustCLICommandDefinition(t, "search")
	pretty, err := parseCommand(definition, []string{"query", "--quiet", "--pretty"})
	if err != nil {
		t.Fatal(err)
	}
	if !pretty.Bool("pretty") || !pretty.ValueIsSet("pretty") || !pretty.Bool("quiet") {
		t.Fatalf("pretty/quiet values = %#v", pretty.Values)
	}

	compact, err := parseCommand(definition, []string{"query", "--format", "content", "--pretty=false"})
	if err != nil {
		t.Fatal(err)
	}
	if compact.Bool("pretty") || compact.ValueIsSet("pretty") || compact.String("format") != "content" {
		t.Fatalf("compact content values = %#v", compact.Values)
	}

	verbose, err := parseCommand(definition, []string{"query", "--verbose", "--pretty=true"})
	if err != nil {
		t.Fatal(err)
	}
	if !verbose.Bool("pretty") || !verbose.Bool("verbose") {
		t.Fatalf("pretty/verbose values = %#v", verbose.Values)
	}
}

func TestDeepPlanCommandArgvAreAcceptedByCLIParser(t *testing.T) {
	plan := service.New(nil).DeepPlan(`PowerShell's $HOME; "quoted" & <tag> --literal`, "deep", t.TempDir())
	groups := [][]map[string]any{
		plan["preflight"].([]map[string]any),
		plan["steps"].([]map[string]any),
	}
	for _, group := range groups {
		for _, item := range group {
			argv, ok := item["command_argv"].([]string)
			if !ok || len(argv) < 2 || argv[0] != "onesearch" {
				t.Fatalf("invalid command_argv: %#v", item["command_argv"])
			}
			invocation, ok := resolveInvocation(argv[1:])
			if !ok {
				t.Fatalf("registry did not resolve argv: %#v", argv)
			}
			if _, err := parseCommand(invocation.Definition, argv[1+invocation.Consumed:]); err != nil {
				t.Fatalf("parse-only rejected argv %#v: %v", argv, err)
			}
		}
	}
}

func TestEveryPublicCommandHasAParseableMinimumArgv(t *testing.T) {
	for _, definition := range commandRegistry.Commands() {
		values := map[string]any{}
		for _, positional := range definition.Positionals {
			if !positional.Required {
				continue
			}
			if positional.Variadic {
				values[positional.Name] = []string{"value"}
			} else {
				values[positional.Name] = "value"
			}
		}
		for _, constraint := range definition.Constraints {
			if constraint.Kind != "at_least_one" {
				continue
			}
			member := constraint.Members[0]
			for _, positional := range definition.Positionals {
				if positional.Name == member && positional.Variadic {
					values[member] = []string{"value"}
				}
			}
		}
		argv, err := commandRegistry.BuildArgv(definition.ID, values)
		if err != nil {
			t.Fatalf("BuildArgv(%s) failed: %v", definition.ID, err)
		}
		invocation, ok := resolveInvocation(argv[1:])
		if !ok || invocation.Definition.ID != definition.ID {
			t.Fatalf("minimum argv did not resolve %s: %#v", definition.ID, argv)
		}
		if _, err := parseCommand(invocation.Definition, argv[1+invocation.Consumed:]); err != nil {
			t.Fatalf("minimum argv for %s did not parse: %#v: %v", definition.ID, argv, err)
		}
	}
}

func mustCLICommandDefinition(t *testing.T, id string) commandcontract.CommandDefinition {
	t.Helper()
	definition, ok := commandRegistry.LookupID(id)
	if !ok {
		t.Fatalf("command %q not found", id)
	}
	return definition
}
