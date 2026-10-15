package execution

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/deqiying/onesearch/internal/commandcontract"
	"github.com/deqiying/onesearch/internal/providers"
	"github.com/deqiying/onesearch/internal/service"
)

var _ Backend = (*service.Service)(nil)

type fakeBackend struct {
	calls       int
	id          string
	args        []string
	options     any
	ctx         context.Context
	deadline    time.Time
	hasDeadline bool
	wait        bool
}

func (f *fakeBackend) record(ctx context.Context, id string, options any, args ...string) map[string]any {
	f.calls++
	f.ctx, f.id, f.options, f.args = ctx, id, options, args
	f.deadline, f.hasDeadline = ctx.Deadline()
	if f.wait {
		<-ctx.Done()
	}
	return map[string]any{"ok": true, "marker": "backend"}
}
func (f *fakeBackend) Search(ctx context.Context, q string, o service.SearchOptions) map[string]any {
	return f.record(ctx, "search", o, q)
}
func (f *fakeBackend) Fetch(ctx context.Context, u string, o service.FetchOptions) map[string]any {
	return f.record(ctx, "fetch", o, u)
}
func (f *fakeBackend) Map(ctx context.Context, u string, o service.MapOptions) map[string]any {
	return f.record(ctx, "map", o, u)
}
func (f *fakeBackend) Crawl(ctx context.Context, u string, o service.CrawlOptions) map[string]any {
	return f.record(ctx, "crawl", o, u)
}
func (f *fakeBackend) RepoWiki(ctx context.Context, r, q string, o service.RepoWikiOptions) map[string]any {
	return f.record(ctx, "repo-wiki", o, r, q)
}
func (f *fakeBackend) ExaSearch(ctx context.Context, q string, o providers.ExaOptions) map[string]any {
	return f.record(ctx, "exa.web-search", o, q)
}
func (f *fakeBackend) Context7Library(ctx context.Context, n, q string) map[string]any {
	return f.record(ctx, "context7.resolve-library-id", nil, n, q)
}
func (f *fakeBackend) Context7Docs(ctx context.Context, l, q string) map[string]any {
	return f.record(ctx, "context7.query-docs", nil, l, q)
}

func TestExecuteMappings(t *testing.T) {
	cases := []struct {
		id      string
		values  map[string]any
		args    []string
		options any
		timeout time.Duration
	}{
		{"search", map[string]any{"query": "query", "platform": "web", "model": "model", "extra_sources": 2, "fetch_sources": 3.0, "validation": "strict", "fallback": "off", "providers": "exa,ddg", "repo_wiki": "owner/repo", "repo_wiki_mode": "ask", "repo_wiki_query": "repo question", "timeout": 1.25}, []string{"query"}, service.SearchOptions{Platform: "web", Model: "model", ExtraSources: 2, FetchSources: 3, Validation: "strict", Fallback: "off", Providers: "exa,ddg", RepoWiki: "owner/repo", RepoWikiMode: "ask", RepoWikiQuery: "repo question"}, 1250 * time.Millisecond},
		{"fetch", map[string]any{"url": "https://example.com", "provider": "exa"}, []string{"https://example.com"}, service.FetchOptions{Provider: "exa"}, 0},
		{"map", map[string]any{"url": "https://example.com", "instructions": "docs", "max_depth": 3.0, "max_breadth": 4, "limit": 5, "timeout": 6, "provider": "tavily"}, []string{"https://example.com"}, service.MapOptions{Instructions: "docs", MaxDepth: 3, MaxBreadth: 4, Limit: 5, Timeout: 6, Provider: "tavily"}, 0},
		{"crawl", map[string]any{"url": "https://example.com", "max_depth": 3, "limit": 4.0, "timeout": 5.0, "provider": "firecrawl"}, []string{"https://example.com"}, service.CrawlOptions{MaxDepth: 3, Limit: 4, Timeout: 5, Provider: "firecrawl"}, 5 * time.Second},
		{"repo-wiki", map[string]any{"repo": "owner/repo", "question": "why?", "mode": "ask", "provider": "deepwiki", "timeout": 1.5}, []string{"owner/repo", "why?"}, service.RepoWikiOptions{Mode: "ask", Provider: "deepwiki"}, 1500 * time.Millisecond},
		{"exa.web-search", map[string]any{"query": "query", "num_results": 7, "max_results": 9.0, "search_type": "deep", "include_text": false, "include_highlights": true, "start_published_date": "2026-01-01", "include_domains": []string{"example.com"}, "exclude_domains": []string{"other.com"}, "category": "news"}, []string{"query"}, providers.ExaOptions{NumResults: 9, SearchType: "deep", IncludeText: false, IncludeHighlights: true, StartPublishedDate: "2026-01-01", IncludeDomains: []string{"example.com"}, ExcludeDomains: []string{"other.com"}, Category: "news"}, 0},
		{"context7.resolve-library-id", map[string]any{"name": "react", "query": "hooks"}, []string{"react", "hooks"}, nil, 0},
		{"context7.query-docs", map[string]any{"library_id": "/org/lib", "query": "hooks"}, []string{"/org/lib", "hooks"}, nil, 0},
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			f := &fakeBackend{}
			ctx := context.WithValue(context.Background(), struct{}{}, "preserved")
			before := time.Now()
			got := Execute(ctx, f, tc.id, tc.values)
			after := time.Now()
			if got["marker"] != "backend" || f.calls != 1 || f.id != tc.id || !reflect.DeepEqual(f.args, tc.args) || !reflect.DeepEqual(f.options, tc.options) {
				t.Fatalf("result=%v call=%+v; want args=%v options=%+v", got, f, tc.args, tc.options)
			}
			if f.ctx.Value(struct{}{}) != "preserved" {
				t.Fatal("lost context value")
			}
			if tc.timeout == 0 {
				if f.ctx != ctx || f.hasDeadline {
					t.Fatal("unexpected context wrapper")
				}
			} else if !f.hasDeadline || f.deadline.Before(before.Add(tc.timeout)) || f.deadline.After(after.Add(tc.timeout)) {
				t.Fatalf("deadline %v outside expected range", f.deadline)
			}
		})
	}
}

