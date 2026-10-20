---
name: onesearch-deepseek
description: Use when an AI agent needs DeepSeek through Onesearch, especially when the user names DeepSeek search, needs query operators such as site:, -site:, lang:, after:, or before:, wants quality-scored source candidates, or needs DeepSeek single-page Markdown fetch through the Onesearch CLI.
---

# Onesearch DeepSeek

Use DeepSeek direct commands when the user names DeepSeek or needs its operator-aware search or single-page Markdown fetch. Prefer workflow commands when the upstream provider is not material.

## Discover the Contracts

```text
onesearch schema deepseek search --format json
onesearch schema deepseek fetch --format json
```

Run `onesearch status --format json` and require `status.direct_endpoints.deepseek.available == true` before a direct call. DeepSeek is disabled by default; enable it with `config setup deepseek` before use.

```text
onesearch deepseek search "OpenAI Responses API site:platform.openai.com lang:en after:2025-01-01" --count 5 --format json
onesearch deepseek search "大模型 评测 -site:zhihu.com lang:zh,en" --count 10 --need-content --format json
onesearch deepseek fetch "https://example.com/article" --format json
```

## Query Operators

Queries are 1-128 characters. Operators are passed through to DeepSeek and can be combined:

- `site:example.com` limits results to a domain; `-site:example.com` excludes it.
- `-keyword` excludes a term; `-"exact phrase"` excludes a phrase; `"exact phrase"` requires a phrase.
- `lang:zh,en` limits languages.
- `after:2025-01-01` and `before:2025-12-31` bound the publication date.

`--need-content` returns page bodies and increases output size; use it only when snippets are not enough. `--ori-question` passes the original user question, and `--localization` passes an IP address to prefer localized results; inspect the schema before using either.

## Reading Results

- `rerank_score` ranges 0-3; `>= 1.5` indicates good relevance quality.
- `authority` ranges 0-4, `-1` means unknown; `>= 3` indicates a very authoritative source.

Scores rank candidates; they are not evidence. Fetch authoritative pages before claim-level conclusions. `deepseek fetch` accepts one http(s) URL and returns the title plus Markdown content.

## Cost and Limits

Search and Fetch are billed per successful request, and each API allows about 5 QPS per account; Fetch also limits requests per target domain. Do not loop or fan out requests; keep `--count` small.

## Output and Recovery

Use compact JSON for candidates, scores, and metadata. Use `--format content` for complete fetched Markdown or `--verbose` for full structured data.

- On `parameter_error`, inspect only the failing DeepSeek leaf schema.
- On `config_error`, run `doctor` and `status`.
- On 402 insufficient balance (`provider_error`), do not retry; report it or use another provider.
- On `rate_limited`, `timeout`, or `upstream_error`, wait and retry at most once, or switch to another provider for the same capability.
- On Fetch 403 (`provider_error`) or 404 (`empty_result`), switch to another `page_fetch` provider.
- On auth failure (`auth_error`), check the DeepSeek key through `status` and `doctor` without printing it.

If DeepSeek is unavailable, choose another provider listed for the same capability or report the explicit DeepSeek requirement.
