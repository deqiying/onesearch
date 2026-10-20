# CLI reference

[Documentation hub](../README.md) · [简体中文](cli.md)

This page covers the current CLI command map, output boundaries, and recipes. Query targeted `schema` from the actual binary for complete parameters, constraints, and argv bindings.

## Discovery and contracts

```powershell
onesearch schema --format json
onesearch schema search --format json
onesearch schema exa web-search --format json
onesearch schema --pretty
onesearch search --help
```

`schema` returns a V2 CLI command manifest. Commands expose `name`, `description`, `input_schema`, canonical `path`, argv bindings, constraints, side effects, output contracts, and status preflight. It is neither native tool registration nor runtime configuration schema. Adapters map `input_schema` to platform tool parameters and execute canonical `path`.

V2 replaces V1 `commands[].id` with `name` and `commands[].summary` with `description`; `input_schema` is unchanged.

`schema`, command-level `--help`, and `skills list/show` return before runtime configuration/provider loading. They do not initialize configuration, read credentials, or access the network. Explicit `--output` writes the requested file. Unknown flags, extra positional arguments, and conflicting options for recognized commands, as well as unknown schema canonical paths, return `parameter_error` with exit code `2`.

## Capability map

| Task | Commands | Capability / providers |
| --- | --- | --- |
| Answer search | `search` | `answer_search`: xAI, OpenAI-compatible, OpenAI Responses |
| Source discovery | `search --extra-sources`, provider search | `source_search`: Exa, Zhipu, Tavily, Firecrawl, DeepSeek; DDG/Freecrawl are direct-only by default |
| Documentation | `context7 resolve-library-id/query-docs`, `exa web-search` | `docs_search`: Context7, Exa |
| Page fetch | `fetch --provider`, provider fetch/extract/scrape | `page_fetch`: Tavily, Firecrawl, Exa, AnySearch, DeepSeek; DDG/Freecrawl are direct-only by default |
| Site map | `map --provider`, `tavily map`, `firecrawl map` | `site_map`: Tavily, Firecrawl |
| Site crawl | `crawl --provider`, provider crawl | `site_crawl`: Tavily, Firecrawl; Freecrawl is direct-only by default |
| Repository wiki | `repo-wiki`, `search --repo-wiki` | `repo_wiki`: DeepWiki |
| Vertical search | `anysearch domains/search/extract/batch` | `vertical_search`: AnySearch |
| Offline planning | `deep`, `dr` | Local planner, no provider calls |
| Skill discovery | `skills list/show` | Router, workflow, and provider Skills |
| Diagnostics | `doctor`, `status`, `config list` | Configuration health and local preflight |
| Local self-checks | `smoke`, `regression` | Default / fixed mock; `smoke --live` currently checks only the local minimum profile |
| Downstream MCP server | `mcp` | Local stdio, five tools; no runtime flags / `serve`; see the [MCP guide](../guides/mcp.md) and its release boundary |

Availability depends on local configuration. Run `status` and pass the [appropriate readiness gate](../guides/getting-started.en.md) before execution.

`smoke` defaults to mock and `regression` always uses mock. Currently, `smoke --live` only calls local `doctor`; it does not verify remote connectivity. These commands still load runtime configuration and may initialize configuration files. They do not share static discovery's no-configuration-side-effect guarantee and do not replace `go test` or real provider verification.

## Recipes

PowerShell examples require the corresponding capabilities/providers. Output files are relative to the current directory.

```powershell
# Search with extra source discovery
onesearch search "Important AI news today" --validation balanced --extra-sources 2 --format json

# Select providers, discover candidates, and fetch one
onesearch search "Current official leaderboard" `
  --providers "answer_search=openai_responses;source_search=tavily;page_fetch=firecrawl" `
  --extra-sources 2 --fetch-sources 1 --validation strict --format json

# Fetch a key page
onesearch fetch "https://example.com/article" --provider tavily --format markdown --output evidence.md

# Library documentation
onesearch context7 resolve-library-id "react" "useEffect cleanup" --format json
onesearch context7 query-docs "/facebook/react" "useEffect cleanup" --format json

# Repository context
onesearch repo-wiki "microsoft/playwright" "How is its MCP browser server implemented?" --provider deepwiki --format json

# Offline plan only
onesearch deep "Compare web search in OpenAI Responses and Chat Completions" --budget deep --format json
```

`--fetch-sources N` fetches the first `N` URLs discovered by `source_search`. Pages appear under `used.page_fetch` as `source_evidence`; this does not verify all discovered candidates.

## Provider-direct catalog

