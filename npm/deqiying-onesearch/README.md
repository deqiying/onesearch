# @deqiying/onesearch

Scoped entry point for the `onesearch` CLI.

```powershell
npm install -g @deqiying/onesearch
onesearch --version
onesearch --help
onesearch schema --format json
onesearch schema search --format json
onesearch status --format json
```

A fresh configuration is not ready for the full `search` workflow. DeepWiki remains available as an important independent capability for public repository research. When `capabilities.repo_wiki.ok` is `true`, it can run before the full search profile is ready:

```powershell
onesearch repo-wiki "microsoft/playwright" "How is the MCP browser server structured?" --provider deepwiki --format json
```

For full search, configure at least one `answer_search` provider and providers for `docs_search` and `page_fetch`, then verify that `minimum_profile.ok` is `true`. For example:

```powershell
onesearch config setup openai-compatible
onesearch config setup exa
onesearch status --format json
```

After the readiness gate passes:

```powershell
onesearch search "query" --format json
onesearch exa web-search "query" --format json
onesearch schema --pretty
onesearch skills list --format json
onesearch skills show exa --format content
onesearch skills show onesearch --file references/agent-execution-contract.md --format content
onesearch skills show tavily --format content
```

JSON output is one-line compact by default and keeps a trailing newline. Use `--pretty` for two-space indentation during human inspection; output layout does not switch automatically for TTYs.

`schema` is the static V2 CLI command manifest. Command entries use `name`, `description`, and `input_schema`; V1 `id` and `summary` are no longer emitted. Runtime configuration remains available through `config list --format json`. The manifest is a CLI contract, not a native tool registration: adapters map `input_schema` to the target platform and execute the canonical `path`. Schema queries, command `--help`, and `skills list/show` do not initialize config files or access the network. `skills show` reads `SKILL.md` by default; `--file <relative-path>` reads one bundled file. Its default compact JSON includes metadata, the relative file path, and `content`; use `--format content` for plain text. `schema --output` only writes the explicitly requested manifest file. Invalid flags or extra positional arguments return exit code 2.

This package delegates to the unscoped `onesearch` package.

For agent integration, copy the complete host Skill from the repository's [Agent integration section](https://github.com/deqiying/onesearch#connect-onesearch-to-an-agent).

Licensed under the [Apache License 2.0](https://github.com/deqiying/onesearch/blob/main/LICENSE).