func TestSearchStream(t *testing.T) {
	for _, tc := range []struct {
		name             string
		stream, noStream bool
		want             *bool
	}{
		{"defaults", false, false, nil}, {"enabled", true, false, boolPointer(true)}, {"disabled", false, true, boolPointer(false)}, {"disable-wins", true, true, boolPointer(false)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeBackend{}
			Execute(context.Background(), f, "search", map[string]any{"query": "q", "timeout": 90, "stream": tc.stream, "no_stream": tc.noStream})
			if got := f.options.(service.SearchOptions).Stream; !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("stream=%v want %v", got, tc.want)
			}
		})
	}
}
func boolPointer(value bool) *bool { return &value }

func TestSearchFilters(t *testing.T) {
	f := &fakeBackend{}
	Execute(context.Background(), f, "search", map[string]any{
		"query": "q", "timeout": 90.0,
		"providers":        " ANSWER = old ; sources: exa ; documentation=old-docs ; page:old-fetch ; repository_wiki=old-repo ; map=tavily ; crawl=firecrawl ; vertical=anysearch ; main_search=legacy-answer ; web_search=legacy-source ; web_fetch=legacy-fetch ; custom=custom-provider ; ;",
		"answer_providers": " new-answer ", "source_providers": " new-source ", "docs_providers": " new-docs ", "fetch_providers": " new-fetch ", "repo_providers": " new-repo ",
	})
	o := f.options.(service.SearchOptions)
	want := map[string]string{"answer_search": "new-answer", "source_search": "new-source", "docs_search": "new-docs", "page_fetch": "new-fetch", "repo_wiki": "new-repo", "site_map": "tavily", "site_crawl": "firecrawl", "vertical_search": "anysearch", "custom": "custom-provider"}
	if o.Providers != "auto" || !reflect.DeepEqual(o.ProviderFilters, want) {
		t.Fatalf("options=%+v want filters=%v", o, want)
	}
	for _, raw := range []string{"", " AuTo ", "exa,ddg", "answer=exa"} {
		f := &fakeBackend{}
		Execute(context.Background(), f, "search", map[string]any{"timeout": 90.0, "providers": raw, "answer_providers": "   "})
		o := f.options.(service.SearchOptions)
		if raw == "answer=exa" {
			if o.ProviderFilters["answer_search"] != "exa" {
				t.Fatal("blank override removed filter")
			}
		} else if o.ProviderFilters != nil {
			t.Fatalf("unexpected filters: %v", o.ProviderFilters)
		}
	}
	for _, tc := range []struct{ raw, message string }{
		{"answer=exa;broken", `provider filter "broken" must use capability=providers`},
		{"answer=", `provider filter "answer=" must include capability and provider list`},
		{"=exa", `provider filter "=exa" must include capability and provider list`},
	} {
		f := &fakeBackend{}
		got := Execute(context.Background(), f, "search", map[string]any{"providers": tc.raw})
		if !reflect.DeepEqual(got, map[string]any{"ok": false, "error_type": "parameter_error", "error": tc.message}) || f.calls != 0 {
			t.Fatalf("got %v calls=%d", got, f.calls)
		}
	}
}

func TestRepoModesAndOptionalQueries(t *testing.T) {
	for _, mode := range []string{"", "ask", "structure", "contents"} {
		f := &fakeBackend{}
		Execute(context.Background(), f, "repo-wiki", map[string]any{"repo": "owner/repo", "mode": mode, "provider": "auto", "timeout": 60})
		if !reflect.DeepEqual(f.args, []string{"owner/repo", ""}) || f.options.(service.RepoWikiOptions).Mode != mode {
			t.Fatalf("mode %q: %+v", mode, f)
		}
	}
	f := &fakeBackend{}
	Execute(context.Background(), f, "context7.resolve-library-id", map[string]any{"name": "react"})
	if !reflect.DeepEqual(f.args, []string{"react", ""}) {
		t.Fatalf("args=%v", f.args)
	}
}

