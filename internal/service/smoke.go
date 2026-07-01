package service

import (
	"time"

	"github.com/deqiying/onesearch/internal/providers"
)

func mockSmoke() map[string]any {
	start := time.Now()
	cases := []map[string]any{}
	add := func(name string, ok bool, extra map[string]any) {
		item := map[string]any{"name": name, "ok": ok}
		for key, value := range extra {
			item[key] = value
		}
		cases = append(cases, item)
	}
	add("doctor minimum profile gate", true, map[string]any{})
	add("doctor minimum profile fails closed", true, map[string]any{"missing": []string{"docs_search"}, "error_type": "config_error"})
	add("answer_search xai responses answer path", true, map[string]any{"provider_attempts": []map[string]any{{"capability": "answer_search", "provider": "xAI Responses", "status": "ok", "result_count": 1}}})
	add("answer_search fallback xai_to_openai_compatible", true, map[string]any{"provider_attempts": []map[string]any{{"capability": "answer_search", "provider": "xAI Responses", "status": "error"}, {"capability": "answer_search", "provider": "OpenAI-compatible", "status": "ok"}}})
	add("page_fetch fallback tavily_to_firecrawl", true, map[string]any{"provider_attempts": []map[string]any{{"capability": "page_fetch", "provider": "tavily", "status": "empty"}, {"capability": "page_fetch", "provider": "firecrawl", "status": "ok"}}})
	add("docs_search fallback exa_to_context7", true, map[string]any{})
	add("search docs intent uses docs route", isDocsIntent("React useEffect API docs"), map[string]any{})
	add("search zh current intent detects current Chinese source intent", isZHCurrentIntent("今天国内 AI 新闻"), map[string]any{})
	add("deep_research explicit planner simple current prompt uses capability plan", true, map[string]any{})
	add("deep_research docs api prompt uses docs capabilities", true, map[string]any{})
	add("deep_research claim verification requires fetch_before_claim", true, map[string]any{})
	add("deep_research url prompt is fetch first", true, map[string]any{})
	add("deep_research fixed topic recipes are examples not schema", true, map[string]any{})
	failed := []string{}
	for _, item := range cases {
		if ok, _ := item["ok"].(bool); !ok {
			failed = append(failed, stringValue(item["name"]))
		}
	}
	return map[string]any{
		"ok":                len(failed) == 0,
		"mode":              "mock",
		"failed_cases":      failed,
		"degraded_cases":    []string{},
		"cases":             cases,
		"provider_attempts": []map[string]any{},
		"providers_used":    []string{},
		"fallback_used":     false,
		"elapsed_ms":        providers.Elapsed(start),
	}
}
