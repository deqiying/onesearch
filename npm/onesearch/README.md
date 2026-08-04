# onesearch

`onesearch` is a CLI-first multi-source web research tool for AI agents and terminal users.

This npm package installs a small JavaScript launcher and downloads the matching prebuilt binary for your platform through optional dependencies.

```powershell
npm install -g onesearch
onesearch --version
onesearch --help
onesearch search "query" --format json
onesearch search --help
onesearch tavily search "query" --format json
onesearch exa web-search "query" --format json
onesearch fetch "https://example.com" --provider exa --format json
onesearch schema --format json
onesearch schema exa web-search --format json
onesearch schema --pretty
onesearch status --format json
onesearch skills list --format json
onesearch skills show exa --format content
onesearch skills show onesearch --file references/agent-execution-contract.md --format content
onesearch skills show tavily --format content
```

JSON output is one-line compact by default and keeps a trailing newline. Use `--pretty` for two-space indentation during human inspection; output layout does not switch automatically for TTYs, and `--pretty` does not change quiet/verbose payload fields.

`schema` returns the versioned CLI command manifest for agents and scripts; `config list --format json` remains the runtime configuration schema. Schema queries, every command-level `--help`, and `skills list/show` are static discovery paths: they return before runtime config/provider loading, do not initialize config files, and do not access the network. `skills show` reads `SKILL.md` by default; `--file <relative-path>` reads one bundled file. Its default compact JSON includes metadata, the relative file path, and `content`; use `--format content` for plain text. `schema --output` only writes the explicitly requested manifest file. Unknown flags or extra positional arguments are parameter errors (exit code 2).

Supported npm binary packages:

- `@deqiying/onesearch-win32-x64`
- `@deqiying/onesearch-linux-x64`
- `@deqiying/onesearch-darwin-arm64`
