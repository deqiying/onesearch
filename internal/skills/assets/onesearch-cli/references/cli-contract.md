# Onesearch CLI Contract

This reference defines the public CLI contract for `onesearch`.

## Agent Routing Primer

- If the user describes a task, prefer workflow commands.
- If the user names an upstream provider, use provider direct commands.
- If the user wants workflow output while specifying the upstream provider, use workflow `--provider`.

## Public Command Surface

Workflow / capability commands:

- `onesearch search "query"`
- `onesearch fetch "https://example.com/page" --provider tavily`
- `onesearch map "https://example.com" --provider firecrawl`
- `onesearch crawl "https://example.com" --provider tavily`
- `onesearch repo-wiki "owner/repo" ["question"] --provider deepwiki`
- `onesearch deep "research question"`

Provider direct commands:

- `onesearch exa web-search "query"`
- `onesearch exa web-fetch "https://example.com/page"`
- `onesearch exa similar "https://example.com/page"`
- `onesearch tavily search "query"`
- `onesearch tavily extract "https://example.com/page"`
- `onesearch tavily map "https://example.com"`
- `onesearch tavily crawl "https://example.com"`
- `onesearch firecrawl search "query"`
- `onesearch firecrawl scrape "https://example.com/page"`
- `onesearch firecrawl map "https://example.com"`
- `onesearch firecrawl crawl "https://example.com"`
- `onesearch context7 resolve-library-id "library"`
- `onesearch context7 query-docs "/org/project" "query"`
- `onesearch deepwiki ask-question "owner/repo" "question"`
- `onesearch deepwiki read-wiki-structure "owner/repo"`
- `onesearch deepwiki read-wiki-contents "owner/repo"`
- `onesearch anysearch domains [domain]`
- `onesearch anysearch search "query"`
- `onesearch anysearch extract "https://example.com/page"`
- `onesearch anysearch batch "query 1" "query 2"`
- `onesearch zhipu search "query"`

Utility commands:

- `onesearch doctor`
- `onesearch status`
- `onesearch smoke`
- `onesearch config path|list`
- `onesearch model current`
- `onesearch skills list`
- `onesearch skills show NAME`

Supported output formats are `json`, `markdown`, and `content`. JSON is the default and is the stable format for agents and scripts. Search success output defaults to a compact unified result object; use `--verbose` to include full diagnostics such as routing decisions, provider attempts, and capability status. Error output also defaults to `--quiet`. `--quiet` overrides debug defaults.

Explicit non-contract:

- `onesearch mcp <tool>` is not a public entry.
- Provider group snake_case tool aliases are not public CLI subcommands.
- Flat provider commands are not public entries.

## Runtime configuration

Configuration is organized by runtime schema:

```json
{
  "schema_version": 1,
  "defaults": {},
  "pipelines": {},
  "routes": {},
  "profiles": {},
  "providers": {}
}
```

Responsibilities:

- `defaults`: pipeline, fallback mode, validation level, minimum profile, timeout, retry, log, SSL, and cleanup defaults.
- `pipelines`: ordered capability sets for tasks such as `default`, `research`, `docs`, and `crawl`.
- `routes`: ordered provider fallback chains for each capability; providers that declare a capability in `providers.<id>.capabilities` are automatically appended to that capability route if they are not already listed.
- `profiles`: required and optional capabilities for readiness checks.
- `providers`: adapter, capability list, base URL, direct API key, API-key environment key, enabled state, aliases, and settings.

Configuration file defaults:

- Windows: `%LOCALAPPDATA%\onesearch\config.json`
- macOS/Linux: `~/.config/onesearch/config.json`
- Override: `ONESEARCH_CONFIG_DIR`

Provider secrets can be configured with either `providers.<id>.api_key` or the environment variable named by `providers.<id>.api_key_env`. When both are set, `api_key` wins. Runtime orchestration is read from the schema, not inferred from legacy KEY/value config entries.

On first normal execution, the CLI creates the config directory and an initial `config.json` when the file is missing, then continues with that initial runtime schema. Only `doctor` reports whether the config file was just created. Initial provider defaults keep API-key-required providers disabled; anonymous providers that do not force an API key may be enabled. DeepWiki uses the public MCP endpoint anonymously for public repository docs; `DEEPWIKI_API_KEY` is optional and only needed for private documentation access.

