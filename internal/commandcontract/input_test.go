package commandcontract

import (
	"math"
	"testing"
)

func TestNormalizeStructuredInput(t *testing.T) {
	r := MustDefaultRegistry()
	d, _ := r.LookupID("exa.web-search")
	input, err := NormalizeInput(d, map[string]any{"query": "q", "num_results": float64(3), "include_text": false, "include_domains": []any{"example.com"}})
	if err != nil {
		t.Fatal(err)
	}
	if input.Values["num_results"] != 3 || input.Values["include_text"] != false || !input.Present["include_text"] || input.Present["search_type"] {
		t.Fatalf("normalized: %#v", input)
	}
	if input.Values["search_type"] == nil {
		t.Fatal("missing default")
	}
	if values, ok := input.Values["include_domains"].([]string); !ok || len(values) != 1 {
		t.Fatal("array not normalized")
	}
	for name, raw := range map[string]map[string]any{
		"missing": {"num_results": 1}, "unknown": {"query": "q", "shell": "echo"}, "null": {"query": nil},
		"string-number": {"query": "q", "num_results": "3"}, "fraction": {"query": "q", "num_results": 1.5},
		"nan": {"query": "q", "num_results": math.NaN()}, "infinity": {"query": "q", "num_results": math.Inf(1)},
		"overflow": {"query": "q", "num_results": float64(math.MaxInt)},
		"array":    {"query": "q", "include_domains": []any{1}}, "enum": {"query": "q", "search_type": "invalid"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := NormalizeInput(d, raw); err == nil {
				t.Fatal("invalid structured input accepted")
			}
		})
	}
	s, _ := r.LookupID("search")
	if _, err := NormalizeInput(s, map[string]any{"query": "q", "stream": true, "no_stream": true}); err == nil {
		t.Fatal("conflict accepted")
	}
}
