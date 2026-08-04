---
name: onesearch-fetch
description: Use when an AI agent needs Onesearch to fetch URL content, inspect evidence pages, map a site, or verify claims from source pages.
---

# Onesearch Fetch

Use this workflow when the user supplies a URL, when a search candidate must become evidence, or when a site must be mapped or crawled within explicit bounds.

## Discover the Contracts

```text
onesearch schema fetch --format json
onesearch schema map --format json
onesearch schema crawl --format json
```

## Choose the Operation

- `fetch`: read one page as claim-level evidence.
- `map`: discover site URLs before selecting pages.
- `crawl`: collect a bounded section of a site.

Run `onesearch status --format json` before selecting `--provider`. Match `fetch`, `map`, and `crawl` to `page_fetch`, `site_map`, and `site_crawl` availability respectively. DDG and Freecrawl are direct-only unless runtime config explicitly includes them in a workflow route.

```text
onesearch fetch "https://example.com/page" --format json
onesearch map "https://example.com" --provider firecrawl --format json
onesearch crawl "https://example.com/docs" --provider tavily --max-depth 2 --limit 20 --format json
```

Fetch only the pages needed to support the answer. Keep discovery candidates separate from fetched evidence, and keep maps/crawls bounded to the relevant domain and scope.

## Output and Recovery

Use compact JSON for URLs, provider metadata, previews, and crawl state. Use `--format content` for the complete primary page text or `--verbose` for full structured pages.

On `parameter_error`, inspect only the failing leaf schema. On `config_error`, run `doctor` then `status`. If a provider is unavailable, choose another provider listed for the same capability; do not silently change from fetch to search.
