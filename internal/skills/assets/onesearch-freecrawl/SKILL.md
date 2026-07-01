---
name: onesearch-freecrawl
description: Use when an AI agent needs Freecrawl through Onesearch, especially when the user names freecrawl, freecrawl-mcp, local MCP stdio scraping, crawl, or deep research through the Onesearch CLI.
---

# Onesearch Freecrawl

Use Freecrawl for local MCP stdio-backed search, page scraping, site crawling, and provider-specific deep research. Prefer Freecrawl direct commands when the task names Freecrawl or requires its local MCP tool surface.

## Bridge Contract

This provider is direct-only by default. It is not automatically added to normal `search`, `fetch`, or `crawl` workflow routes unless the user explicitly adds it to runtime `routes`.

If command details may have changed, run:

```powershell
onesearch skills show freecrawl --format content
```

Use the provider command family by default:

```powershell
onesearch status --format json
onesearch freecrawl search "query" --num-results 5 --format json
onesearch freecrawl scrape "https://example.com" --formats markdown --format json
onesearch freecrawl crawl "https://docs.example.com" --max-depth 2 --max-pages 20 --format json
onesearch freecrawl deep-research "topic" --num-sources 8 --max-depth 3 --format json
```

Before using Freecrawl direct commands, confirm `status.direct_endpoints.freecrawl.available == true`. If Freecrawl is unavailable, choose another provider from the relevant `status.capabilities.<capability>.available` list or report the unavailable local provider.

## Commands

| Purpose | Command |
| --- | --- |
| Search | `onesearch freecrawl search "query"` |
| Scrape page | `onesearch freecrawl scrape "https://example.com"` |
| Crawl site | `onesearch freecrawl crawl "https://example.com"` |
| Deep research | `onesearch freecrawl deep-research "topic"` |

## Options

- `search --num-results`: maximum search results.
- `search --search-engine`: upstream search engine.
- `search --scrape-results`: ask Freecrawl to scrape discovered results when supported.
- `scrape --formats`: comma-separated output formats such as `markdown,text`.
- `scrape --javascript`, `--anti-bot`, `--cache`: provider-specific scrape toggles.
- `scrape --timeout`: scrape timeout in milliseconds.
- `scrape --wait-for`: page wait time in milliseconds.
- `crawl --max-depth`: crawl depth.
- `crawl --max-pages`: maximum pages.
- `crawl --same-domain-only`: restrict crawl to the start URL domain.
- `crawl --include-patterns`, `--exclude-patterns`: crawl URL filters.
- `deep-research --num-sources`: maximum sources.
- `deep-research --max-depth`: research depth.
- `deep-research --include-academic`: include academic sources when supported.
- `deep-research --search-queries`: explicit supporting queries.

## Guardrails

- Do not assume Freecrawl is enabled just because the skill exists; `status.direct_endpoints.freecrawl.available` is the source of truth.
- Freecrawl may need Playwright browsers installed first; if startup fails with browser or Playwright errors, run the upstream install command for the configured package, such as `uvx freecrawl-mcp --install-browsers`.
- Keep crawl and deep research limits small unless the user explicitly asks for broad collection.
- `status` only validates configuration shape and local command lookup; it does not start `uvx`, install browser dependencies, or prove the upstream MCP package can fetch the web.
