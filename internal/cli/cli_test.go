package cli

import (
	"reflect"
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
