---
name: onesearch-docs
description: Use when an AI agent needs Onesearch for API, SDK, package, library, framework, or official documentation lookup with docs_search routing, especially current docs, version-specific usage, setup, migration, Context7 resolution, query_docs, or Exa official docs discovery through the Onesearch CLI.
---

# Onesearch Docs Skill

Use this skill for documentation and API-reference questions. Run `onesearch doctor --format json` first when configuration is uncertain.

## Bridge Contract

For documentation tasks, prefer provider-specific bridge docs when the user names a provider or original MCP tool:

- Context7, `resolve_library_id`, or `query_docs`: load `onesearch skills show context7 --format content`.
- Exa, `web_search_exa`, or official docs page discovery: load `onesearch skills show exa --format content`.

Use this workflow skill when the user asks by intent, such as current API docs, SDK setup, package options, library examples, framework configuration, version-specific behavior, or migration guidance.

Prefer these commands:

```powershell
onesearch context7 resolve-library-id "library name" --format json
onesearch context7 query-docs "/org/project" "specific docs question" --format json
onesearch exa web-search "official docs query" --include-domains docs.example.com --format json
onesearch context7-library "library name" "task or API question" --format json
onesearch context7-docs "/org/project" "specific docs question" --format json
onesearch exa-search "official docs query" --include-domains docs.example.com --format json
```

Workflow:

- Decide whether the user is asking for docs/API behavior, versioned library usage, SDK setup, migration, or framework configuration.
- For Context7, resolve the library first with `context7 resolve-library-id`, then fetch focused docs with `context7 query-docs`. `context7-library` and `context7-docs` remain supported as legacy flat commands.
- Use `exa web-search` when Context7 does not cover the library, when official docs domains matter, or when the target is a spec, paper, changelog, or product page. `exa-search` remains supported as the legacy flat command.
- Keep docs search separate from news/current search. Do not present general web results as official documentation.
- Fetch or quote only the relevant docs evidence before making exact API claims.

Provider keys and route order are determined by Onesearch config, not by this skill.
