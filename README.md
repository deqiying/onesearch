<p align="center">
  <a href="README.md">English</a> · <a href="README.zh-CN.md">简体中文</a>
</p>

<p align="center">
  <img src="./assets/readme/hero.svg" width="100%" alt="onesearch routes one CLI query through capabilities and providers into normalized evidence">
</p>

<p align="center">
  <a href="https://github.com/deqiying/onesearch/actions/workflows/npm-publish.yml"><img src="https://github.com/deqiying/onesearch/actions/workflows/npm-publish.yml/badge.svg" alt="Release workflow status"></a>
  <a href="https://www.npmjs.com/package/onesearch"><img src="https://img.shields.io/npm/v/onesearch?style=flat-square&label=npm" alt="onesearch npm version"></a>
  <img src="https://img.shields.io/badge/Go-1.26-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go 1.26">
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-Apache--2.0-D22128?style=flat-square" alt="Apache License 2.0"></a>
  <a href="https://linux.do/"><img src="./assets/readme/linuxdo-badge.svg" alt="LINUX DO"></a>
</p>

`onesearch` is a CLI-first research and evidence tool for AI agents, scripts, and terminal users. It puts answer search, source discovery, documentation lookup, page fetching, site mapping and crawling, repository wiki access, offline research planning, configuration diagnostics, and built-in Skill distribution behind one reproducible command layer.

> `onesearch` is not an MCP server. AI tools can read its bundled workflows with `skills show`, then execute explicit CLI commands for search, retrieval, and verification.

<p align="center">
  <a href="#quick-start">Quick start</a> ·
  <a href="#capability-map">Capabilities</a> ·
  <a href="#configuration">Configuration</a> ·
  <a href="#connect-onesearch-to-an-agent">Agent integration</a> ·
  <a href="#development">Development</a>
</p>

## Quick start

### 1. Install the CLI

Install the npm launcher and its matching prebuilt binary:

```powershell
npm install -g onesearch
onesearch --version
```