## Capability names

Public diagnostics and provider attempts use the schema capability names:

- `answer_search`
- `source_search`
- `docs_search`
- `page_fetch`
- `site_map`
- `site_crawl`
- `repo_wiki`
- `vertical_search`

Default routes:

```json
{
  "answer_search": ["xai", "openai_compatible", "openai_responses"],
  "source_search": ["exa", "zhipu", "tavily", "firecrawl"],
  "docs_search": ["exa", "context7"],
  "page_fetch": ["tavily", "firecrawl", "exa"],
  "site_map": ["tavily", "firecrawl"],
  "site_crawl": ["tavily", "firecrawl"],
  "repo_wiki": ["deepwiki"],
  "vertical_search": ["anysearch"]
}
```

The standard profile requires `answer_search`, `docs_search`, and `page_fetch`.

## Provider behavior

- `answer_search`: xAI Responses, OpenAI-compatible Chat Completions, and optional OpenAI Responses.
- `source_search`: Exa, Zhipu Web Search, Tavily, and Firecrawl.
- `docs_search`: Exa first, Context7 for library/API/framework docs.
- `page_fetch`: Tavily first, Firecrawl fallback, with Exa contents available for explicit `exa web-fetch` and configured fetch routes.
- `site_map`: Tavily first, Firecrawl fallback.
- `site_crawl`: Tavily first, Firecrawl fallback.
- `repo_wiki`: DeepWiki public repository docs are available anonymously; optional `DEEPWIKI_API_KEY` enables private documentation access.
- `vertical_search`: AnySearch explicit commands only by default.

`repo-wiki` accepts GitHub repository names in `owner/repo` form or GitHub repository URLs such as `https://github.com/microsoft/playwright`. Bare repository names are rejected because they cannot be resolved to a unique owner. `repo-wiki --mode ask|structure|contents` selects DeepWiki's question answering, wiki structure, or full wiki contents. If no mode is supplied, a question selects `ask`; otherwise the command defaults to `structure`.

OpenAI protocol adapters normalize base URLs before appending endpoints. `openai_responses` always calls `/v1/responses`; `openai_chat_completions` always calls `/v1/chat/completions`; neither adapter downgrades into the other. Both adapters parse JSON and SSE responses, including provider error payloads. `providers.<id>.settings.stream` controls whether the request asks for streaming, and `search --stream` / `search --no-stream` temporarily overrides that setting. `openai_responses` defaults to `tools: ["web_search"]` and `tool_choice: "required"`; `openai_chat_completions` defaults to no tools but can pass configured `settings.tools` and `settings.tool_choice` through to compatible services. Empty strings, empty arrays, and empty objects in provider `settings` keep the built-in defaults.

AnySearch must not be treated as the default `source_search` route unless runtime configuration explicitly changes the route.

`search` supports capability-level provider filters. The explicit flags are:

- `--answer-providers`
- `--source-providers`
- `--docs-providers`
- `--fetch-providers`
- `--repo-providers`

`--providers` also accepts a scoped expression such as:

```powershell
onesearch search "query" --providers "answer_search=openai_responses;source_search=tavily;page_fetch=firecrawl" --format json
```

A legacy unscoped value such as `--providers openai_responses` is still treated as the provider filter for normal search routing (`answer_search`, `source_search`, `docs_search`). `page_fetch` and `repo_wiki` continue to use their capability routes unless an explicit `--fetch-providers`, `--repo-providers`, or scoped `--providers` filter is supplied. Agents should perform vertical intent recognition themselves, then choose the appropriate `onesearch` capability and provider filters; `onesearch` does not infer domain-specific vertical tasks.

`search --fetch-sources N` automatically runs `page_fetch` on the first N URL candidates discovered by `source_search`. This is intended for agent-selected high-confidence source discovery flows, such as official ranking pages or current structured pages. The fetched evidence appears under `used.page_fetch` with role `source_evidence`.

Standalone workflow provider filters:

- `fetch --provider` maps to `page_fetch`.
- `map --provider` maps to `site_map`.
- `crawl --provider` maps to `site_crawl`.
- `repo-wiki --provider` maps to `repo_wiki`.
- `--provider` accepts provider IDs or aliases, including comma-separated fallback filters.

