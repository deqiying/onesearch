package mcpserver

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func catalogForTest(t *testing.T) map[string]toolDefinition {
	t.Helper()
	catalog, err := buildCatalog()
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]toolDefinition{}
	for _, d := range catalog {
		out[d.tool.Name] = d
	}
	if len(out) != 5 {
		t.Fatalf("catalog count = %d", len(out))
	}
	return out
}

func TestCatalogInputsAndBindings(t *testing.T) {
	catalog := catalogForTest(t)
	for _, tc := range []struct {
		name, tool, args, id, operation string
		valid                           bool
	}{
		{"answer", "search", `{"mode":"answer","query":"q","answer_options":{"fetch_sources":0}}`, "search", "answer", true},
		{"sources", "search", `{"mode":"sources","query":"q","sources_options":{"include_text":false,"num_results":20,"include_domains":["example.com"]}}`, "exa.web-search", "sources", true},
		{"resolve", "search", `{"mode":"docs","query":"q","docs_options":{"library_name":"go"}}`, "context7.resolve-library-id", "docs.resolve", true},
		{"docs", "search", `{"mode":"docs","query":"q","docs_options":{"library_id":"/org/project/v1"}}`, "context7.query-docs", "docs.query", true},
		{"fetch", "fetch", `{"url":"https://example.com"}`, "fetch", "fetch", true},
		{"map", "site", `{"action":"map","url":"https://example.com","map_options":{"max_breadth":50}}`, "map", "map", true},
		{"crawl", "site", `{"action":"crawl","url":"https://example.com","limit":50}`, "crawl", "crawl", true},
		{"ask", "repo_wiki", `{"repo":"org/repo","mode":"ask","question":"q"}`, "repo-wiki", "ask", true},
		{"structure", "repo_wiki", `{"repo":"org/repo","mode":"structure"}`, "repo-wiki", "structure", true},
		{"contents", "repo_wiki", `{"repo":"org/repo","mode":"contents"}`, "repo-wiki", "contents", true},
		{"status", "status", `{}`, "", "status", true},
		{"implicit-mode", "search", `{"query":"q"}`, "", "", false},
		{"unknown-mode", "search", `{"mode":"all","query":"q"}`, "", "", false},
		{"mixed-options", "search", `{"mode":"answer","query":"q","sources_options":{}}`, "", "", false},
		{"both-docs-selectors", "search", `{"mode":"docs","query":"q","docs_options":{"library_name":"go","library_id":"/go/go"}}`, "", "", false},
		{"no-docs-selector", "search", `{"mode":"docs","query":"q","docs_options":{}}`, "", "", false},
		{"bad-library-id", "search", `{"mode":"docs","query":"q","docs_options":{"library_id":"guess"}}`, "", "", false},
		{"unknown-root", "search", `{"mode":"answer","query":"q","output":"file"}`, "", "", false},
		{"unknown-nested", "search", `{"mode":"answer","query":"q","answer_options":{"stream":true}}`, "", "", false},
		{"null-options", "search", `{"mode":"sources","query":"q","sources_options":null}`, "", "", false},
		{"wrong-bool", "search", `{"mode":"sources","query":"q","sources_options":{"include_text":"false"}}`, "", "", false},
		{"fractional-count", "search", `{"mode":"sources","query":"q","sources_options":{"num_results":1.5}}`, "", "", false},
		{"too-many", "search", `{"mode":"sources","query":"q","sources_options":{"num_results":21}}`, "", "", false},
		{"crawl-map-options", "site", `{"action":"crawl","url":"https://example.com","map_options":{}}`, "", "", false},
		{"crawl-limit", "site", `{"action":"crawl","url":"https://example.com","limit":51}`, "", "", false},
		{"empty-query", "search", `{"mode":"answer","query":""}`, "", "", false},
		{"timeout-zero", "fetch", `{"url":"https://example.com","timeout":0}`, "", "", false},
		{"timeout-max", "fetch", `{"url":"https://example.com","timeout":301}`, "", "", false},
		{"ask-no-question", "repo_wiki", `{"repo":"org/repo","mode":"ask"}`, "", "", false},
		{"structure-question", "repo_wiki", `{"repo":"org/repo","mode":"structure","question":"q"}`, "", "", false},
		{"status-options", "status", `{"verbose":true}`, "", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var args map[string]any
			if err := json.Unmarshal([]byte(tc.args), &args); err != nil {
				t.Fatal(err)
			}
			tool := "onesearch_" + tc.tool
			err := catalog[tool].input.Validate(args)
			if (err == nil) != tc.valid {
				t.Fatalf("validate = %v, valid %v", err, tc.valid)
			}
			if !tc.valid {
				return
			}
			in, err := bind(tool, args)
			if err != nil {
				t.Fatal(err)
			}
			if in.id != tc.id || in.operation != tc.operation {
				t.Fatalf("binding = %#v", in)
			}
		})
	}
	for tool, expected := range map[string]map[string]any{
		"map":   {"max_depth": 1, "limit": 50, "max_breadth": 20, "timeout": 150},
		"crawl": {"max_depth": 2, "limit": 20, "timeout": 180},
	} {
		in, err := bind("onesearch_site", map[string]any{"action": tool, "url": "https://example.com"})
		if err != nil {
			t.Fatal(err)
		}
		for k, v := range expected {
			if !reflect.DeepEqual(in.input.Values[k], v) {
				t.Errorf("%s %s = %v, want %v", tool, k, in.input.Values[k], v)
			}
		}
	}
	in, err := bind("onesearch_fetch", map[string]any{"url": "https://example.com", "timeout": 0.25})
	if err != nil || in.timeout != 250*time.Millisecond {
		t.Fatalf("timeout binding: %#v %v", in, err)
	}
	for _, url := range []string{"file:///tmp/x", "https://user:password@example.com", "https://", "  "} {
		if _, err := bind("onesearch_fetch", map[string]any{"url": url}); err == nil {
			t.Errorf("accepted URL %q", url)
		}
	}
}

func TestCatalogStaticProjection(t *testing.T) {
	catalog := catalogForTest(t)
	var tools []any
	for _, d := range catalog {
		tools = append(tools, d.tool)
	}
	body, err := json.Marshal(tools)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("five-tool catalog JSON: %d bytes", len(body))
	for _, forbidden := range []string{"x-cli-", "evidence_dir", "no_stream", "shell", "api_key", "onesearch_exa"} {
		if strings.Contains(string(body), forbidden) {
			t.Errorf("catalog exposes %s", forbidden)
		}
	}
	before := registry.Commands()
	_, err = buildCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, registry.Commands()) {
		t.Fatal("catalog mutated registry")
	}
}
