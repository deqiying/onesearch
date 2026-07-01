---
name: onesearch-zhipu
description: Use when an AI agent needs Zhipu through Onesearch for Chinese web search, China-specific current/latest/today information, Chinese hot searches, trending topics, social-media hot lists such as 微博热搜/微博热搜前十, domain-filtered Chinese source discovery, or the Zhipu provider direct search command.
---

# Onesearch Zhipu

Use Zhipu for Chinese, China-specific, current, or domain-filtered source discovery only when the runtime reports the Zhipu direct endpoint as available.

## Command

```powershell
onesearch status --format json
onesearch zhipu search "query" --format json
```

Before running `onesearch zhipu search`, confirm `status.direct_endpoints.zhipu.available == true`. If Zhipu is disabled, missing a key, or absent from `status.capabilities.source_search.available`, do not call this direct endpoint; use `onesearch search` with an available `source_search` provider such as `exa`, `tavily`, or `firecrawl`.

## Usage

```powershell
onesearch zhipu search "今天国内 AI 新闻" --count 5 --format json
onesearch zhipu search "微博热搜 前十 当前榜单" --count 10 --content-size medium --format json
onesearch zhipu search "某政策 最新 解读" --count 10 --content-size medium --format json
onesearch zhipu search "站点内信息" --search-domain-filter example.cn --format json
```

## Options

- `--count`: result count.
- `--search-engine`: provider search engine override.
- `--search-recency-filter`: recency filter, default `noLimit`.
- `--search-domain-filter`: restrict search to a domain.
- `--content-size`: returned content size, default `medium`.

## Output

JSON includes `provider`, `results`, `total`, and timing fields. Use results as source candidates and fetch important URLs before claim-level conclusions.

## Guardrails

- Prefer Zhipu for Chinese and China-specific current information only after the status check passes.
- Prefer Exa or Context7 for official English API docs and package docs.
- Run `onesearch doctor --format json` for overall configuration health and `onesearch status --format json` for actual Zhipu availability.
