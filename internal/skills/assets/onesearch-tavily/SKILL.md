---
name: onesearch-tavily
description: Use when an AI agent needs Tavily through Onesearch, especially when the user names Tavily, current web search, recency-sensitive source discovery, page extraction, site mapping, or bounded site crawling through the Onesearch CLI.
---

# Onesearch Tavily

Use Tavily direct commands when the user names Tavily or needs its search, extraction, map, or bounded crawl behavior. Prefer workflow commands when the upstream provider is not material.

## Discover the Contracts

```text
onesearch schema tavily search --format json
onesearch schema tavily extract --format json
onesearch schema tavily map --format json
onesearch schema tavily crawl --format json
```

Run `onesearch status --format json` and require `status.direct_endpoints.tavily.available == true` before a direct call.

```text
onesearch tavily search "latest policy update" --max-results 5 --format json
onesearch tavily extract "https://example.com/article" --extract-format markdown --format json
onesearch tavily crawl "https://docs.example.com" --max-depth 2 --limit 20 --format json
```

Use search results as candidates and extract authoritative pages before citing them. Map before crawling when site structure is unclear. Keep map/crawl limits small, restrict scope to the relevant domain, and inspect targeted schema before using advanced filters.

## Output and Recovery

Use compact JSON for URLs, scores, crawl state, and provider metadata. Use `--format content` for complete extracted text or `--verbose` for full structured data.

On `parameter_error`, inspect the failing Tavily leaf schema. On `config_error`, run `doctor` and `status`. If Tavily is unavailable, choose another provider listed for the same capability or report the explicit Tavily requirement.
