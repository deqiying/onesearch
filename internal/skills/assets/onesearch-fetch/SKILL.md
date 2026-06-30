---
name: onesearch-fetch
description: Use when an AI agent needs Onesearch to fetch URL content, inspect evidence pages, map a site, or verify claims from source pages.
---

# Onesearch Fetch Skill

Use this skill when the user provides URLs, when a search result must become evidence, or when a claim needs page-level verification.

Prefer these commands:

```powershell
onesearch fetch "https://example.com/page" --format json
onesearch fetch "https://example.com/page" --provider exa --format json
onesearch map "https://example.com" --provider firecrawl --instructions "find API docs" --format json
onesearch crawl "https://example.com" --provider tavily --max-depth 2 --limit 20 --format json
onesearch tavily extract "https://example.com/page" --format json
onesearch firecrawl scrape "https://example.com/page" --format json
onesearch exa web-fetch "https://example.com/page" --max-characters 12000 --format json
```

Workflow:

- Use `fetch` for claim-level evidence. Its output is evidence; search output is only discovery.
- Use workflow `--provider` when the task should keep workflow output but force an upstream provider.
- Use `tavily extract`, `firecrawl scrape`, or `exa web-fetch` when the task explicitly needs a provider-direct command.
- Use `map` to discover site structure before selecting pages to fetch.
- Use `tavily map`, `tavily crawl`, `firecrawl map`, or `firecrawl crawl` when you need direct provider behavior.
- For multi-source work, fetch the few key sources that support the answer rather than dumping every discovered URL.
- Distinguish fetched evidence from candidates in your final answer.
- If `fetch` returns a config error, run `doctor` and report the missing `page_fetch` provider instead of silently switching tools.

`page_fetch` currently routes through configured Tavily, Firecrawl, and Exa providers. Provider availability is visible in `doctor`.
