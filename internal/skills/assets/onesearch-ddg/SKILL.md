---
name: onesearch-ddg
description: Use when an AI agent needs DuckDuckGo through Onesearch, especially when the user names ddg, DuckDuckGo, ddg-search, local MCP stdio search, or lightweight page content fetching through the Onesearch CLI.
---

# Onesearch DDG

Use DDG for local MCP stdio-backed DuckDuckGo discovery and lightweight page content fetching. It is direct-only by default and does not automatically join workflow routes.

## Discover the Contracts

```text
onesearch schema ddg search --format json
onesearch schema ddg fetch-content --format json
```

Run `onesearch status --format json` and require `status.direct_endpoints.ddg.available == true` before a direct call. This local preflight does not start the MCP process or prove remote fetch behavior.

```text
onesearch ddg search "query" --max-results 10 --format json
onesearch ddg fetch-content "https://example.com" --max-length 8000 --format json
```

Search results are discovery candidates. Fetch important URLs before citing claims. Do not assume DDG is a normal `source_search` or `page_fetch` route unless runtime config explicitly adds it.

## Output and Recovery

Use compact JSON for candidates, previews, and provider/tool context. Use `--format content` for complete fetched text or `--verbose` for full structured data.

On `parameter_error`, inspect the failing DDG leaf schema. On `config_error`, run `doctor` and `status`. If the local MCP executable is missing or fails to start, report the prerequisite; do not install it automatically.
