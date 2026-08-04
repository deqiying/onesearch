# Onesearch Agent Execution Contract

This reference defines the stable execution rules shared by the bundled Onesearch skills. Command-specific flags and defaults belong to targeted schema output, not this document.

## Contents

- [Static discovery](#static-discovery)
- [Routing and preflight](#routing-and-preflight)
- [Output contract](#output-contract)
- [Evidence workflow](#evidence-workflow)
- [Errors and recovery](#errors-and-recovery)
- [Provider and credential boundaries](#provider-and-credential-boundaries)
- [Deep Research plans](#deep-research-plans)
- [Retired interfaces](#retired-interfaces)

## Static Discovery

These commands are local and static. They neither initialize config nor access the network:

```text
onesearch schema <canonical-path...> --format json
onesearch skills list --format json
onesearch skills show <skill-id> --format content
onesearch skills show <skill-id> --file <relative-path> --format content
```

Use `skills list` to discover a skill, then load only the selected skill. `skills show --file` reads one file inside that skill's embedded asset directory. Absolute paths, drive paths, backslashes, empty segments, and `.` or `..` traversal are rejected.

Use targeted schema when the leaf is known:

```text
onesearch schema search --format json
onesearch schema firecrawl crawl --format json
```

The manifest reports canonical path, aliases, input JSON Schema, `x-cli-binding`, constraints, availability, side effects, and output variants. A full unscoped schema dump is a development/review tool, not a normal agent step.

## Routing and Preflight

Use workflow commands when the user cares about intent and evidence rather than a named upstream provider:

- `search`: answer, documentation, and source discovery routes.
- `fetch`: one page as evidence.
- `map`: site URL discovery.
- `crawl`: bounded site crawl.
- `repo-wiki`: generated repository context.
- `deep`: offline research planning.

Use a provider-direct leaf only when the user explicitly names that provider or provider-specific behavior is material.

Before any dynamic workflow or direct provider command, inspect:

```text
onesearch status --format json
```

- Workflow readiness: `capabilities.<capability>.ok == true` and `.command`; `available` lists the provider IDs ready for that capability.
- Direct readiness: `direct_endpoints.<provider>.available` and `.commands`.
- Provider metadata: `providers.<provider>`.

Availability is a local preflight. It does not prove remote reachability, accepted credentials, quota, or that a remote MCP tool still exists. Do not infer availability from the presence of a bundled skill.

Run `doctor` when configuration itself is suspect, especially after `config_error`. Do not run it before every request.

## Output Contract

JSON is compact by default: one JSON object on one line plus a trailing LF. Use it for agent control flow and machine inspection.

- `--format content`: complete primary text only.
- `--format markdown`: rendered report when supported.
- `--verbose`: full structured fields when quiet/default JSON contains previews.
- `--pretty`: two-space JSON for explicit human inspection only.
- `--output <path>`: writes exactly the rendered stdout bytes as well as stdout.

Success and failure both use typed envelopes. Common fields include `ok`, `error`, `error_type`, command context, and `elapsed_ms`. Do not scrape human prose when structured fields exist.

Default search JSON is a routing record. Inspect `used.<capability>`, provider results, compact previews, source candidates, and `meta`. Do not assume a top-level full answer. Select `content` or `--verbose` when full text is required.

## Evidence Workflow

Search output identifies candidates; it is not claim proof by itself.

1. Use `source_search` to find likely sources.
2. Prefer first-party, current, and directly relevant pages.
3. Fetch the small set of pages needed for material claims.
4. Distinguish discovery snippets from fetched page evidence.
5. If evidence is missing or contradictory, fetch again, narrow the claim, or state the gap.

`search --fetch-sources N` can fetch the first N discovered URL candidates through `page_fetch`. Use a small bound after the result set is likely to contain authoritative pages.

## Errors and Recovery

Parse errors exit before command execution. Runtime failures return a JSON error envelope whenever JSON is selected.

| Error | Agent action |
| --- | --- |
| `parameter_error` | Read the exact leaf schema, correct argv, retry once |
| `config_error` | Run `doctor` and `status`; repair config or select a ready route |
| `network_error` | Preserve context, retry only when safe, then fall back or report |
| `evidence_error` | Acquire better page evidence or downgrade the claim |
| Provider-specific error | Keep provider/tool context; use an equivalent ready provider only if semantics match |
| Local output/file failure | Correct the local path or permissions; do not treat it as evidence failure |

Exit code alone may group multiple failure classes. Inspect `error_type` before deciding recovery, particularly for exit code 5.

Do not loop indefinitely. One repaired retry or one semantically equivalent provider fallback is normally sufficient before reporting the remaining blocker.

## Provider and Credential Boundaries

Capability-level provider filters select upstreams for different roles. Use targeted schema before supplying optional filters. Prefer explicit filters such as `--source-providers`, `--fetch-providers`, or capability-scoped `--providers` when roles must use different providers.

Never expose configured keys or sensitive environment values in commands, logs, reports, or checked-in files. Onesearch redacts known credentials in JSON, content, markdown, stderr, verbose output, and output files, but agents must still avoid echoing or persisting secrets.

Local MCP providers such as DDG and Freecrawl may require executables or browser assets. Missing dependencies are a prerequisite failure. Do not install them without user authorization.

## Deep Research Plans

`onesearch deep` is an offline planner. It does not execute provider calls.

Read `intent_signals`, `decomposition`, `capability_plan`, `preflight`, `steps`, and `gap_check`. Commands in `preflight[]` and `steps[]` expose machine-executable `command_argv` token arrays. Execute those tokens directly after availability checks and placeholder substitution.

The compatible `command` field is display-only. Never split it on spaces or reinterpret shell quoting.

## Retired Interfaces

- Never call `onesearch mcp <tool>`.
- Never transform canonical hyphenated leaf paths into snake_case tool names.
- Never invent a provider namespace from an upstream MCP tool name.
- Do not rely on retired global flags when the canonical leaf schema provides the supported binding.

When a copied recipe conflicts with targeted schema or live status, schema defines the CLI contract and status defines local readiness.
