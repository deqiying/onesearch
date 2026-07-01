---
name: onesearch-cli
description: Use when you need to search the web, look up current or latest public information such as news, prices, rankings, hot searches, or trending topics, verify claims with online sources, read a URL, map or crawl a website, find official API/SDK/package/framework docs, or inspect public GitHub repo docs and architecture.
---

# Onesearch CLI Router

Use this top-level skill as the broad search and research entry point for Onesearch. Load it when the task is about searching the web, checking current facts, reading public docs, fetching URL evidence, exploring a website, or understanding a public repository, even if the user has never mentioned Onesearch by name.

This skill should explain what Onesearch can do, choose the most specific built-in skill to load next, and then use that skill's command guidance. Do not duplicate provider option lists here; workflow and provider skills own provider-specific commands and flags.

## Use When The Task Needs

- Web or internet search, source discovery, query answering, source-backed research, or claim verification.
- Real-time, current, latest, today, news, prices, schedules, rankings, leaderboards, hot searches, hot lists, trending topics, Weibo 热搜, 微博热搜前十, or other fast-changing public information.
- Official API, SDK, package, library, framework, product, changelog, migration, setup, or version-specific documentation lookup.
- URL/page fetching, page-content extraction, evidence inspection, site mapping, bounded crawling, or converting search results into citable evidence.
- Public GitHub repository architecture, generated wiki context, module summaries, implementation overviews, wiki structure, or wiki contents.
- Explicit provider commands for Exa, Tavily, Firecrawl, Context7, DeepWiki, AnySearch, Zhipu, DDG, or Freecrawl.
- Offline decomposition for complex research before running live searches.

Do not use Onesearch for purely local repository inspection, local file editing, spreadsheet/document manipulation, or tasks that do not need public web/search/docs/fetch/research capabilities.

## Agent Routing Primer

Use workflow commands when the user describes the task. Use provider direct commands when the user names a provider. Use workflow `--provider` when the user wants workflow output while forcing an upstream provider.

## First Step

1. Run `onesearch skills list --format json` when you need the available skill inventory.
2. Run `onesearch skills list --capability <capability> --format json` when you know the capability but not the best skill.
3. Load the most specific skill with `onesearch skills show <skill> --format content` before using detailed provider or workflow commands.
4. Run `onesearch doctor --format json` when overall provider readiness, API keys, or runtime routes are uncertain.
5. Run `onesearch status --format json` before choosing a specific capability or provider-direct endpoint; only use providers and capabilities whose status is available.
6. Execute the commands from the loaded workflow or provider skill.

## What Onesearch Provides

| Capability | User wording that should trigger this | Load this skill | Typical command family |
| --- | --- | --- | --- |
| Broad web search and source discovery | search the web, look up, find sources, current info, latest, today | `search` | `onesearch search ...` |
| Fresh facts, rankings, prices, hot lists, trends | news, rankings, top-N, hot search, 热搜, 热榜, 榜单, 前十 | `search`; `zhipu` only when status says available | `onesearch search ...`, or an available provider-direct command |
| Source-backed fact checking | verify this claim, fact check, cite sources, compare evidence | `search`, then `fetch` | `onesearch search ... --extra-sources 2`, `onesearch fetch ...` |
| Official docs and API references | API docs, SDK docs, library docs, package options, migration, setup | `docs`, `context7`, `exa` | `onesearch context7 ...`, `onesearch exa web-search ...` |
| URL/page evidence | fetch this URL, read this page, extract page content, inspect evidence | `fetch` | `onesearch fetch ...`, `onesearch tavily extract ...`, `onesearch firecrawl scrape ...`, `onesearch exa web-fetch ...`, direct-only `ddg` or `freecrawl` when enabled |
| Site discovery | map this site, crawl docs, find pages under a domain | `fetch`, `tavily`, `firecrawl`, `freecrawl` | `onesearch map ...`, `onesearch crawl ...`, provider map/crawl commands |
| Public repository context | GitHub repo architecture, public project docs, repo wiki, module overview | `deepwiki` | `onesearch repo-wiki ...`, `onesearch deepwiki ...` |
| Provider-direct commands | user names Exa, Tavily, Firecrawl, Context7, DeepWiki, AnySearch, Zhipu, DDG, or Freecrawl | matching provider skill | `onesearch <provider> <command> ...` |
| Research planning | complex research, plan the search, decompose before browsing | `deep-research` | `onesearch deep ...` |

