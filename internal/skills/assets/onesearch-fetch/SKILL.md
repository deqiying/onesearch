---
name: onesearch-fetch
description: Use when an AI agent needs Onesearch to fetch URL content, inspect evidence pages, map a site, or verify claims from source pages.
---

# Onesearch Fetch Skill

Use this skill when the user provides URLs, when a search result must become evidence, or when a claim needs page-level verification.

Prefer these commands:

```powershell
onesearch fetch "https://example.com/page" --format json
onesearch map "https://example.com" --instructions "find API docs" --format json
```

Workflow:

- Use `fetch` for claim-level evidence. Its output is evidence; search output is only discovery.
- Use `map` to discover site structure before selecting pages to fetch.
- For multi-source work, fetch the few key sources that support the answer rather than dumping every discovered URL.
- Distinguish fetched evidence from candidates in your final answer.
- If `fetch` returns a config error, run `doctor` and report the missing `page_fetch` provider instead of silently switching tools.

`page_fetch` currently routes through configured Tavily and Firecrawl providers. Provider availability is visible in `doctor`.