## Output contract

Common fields:

- `ok`: boolean success status.
- `error_type`: empty on success, otherwise one of `parameter_error`, `config_error`, `network_error`, `evidence_error`, or provider-specific error category.
- `error`: human-readable failure text.
- `elapsed_ms`: elapsed time.

Quiet error fields:

- `ok`
- `error_type`
- `error`: compact summary with low-level transport details removed.
- `elapsed_ms`
- command context fields when present, such as `session_id`, `query`, `url`, `provider`, `tool`, and `mode`.
- `hint`: suggests `--verbose` or `doctor` for diagnostics.

Default search success fields:

- `ok`
- `query`
- `used`
- `meta`

`used` is the only default result tree. It is grouped by actual runtime usage and indexed by stable names:

- `used.<capability>`: capability used by this run, such as `answer_search`, `docs_search`, `source_search`, or `page_fetch`.
- `used.<capability>.role`: why this capability was used when there is one role, such as `primary_answer`, `documentation_sources`, `current_sources`, `extra_sources`, `page_evidence`, or `repository_wiki`.
- `used.<capability>.roles`: multiple roles when the same capability was used by more than one path.
- `used.<capability>.providers.<provider>`: provider execution that returned usable results for that capability.
- `used.<capability>.providers.<provider>.status`: provider result status, usually `ok`.
- `used.<capability>.providers.<provider>.mode` and `model`: answer-provider metadata when available.
- `used.<capability>.providers.<provider>.elapsed_ms`: elapsed time for that provider execution when available.
- `used.answer_search.providers.<provider>.result.content_preview`: compact answer preview for default JSON output.
- `used.answer_search.providers.<provider>.result.content_length`: original answer length before compacting.
- `used.<capability>.providers.<provider>.result.sources`: provider-owned source candidates. Source entries keep quality and provenance fields such as `capability`, `provider`, `title`, `url`, `published_date`, `description`, `snippet`, `id`, and `library_id`.
- `used.page_fetch.providers.<provider>.result.content_preview`: page text preview for `page_fetch`.
- `used.page_fetch.providers.<provider>.result.pages`: compact fetched pages when `search --fetch-sources N` is used.
- `used.repo_wiki.providers.deepwiki.result.content_preview`: compact DeepWiki repository context when `search --repo-wiki` is used.

`meta` keeps compact run metadata such as `session_id`, `validation_level`, `elapsed_ms`, and `fallback_used`.

Default search JSON does not include top-level `content`, `answer`, `sources`, or `sources_count`, and `answer_search` keeps only `content_preview` plus `content_length` under `used`. Use `--format content` for answer text only, or add `--verbose` when complete JSON answer text is required.

Verbose search result fields include the default fields plus:

- `content`
- `sources`
- `sources_count`
- `primary_sources`
- `extra_sources`
- `routing_decision`
- `provider_attempts`
- `providers_used`
- `fallback_used`
- `validation_level`
- `diagnostics`: includes `minimum_profile`, `capabilities`, and provider/routing details when available.

`provider_attempts[].capability` uses schema capability names. Search result sources are discovery candidates and must not be cited as claim proof until fetched.

Doctor output fields:

- `ok`
- `status`
- `error_type`
- `error`
- `config`: only `file`, `created`, `missing_before_start`, and optional `initialization_error`
- `schema`: only `version` and `source`
- `minimum_profile`: only `ok`, `profile`, `required`, `missing`, and error summary when failed
- `issues`: compact diagnostics such as `missing_required_capability` and `provider_config_error`
- `elapsed_ms`

`doctor` is a compact diagnostic command and defaults to compact JSON for agent-first usage. It should not print the full runtime schema or config file content by default. Use `doctor --format content` for a short human-readable summary, and `config list --format json` for complete `defaults`, `pipelines`, `routes`, `profiles`, and `providers`.

Status output fields:

- `ok`: report generation status.
- `ready`: whether the configured minimum profile is currently satisfied.
- `status`: `ready`, `degraded`, or `initialization_error`.
- `config`: config path and initialization metadata.
- `schema`: runtime schema version and source.
- `minimum_profile`: compact profile readiness summary.
- `capabilities`: capability-command availability for `answer_search`, `source_search`, `docs_search`, `page_fetch`, `site_map`, `site_crawl`, `repo_wiki`, and `vertical_search`.
- `providers`: provider-level availability, enabled state, masked key metadata, aliases, base URL, settings, and supported capabilities.
- `direct_endpoints`: provider-direct command families such as `exa`, `tavily`, `firecrawl`, `context7`, `deepwiki`, `anysearch`, and `zhipu`, with commands and availability copied from `providers`.

`status` is the agent preflight for choosing concrete tools. Use it after or alongside `doctor` when deciding whether to call a specific capability or provider-direct endpoint. A bundled skill such as `zhipu` only describes command usage; it does not imply `providers.zhipu` is enabled.

## Load skill

`onesearch skills list` prints a JSON inventory of bundled skills:

```powershell
onesearch skills list --format json
onesearch skills list --capability page_fetch --format json
```

Each item includes `id`, `aliases`, `capabilities`, and `description`.

`onesearch skills show NAME` prints a selected skill with metadata and content:

```powershell
onesearch skills show onesearch-cli --format content
onesearch skills show exa --format content
onesearch skills show tavily --format content
```

Supported names and aliases:

- `onesearch-cli`, `base`, `onesearch`, `cli`, `router`
- `search`, `web-search`, `source-search`
- `docs`, `api-docs`, `documentation`
- `fetch`, `page-fetch`, `evidence`
- `exa`, `exa-tools`, `exa-web`, `exa-fetch`, `exa-similar-pages`
- `tavily`, `tavily-tools`, `tavily-search`, `tavily-extract`, `tavily-map`, `tavily-crawl`
- `firecrawl`, `firecrawl-tools`, `firecrawl-search`, `firecrawl-scrape`, `firecrawl-map`, `firecrawl-crawl`
- `context7`, `context7-tools`, `ctx7`, `context7-provider`, `context7-library-docs`
- `deepwiki`, `deepwiki-tools`, `repo-wiki`, `repository-wiki`
- `anysearch`, `anysearch-tools`, `as`
- `zhipu`, `zhipu-tools`, `zhipu-web-search`, `zp`
- `deep-research`, `deep`, `research`

`skills show` does not read user provider config, call network providers, write config, or install files. It only returns the bundled skill metadata and content.

## Deep Research

`onesearch deep` is an offline planner. It does not call live providers, run `doctor`, fetch pages, or change runtime routes.

Expected fields:

- `mode = deep_research`
- `query_mode = deep`
- `question`
- `intent_signals`
- `decomposition`
- `capability_plan`
- `preflight`
- `evidence_policy = fetch_before_claim`
- `steps`
- `gap_check`
- `final_answer_policy`
- `allowed_tools`
- `evidence_dir`

Allowed `steps[].tool` values include `search`, `fetch`, `map`, `crawl`, `repo-wiki`, `exa web-search`, `exa web-fetch`, `exa similar`, `tavily search`, `tavily extract`, `context7 resolve-library-id`, `context7 query-docs`, `deepwiki ask-question`, and `zhipu search`.

## Exit codes

- `0`: success.
- `2`: parameter error.
- `3`: configuration error.
- `4`: network or evidence error.
- `5`: unexpected local/render/write error.

## Regression checks

Minimum local checks:

```powershell
go test ./...
go run .\cmd\onesearch smoke --mock --format json
go run .\cmd\onesearch doctor --format json
go run .\cmd\onesearch status --format json
go run .\cmd\onesearch config list --format json
go run .\cmd\onesearch skills list --format json
go run .\cmd\onesearch skills show onesearch-cli --format content
go run .\cmd\onesearch skills show exa --format content
go run .\cmd\onesearch skills show tavily --format content
```

Deep Research examples:

```powershell
onesearch deep "深度搜索一下最近的比特币行情" --format json
onesearch deep "OpenAI Responses API web_search 和 Chat Completions 联网搜索怎么选" --budget deep --format json
onesearch deep "帮我核验这个说法是真是假：某工具已经完全替代 Tavily 做 AI 搜索了" --format json
onesearch deep "https://example.com/source" --format json
```
