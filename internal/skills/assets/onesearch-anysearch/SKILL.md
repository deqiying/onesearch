---
name: onesearch-anysearch
description: Use when an AI agent needs AnySearch through Onesearch for explicit vertical search, domain discovery, AnySearch extraction, or batch AnySearch queries.
---

# Onesearch AnySearch

Use AnySearch only when the user explicitly requests it or the task needs its vertical/experimental domain, search, extraction, or batch surface. It is not part of the default `source_search` route unless runtime config opts in.

## Discover the Contracts

```text
onesearch schema anysearch domains --format json
onesearch schema anysearch search --format json
onesearch schema anysearch extract --format json
onesearch schema anysearch batch --format json
```

Run `onesearch status --format json` and require `status.direct_endpoints.anysearch.available == true` before a direct call.

```text
onesearch anysearch domains --format json
onesearch anysearch search "vertical query" --domain example.com --max-results 5 --format json
onesearch anysearch extract "https://example.com/page" --max-length 20000 --format json
```

Domain/search results are discovery candidates. Extract important pages before claim-level use. Inspect targeted schema before a batch call or provider-specific optional flags, and keep batch size bounded.

## Output and Recovery

Use compact JSON for domain records, candidates, previews, and provider metadata. Use `--format content` for complete extracted text or `--verbose` for full structured data.

On `parameter_error`, inspect the failing AnySearch leaf schema. On `config_error`, run `doctor` and `status`. If AnySearch is unavailable, use a normal available workflow only when it preserves the user's intent; otherwise report the unavailable experimental provider.