func TestDefaultedValues(t *testing.T) {
	for _, d := range commandcontract.MustDefaultRegistry().Commands() {
		var want any
		switch d.ID {
		case "search":
			want = service.SearchOptions{Providers: "auto"}
		case "fetch":
			want = service.FetchOptions{Provider: "auto"}
		case "map":
			want = service.MapOptions{MaxDepth: 1, MaxBreadth: 20, Limit: 50, Timeout: 150, Provider: "auto"}
		case "crawl":
			want = service.CrawlOptions{MaxDepth: 2, Limit: 20, Timeout: 180, Provider: "auto"}
		case "repo-wiki":
			want = service.RepoWikiOptions{Provider: "auto"}
		case "exa.web-search":
			want = providers.ExaOptions{NumResults: 5, SearchType: "auto", IncludeDomains: []string{}, ExcludeDomains: []string{}}
		default:
			continue
		}
		t.Run(d.ID, func(t *testing.T) {
			values := map[string]any{}
			for _, o := range d.Options {
				if o.HasDefault {
					values[o.Name] = o.Default
				}
			}
			f := &fakeBackend{}
			Execute(context.Background(), f, d.ID, values)
			if !reflect.DeepEqual(f.options, want) {
				t.Fatalf("got %+v want %+v", f.options, want)
			}
		})
	}
}

func TestExaLegacyOverrideAndListIsolation(t *testing.T) {
	for _, max := range []int{0, -1, 11} {
		values := map[string]any{"query": "q", "num_results": 5, "max_results": max, "include_text": true, "include_highlights": false, "include_domains": []string{"example.com"}}
		f := &fakeBackend{}
		Execute(context.Background(), f, "exa.web-search", values)
		o := f.options.(providers.ExaOptions)
		want := 5
		if max > 0 {
			want = max
		}
		if o.NumResults != want || !o.IncludeText || o.IncludeHighlights {
			t.Fatalf("options=%+v", o)
		}
		o.IncludeDomains[0] = "changed"
		if values["include_domains"].([]string)[0] != "example.com" {
			t.Fatal("mutated input list")
		}
	}
}

func TestDeadlineResults(t *testing.T) {
	for _, tc := range []struct {
		id      string
		timeout any
		want    map[string]any
	}{
		{"search", 0.001, map[string]any{"ok": false, "error_type": "network_error", "error": "Search timed out after 0.001 seconds", "query": "q", "content": "", "sources": []map[string]any{}, "sources_count": 0, "timeout_seconds": 0.001}},
		{"crawl", 0, map[string]any{"ok": false, "error_type": "network_error", "error": "Crawl timed out after 0 seconds", "url": "https://example.com", "timeout_seconds": 0}},
		{"repo-wiki", 0.001, map[string]any{"ok": false, "error_type": "network_error", "error": "Repo wiki timed out after 0.001 seconds", "repo": "owner/repo", "timeout_seconds": 0.001}},
	} {
		t.Run(tc.id, func(t *testing.T) {
			values := map[string]any{"timeout": tc.timeout, "query": "q", "url": "https://example.com", "repo": "owner/repo"}
			f := &fakeBackend{wait: true}
			got := Execute(context.Background(), f, tc.id, values)
			if f.calls != 1 || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v want %v calls=%d", got, tc.want, f.calls)
			}
			ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			defer cancel()
			f = &fakeBackend{}
			got = Execute(ctx, f, tc.id, values)
			if f.calls != 0 || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("expired parent: got %v calls=%d", got, f.calls)
			}
		})
	}
}

func TestCanceledContextAndUnknownID(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, id := range []string{"search", "fetch", "map", "crawl", "repo-wiki", "exa.web-search", "context7.resolve-library-id", "context7.query-docs"} {
		f := &fakeBackend{}
		got := Execute(ctx, f, id, nil)
		if f.calls != 0 || !reflect.DeepEqual(got, map[string]any{"ok": false, "error_type": "network_error", "error": "context canceled"}) {
			t.Fatalf("%s: got %v calls=%d", id, got, f.calls)
		}
	}
	for _, ctx := range []context.Context{context.Background(), ctx} {
		got := Execute(ctx, nil, "unknown", nil)
		if !reflect.DeepEqual(got, map[string]any{"ok": false, "error_type": "parameter_error", "error": "unsupported command binding: unknown"}) {
			t.Fatalf("got %v", got)
		}
	}
}

func TestParentDeadlinePreserved(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	deadline, _ := ctx.Deadline()
	for _, id := range []string{"search", "fetch", "map", "crawl", "repo-wiki", "exa.web-search", "context7.resolve-library-id", "context7.query-docs"} {
		f := &fakeBackend{}
		Execute(ctx, f, id, map[string]any{"timeout": 180})
		if !f.hasDeadline || f.deadline != deadline {
			t.Fatalf("%s changed parent deadline: %v want %v", id, f.deadline, deadline)
		}
	}
}
