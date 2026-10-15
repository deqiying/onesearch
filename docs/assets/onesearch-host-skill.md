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
