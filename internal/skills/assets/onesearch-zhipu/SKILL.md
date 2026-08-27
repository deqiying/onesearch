---
name: onesearch-zhipu
description: Use when an AI agent needs Zhipu through Onesearch for Chinese web search, China-specific current/latest/today information, Chinese hot searches, trending topics, social-media hot lists such as 微博热搜/微博热搜前十, domain-filtered Chinese source discovery, or the Zhipu provider direct search command.
---

# Onesearch Zhipu

Use Zhipu direct search for Chinese-language, China-specific, current, hot-list, or domain-filtered discovery when that provider is explicitly useful. Zhipu exposes one canonical search leaf; do not invent a separate hot-list command.

## Discover the Contract

```text
onesearch schema zhipu search --format json
```

Run `onesearch status --format json` and require `status.direct_endpoints.zhipu.available == true` before a direct call.

```text
onesearch zhipu search "今天国内 AI 新闻" --count 5 --format json
onesearch zhipu search "微博热搜 前十 当前榜单" --count 10 --content-size medium --format json
onesearch zhipu search "站点内信息" --search-domain-filter example.cn --format json
```

Zhipu keeps the China-compatible `bigmodel_cn` profile by default. A configured `zai_global` profile uses the global `api.z.ai` contract; do not mix engines or regional base URLs.

Treat returned summaries and links as discovery candidates. Fetch authoritative pages before claim-level conclusions, especially for rankings and rapidly changing topics.

## Output and Recovery

Use compact JSON for candidates and metadata. Use `--format content` for complete primary text or `--verbose` for the full structured provider response.

On `parameter_error`, inspect only `schema zhipu search`. On `config_error`, run `doctor` and `status`. If Zhipu is unavailable, use an available `source_search` provider and state that the Chinese-provider route changed.