The npm package currently ships binaries for Windows x64, Linux x64, and macOS arm64. Additional Windows, Linux, and macOS archives are published on [GitHub Releases](https://github.com/deqiying/onesearch/releases).

### 2. Inspect the command and initialize configuration

Read the static search contract first. This command is offline and does not create a configuration file:

```powershell
onesearch schema search --format json
```

Now initialize the runtime configuration and inspect local readiness:

```powershell
onesearch status --format json --pretty
```

On a fresh install, `status` is expected to report `ready: false` and `minimum_profile.ok: false`. This is not a failed installation. The readiness report also includes important capabilities that can run independently of the full `search` workflow:

| Capability | Onboarding role | Provider choices / default |
| --- | --- | --- |
| `answer_search` | Required by the `standard` profile | Configure `xai`, `openai-compatible`, or `openai-responses` |
| `docs_search` | Required by the `standard` profile | Configure `exa` or `context7` |
| `page_fetch` | Required by the `standard` profile | Configure `exa`, `tavily`, or `firecrawl`; `anysearch` is initially enabled |
| `repo_wiki` | Important independent capability, ready by default | `deepwiki`; public repositories may work anonymously |
| `vertical_search` | Optional experimental capability, ready by default | `anysearch` |

The initial configuration enables only the anonymous `deepwiki` and `anysearch` endpoints. Answer, documentation, general source-search, and crawl providers remain disabled until you configure them. DeepWiki is not listed as a `standard` requirement because `repo-wiki` has its own readiness gate and can run before the full search profile is ready.

### 3. Use DeepWiki for public repository research

When `capabilities.repo_wiki.ok == true` or `direct_endpoints.deepwiki.available == true`, you can immediately inspect a public GitHub repository without completing the full `standard` profile:

```powershell
onesearch repo-wiki "microsoft/playwright" `
  "How is the MCP browser server structured?" `
  --provider deepwiki `
  --format json
```

DeepWiki provides generated repository context, architecture summaries, wiki structure, and wiki contents. Treat it as secondary context: verify exact implementation claims against the repository source. Private repository access may require configured DeepWiki credentials.

### 4. Configure providers for full search

For the shortest complete setup, configure one answer provider plus Exa. This pair covers `answer_search`, `docs_search`, `page_fetch`, and optional `source_search`:

```powershell
onesearch config setup openai-compatible
onesearch config setup exa
```

The interactive prompts hide API keys. Use `xai` or `openai-responses` instead of `openai-compatible` when that matches your endpoint. For a compatible gateway, provide its API root explicitly:

```powershell
onesearch config setup openai-compatible `
  --base-url "https://gateway.example.com/v1"
```

Run the readiness check again:

```powershell
onesearch status --format json --pretty
```

Before searching, require `minimum_profile.ok == true`. For capability-specific or provider-direct commands, also require `capabilities.<capability>.ok == true` or `direct_endpoints.<provider>.available == true` respectively. These are local preflight signals; they do not prove that the remote endpoint is currently reachable.

### 5. Run the first search

After the readiness gate passes:

```powershell
onesearch search "Explain Go context cancellation patterns" `
  --validation balanced `
  --format json
```

Add `--extra-sources 2` only when `capabilities.source_search.ok == true`.

The command layer stays explicit:

```text
task → capability route → provider selection / fallback → normalized result
```

## Why onesearch

- **One contract for many research operations.** Use workflow commands for common tasks or provider-direct commands when you need exact control.
- **Designed for agents and scripts.** Versioned command schemas, strict argument parsing, compact JSON, stable exit semantics, and bundled Skills make commands discoverable without guesswork.
- **Evidence is separate from discovery.** Candidate URLs do not automatically count as verified page content; fetch important sources before making claims.
- **Credentials stay out of routine output.** Dynamic output is redacted across JSON, Markdown, content, stderr, and output files.
- **Local planning remains local.** `deep` creates an offline research plan without calling providers or fetching pages.

## Capability map

| Task | Commands | Routing / providers |
| --- | --- | --- |
| Answer search | `search` | `answer_search`: xAI, OpenAI-compatible, OpenAI Responses |
| Source discovery | `search --extra-sources`, `exa web-search`, `zhipu search`, `ddg search`, `freecrawl search` | `source_search`: Exa, Zhipu, Tavily, Firecrawl; DDG and Freecrawl are direct-only by default |
| Documentation | `context7 resolve-library-id`, `context7 query-docs`, `exa web-search` | `docs_search`: Context7, Exa |
| Page fetch | `fetch --provider`, `exa web-fetch`, `tavily extract`, `firecrawl scrape`, `ddg fetch-content`, `freecrawl scrape` | `page_fetch`: Tavily, Firecrawl, Exa; DDG and Freecrawl are direct-only by default |
| Site map | `map --provider`, `tavily map`, `firecrawl map` | `site_map`: Tavily, Firecrawl |
| Site crawl | `crawl --provider`, `tavily crawl`, `firecrawl crawl`, `freecrawl crawl` | `site_crawl`: Tavily, Firecrawl; Freecrawl is direct-only by default |
| Repository wiki | `repo-wiki`, `search --repo-wiki` | `repo_wiki`: DeepWiki |
| Vertical search | `anysearch domains/search/extract/batch` | `vertical_search`: AnySearch |
| Research planning | `deep`, `dr` | Local offline planner |
| Skill discovery | `skills list`, `skills show` | Bundled router, workflow, and provider Skills |
| Diagnostics | `doctor`, `status`, `config list`, `smoke`, `regression` | Configuration health, live preflight, and public regression checks |
| CLI discovery | `schema`, command-level `--help` | Versioned CLI manifest and human-readable help |

Actual availability is configuration-dependent. Run `onesearch status --format json` before selecting a provider or capability.

## Configuration

The default configuration path on Windows, macOS, and Linux is:

```text
~/.config/onesearch/config.json
```

Set `ONESEARCH_CONFIG_DIR` to use another directory. On the first ordinary command, `onesearch` creates the configuration directory and initial `config.json` if they do not exist, then continues the command. The initial schema enables only `deepwiki` and `anysearch`; `xai`, both OpenAI adapters, Exa, Context7, Zhipu, Tavily, Firecrawl, DDG, and Freecrawl start disabled. A normal `search` therefore requires provider setup before first use.

The runtime configuration has five top-level areas:

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

- `defaults` — pipeline, fallback, validation, minimum profile, timeout, logging, retry, and output-cleaning defaults.
- `pipelines` — capability chains such as `default`, `research`, `docs`, and `crawl`.
- `routes` — ordered providers for each capability. Providers that declare a capability are appended automatically unless `settings.direct_only=true`.
- `profiles` — minimum capability sets; for example, `standard` requires `answer_search`, `docs_search`, and `page_fetch`.
- `providers` — adapter, capabilities, base URL, credential source, enabled state, and provider-specific settings.

See [config.example.json](config.example.json) for a complete portable example.

### Configure a provider safely

```powershell
onesearch config path --format json
onesearch config list --format json
onesearch config setup exa
onesearch doctor --format json
onesearch status --format json
```

Interactive setup hides the API key. Scripts and pipelines must pass secrets through stdin; there is intentionally no `--api-key` flag that would leave a key in shell history:

```powershell
$env:EXA_API_KEY |
  onesearch config setup exa --api-key-stdin --format json

$env:OPENAI_COMPATIBLE_API_KEY |
  onesearch config setup openai-compatible `
    --api-key-stdin `
    --base-url "https://gateway.example.com/v1" `
    --format json
```

`config setup` updates the key and sets the provider to `enabled: "auto"`. Edit `config.json` directly for models, capabilities, routes, provider settings, or disabling a provider. A direct `api_key` takes precedence over `api_key_env` when both exist.

### OpenAI-compatible adapters

- `openai_responses` targets the JSON-only standalone SearchRequest route at `/v1/alpha/search`.
- `openai_chat_completions` targets `/v1/chat/completions` and supports JSON/SSE plus optional `tools` and `tool_choice` passthrough.
- The two adapters do not downgrade into each other. An Alpha failure enters the normal cross-provider fallback path.
- `openai_responses` expects an API root base URL. Full endpoints ending in `/alpha/search` or `/responses` are rejected as parameter errors.

## Common recipes

```powershell
# Search with answer and extra source discovery
onesearch search "Important AI news today" --validation balanced --extra-sources 2 --format json

# Constrain each capability to selected providers
onesearch search "Current official leaderboard" `
  --providers "answer_search=openai_responses;source_search=tavily;page_fetch=firecrawl" `
  --fetch-sources 1 `
  --validation strict `
  --format json

# Fetch a page as Markdown
onesearch fetch "https://example.com/article" --provider tavily --format markdown --output evidence.md

# Look up current library documentation
onesearch context7 resolve-library-id "react" "useEffect cleanup" --format json
onesearch context7 query-docs "/facebook/react" "useEffect cleanup" --format json

# Ask about a public repository through DeepWiki
onesearch repo-wiki "microsoft/playwright" "How is its MCP browser server implemented?" --provider deepwiki --format json

# Generate an offline research plan
onesearch deep "Compare web search in OpenAI Responses and Chat Completions" --budget deep --format json
```

When `--fetch-sources N` is set, `search` fetches the first `N` URLs discovered by `source_search`. The retrieved pages appear under `used.page_fetch` with the role `source_evidence`.

## Connect onesearch to an agent

After installing the CLI and completing the [readiness setup](#quick-start), create a custom Skill folder named `onesearch` in the Skills directory supported by your agent host. Save the following content as `SKILL.md`, then refresh or restart the host if it requires Skill rediscovery.

This host Skill is intentionally thin: it resolves the installed executable and loads the version-matched workflow embedded in that same `onesearch` binary. It does not copy provider contracts into the agent, configure endpoints, or grant permission for network and environment changes.

````markdown
---
name: onesearch
description: Use when an agent needs to search the web, look up current or latest public information such as news, prices, rankings, hot searches, or trending topics, verify claims with online sources, read a URL, map or crawl a website, find official API/SDK/package/framework docs, or inspect public GitHub repo docs and architecture.
---

# Onesearch CLI Entry

Use this Skill only as the host-level entry for the installed `onesearch` CLI. Treat the main Skill embedded in the resolved CLI binary as the sole versioned Onesearch workflow. Do not duplicate or infer provider inventories, command flags, defaults, output contracts, or recovery behavior here.

## Load the version-matched workflow

1. Resolve the actual executable and record its version. On Windows, use `Get-Command onesearch -All`; on macOS or Linux, use `type -a onesearch` when available or `command -v onesearch`. Then run `onesearch --version`.
2. Read the canonical main Skill from that same executable:

```text
onesearch skills show onesearch --format content
```

3. Read the complete stdout and follow it as the authoritative workflow for the current executable. Do not merge it with remembered or repository-copied Onesearch instructions.
4. Reuse the loaded main Skill for the rest of the task while the executable path and version stay unchanged. Do not reload it recursively.
5. When the loaded router selects a child workflow or provider, load only that child with its advertised canonical `onesearch skills show <skill-id> --format content` command. Treat the returned Markdown as workflow guidance; it is not a newly registered host Skill.

## Preserve host boundaries

- Call `skills show` without `--output`; loading embedded guidance must not create files or initialize provider configuration.
- Loading a Skill does not authorize network calls, credential reads, configuration writes, package installation, or other side effects. Follow the user's existing authorization boundary and the loaded Skill's narrower preflight rules.
- Never silently install or update `onesearch`, provider runtimes, browser dependencies, or global packages. Request authorization before changing the environment.
- If the CLI is missing and the user wants it installed, verify Node/npm first and request authorization before running `npm install -g onesearch`.
- Never expose credential values in commands, logs, generated files, or responses.

## Recover without stale guidance

If the CLI is missing or the canonical `skills show` command fails, report the resolved path, version when available, exit status, and concise error. Do not guess aliases or flags, silently upgrade the CLI, or fall back to a copied CLI contract. Use an available purpose-built host search capability when it is allowed and preserves the task semantics; otherwise report the missing prerequisite.
````

## Agent-facing contracts

`onesearch schema` returns the V2 CLI command manifest. Each command exposes the Agent-friendly core fields `name`, `description`, and `input_schema`, followed by its canonical CLI `path`, argv bindings, constraints, side effects, output contract, and status preflight. It is not a native tool registration or the runtime configuration schema; adapters still map `input_schema` to a platform's tool parameters and execute the canonical `path`.

V2 intentionally replaces V1 `commands[].id` with `commands[].name` and `commands[].summary` with `commands[].description`. `commands[].input_schema` is unchanged.

```powershell
onesearch schema --format json
onesearch schema search --format json
onesearch schema exa web-search --format json
onesearch schema --pretty
onesearch search --help
```

`schema`, command-level `--help`, and `skills list/show` run before runtime configuration and provider loading. They do not create `config.json`, read credentials, or access the network. Unknown flags, extra positional arguments, conflicting options, and unknown canonical paths return a `parameter_error` with exit code `2`.

### Output contract

- `json` — compact, one-line JSON with a trailing newline by default; add `--pretty` for two-space indentation.
- `markdown` — readable diagnostics, results, and fetched content.
- `content` — the core answer, page body, or short diagnostic summary.
- default / `--quiet` errors — concise recovery-oriented fields.
- `--verbose` — provider attempts, routing decisions, capability state, and other detailed diagnostics.

`search --format json` returns `ok`, `query`, `used`, and `meta`. The `used` tree records the capabilities and providers that actually ran. Use `--format content` for the full answer body, or `--verbose` for detailed routing and provider diagnostics.

All dynamic output passes through credential redaction, including JSON, Markdown, content, stderr, `--quiet`, `--verbose`, `--pretty`, and `--output` files. Sensitive values are replaced with `********`; there is no debug switch that disables redaction.

## Provider-direct commands

Use this form when you need a specific adapter instead of workflow routing:

```text
onesearch <provider> <command> [args] [--format json|markdown|content] [--pretty]
```

<details>
<summary><strong>Show the provider command catalog</strong></summary>

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
onesearch ddg search "query"
onesearch ddg fetch-content "https://example.com"
onesearch freecrawl search "query"
onesearch freecrawl scrape "https://example.com"
onesearch freecrawl crawl "https://example.com"
onesearch freecrawl deep-research "topic"
```

Protocol notes: Exa uses the current `auto` wire search type (`neural` is a deprecated alias), Context7 library resolution uses `/api/v2/libs/search` and requires an API key, and `firecrawl crawl` returns an asynchronous job (`job_id`/`status`) rather than completed page content. AnySearch domain discovery requires an explicit domain or `--domains`; Freecrawl MCP deployments should pin a package revision and rely on runtime tool discovery.

</details>

Run `status` first and verify `direct_endpoints.<provider>.available`. DDG and Freecrawl use a local `mcp_stdio` bridge and are disabled plus direct-only by default, so they do not change the normal workflow routes unless you explicitly enable and route them.

## Built-in Skills

The bundled `onesearch` Skill is a thin router. Workflow Skills cover `search`, `docs`, `fetch`, and deep research; provider Skills describe the exact commands for Exa, Tavily, Firecrawl, Context7, DeepWiki, AnySearch, Zhipu, DDG, and Freecrawl.

```powershell
onesearch skills list --format json
onesearch skills list --capability page_fetch --format json
onesearch skills show onesearch --format content
onesearch skills show onesearch --file references/agent-execution-contract.md --format content
onesearch skills show exa --format content
```

`skills show` reads `SKILL.md` by default. `--file <relative-path>` reads one bundled file from the selected Skill. Except for an explicit `--output`, Skill discovery is read-only and offline.

## Evidence strategy

Provider sources returned by `search` are discovery candidates, not proof that their page content has been checked. For news, policy, finance, medical information, serious evaluations, and tool selection, discover the URLs first and then fetch the critical pages with `fetch` or `search --fetch-sources N`. Base final claims on the retrieved content.

`onesearch deep` follows the same boundary. It produces a plan with `intent_signals`, `decomposition`, `capability_plan`, `preflight`, `steps`, and `gap_check`, but does not call providers, access the network, or fetch pages. Agents should execute the emitted `command_argv` token arrays and satisfy the default `fetch_before_claim` requirement before treating a candidate as verified.

## Development

The repository pins Go through both `go.mod` and `mise.toml`.

```powershell
mise install
mise exec -- go test ./...
mise exec -- go build -o .\bin\onesearch.exe .\cmd\onesearch
.\bin\onesearch.exe --version
```

`mise.toml` keeps Go module and build caches under the repository. If you run Go outside mise and the system cache is not writable, use a project-local cache:

```powershell
$env:GOCACHE = Join-Path (Get-Location) '.gocache'
go test ./...
```

### Project layout

```text
cmd/onesearch/          CLI entry point
internal/cli/           Parsing, flags, command routing, and exit codes
internal/config/        Runtime schema, providers, defaults, and redaction
internal/service/       Capability routing, fallback, diagnostics, and planning
internal/providers/     Provider adapters and result normalization
internal/sources/       Source parsing, deduplication, and result roles
internal/output/        JSON, Markdown, and content renderers
internal/skills/        Bundled Skills and their assets
npm/                    npm launchers and platform packages
```

### Releases

Pushing a `v*.*.*` tag runs the shared test suite, builds GitHub Release archives, creates checksums, and publishes the npm entry and platform packages. Release binaries cover Windows, Linux, and macOS on the architectures declared in the workflow.

## License

Licensed under the [Apache License 2.0](LICENSE).
