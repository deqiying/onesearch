# @deqiying/onesearch

Scoped entry point for the `onesearch` CLI.

```powershell
npm install -g @deqiying/onesearch
onesearch --version
onesearch --help
onesearch tavily search "query" --format json
onesearch exa web-search "query" --format json
onesearch schema --format json
onesearch schema search --format json
onesearch schema --pretty
onesearch status --format json
onesearch skills list --format json
onesearch skills show exa --format content
onesearch skills show onesearch --file references/agent-execution-contract.md --format content
onesearch skills show tavily --format content
```

JSON output is one-line compact by default and keeps a trailing newline. Use `--pretty` for two-space indentation during human inspection; output layout does not switch automatically for TTYs.

`schema` is the static CLI command manifest; runtime configuration remains available through `config list --format json`. Schema queries, command `--help`, and `skills list/show` do not initialize config files or access the network. `skills show` reads `SKILL.md` by default; `--file <relative-path>` reads one bundled file. Its default compact JSON includes metadata, the relative file path, and `content`; use `--format content` for plain text. `schema --output` only writes the explicitly requested manifest file. Invalid flags or extra positional arguments return exit code 2.

This package delegates to the unscoped `onesearch` package.
