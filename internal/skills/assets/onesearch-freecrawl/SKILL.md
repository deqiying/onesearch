---
name: onesearch-freecrawl
description: Use when an AI agent needs Freecrawl through Onesearch, especially when the user names freecrawl, freecrawl-mcp, local MCP stdio scraping, crawl, or deep research through the Onesearch CLI.
---

# Onesearch Freecrawl

Use Freecrawl for local MCP stdio-backed search, scrape, bounded crawl, or provider-specific deep research. It is direct-only by default. It supports crawl, not the workflow `site_map` capability.

## Discover the Contracts

```text
onesearch schema freecrawl search --format json
onesearch schema freecrawl scrape --format json
onesearch schema freecrawl crawl --format json
onesearch schema freecrawl deep-research --format json
```

Run `onesearch status --format json` and require `status.direct_endpoints.freecrawl.available == true` before a direct call. This local preflight does not start the MCP process, install browser assets, or prove network behavior.

```text
onesearch freecrawl search "query" --num-results 5 --format json
onesearch freecrawl scrape "https://example.com" --formats markdown --format json
onesearch freecrawl crawl "https://docs.example.com" --max-depth 2 --max-pages 20 --format json
onesearch freecrawl deep-research "topic" --num-sources 8 --max-depth 3 --format json
```

The bundled example pins Freecrawl to `pypi:freecrawl-mcp==0.1.2` when configured. `--wait-for` accepts a CSS selector or a millisecond string; tool availability is determined by runtime discovery and missing tools return `capability_unavailable`.

Keep crawl and deep-research limits small and restrict crawling to the relevant domain. Treat search output as discovery and scrape key pages before citing claims.

Freecrawl may require Playwright browser binaries. If startup reports missing browser/runtime assets, report the exact prerequisite and ask the user to authorize installation; do not run an installer or `--install-browsers` automatically.

## Output and Recovery

Use compact JSON for candidates, crawl state, previews, and provider/tool context. Use `--format content` for complete scraped text or `--verbose` for full structured output.

On `parameter_error`, inspect the failing Freecrawl leaf schema. On `config_error`, run `doctor` and `status`. If local dependencies are missing, stop at the prerequisite boundary rather than silently switching or installing software.
