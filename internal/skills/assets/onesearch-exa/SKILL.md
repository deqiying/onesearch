---
name: onesearch-exa
description: Use when an AI agent needs Exa through Onesearch, especially when the user names Exa, semantic web discovery, low-noise source discovery, official docs discovery, product pages, papers, known-domain search, similar-page discovery, or clean page content fetch via the Onesearch CLI.
---

# Onesearch Exa

Use Exa direct commands when the user names Exa or needs semantic discovery, similar pages, known-domain search, papers, product/docs pages, or clean page text. For provider-agnostic synthesis, prefer the `search` workflow.

## Discover the Contracts

```text
onesearch schema exa web-search --format json
onesearch schema exa similar --format json
onesearch schema exa web-fetch --format json
```

Run `onesearch status --format json` and require `status.direct_endpoints.exa.available == true` before a direct call.

```text
onesearch exa web-search "official API documentation" --max-results 5 --format json
onesearch exa similar "https://example.com/article" --num-results 5 --format json
onesearch exa web-fetch "https://example.com/article" --max-characters 12000 --format json
```

Exa `--search-type neural` is a deprecated compatibility alias and is sent upstream as `auto`; current wire types are `auto`, `fast`, `instant`, `deep-lite`, `deep`, and `deep-reasoning`. `exa similar` remains a deprecated compatibility surface.

Use search/similar results for discovery, then fetch key URLs before claim-level use. Restrict to known official domains when provenance matters; inspect the targeted schema before adding domain, date, or content flags.

## Output and Recovery

Compact JSON may expose `content_preview` and `content_length` for long text. Use `--format content` for complete fetched text or `--verbose` for the full structured payload.

On `parameter_error`, inspect the failing Exa leaf schema. On `config_error`, run `doctor` and `status`. If Exa is unavailable, use an available provider for the same capability or report the provider-specific requirement.
