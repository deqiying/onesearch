---
name: onesearch-cli
description: Use when an AI agent needs Onesearch CLI capabilities, current web search, source-backed fact checking, URL/page fetching, site mapping or crawling, API/SDK/library documentation lookup, GitHub repository wiki context, offline deep research planning, or provider-direct Exa/Tavily/Firecrawl/Context7/DeepWiki/AnySearch/Zhipu commands through a reproducible local CLI.
---

# Onesearch CLI Router

Use this skill as the entry router for Onesearch. It should decide which built-in skill to load next, then use that skill's command guidance. Do not duplicate provider option lists here; provider skills own provider-specific commands and flags.

## Agent Routing Primer

Use workflow commands when the user describes the task. Use provider direct commands when the user names a provider. Use workflow `--provider` when the user wants workflow output while forcing an upstream provider.

## First Step

1. Run `onesearch skills list --format json` when you need the available skill inventory.
2. Load the most specific skill with `onesearch skills show <skill> --format content` or `onesearch load_skill <skill>`.
3. Run `onesearch doctor --format json` when provider availability, API keys, or runtime routes are uncertain.
4. Execute the commands from the loaded workflow or provider skill.

## Route By User Intent

| User intent | Load this skill | Typical command family |
| --- | --- | --- |
| Exa source search, official docs discovery, papers, product pages, or Exa page fetch | `exa` | `onesearch exa ...` |
| Tavily search, extract, map, or crawl | `tavily` | `onesearch tavily ...` |
| Firecrawl search, scrape, map, or crawl | `firecrawl` | `onesearch firecrawl ...` |
| Context7 library resolution or docs snippets | `context7` | `onesearch context7 ...` |
| DeepWiki repository architecture or wiki context | `deepwiki` | `onesearch deepwiki ...` |
| Explicit AnySearch vertical or experimental search | `anysearch` | `onesearch anysearch ...` |
| Chinese, China-specific, or Zhipu direct source discovery | `zhipu` | `onesearch zhipu search ...` |
| Broad answer search, current source discovery, or source triage | `search` | `onesearch search ...` |
| API, SDK, package, framework, or official documentation lookup | `docs` | `onesearch context7 ...`, `onesearch exa ...` |
| URL evidence, page fetch, site map, or bounded crawl | `fetch` | `onesearch fetch ...`, `onesearch map ...`, provider fetch/map/crawl |
| Offline multi-step research planning | `deep-research` | `onesearch deep ...` |

## Runtime Capability Names

| Capability | Route to |
| --- | --- |
| `answer_search` | `search` |
| `source_search` | `search`, or provider skills such as `exa`, `tavily`, `firecrawl`, `zhipu` |
| `docs_search` | `docs`, `context7`, or `exa` |
| `page_fetch` | `fetch`, `exa`, `tavily`, or `firecrawl` |
| `site_map` | `fetch`, `tavily`, or `firecrawl` |
| `site_crawl` | `fetch`, `tavily`, or `firecrawl` |
| `repo_wiki` | `deepwiki` |
| `vertical_search` | `anysearch` |
| `routing` | `onesearch-cli` |

## Common Routing Commands

```powershell
onesearch skills list --format json
onesearch skills list --capability page_fetch --format json
onesearch skills show exa --format content
onesearch skills show tavily --format content
onesearch load_skill exa
onesearch load_skill tavily
onesearch doctor --format json
onesearch config list --format json
```

## Routing Rules

- Prefer the most specific provider skill when the user names a provider.
- Use workflow `--provider` when the user names a provider but still wants workflow-shaped output.
- Use workflow skills (`search`, `docs`, `fetch`, `deep-research`) when the user asks by task intent rather than provider name.
- Treat search results as discovery candidates. Fetch or extract key URLs before claim-level conclusions.
- Keep API keys out of final answers. `doctor` and `config list` mask secrets.

## Supporting Reference

Read `references/cli-contract.md` when you need the full command contract, output fields, exit codes, runtime schema, or regression checks.
