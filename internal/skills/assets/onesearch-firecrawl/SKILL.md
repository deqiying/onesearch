---
name: onesearch-firecrawl
description: Use when an AI agent needs Firecrawl through Onesearch for firecrawl_search, firecrawl_scrape, firecrawl_map, firecrawl_crawl, web search, page scraping, site mapping, or crawl job submission.
---

# Onesearch Firecrawl

Use Firecrawl for web search, robust page scraping, site mapping, and crawl job submission. Prefer Firecrawl direct commands when the task names a Firecrawl MCP tool or needs Firecrawl-specific output.

## Commands

| Purpose | Preferred command | MCP-compatible alias |
| --- | --- | --- |
| Search | `onesearch firecrawl search "query"` | `onesearch firecrawl firecrawl_search "query"` |
| Scrape page | `onesearch firecrawl scrape "https://example.com"` | `onesearch firecrawl firecrawl_scrape "https://example.com"` |
| Site map | `onesearch firecrawl map "https://example.com"` | `onesearch firecrawl firecrawl_map "https://example.com"` |
| Crawl job | `onesearch firecrawl crawl "https://example.com"` | `onesearch firecrawl firecrawl_crawl "https://example.com"` |

Global MCP migration aliases:

```powershell
onesearch mcp firecrawl_search "query" --format json
onesearch mcp firecrawl_scrape "https://example.com" --format json
onesearch mcp firecrawl_map "https://example.com" --format json
onesearch mcp firecrawl_crawl "https://example.com" --format json
```

## Usage

```powershell
onesearch firecrawl search "OpenAI API docs" --limit 5 --format json
onesearch firecrawl scrape "https://example.com/article" --format json
onesearch firecrawl scrape "https://example.com/article" --format content
onesearch firecrawl map "https://docs.example.com" --limit 50 --format json
onesearch firecrawl crawl "https://docs.example.com" --max-depth 2 --limit 20 --format json
```

## Options

- `search --limit`: maximum search results.
- `scrape --attempts`: retry attempts for markdown extraction.
- `map --limit`: maximum discovered links.
- `crawl --max-depth`: crawl discovery depth.
- `crawl --limit`: maximum crawl pages submitted.
- `crawl --timeout`: CLI timeout around the submission request.

## Output

Provider-direct JSON includes `provider: "firecrawl"` and `tool` set to the original MCP tool name.

`firecrawl_crawl` submits an async crawl job. Read top-level `id`, `status`, and `url`; do not expect page content in the initial response.

Use `--format content` for scraped markdown body only.

## Guardrails

- Use `fetch` or `firecrawl scrape` for claim-level page evidence.
- Keep crawl limits small unless the user explicitly asks for broad crawling.
- Run `onesearch doctor --format json` when Firecrawl returns `config_error`.
