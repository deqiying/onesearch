---
name: onesearch-context7
description: Use when an AI agent needs Context7 through Onesearch, especially when the user names Context7, library resolution, current API docs, SDK docs, framework docs, package documentation, version-specific usage, setup, or migration through the Onesearch CLI.
---

# Onesearch Context7

Use Context7 for focused library, SDK, framework, and package documentation. It is a docs provider, not a general current-news search engine.

## Discover the Contracts

```text
onesearch schema context7 resolve-library-id --format json
onesearch schema context7 query-docs --format json
```

Run `onesearch status --format json` and require `status.direct_endpoints.context7.available == true` before a direct call.

```text
onesearch context7 resolve-library-id "react" --format json
onesearch context7 query-docs "/facebook/react" "useEffect cleanup behavior" --format json
```

Always resolve a library name before querying docs unless a canonical Context7 ID is already known from trusted context. Ask a focused, version-aware question. Verify exact signatures, defaults, and migration claims against the returned snippets and metadata.

## Output and Recovery

Use compact JSON for candidate library IDs, snippets, and metadata. Use `--format content` for the complete selected docs response or `--verbose` for full structured results; do not assume default JSON always contains an unabridged `content` field.

On `parameter_error`, inspect the failing Context7 leaf schema. On `config_error`, run `doctor` and `status`. If Context7 is unavailable or lacks coverage, use another available `docs_search` provider such as Exa and state the change in provenance.
