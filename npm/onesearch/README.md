# onesearch

`onesearch` is a CLI-first multi-source web research tool for AI agents and terminal users.

This npm package installs a small JavaScript launcher and downloads the matching prebuilt binary for your platform through optional dependencies.

```powershell
npm install -g onesearch
onesearch --version
onesearch --help
onesearch schema --format json
onesearch schema search --format json
onesearch status --format json
```

A fresh configuration is not ready for the full `search` workflow. Its default `standard` profile requires `answer_search`, `docs_search`, and `page_fetch`.

DeepWiki is an important independent capability that is enabled by default for public repository research. When `capabilities.repo_wiki.ok` is `true`, it can run before the full search profile is ready:

```powershell
onesearch repo-wiki "microsoft/playwright" "How is the MCP browser server structured?" --provider deepwiki --format json
```

For full search, configure at least one answer provider plus providers for documentation and page fetching, then run `status` again. For example, `openai-compatible` plus Exa covers the required capabilities:

```powershell
onesearch config setup openai-compatible
onesearch config setup exa
onesearch status --format json
```

Only continue when `minimum_profile.ok` is `true`:

```powershell
onesearch search "query" --format json
onesearch search --help
onesearch exa web-search "query" --format json
onesearch fetch "https://example.com" --provider exa --format json
onesearch schema exa web-search --format json
onesearch schema --pretty
onesearch skills list --format json
onesearch skills show exa --format content
onesearch skills show onesearch --file references/agent-execution-contract.md --format content
onesearch skills show tavily --format content
```

## Agent integration

Create a custom Skill folder named `onesearch` in the Skills directory supported by your agent host and save the following content as `SKILL.md`:

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

JSON output is one-line compact by default and keeps a trailing newline. Use `--pretty` for two-space indentation during human inspection; output layout does not switch automatically for TTYs, and `--pretty` does not change quiet/verbose payload fields.

`schema` returns the versioned CLI command manifest for agents and scripts; `config list --format json` remains the runtime configuration schema. Schema queries, every command-level `--help`, and `skills list/show` are static discovery paths: they return before runtime config/provider loading, do not initialize config files, and do not access the network. `skills show` reads `SKILL.md` by default; `--file <relative-path>` reads one bundled file. Its default compact JSON includes metadata, the relative file path, and `content`; use `--format content` for plain text. `schema --output` only writes the explicitly requested manifest file. Unknown flags or extra positional arguments are parameter errors (exit code 2).

Supported npm binary packages:

- `@deqiying/onesearch-win32-x64`
- `@deqiying/onesearch-linux-x64`
- `@deqiying/onesearch-darwin-arm64`

Licensed under the [Apache License 2.0](https://github.com/deqiying/onesearch/blob/main/LICENSE).
