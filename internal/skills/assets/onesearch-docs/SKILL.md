---
name: onesearch-docs
description: Use when an AI agent needs Onesearch for API, SDK, package, library, framework, or official documentation lookup with docs_search routing, especially current docs, version-specific usage, setup, migration, Context7 resolution, or Exa official docs discovery through the Onesearch CLI.
---

# Onesearch Docs Skill

Use this skill for documentation and API-reference questions. Run `onesearch doctor --format json` when overall configuration is uncertain. Run `onesearch status --format json` before choosing Context7, Exa, or another docs provider; use only providers listed in `status.capabilities.docs_search.available` or available provider-direct endpoints.

Use this workflow skill when the user asks by intent, such as current API docs, SDK setup, package options, library examples, framework configuration, version-specific behavior, or migration guidance.

Prefer these commands:

```powershell
onesearch status --format json
onesearch context7 resolve-library-id "library name" --format json
onesearch context7 query-docs "/org/project" "specific docs question" --format json
onesearch exa web-search "official docs query" --include-domains docs.example.com --format json
```

Workflow:

- Decide whether the user is asking for docs/API behavior, versioned library usage, SDK setup, migration, or framework configuration.
- For Context7, resolve the library first with `context7 resolve-library-id`, then fetch focused docs with `context7 query-docs`.
- Use `exa web-search` when Context7 does not cover the library, when official docs domains matter, or when the target is a spec, paper, changelog, or product page.
- Do not call a provider-direct docs command unless `status.direct_endpoints.<provider>.available` is true.
- Keep docs search separate from news/current search. Do not present general web results as official documentation.
- Fetch or quote only the relevant docs evidence before making exact API claims.

Provider keys and route order are determined by Onesearch config, not by this skill. `status` is the source of truth for actual docs provider availability.
