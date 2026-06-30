---
name: onesearch-context7
description: Use when an AI agent needs Context7 through Onesearch, especially when the user names Context7, library resolution, current API docs, SDK docs, framework docs, package documentation, version-specific usage, setup, or migration through the Onesearch CLI.
---

# Onesearch Context7

Use Context7 for library, SDK, API, package, and framework documentation. Resolve the library first, then query focused docs.

## Bridge Contract

This skill is the source document for agent-facing Context7 provider direct commands. When a task names Context7, current library docs, API signatures, SDK setup, package options, framework configuration, version-specific examples, or migration guidance, route through Onesearch.

If command details may have changed, run:

```powershell
onesearch skills show context7 --format content
```

Use the provider command family by default:

```powershell
onesearch context7 resolve-library-id "library" --format json
onesearch context7 query-docs "/org/project" "focused docs question" --format json
```

## Commands

| Purpose | Command |
| --- | --- |
| Resolve library | `onesearch context7 resolve-library-id "react"` |
| Query docs | `onesearch context7 query-docs "/facebook/react" "query"` |

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
