package providers

import (
	"context"
	"strings"
	"time"
)

type Tavily struct {
	APIURL  string
	APIKey  string
	Timeout time.Duration
}

type TavilySearchOptions struct {
	MaxResults        int
	SearchDepth       string
	Topic             string
	TimeRange         string
	StartDate         string
	EndDate           string
	IncludeDomains    []string
	ExcludeDomains    []string
	Country           string
	IncludeRawContent bool
	IncludeImages     bool
	IncludeFavicon    bool
}

type TavilyExtractOptions struct {
	Format         string
	ExtractDepth   string
	Query          string
	IncludeImages  bool
	IncludeFavicon bool
	TimeoutSeconds float64
}

type TavilyMapOptions struct {
	Instructions   string
	MaxDepth       int
	MaxBreadth     int
	Limit          int
	TimeoutSeconds int
	AllowExternal  bool
	SelectDomains  []string
	SelectPaths    []string
	ExcludeDomains []string
	ExcludePaths   []string
}

type TavilyCrawlOptions struct {
	Instructions   string
	MaxDepth       int
	MaxBreadth     int
	Limit          int
	TimeoutSeconds int
	AllowExternal  bool
	SelectDomains  []string
	SelectPaths    []string
	ExcludeDomains []string
	ExcludePaths   []string
	ExtractDepth   string
	Format         string
	IncludeImages  bool
	IncludeFavicon bool
}

func (p Tavily) Search(ctx context.Context, query string, maxResults int) ([]map[string]any, error) {
	data, err := p.search(ctx, query, TavilySearchOptions{MaxResults: maxResults, SearchDepth: "advanced"})
	if err != nil {
		return nil, err
	}
	return normalizeTavilySearchResults(asMaps(data["results"])), nil
}

func (p Tavily) SearchResult(ctx context.Context, query string, options TavilySearchOptions) map[string]any {
	start := time.Now()
	data, err := p.search(ctx, query, options)
	if err != nil {
		errorType, message := ErrorPayload(err)
		return map[string]any{"ok": false, "provider": "tavily", "tool": "tavily_search", "query": query, "error_type": errorType, "error": message, "elapsed_ms": Elapsed(start)}
	}
	results := normalizeTavilySearchResults(asMaps(data["results"]))
	out := map[string]any{
		"ok":         true,
		"provider":   "tavily",
		"tool":       "tavily_search",
		"query":      query,
		"results":    results,
		"total":      len(results),
		"elapsed_ms": Elapsed(start),
	}
	for _, key := range []string{"answer", "images", "response_time", "auto_parameters", "usage", "request_id"} {
		if data[key] != nil {
			out[key] = data[key]
		}
	}
	return out
}

