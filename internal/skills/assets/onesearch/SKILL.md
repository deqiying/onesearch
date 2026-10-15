---
name: onesearch
description: Use when you need to search the web, look up current or latest public information such as news, prices, rankings, hot searches, or trending topics, verify claims with online sources, read a URL, map or crawl a website, find official API/SDK/package/framework docs, or inspect public GitHub repo docs and architecture.
---

# Onesearch Router

Use this router to choose the smallest Onesearch workflow or provider skill that matches the task. Do not load every child skill.

## Route the Task

| Intent | Start with | Load when needed |
| --- | --- | --- |
| Current facts, news, prices, rankings, hot lists, or source discovery | `onesearch-search` | A named provider skill only when the user or route requires it |
| API, SDK, package, framework, or official documentation | `onesearch-docs` | `onesearch-context7` or `onesearch-exa` |
| Read a URL or turn a candidate into evidence | `onesearch-fetch` | The chosen fetch provider skill |
| Discover site structure | `onesearch-fetch` with `map` | `onesearch-tavily` or `onesearch-firecrawl` |
| Bounded site crawl | `onesearch-fetch` with `crawl` | `onesearch-tavily`, `onesearch-firecrawl`, or direct-only `onesearch-freecrawl` |
| Public repository architecture or generated wiki context | `onesearch-deepwiki` | — |
| Complex multi-step research planning | `onesearch-deep-research` | Skills named by the returned plan |
| Explicit vertical or experimental search | `onesearch-anysearch` | — |
| Explicit Chinese current search | `onesearch-zhipu` | — |
| Explicit local DuckDuckGo or browser-backed MCP task | `onesearch-ddg` or `onesearch-freecrawl` | — |

`skills list`, `skills show`, and targeted `schema` are static discovery commands: they do not initialize config or access the network.

Schema V2 command entries use `name`, `description`, and `input_schema`. The manifest is a CLI contract, not a native tool registration: use `path` plus `x-cli-binding` to build argv tokens, and do not derive CLI tokens by splitting `name`.

```text
onesearch skills list --capability source_search --format json
onesearch skills show search --format content
onesearch schema search --format json
```

Load a child skill only after routing. Do not call `skills show` again from an already loaded child skill.

## Agent Execution Loop

1. Classify the intent and load the most specific skill.
2. If a command's exact path, description, required fields, flags, defaults, or side effects are uncertain, inspect only its targeted schema. Do not dump the full schema during normal agent work.
3. Before a dynamic workflow or provider-direct call, run `onesearch status --format json`. A workflow is locally ready only when `capabilities.<capability>.ok == true`; choose from its `available` provider IDs. A direct endpoint is locally ready when `direct_endpoints.<provider>.available == true`. Neither check proves network or credential validity.
4. Execute the narrowest command with compact JSON.
5. Treat search results as discovery candidates. Fetch the few pages that support material claims.
6. Return sourced conclusions and state any remaining evidence gap.

Use `onesearch doctor --format json` only for configuration diagnosis, normally after `config_error`; it is not a mandatory first step.

## Output Selection

- Use default compact `--format json` for routing, automation, errors, and structured results.
- Use `--format content` when only the complete primary text is needed; use `--format markdown` for a rendered report.
- Default JSON may replace long text with `content_preview` and `content_length`. Add `--verbose` only when the full structured payload is required.
- `--pretty` is for explicit human inspection, not agent recipes. `--output` must match stdout bytes.

## Recovery

Read the JSON error envelope before changing course.

| Signal | Recovery |
| --- | --- |
| Parse failure or `parameter_error` | Read the failing leaf's targeted schema, repair argv, and retry once |
| `config_error` | Run `doctor` and `status`; report or select an actually available route |
| `network_error` | Retry only when safe, then use an available provider or report the outage |
| `evidence_error` | Fetch better sources or narrow the claim |
| Local output/file error | Correct the path or permissions; do not misclassify it as evidence failure |
| Provider-specific failure | Preserve provider context and try an allowed fallback only when semantics remain valid |

## Native MCP Mode

When a host already exposes the Onesearch MCP tools, use their `tools/list` schemas instead of converting CLI flags or inventing tool names. The source implementation starts with `onesearch mcp` (no runtime flags or `serve` subcommand); npm manifest version `1.0.6` does not include it. Before release use a locally built binary; after a release containing MCP, `npx -y onesearch mcp` is a launcher entry. Pin an actually published version for production, and follow the host's Windows `npx.cmd` / `cmd` rules; real-host acceptance remains pending.

- `onesearch_search` requires `mode` and `query`: `answer` uses configured routes and the minimum profile; `sources` uses Exa; `docs` uses Context7 with `docs_options`. `library_name` only resolves candidates; select a returned `library_id` and call again to query. Never guess an ID or combine options from different modes.
- `onesearch_fetch` fetches one URL. `onesearch_site` requires `action=map/crawl`; `map_options` are only valid for map. A submitted crawl job is not completed evidence.
- `onesearch_repo_wiki` requires `mode=ask/structure/contents`; `ask` requires `question`, other modes forbid it.
- Call `onesearch_status` with `{}` before dynamic work. Inspect `data.tools[name].operations[operation].available`; `available` means any branch, `fully_available` means every branch, and overall `data.ready` only covers the minimum profile. No check proves remote access or quota.
- Discovery does not read configuration, create configuration, or access the network. Each tool request uses a read-only snapshot; repair configuration outside MCP. Network calls can incur fees or launch configured local provider processes; this does not authorize installs or configuration changes.
- Limits are 64 KiB per input frame, 2 MiB per complete response, and four concurrent calls. Handle `result_too_large` by narrowing the request, `server_busy` by waiting, and `timeout` by reporting or safely narrowing/retrying. Timeout seconds satisfy `0 < timeout <= 300`; site accepts integer, other timeout-enabled tools accept number. Defaults: search/fetch 90, map 150, crawl 180, repo wiki 60 seconds.
- Only stdio/tools are exposed, not HTTP, resources, prompts, tasks, OAuth, arbitrary shell, or file-writing tools. Modern protocol `2026-07-28` has no ping (`-32601`); use `tools/list` for liveness. Also supported: `2025-11-25`, `2025-06-18`, and text-only `2025-03-26`. SDK is pinned to `v1.8.0`.

The CLI execution/output conventions elsewhere in this Skill apply to CLI calls, not MCP transport. MCP returns its own redacted result envelope; do not apply CLI quiet/verbose rendering rules to it.

## Boundaries

- Use canonical leaf commands such as `onesearch exa web-search`; never use the retired `onesearch mcp <tool>` bridge. The new standalone `onesearch mcp` starts a downstream server, not a provider command router.
- Do not invent flat provider commands, snake_case command names, or retired top-level flags.
- Execute Deep Research `command_argv` token arrays directly. Never reconstruct argv by splitting the display-only `command` string.
- Do not print, persist, or quote credential values. `status` and `doctor` expose metadata only; pass secrets through supported environment/config mechanisms.
- Do not silently install local MCP runtimes, browser binaries, or other dependencies. Report the missing prerequisite and request authorization.

For the complete shared contract, read only the bundled reference file:

```text
onesearch skills show onesearch --file references/agent-execution-contract.md --format content
```
