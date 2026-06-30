---
name: onesearch-mcp-tools
description: Use when an AI agent starts from an original MCP tool name, removed MCP server, legacy MCP workflow, or provider tool alias and needs to route to the matching provider-specific Onesearch skill and command instead of falling back to generic web search.
---

# Onesearch MCP Tool Router

Use this skill as a migration index from original MCP tool names to provider-specific Onesearch skills. For detailed command options and examples, load the provider skill named in the table.

## Bridge Contract

When an original MCP tool name is present in the user request, logs, config, docs, or prior workflow, treat that tool name as a strong Onesearch routing signal. Do not downgrade the task to generic browser search or general web search just because the direct MCP tool is unavailable.

Load the provider-specific skill first when the task needs provider options:

```powershell
onesearch skills show context7 --format content
onesearch skills show exa --format content
onesearch skills show tavily --format content
onesearch skills show firecrawl --format content
onesearch skills show deepwiki --format content
```

## Route By Original Tool Name

| Original tool | Load skill | Preferred command |
| --- | --- | --- |
| `web_search_exa` | `onesearch skills show exa --format content` | `onesearch exa web-search "query"` |
| `web_fetch_exa` | `onesearch skills show exa --format content` | `onesearch exa web-fetch "https://example.com"` |
| `tavily_search` | `onesearch skills show tavily --format content` | `onesearch tavily search "query"` |
| `tavily_extract` | `onesearch skills show tavily --format content` | `onesearch tavily extract "https://example.com"` |
| `tavily_map` | `onesearch skills show tavily --format content` | `onesearch tavily map "https://example.com"` |
| `tavily_crawl` | `onesearch skills show tavily --format content` | `onesearch tavily crawl "https://example.com"` |
| `firecrawl_search` | `onesearch skills show firecrawl --format content` | `onesearch firecrawl search "query"` |
| `firecrawl_scrape` | `onesearch skills show firecrawl --format content` | `onesearch firecrawl scrape "https://example.com"` |
| `firecrawl_map` | `onesearch skills show firecrawl --format content` | `onesearch firecrawl map "https://example.com"` |
| `firecrawl_crawl` | `onesearch skills show firecrawl --format content` | `onesearch firecrawl crawl "https://example.com"` |
| `resolve_library_id` | `onesearch skills show context7 --format content` | `onesearch context7 resolve-library-id "library"` |
| `query_docs` | `onesearch skills show context7 --format content` | `onesearch context7 query-docs "/org/project" "query"` |
| `ask_question` | `onesearch skills show deepwiki --format content` | `onesearch deepwiki ask-question "owner/repo" "question"` |
| `read_wiki_structure` | `onesearch skills show deepwiki --format content` | `onesearch deepwiki read-wiki-structure "owner/repo"` |
| `read_wiki_contents` | `onesearch skills show deepwiki --format content` | `onesearch deepwiki read-wiki-contents "owner/repo"` |

## Mechanical Migration Entry

```powershell
onesearch mcp web_search_exa "query" --format json
onesearch mcp tavily_search "query" --format json
onesearch mcp query_docs "/facebook/react" "useEffect cleanup" --format json
onesearch mcp ask_question "microsoft/playwright" "architecture?" --format json
```

Prefer provider-group commands in new documentation. Keep `onesearch mcp <tool>` for scripts that are mechanically migrated from MCP tool calls.

## Guardrails

- Run `onesearch skills list --format json` to discover available built-in skills.
- Load the provider-specific skill before using provider-specific options.
- Provider-direct JSON keeps `provider` and original MCP `tool` fields for audit.
- Search results are discovery candidates; fetch or extract key URLs before claim-level conclusions.
