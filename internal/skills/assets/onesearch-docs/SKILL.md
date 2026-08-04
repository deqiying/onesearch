---
name: onesearch-docs
description: Use when an AI agent needs Onesearch for API, SDK, package, library, framework, or official documentation lookup with docs_search routing, especially current docs, version-specific usage, setup, migration, Context7 resolution, or Exa official docs discovery through the Onesearch CLI.
---

# Onesearch Docs

Use this workflow for exact API behavior, SDK setup, version-specific examples, migrations, and official package or framework documentation. Do not route ordinary news or general current-event questions here.

## Discover the Contracts

```text
onesearch schema context7 resolve-library-id --format json
onesearch schema context7 query-docs --format json
onesearch schema exa web-search --format json
```

## Workflow

1. Run `onesearch status --format json`. Use a direct provider only when its `direct_endpoints` entry is available.
2. Prefer Context7 for covered libraries: resolve the library ID first, then ask one focused question.
3. Use Exa when Context7 lacks coverage or when an official docs domain, spec, changelog, paper, or product page is the better source.
4. Verify exact signatures, defaults, version behavior, and migration claims against returned documentation evidence.

```text
onesearch context7 resolve-library-id "library name" --format json
onesearch context7 query-docs "/org/project" "focused API question" --format json
onesearch exa web-search "official product API documentation" --include-domains docs.example.com --format json
```

Never guess a Context7 library ID. Resolve it and preserve the returned canonical identifier. General web results are not automatically official documentation.

## Output and Recovery

Use compact JSON to inspect library IDs, snippets, URLs, and provider metadata. Use `--format content` for the complete selected documentation text or `--verbose` when full structured content is required.

On `parameter_error`, inspect the failing leaf schema. On `config_error`, run `doctor` and `status`. If Context7 is unavailable or lacks coverage, use an available docs provider such as Exa and state the source boundary.
