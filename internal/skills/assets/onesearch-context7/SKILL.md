---
name: onesearch-context7
description: Use when an AI agent needs Context7 through Onesearch for resolve_library_id, query_docs, library resolution, API docs, SDK docs, framework docs, or package documentation lookup.
---

# Onesearch Context7

Use Context7 for library, SDK, API, package, and framework documentation. Resolve the library first, then query focused docs.

## Commands

| Purpose | Preferred command | MCP-compatible alias |
| --- | --- | --- |
| Resolve library | `onesearch context7 resolve-library-id "react"` | `onesearch context7 resolve_library_id "react"` |
| Query docs | `onesearch context7 query-docs "/facebook/react" "query"` | `onesearch context7 query_docs "/facebook/react" "query"` |
| Legacy resolve | `onesearch context7-library "react" "hooks"` | Legacy flat command |
| Legacy docs | `onesearch context7-docs "/facebook/react" "query"` | Legacy flat command |

Global MCP migration aliases:

```powershell
onesearch mcp resolve_library_id "react" --format json
onesearch mcp query_docs "/facebook/react" "useEffect cleanup" --format json
```

## Usage

```powershell
onesearch context7 resolve-library-id "react" --format json
onesearch context7 resolve-library-id "react" "hooks" --format json
onesearch context7 query-docs "/facebook/react" "useEffect cleanup" --format json
onesearch context7 query-docs "/vercel/next.js" "app router metadata" --format json
```

## Output

Resolve output includes library candidates with IDs, descriptions, trust scores, snippets, and stars when available.

Docs output includes `code_snippets`, `info_snippets`, `results`, `content`, `provider: "context7"`, and `tool: "query_docs"` for provider-direct commands.

## Guardrails

- Use Context7 only for documentation intent.
- Keep exact API claims tied to returned docs snippets or fetched official docs.
- Use Exa for official docs pages when Context7 coverage is missing or stale.
