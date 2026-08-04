---
name: onesearch-firecrawl
description: Use when an AI agent needs Firecrawl through Onesearch, especially when the user names Firecrawl, robust web search, page scraping, markdown extraction, site mapping, or crawl job submission through the Onesearch CLI.
---

# Onesearch Firecrawl

Use Firecrawl direct commands when the user names Firecrawl or needs robust scrape, map, or crawl-job behavior. Prefer workflow commands when provider-specific output is not required.

## Discover the Contracts

```text
onesearch schema firecrawl search --format json
onesearch schema firecrawl scrape --format json
onesearch schema firecrawl map --format json
onesearch schema firecrawl crawl --format json
```

Run `onesearch status --format json` and require `status.direct_endpoints.firecrawl.available == true` before a direct call.

```text
onesearch firecrawl search "official API docs" --limit 5 --format json
onesearch firecrawl scrape "https://example.com/article" --format json
onesearch firecrawl map "https://docs.example.com" --limit 50 --format json
```

Map before a crawl when site structure is uncertain. Keep crawl depth and page limits bounded and same-domain unless the task explicitly requires broader collection. `firecrawl crawl` submits an asynchronous job; the initial result contains job identity/status rather than completed page content.

## Output and Recovery

Use compact JSON for job state, URLs, previews, and provider metadata. Use `--format content` for complete scraped text or `--verbose` for full structured data.

On `parameter_error`, inspect the failing Firecrawl leaf schema. On `config_error`, run `doctor` and `status`. On an async crawl, preserve the returned job context instead of treating submission as completed evidence.
