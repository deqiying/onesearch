---
name: onesearch-tavily
description: Use when an AI agent needs Tavily through Onesearch, especially when the user names Tavily, current web search, recency-sensitive source discovery, page extraction, site mapping, or bounded site crawling through the Onesearch CLI.
---

# Onesearch Tavily

Use Tavily for current web search, page extraction, site maps, and bounded site crawls. Prefer Tavily direct commands when the task names Tavily or needs Tavily-specific flags.

## Bridge Contract

This skill is the source document for agent-facing Tavily provider direct commands. When a task names Tavily, current web search, recency-sensitive research, page extraction, site mapping, or bounded crawling, route through Onesearch.

If command details may have changed, run:

```powershell
onesearch skills show tavily --format content
```

Use the provider command family by default:

```powershell
onesearch tavily search "query" --format json
onesearch tavily extract "https://example.com" --format json
onesearch tavily map "https://example.com" --format json
onesearch tavily crawl "https://example.com" --format json
```

## Commands

| Purpose | Command |
| --- | --- |
| Search | `onesearch tavily search "query"` |
| Extract pages | `onesearch tavily extract "https://example.com"` |
| Site map | `onesearch tavily map "https://example.com"` |
| Site crawl | `onesearch tavily crawl "https://example.com"` |

## Usage

```powershell
onesearch tavily search "latest AI policy news" --max-results 5 --search-depth advanced --format json
onesearch tavily search "official changelog" --include-domains example.com --include-raw-content --format json
onesearch tavily extract "https://example.com/article" --extract-format markdown --format json
onesearch tavily extract "https://example.com/a" "https://example.com/b" --query "pricing" --format json
onesearch tavily map "https://docs.example.com" --instructions "find API reference pages" --limit 50 --format json
onesearch tavily crawl "https://docs.example.com" --max-depth 2 --limit 20 --format json
```

## Options

Search:

- `--max-results`
- `--search-depth basic|advanced`
- `--topic`, `--time-range`, `--start-date`, `--end-date`, `--country`
- `--include-domains`, `--exclude-domains`
- `--include-raw-content`, `--include-images`, `--include-favicon`

Extract:

- Multiple URL positionals or `--urls`.
- `--extract-format markdown|text`
- `--extract-depth basic|advanced`
- `--query`
- `--include-images`, `--include-favicon`

Map and crawl:

- `--instructions`
- `--max-depth`, `--max-breadth`, `--limit`, `--timeout`
- `--allow-external`
- `--select-domains`, `--select-paths`
- `--exclude-domains`, `--exclude-paths`

## Output

Provider-direct JSON includes `provider: "tavily"` and `tool` set to the original MCP tool name such as `tavily_search` or `tavily_extract`.

Use `--format content` for extracted page body only. Use JSON for route-safe metadata and result lists.

## Guardrails

- Default map/crawl behavior keeps same-domain results unless `--allow-external` is set.
- Keep crawl `--limit` bounded to avoid huge outputs.
- Fetch or extract primary URLs before making high-risk claims.