For an explicit adapter rather than workflow routing:

```text
onesearch <provider> <command> [args] [--format json|markdown|content] [--pretty]
```

```powershell
onesearch exa web-search "query"
onesearch exa web-fetch "https://example.com"
onesearch exa similar "https://example.com"
onesearch tavily search "query"
onesearch tavily extract "https://example.com"
onesearch tavily map "https://example.com"
onesearch tavily crawl "https://example.com"
onesearch firecrawl search "query"
onesearch firecrawl scrape "https://example.com"
onesearch firecrawl map "https://example.com"
onesearch firecrawl crawl "https://example.com"
onesearch context7 resolve-library-id "react"
onesearch context7 query-docs "/facebook/react" "useEffect cleanup"
onesearch deepwiki ask-question "microsoft/playwright" "How is it structured?"
onesearch deepwiki read-wiki-structure "microsoft/playwright"
onesearch deepwiki read-wiki-contents "microsoft/playwright"
onesearch anysearch search "query"
onesearch zhipu search "query"
onesearch deepseek search "query site:example.com lang:zh,en"
onesearch deepseek fetch "https://example.com"
onesearch ddg search "query"
onesearch ddg fetch-content "https://example.com"
onesearch freecrawl search "query"
onesearch freecrawl scrape "https://example.com"
onesearch freecrawl crawl "https://example.com"
onesearch freecrawl deep-research "topic"
```

Check `direct_endpoints.<provider>.available == true` first. Protocol boundaries:

- Exa uses wire search type `auto`; `neural` is a deprecated compatibility alias.
- Context7 library resolution uses `/api/v2/libs/search` and requires an API key.
- `firecrawl crawl` returns async submission `job_id`/`status`, not completed page content.
- AnySearch domain discovery requires a domain or `--domains`. Extraction unwraps `url`/`title`/`content`, including REST `data` envelopes; `content` and `--format content` return page Markdown rather than the JSON shell.
- DeepSeek is disabled by default. Search and Fetch are billed per successful request (about 5 QPS per API), and the base URL is the API root `https://api.deepseek.com`; full endpoints are rejected. `deepseek search` takes a 1-128 character query and passes through operators such as `site:`, `-site:`, `lang:`, `after:`, and `before:`; `--count` defaults to 10 (1-50) and page bodies are returned only with `--need-content`. `rerank_score` >= 1.5 and `authority` >= 3 only rank candidates and are not evidence. `deepseek fetch` accepts an http(s) URL without userinfo and at most 8192 characters, and returns the title and Markdown content.
- See [configuration](configuration.en.md) for DDG/Freecrawl enablement and version constraints.

## Output and errors

| Option | Meaning |
| --- | --- |
| `--format json` | Compact single-line JSON with a trailing newline by default |
| `--pretty` | Two-space JSON indentation, without changing quiet/verbose payload fields |
| `--format markdown` | Human-readable diagnostics, results, and page bodies |
| `--format content` | Core answer, page body, or short diagnostic summary |
| Default / `--quiet` errors | Concise recovery-oriented fields |
| `--verbose` | Provider attempts, routing decisions, capability state, and detailed diagnostics |
| `--output <path>` | Write the explicitly requested output file |

`search --format json` returns `ok`, `query`, `used`, and `meta`. `used` records capabilities/providers actually executed. Use `content` for full answers and `--verbose` for detailed routing diagnostics.

All dynamic output is credential-redacted, including JSON, Markdown, content, stderr, quiet/verbose/pretty output, and output files. Sensitive values become `********`; there is no debug bypass.

## Evidence and offline planning

Provider sources are candidates, not proof that page bodies were inspected. For news, policy, finance, medicine, serious evaluations, and tool selection, discover URLs and fetch critical pages with `fetch` or `search --fetch-sources N` before making claims.

`deep` only generates a plan containing `intent_signals`, `decomposition`, `capability_plan`, `preflight`, `steps`, and `gap_check`. It does not call providers, access the network, or fetch pages. Agents execute the emitted `command_argv` token arrays and satisfy the default `fetch_before_claim` requirement before treating candidates as verified evidence.

## Implementation and rationale

- [Command contracts](../../internal/commandcontract/), [CLI parsing](../../internal/cli/), [output](../../internal/output/output.go), [offline planning](../../internal/service/deep.go)
- [Manifest V2](../decisions/cli-command-manifest-tool-declaration-alignment-technical-design.md), [JSON formatting](../decisions/cli-json-output-format-technical-design.md) decisions (Chinese)
- [Agent integration and Skill discovery](../guides/agent-integration.en.md)
