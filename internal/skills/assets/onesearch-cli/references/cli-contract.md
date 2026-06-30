# Onesearch CLI Contract

This reference defines the public CLI contract for `onesearch`.

## Commands

Core commands:

- `onesearch search "query"`
- `onesearch fetch "https://example.com/page"`
- `onesearch map "https://example.com"`
- `onesearch crawl "https://example.com"`
- `onesearch repo-wiki "owner/repo" ["question"]`
- `onesearch exa-search "query"`
- `onesearch exa-similar "https://example.com/page"`
- `onesearch exa web-search "query"`
- `onesearch exa web-fetch "https://example.com/page"`
- `onesearch tavily search "query"`
- `onesearch tavily extract "https://example.com/page"`
- `onesearch tavily map "https://example.com"`
- `onesearch tavily crawl "https://example.com"`
- `onesearch firecrawl search "query"`
- `onesearch firecrawl scrape "https://example.com/page"`
- `onesearch firecrawl map "https://example.com"`
- `onesearch firecrawl crawl "https://example.com"`
- `onesearch zhipu-search "query"`
- `onesearch context7-library "library" "query"`
- `onesearch context7-docs "/org/project" "query"`
- `onesearch context7 resolve-library-id "library"`
- `onesearch context7 query-docs "/org/project" "query"`
- `onesearch deepwiki ask-question "owner/repo" "question"`
- `onesearch deepwiki read-wiki-structure "owner/repo"`
- `onesearch deepwiki read-wiki-contents "owner/repo"`
- `onesearch anysearch-domains [domain]`
- `onesearch anysearch-search "query"`
- `onesearch anysearch-extract "https://example.com/page"`
- `onesearch anysearch-batch "query 1" "query 2"`
- `onesearch anysearch domains [domain]`
- `onesearch anysearch search "query"`
- `onesearch anysearch extract "https://example.com/page"`
- `onesearch anysearch batch "query 1" "query 2"`
- `onesearch deep "research question"`
- `onesearch doctor`
- `onesearch smoke`
- `onesearch config path|list`
- `onesearch model current`
- `onesearch skills list`
- `onesearch skills show NAME`
- `onesearch load_skill NAME`

Supported output formats are `json`, `markdown`, and `content`. JSON is the default and is the stable format for agents and scripts. Search success output defaults to a compact unified result object; use `--verbose` to include full diagnostics such as routing decisions, provider attempts, and capability status. Error output also defaults to `--quiet`. `--quiet` overrides debug defaults.

Provider group commands use:

```text
onesearch <provider> <command> [args] [flags]
```

Provider groups also accept original MCP tool names as aliases, for example:

- `onesearch exa web_search_exa "query"`
- `onesearch exa web_fetch_exa "https://example.com/page"`
- `onesearch tavily tavily_search "query"`
- `onesearch tavily tavily_extract "https://example.com/page"`
- `onesearch tavily tavily_map "https://example.com"`
- `onesearch tavily tavily_crawl "https://example.com"`
- `onesearch context7 resolve_library_id "react"`
- `onesearch context7 query_docs "/facebook/react" "query"`
- `onesearch deepwiki ask_question "owner/repo" "question"`
- `onesearch deepwiki read_wiki_structure "owner/repo"`
- `onesearch deepwiki read_wiki_contents "owner/repo"`

The global MCP compatibility entry is available for mechanical migration scripts:

- `onesearch mcp web_search_exa "query"`
- `onesearch mcp tavily_search "query"`
- `onesearch mcp query_docs "/facebook/react" "query"`
- `onesearch mcp ask_question "owner/repo" "question"`

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
onesearch skills show mcp-tools --format json
onesearch skills show mcp-tools --format content
```

`onesearch load_skill NAME` prints the bundled `SKILL.md` for the requested built-in skill to stdout and returns exit code `0`.

Supported names and aliases:

- `onesearch-cli`, `base`, `onesearch`, `cli`, `router`
- `search`, `web-search`, `source-search`
- `docs`, `api-docs`, `documentation`
- `fetch`, `page-fetch`, `evidence`
- `exa`, `exa-tools`, `web_search_exa`, `web_fetch_exa`
- `tavily`, `tavily-tools`, `tavily_search`, `tavily_extract`, `tavily_map`, `tavily_crawl`
- `firecrawl`, `firecrawl-tools`, `firecrawl_search`, `firecrawl_scrape`, `firecrawl_map`, `firecrawl_crawl`
- `context7`, `context7-tools`, `ctx7`, `resolve_library_id`, `query_docs`
- `deepwiki`, `deepwiki-tools`, `ask_question`, `read_wiki_structure`, `read_wiki_contents`
- `anysearch`, `anysearch-tools`, `as`
- `zhipu`, `zhipu-tools`, `zhipu-search`, `zp`
- `mcp-tools`, `mcp`, `mcp-compat`, `mcp-tool-compat`, `provider-tools`
- `deep-research`, `deep`, `research`

`onesearch load_skill list --format json` is a compatibility alias for `onesearch skills list --format json`.

`load_skill` does not read user provider config, call network providers, write config, install files, or wrap skill content in JSON/Markdown reports unless the `list` compatibility query is used. Unknown names return exit code `2`, write the error and available skill names to stderr, and leave stdout empty.

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

Allowed `steps[].tool` values are `search`, `exa-search`, `exa-similar`, `exa web-search`, `exa web-fetch`, `tavily search`, `tavily extract`, `zhipu-search`, `context7-library`, `context7-docs`, `context7 query-docs`, `fetch`, `map`, `crawl`, `deepwiki ask-question`, and `repo-wiki`.

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
go run .\cmd\onesearch config list --format json
go run .\cmd\onesearch skills list --format json
go run .\cmd\onesearch skills show onesearch-cli --format content
go run .\cmd\onesearch skills show exa --format content
go run .\cmd\onesearch skills show tavily --format content
go run .\cmd\onesearch skills show mcp-tools --format content
go run .\cmd\onesearch load_skill onesearch-cli
go run .\cmd\onesearch load_skill search
go run .\cmd\onesearch load_skill docs
go run .\cmd\onesearch load_skill fetch
go run .\cmd\onesearch load_skill exa
go run .\cmd\onesearch load_skill tavily
go run .\cmd\onesearch load_skill mcp-tools
go run .\cmd\onesearch load_skill deep-research
```

Deep Research examples:

```powershell
onesearch deep "深度搜索一下最近的比特币行情" --format json
onesearch deep "OpenAI Responses API web_search 和 Chat Completions 联网搜索怎么选" --budget deep --format json
onesearch deep "帮我核验这个说法是真是假：某工具已经完全替代 Tavily 做 AI 搜索了" --format json
onesearch deep "https://example.com/source" --format json
```