func (p Tavily) search(ctx context.Context, query string, options TavilySearchOptions) (map[string]any, error) {
	if options.MaxResults <= 0 {
		options.MaxResults = 6
	}
	if options.SearchDepth == "" {
		options.SearchDepth = "advanced"
	}
	payload := map[string]any{
		"query":          query,
		"max_results":    options.MaxResults,
		"search_depth":   options.SearchDepth,
		"include_answer": false,
	}
	if options.Topic != "" {
		payload["topic"] = options.Topic
	}
	if options.TimeRange != "" {
		payload["time_range"] = options.TimeRange
	}
	if options.StartDate != "" {
		payload["start_date"] = options.StartDate
	}
	if options.EndDate != "" {
		payload["end_date"] = options.EndDate
	}
	if len(options.IncludeDomains) > 0 {
		payload["include_domains"] = options.IncludeDomains
	}
	if len(options.ExcludeDomains) > 0 {
		payload["exclude_domains"] = options.ExcludeDomains
	}
	if options.Country != "" {
		payload["country"] = options.Country
	}
	if options.IncludeRawContent {
		payload["include_raw_content"] = true
	}
	if options.IncludeImages {
		payload["include_images"] = true
	}
	if options.IncludeFavicon {
		payload["include_favicon"] = true
	}
	var data map[string]any
	err := PostJSON(ctx, Client(p.Timeout), strings.TrimRight(p.APIURL, "/")+"/search", map[string]string{"Authorization": "Bearer " + p.APIKey}, payload, &data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (p Tavily) Extract(ctx context.Context, targetURL string) (string, error) {
	data, err := p.extract(ctx, []string{targetURL}, TavilyExtractOptions{Format: "markdown"})
	if err != nil {
		return "", err
	}
	results := normalizeTavilyExtractResults(asMaps(data["results"]))
	if len(results) == 0 {
		return "", nil
	}
	return strings.TrimSpace(stringValue(results[0]["content"])), nil
}

func (p Tavily) ExtractResult(ctx context.Context, urls []string, options TavilyExtractOptions) map[string]any {
	start := time.Now()
	data, err := p.extract(ctx, urls, options)
	if err != nil {
		errorType, message := ErrorPayload(err)
		return map[string]any{"ok": false, "provider": "tavily", "tool": "tavily_extract", "urls": nonEmptyStrings(urls), "error_type": errorType, "error": message, "elapsed_ms": Elapsed(start)}
	}
	results := normalizeTavilyExtractResults(asMaps(data["results"]))
	content := joinedContentByKey(results, "content")
	out := map[string]any{
		"ok":             len(results) > 0,
		"provider":       "tavily",
		"tool":           "tavily_extract",
		"urls":           nonEmptyStrings(urls),
		"results":        results,
		"total":          len(results),
		"failed_results": data["failed_results"],
		"content":        content,
		"elapsed_ms":     Elapsed(start),
	}
	if len(urls) == 1 {
		out["url"] = strings.TrimSpace(urls[0])
	}
	if len(results) == 0 {
		out["error_type"] = "empty_result"
		out["error"] = "Tavily extract returned no results"
	}
	for _, key := range []string{"response_time", "usage", "request_id"} {
		if data[key] != nil {
			out[key] = data[key]
		}
	}
	return out
}

func (p Tavily) extract(ctx context.Context, urls []string, options TavilyExtractOptions) (map[string]any, error) {
	urls = nonEmptyStrings(urls)
	if len(urls) == 0 {
		return nil, &ProviderError{Type: "parameter_error", Message: "tavily_extract requires at least one url"}
	}
	if options.Format == "" {
		options.Format = "markdown"
	}
	payload := map[string]any{"urls": urls, "format": options.Format}
	if options.ExtractDepth != "" {
		payload["extract_depth"] = options.ExtractDepth
	}
	if options.Query != "" {
		payload["query"] = options.Query
	}
	if options.IncludeImages {
		payload["include_images"] = true
	}
	if options.IncludeFavicon {
		payload["include_favicon"] = true
	}
	if options.TimeoutSeconds > 0 {
		payload["timeout"] = options.TimeoutSeconds
	}
	var data map[string]any
	err := PostJSON(ctx, Client(p.Timeout), strings.TrimRight(p.APIURL, "/")+"/extract", map[string]string{"Authorization": "Bearer " + p.APIKey}, payload, &data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (p Tavily) Map(ctx context.Context, targetURL, instructions string, maxDepth, maxBreadth, limit, timeoutSeconds int) map[string]any {
	return p.MapWithOptions(ctx, targetURL, TavilyMapOptions{Instructions: instructions, MaxDepth: maxDepth, MaxBreadth: maxBreadth, Limit: limit, TimeoutSeconds: timeoutSeconds})
}

func (p Tavily) MapWithOptions(ctx context.Context, targetURL string, options TavilyMapOptions) map[string]any {
	start := time.Now()
	if options.MaxDepth <= 0 {
		options.MaxDepth = 1
	}
	if options.MaxBreadth <= 0 {
		options.MaxBreadth = 20
	}
	if options.Limit <= 0 {
		options.Limit = 50
	}
	if options.TimeoutSeconds <= 0 {
		options.TimeoutSeconds = 150
	}
	payload := map[string]any{
		"url":            targetURL,
		"max_depth":      options.MaxDepth,
		"max_breadth":    options.MaxBreadth,
		"limit":          options.Limit,
		"timeout":        options.TimeoutSeconds,
		"allow_external": options.AllowExternal,
	}
	if options.Instructions != "" {
		payload["instructions"] = options.Instructions
	}
	if len(options.SelectDomains) > 0 {
		payload["select_domains"] = options.SelectDomains
	}
	if len(options.SelectPaths) > 0 {
		payload["select_paths"] = options.SelectPaths
	}
	if len(options.ExcludeDomains) > 0 {
		payload["exclude_domains"] = options.ExcludeDomains
	}
	if len(options.ExcludePaths) > 0 {
		payload["exclude_paths"] = options.ExcludePaths
	}
	var data map[string]any
	err := PostJSON(ctx, Client(time.Duration(options.TimeoutSeconds+10)*time.Second), strings.TrimRight(p.APIURL, "/")+"/map", map[string]string{"Authorization": "Bearer " + p.APIKey}, payload, &data)
	if err != nil {
		errorType, message := ErrorPayload(err)
		return map[string]any{"ok": false, "provider": "tavily", "tool": "tavily_map", "url": targetURL, "error_type": errorType, "error": message, "elapsed_ms": Elapsed(start)}
	}
	return map[string]any{"ok": true, "provider": "tavily", "tool": "tavily_map", "url": targetURL, "base_url": data["base_url"], "results": data["results"], "response_time": data["response_time"], "usage": data["usage"], "request_id": data["request_id"], "elapsed_ms": Elapsed(start)}
}

func (p Tavily) Crawl(ctx context.Context, targetURL string, options TavilyCrawlOptions) map[string]any {
	start := time.Now()
	if options.MaxDepth <= 0 {
		options.MaxDepth = 2
	}
	if options.MaxBreadth <= 0 {
		options.MaxBreadth = 20
	}
	if options.Limit <= 0 {
		options.Limit = 20
	}
	if options.TimeoutSeconds <= 0 {
		options.TimeoutSeconds = 150
	}
	if options.Format == "" {
		options.Format = "markdown"
	}
	payload := map[string]any{
		"url":            targetURL,
		"max_depth":      options.MaxDepth,
		"max_breadth":    options.MaxBreadth,
		"limit":          options.Limit,
		"timeout":        options.TimeoutSeconds,
		"allow_external": options.AllowExternal,
		"format":         options.Format,
	}
	if options.Instructions != "" {
		payload["instructions"] = options.Instructions
	}
	if len(options.SelectDomains) > 0 {
		payload["select_domains"] = options.SelectDomains
	}
	if len(options.SelectPaths) > 0 {
		payload["select_paths"] = options.SelectPaths
	}
	if len(options.ExcludeDomains) > 0 {
		payload["exclude_domains"] = options.ExcludeDomains
	}
	if len(options.ExcludePaths) > 0 {
		payload["exclude_paths"] = options.ExcludePaths
	}
	if options.ExtractDepth != "" {
		payload["extract_depth"] = options.ExtractDepth
	}
	if options.IncludeImages {
		payload["include_images"] = true
	}
	if options.IncludeFavicon {
		payload["include_favicon"] = true
	}
	var data map[string]any
	err := PostJSON(ctx, Client(time.Duration(options.TimeoutSeconds+10)*time.Second), strings.TrimRight(p.APIURL, "/")+"/crawl", map[string]string{"Authorization": "Bearer " + p.APIKey}, payload, &data)
	if err != nil {
		errorType, message := ErrorPayload(err)
		return map[string]any{"ok": false, "provider": "tavily", "tool": "tavily_crawl", "url": targetURL, "error_type": errorType, "error": message, "elapsed_ms": Elapsed(start)}
	}
	results := normalizeTavilyExtractResults(asMaps(data["results"]))
	out := map[string]any{
		"ok":            true,
		"provider":      "tavily",
		"tool":          "tavily_crawl",
		"url":           targetURL,
		"base_url":      data["base_url"],
		"results":       results,
		"total":         len(results),
		"content":       joinedContentByKey(results, "content"),
		"response_time": data["response_time"],
		"usage":         data["usage"],
		"request_id":    data["request_id"],
		"elapsed_ms":    Elapsed(start),
	}
	return out
}

type Firecrawl struct {
	APIURL string
	APIKey string
}

func (p Firecrawl) Search(ctx context.Context, query string, limit int) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 14
	}
	payload := map[string]any{"query": query, "limit": limit}
	var data map[string]any
	err := PostJSON(ctx, Client(90*time.Second), strings.TrimRight(p.APIURL, "/")+"/search", map[string]string{"Authorization": "Bearer " + p.APIKey}, payload, &data)
	if err != nil {
		return nil, err
	}
	var web []map[string]any
	if nested, ok := data["data"].(map[string]any); ok {
		web = asMaps(nested["web"])
	}
	results := make([]map[string]any, 0, len(web))
	for _, item := range web {
		results = append(results, map[string]any{
			"title":       stringValue(item["title"]),
			"url":         stringValue(item["url"]),
			"description": stringValue(item["description"]),
		})
	}
	return results, nil
}

func (p Firecrawl) SearchResult(ctx context.Context, query string, limit int) map[string]any {
	start := time.Now()
	results, err := p.Search(ctx, query, limit)
	if err != nil {
		errorType, message := ErrorPayload(err)
		return map[string]any{"ok": false, "provider": "firecrawl", "tool": "firecrawl_search", "query": query, "error_type": errorType, "error": message, "elapsed_ms": Elapsed(start)}
	}
	return map[string]any{"ok": len(results) > 0, "provider": "firecrawl", "tool": "firecrawl_search", "query": query, "results": results, "total": len(results), "elapsed_ms": Elapsed(start)}
}

func (p Firecrawl) Map(ctx context.Context, targetURL string, limit int) map[string]any {
	start := time.Now()
	if limit <= 0 {
		limit = 50
	}
	payload := map[string]any{"url": targetURL, "limit": limit}
	var data map[string]any
	err := PostJSON(ctx, Client(90*time.Second), strings.TrimRight(p.APIURL, "/")+"/map", map[string]string{"Authorization": "Bearer " + p.APIKey}, payload, &data)
	if err != nil {
		errorType, message := ErrorPayload(err)
		return map[string]any{"ok": false, "url": targetURL, "provider": "firecrawl", "error_type": errorType, "error": message, "elapsed_ms": Elapsed(start)}
	}
	results := data["links"]
	if results == nil {
		results = data["data"]
	}
	return map[string]any{"ok": true, "url": targetURL, "provider": "firecrawl", "results": results, "elapsed_ms": Elapsed(start)}
}

func (p Firecrawl) Crawl(ctx context.Context, targetURL string, maxDepth, limit int) map[string]any {
	start := time.Now()
	if maxDepth <= 0 {
		maxDepth = 2
	}
	if limit <= 0 {
		limit = 20
	}
	payload := map[string]any{
		"url":               targetURL,
		"maxDiscoveryDepth": maxDepth,
		"limit":             limit,
		"scrapeOptions":     map[string]any{"formats": []string{"markdown"}, "onlyMainContent": true},
	}
	var data map[string]any
	err := PostJSON(ctx, Client(120*time.Second), strings.TrimRight(p.APIURL, "/")+"/crawl", map[string]string{"Authorization": "Bearer " + p.APIKey}, payload, &data)
	if err != nil {
		errorType, message := ErrorPayload(err)
		return map[string]any{"ok": false, "url": targetURL, "provider": "firecrawl", "error_type": errorType, "error": message, "elapsed_ms": Elapsed(start)}
	}
	ok := true
	if success, hasSuccess := data["success"].(bool); hasSuccess {
		ok = success
	}
	out := map[string]any{
		"ok":         ok,
		"url":        firstNonEmpty(stringValue(data["url"]), targetURL),
		"provider":   "firecrawl",
		"tool":       "firecrawl_crawl",
		"result":     data,
		"elapsed_ms": Elapsed(start),
	}
	for _, key := range []string{"success", "id"} {
		if data[key] != nil {
			out[key] = data[key]
		}
	}
	if status := stringValue(data["status"]); status != "" {
		out["status"] = status
	} else if stringValue(data["id"]) != "" {
		out["status"] = "submitted"
	}
	if !ok {
		out["error_type"] = "provider_error"
		out["error"] = firstNonEmpty(stringValue(data["error"]), "Firecrawl crawl returned success=false")
	}
	return out
}

func (p Firecrawl) Scrape(ctx context.Context, targetURL string, attempts int) (string, error) {
	if attempts <= 0 {
		attempts = 1
	}
	var lastErr error
	for i := 0; i < attempts; i++ {
		payload := map[string]any{
			"url":     targetURL,
			"formats": []string{"markdown"},
			"timeout": 60000,
			"waitFor": (i + 1) * 1500,
		}
		var data map[string]any
		err := PostJSON(ctx, Client(90*time.Second), strings.TrimRight(p.APIURL, "/")+"/scrape", map[string]string{"Authorization": "Bearer " + p.APIKey}, payload, &data)
		if err != nil {
			lastErr = err
			continue
		}
		if nested, ok := data["data"].(map[string]any); ok {
			if markdown := strings.TrimSpace(stringValue(nested["markdown"])); markdown != "" {
				return markdown, nil
			}
		}
	}
	return "", lastErr
}

func (p Firecrawl) ScrapeResult(ctx context.Context, targetURL string, attempts int) map[string]any {
	start := time.Now()
	content, err := p.Scrape(ctx, targetURL, attempts)
	if err != nil {
		errorType, message := ErrorPayload(err)
		return map[string]any{"ok": false, "provider": "firecrawl", "tool": "firecrawl_scrape", "url": targetURL, "error_type": errorType, "error": message, "elapsed_ms": Elapsed(start)}
	}
	ok := strings.TrimSpace(content) != ""
	out := map[string]any{"ok": ok, "provider": "firecrawl", "tool": "firecrawl_scrape", "url": targetURL, "content": content, "elapsed_ms": Elapsed(start)}
	if !ok {
		out["error_type"] = "empty_result"
		out["error"] = "Firecrawl scrape returned no markdown content"
	}
	return out
}

func normalizeTavilySearchResults(items []map[string]any) []map[string]any {
	results := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result := map[string]any{
			"title":   stringValue(item["title"]),
			"url":     stringValue(item["url"]),
			"content": stringValue(item["content"]),
			"score":   item["score"],
		}
		for _, key := range []string{"raw_content", "favicon", "images"} {
			if value := item[key]; stringValue(value) != "" {
				result[key] = value
			}
		}
		results = append(results, result)
	}
	return results
}

func normalizeTavilyExtractResults(items []map[string]any) []map[string]any {
	results := make([]map[string]any, 0, len(items))
	for _, item := range items {
		content := strings.TrimSpace(firstNonEmpty(stringValue(item["raw_content"]), stringValue(item["content"])))
		result := map[string]any{
			"url":     stringValue(item["url"]),
			"content": content,
		}
		for _, key := range []string{"favicon", "images"} {
			if value := item[key]; stringValue(value) != "" {
				result[key] = value
			}
		}
		results = append(results, result)
	}
	return results
}

func joinedContentByKey(items []map[string]any, key string) string {
	var parts []string
	for _, item := range items {
		if text := strings.TrimSpace(stringValue(item[key])); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}
