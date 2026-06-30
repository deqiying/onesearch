---
name: onesearch-search
description: Use when an AI agent needs Onesearch for answer search, source discovery, current information, Chinese or domain-filtered web search, or search-result triage before fetching evidence.
---

# Onesearch Search Skill

Use this skill for broad answer search and source discovery. Run `onesearch doctor --format json` first when provider availability is uncertain, and treat `ok: false` as a hard configuration issue unless the user explicitly asks for offline planning.

Prefer these commands:

```powershell
onesearch search "query" --validation balanced --extra-sources 3 --format json
onesearch search "query" --validation strict --source-providers tavily --fetch-providers tavily --fetch-sources 1 --format json
onesearch search "query" --repo-wiki owner/repo --validation strict --format json
onesearch tavily search "query" --max-results 5 --format json
onesearch exa web-search "query" --include-highlights --format json
onesearch zhipu search "query" --count 10 --format json
```

Routing guidance:

- `search` is the normal first pass for broad answers. For real-time or fast-changing information, prefer `search --extra-sources 2` or `search --extra-sources 3` as the first pass so the agent can compare source candidates instead of relying on one synthesized answer.
- Default `search --format json` returns `ok`, `query`, `used`, and `meta`; inspect `used.answer_search.providers.<provider>.result.content_preview` for the compact answer preview and `used.<capability>.providers.<provider>.result.sources` for provider-owned source candidates. Use `--format content` or `--verbose` when complete answer text is required.
- Add `--extra-sources 2` or `--extra-sources 3` when the task needs source coverage instead of only one synthesized answer. Good cases include volatile current information, rankings or lists, market/public-opinion snapshots, comparison research, and any question where different sources may disagree or update at different speeds.
- For rankings, lists, schedules, prices, leaderboards, and other structured current results, treat `answer_search` as synthesis only. Prefer list/table content from `source_search` results or fetched pages as the final basis.
- When the agent has narrowed a task to a likely official or high-confidence page, use `--fetch-sources 1` to fetch the best `source_search` candidate in the same run. Inspect `used.page_fetch` with role `source_evidence`.
- Use capability-level provider filters when a task needs different providers for answer synthesis, source discovery, and page fetch. Prefer explicit flags such as `--source-providers tavily --fetch-providers firecrawl`, or use `--providers "answer_search=openai_responses;source_search=tavily;page_fetch=firecrawl"`.
- Do not expect `onesearch` to infer vertical tasks such as a specific social-media ranking. The calling agent should identify the vertical intent, then choose the right command flags and provider filters.
- Do not bind `--extra-sources` to fixed keywords. The calling agent should decide from user intent, required confidence, freshness risk, and whether multi-source evidence is needed.
- Keep `--extra-sources` small by default. Use `2` or `3` for normal multi-source checks; increase it only when the user explicitly asks for broad coverage or deep comparison.
- Add `--repo-wiki owner/repo` when the agent knows repository architecture context is needed; results appear under `used.repo_wiki`.
- `tavily search` is the preferred direct Tavily command when the user explicitly names Tavily.
- `exa web-search` is the preferred direct Exa command for low-noise source discovery, official docs pages, papers, product pages, and known-domain searches.
- `zhipu search` is best for Chinese, China-specific, current, or domain-filtered source discovery.
- Search results are discovery candidates. Fetch key URLs before making high-risk or claim-level statements.
- Do not use AnySearch as the default `source_search` route; keep it to explicit experimental commands unless the runtime route is explicitly configured.

Provider keys and routes are owned by local config and `doctor`; this skill does not imply any provider is available.
