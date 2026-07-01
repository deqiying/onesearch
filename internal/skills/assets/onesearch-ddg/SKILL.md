---
name: onesearch-ddg
description: Use when an AI agent needs DuckDuckGo through Onesearch, especially when the user names ddg, DuckDuckGo, ddg-search, local MCP stdio search, or lightweight page content fetching through the Onesearch CLI.
---

# Onesearch DDG

Use DDG for local MCP stdio-backed DuckDuckGo source discovery and page content fetching. Prefer DDG direct commands when the task names DDG, DuckDuckGo, or `duckduckgo-mcp-server`.

## Bridge Contract

This provider is direct-only by default. It is not automatically added to normal `search`, `fetch`, or `crawl` workflow routes unless the user explicitly adds it to runtime `routes`.

If command details may have changed, run:

```powershell
onesearch skills show ddg --format content
```

Use the provider command family by default:

```powershell
onesearch status --format json
onesearch ddg search "query" --max-results 10 --format json
onesearch ddg fetch-content "https://example.com" --max-length 8000 --format json
```

Before using DDG direct commands, confirm `status.direct_endpoints.ddg.available == true`. If DDG is unavailable, choose another provider from the relevant `status.capabilities.<capability>.available` list or report the unavailable local provider.

## Commands

| Purpose | Command |
| --- | --- |
| Search | `onesearch ddg search "query"` |
| Fetch content | `onesearch ddg fetch-content "https://example.com"` |

## Options

- `search --max-results`: maximum search results.
- `search --region`: DuckDuckGo region, passed to the local MCP server.
- `fetch-content --start-index`: starting character index for content extraction.
- `fetch-content --max-length`: maximum extracted content length.
- `fetch-content --backend`: upstream fetch backend, such as `auto`.

## Guardrails

- Do not assume DDG is enabled just because the skill exists; `status.direct_endpoints.ddg.available` is the source of truth.
- DDG search results are discovery candidates. Fetch important URLs before citing claims.
- `status` only validates configuration shape and local command lookup; it does not start `uvx` or prove the upstream MCP package can fetch the web.
