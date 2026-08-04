---
name: onesearch-search
description: Use when an AI agent needs Onesearch for answer search, source discovery, current/latest/today information, news, prices, rankings, hot searches, trending topics, social-media hot lists such as Weibo 热搜/微博热搜前十, Chinese or domain-filtered web search, or search-result triage before fetching evidence.
---

# Onesearch Search

Use this workflow for current facts, answer synthesis, source discovery, rankings, news, prices, and hot lists. Search results are candidates; fetch source pages before using them as claim evidence.

## Discover the Contract

```text
onesearch schema search --format json
```

Inspect the targeted schema before using optional provider filters, validation levels, streaming, or repository enrichment.

## Workflow

1. Run `onesearch status --format json` and inspect the needed capability and provider availability.
2. Start with the routed workflow unless the user explicitly requires a provider.
3. Use `answer_search` for synthesis, `source_search` for candidates, and `page_fetch` for evidence. Do not substitute the answer preview for a source page.
4. For lists, rankings, schedules, and prices, base the result on source-search records or fetched pages rather than synthesis alone.
5. Fetch a small number of authoritative candidates with `--fetch-sources` or the `onesearch-fetch` skill.

```text
onesearch search "current question" --format json
onesearch search "current ranked list" --source-providers tavily --fetch-sources 2 --format json
onesearch search "question" --providers "answer_search=openai_responses;source_search=tavily;page_fetch=firecrawl" --format json
```

Provider filters are capability-specific. Use only providers shown as available for that capability. AnySearch is explicit/experimental rather than a default `source_search` route. DDG and Freecrawl are direct-only unless runtime config explicitly routes them.

For Chinese current or hot-list discovery, use Zhipu only when `status.direct_endpoints.zhipu.available` is true; otherwise use an available routed source provider.

## Output and Recovery

Use compact `--format json` to inspect `used.<capability>`, provider results, source candidates, previews, and `meta`. Use `--format content` for complete answer text or `--verbose` for the full structured payload.

On `parameter_error`, inspect only `schema search`. On `config_error`, run `doctor` then `status`. On network/provider failure, use a semantically equivalent available route or report the gap. On `evidence_error`, fetch better pages or narrow the claim.
