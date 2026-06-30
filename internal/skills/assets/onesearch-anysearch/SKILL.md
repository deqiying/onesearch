---
name: onesearch-anysearch
description: Use when an AI agent needs AnySearch through Onesearch for explicit vertical search, domain discovery, AnySearch extraction, or batch AnySearch queries.
---

# Onesearch AnySearch

Use AnySearch only as an explicit vertical or experimental provider. It is not part of the default `source_search` route unless runtime configuration opts into it.

## Commands

| Purpose | Command |
| --- | --- |
| Domains | `onesearch anysearch domains [domain]` |
| Search | `onesearch anysearch search "query"` |
| Extract | `onesearch anysearch extract "https://example.com"` |
| Batch | `onesearch anysearch batch "query1" "query2"` |

## Usage

```powershell
onesearch anysearch domains --format json
onesearch anysearch domains "example.com" --format json
onesearch anysearch search "query" --domain example.com --max-results 5 --format json
onesearch anysearch extract "https://example.com/page" --max-length 20000 --format json
onesearch anysearch batch "query one" "query two" --max-results 3 --format json
```

## Options

- `search --domain`
- `search --sub-domain`
- `search --max-results`
- `extract --max-length`
- `batch --max-results`

## Guardrails

- Do not use AnySearch as default broad web search.
- Use `onesearch search` or provider-specific Exa/Tavily/Firecrawl commands for normal web research.
- Run `onesearch doctor --format json` when AnySearch availability is uncertain.
