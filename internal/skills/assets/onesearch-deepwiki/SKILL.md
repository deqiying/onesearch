---
name: onesearch-deepwiki
description: Use when an AI agent needs DeepWiki through Onesearch, especially when the user names DeepWiki, public GitHub repository architecture, module summaries, implementation overviews, generated repository wiki context, wiki structure, or wiki contents through the Onesearch CLI.
---

# Onesearch DeepWiki

Use DeepWiki for generated context about a public GitHub repository: architecture questions, wiki structure, module summaries, and wiki contents. Prefer source-code inspection when an exact implementation claim must be verified.

## Discover the Contracts

```text
onesearch schema deepwiki ask-question --format json
onesearch schema deepwiki read-wiki-contents --format json
onesearch schema deepwiki read-wiki-structure --format json
```

Run `onesearch status --format json` and require `status.direct_endpoints.deepwiki.available == true` before a direct call.

```text
onesearch deepwiki ask-question "owner/repo" "How is the service structured?" --format json
onesearch deepwiki read-wiki-structure "owner/repo" --format json
onesearch deepwiki read-wiki-contents "https://github.com/owner/repo" --format json
```

Accept `owner/repo` or a supported GitHub URL as defined by targeted schema. Public repositories may work anonymously; private repository access can require configured credentials. Treat generated wiki text as secondary context and verify code-level assertions locally or against repository sources.

## Output and Recovery

Compact JSON may include a preview and original length. Use `--format content` for complete wiki text or `--verbose` for full structured output.

On `parameter_error`, inspect the failing DeepWiki leaf schema. On `config_error`, run `doctor` and `status`. If `repo_wiki` is unavailable, report that boundary rather than fabricating generated repository context.
