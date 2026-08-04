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

```text
onesearch skills list --capability source_search --format json
onesearch skills show search --format content
onesearch schema search --format json
```

Load a child skill only after routing. Do not call `skills show` again from an already loaded child skill.

## Agent Execution Loop

1. Classify the intent and load the most specific skill.
2. If a command's exact path, required fields, flags, defaults, or side effects are uncertain, inspect only its targeted schema. Do not dump the full schema during normal agent work.
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

## Boundaries

- Use canonical leaf commands such as `onesearch exa web-search`; never use the retired `onesearch mcp <tool>` bridge.
- Do not invent flat provider commands, snake_case command names, or retired top-level flags.
- Execute Deep Research `command_argv` token arrays directly. Never reconstruct argv by splitting the display-only `command` string.
- Do not print, persist, or quote credential values. `status` and `doctor` expose metadata only; pass secrets through supported environment/config mechanisms.
- Do not silently install local MCP runtimes, browser binaries, or other dependencies. Report the missing prerequisite and request authorization.

For the complete shared contract, read only the bundled reference file:

```text
onesearch skills show onesearch --file references/agent-execution-contract.md --format content
```