## Built-In Skill Inventory

| Skill | Use for |
| --- | --- |
| `search` | Broad answer search, current source discovery, source triage, hot/trending/top-N queries, and search result comparison. |
| `docs` | API, SDK, package, library, framework, official docs, version-specific usage, setup, and migration lookup. |
| `fetch` | URL/page content, evidence extraction, site maps, bounded crawls, and claim-level source verification. |
| `deep-research` | Offline multi-step research planning before executing provider calls. |
| `exa` | Exa source search, official docs discovery, papers, product pages, similar pages, and Exa page fetch. |
| `tavily` | Tavily current search, page extraction, site mapping, and bounded crawling. |
| `firecrawl` | Firecrawl robust search, page scraping, markdown extraction, site mapping, and crawl jobs. |
| `context7` | Context7 library resolution and focused current docs snippets. |
| `deepwiki` | DeepWiki public GitHub repository architecture, wiki structure, and wiki contents. |
| `anysearch` | Explicit AnySearch vertical or experimental search, domain discovery, extraction, or batch queries. |
| `zhipu` | Chinese, China-specific, current/latest/today search, hot searches, social-media hot lists, and domain-filtered Chinese source discovery. |
| `ddg` | Direct-only local DuckDuckGo MCP stdio search and page content fetching. |
| `freecrawl` | Direct-only local Freecrawl MCP stdio search, scraping, crawling, and provider deep research. |

## Runtime Capability Names

| Capability | Route to |
| --- | --- |
| `answer_search` | `search` |
| `source_search` | `search`, or provider skills such as `exa`, `tavily`, `firecrawl`, `zhipu`, `ddg`, `freecrawl` |
| `docs_search` | `docs`, `context7`, or `exa` |
| `page_fetch` | `fetch`, `exa`, `tavily`, `firecrawl`, `ddg`, or `freecrawl` |
| `site_map` | `fetch`, `tavily`, or `firecrawl` |
| `site_crawl` | `fetch`, `tavily`, `firecrawl`, or `freecrawl` |
| `repo_wiki` | `deepwiki` |
| `vertical_search` | `anysearch` |
| `routing` | `onesearch-cli` |

## Common Routing Commands

```powershell
onesearch skills list --format json
onesearch skills list --capability source_search --format json
onesearch skills list --capability docs_search --format json
onesearch skills list --capability page_fetch --format json
onesearch skills show search --format content
onesearch skills show docs --format content
onesearch skills show fetch --format content
onesearch skills show exa --format content
onesearch skills show tavily --format content
onesearch skills show ddg --format content
onesearch skills show freecrawl --format content
onesearch doctor --format json
onesearch status --format json
onesearch config list --format json
```

## Routing Rules

- Prefer the most specific provider skill when the user names a provider.
- Use workflow `--provider` when the user names a provider but still wants workflow-shaped output.
- Use workflow skills (`search`, `docs`, `fetch`, `deep-research`) when the user asks by task intent rather than provider name.
- Prefer `search` for broad online questions, `docs` for documentation/API questions, and `fetch` when a URL or claim-level evidence is needed.
- For current, latest, today, rankings, prices, schedules, hot lists, or social-media trends, use Onesearch instead of answering from memory.
- Treat search results as discovery candidates. Fetch or extract key URLs before claim-level conclusions.
- Do not assume a provider skill means that provider is enabled. Check `onesearch status --format json` before calling direct providers such as `exa`, `tavily`, `firecrawl`, `zhipu`, `ddg`, `freecrawl`, or answer providers such as `xai`.
- Treat `ddg` and `freecrawl` as direct-only local MCP stdio provider templates unless runtime routes explicitly include them.
- Keep API keys out of final answers. `doctor`, `status`, and `config list` mask secrets.

## Supporting Reference

Read `references/cli-contract.md` when you need the full command contract, output fields, exit codes, runtime schema, or regression checks.
