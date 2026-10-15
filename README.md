<p align="center">
  <a href="README.md">English</a> · <a href="README.zh-CN.md">简体中文</a>
</p>

<p align="center">
  <img src="./assets/readme/hero.svg" width="100%" alt="onesearch routes one CLI query through capabilities and providers into normalized evidence">
</p>

<p align="center">
  <a href="https://github.com/deqiying/onesearch/actions/workflows/npm-publish.yml"><img src="https://github.com/deqiying/onesearch/actions/workflows/npm-publish.yml/badge.svg" alt="Release workflow status"></a>
  <a href="https://www.npmjs.com/package/onesearch"><img src="https://img.shields.io/npm/v/onesearch?style=flat-square&label=npm" alt="onesearch npm version"></a>
  <img src="https://img.shields.io/badge/Go-1.26-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go 1.26">
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-Apache--2.0-D22128?style=flat-square" alt="Apache License 2.0"></a>
  <a href="https://linux.do/"><img src="./assets/readme/linuxdo-badge.svg" alt="LINUX DO"></a>
</p>

`onesearch` is a CLI-first research and evidence tool for AI agents, scripts, and terminal users. It unifies answer search, source discovery, documentation lookup, page fetching, site mapping and crawling, repository wiki access, offline research planning, configuration diagnostics, and built-in Skill distribution.

> `onesearch` is not an MCP server. AI tools read bundled workflows with `skills show`, then execute explicit CLI commands.

## Why onesearch

- **Explicit routing**: task → capability route → provider selection / fallback → normalized result.
- **Agent and script contracts**: versioned command manifests, strict parsing, compact JSON, and bundled Skills.
- **Discovery is not evidence**: fetch important sources before treating candidate URLs as verified content.
- **Credential redaction**: dynamic output and output files are redacted; `deep` plans locally without provider calls.

## Quick start

Examples use PowerShell. npm ships Windows x64, Linux x64, and macOS arm64 binaries; additional builds are available from [GitHub Releases](https://github.com/deqiying/onesearch/releases).

```powershell
npm install -g onesearch
onesearch --version
onesearch schema search --format json
onesearch status --format json --pretty
```

`schema` is offline and does not create configuration; `status` initializes missing configuration. A fresh `ready: false` is not an installation failure. Full search needs answer, documentation, and page-fetch capabilities, for example:

```powershell
onesearch config setup openai-compatible
onesearch config setup exa
onesearch status --format json --pretty
```

Interactive prompts hide API keys. Once `minimum_profile.ok == true`, run:

```powershell
onesearch search "Explain Go context cancellation patterns" --validation balanced --format json
```

For public repository research, the default-enabled DeepWiki capability can run independently once `capabilities.repo_wiki.ok == true`, without completing the full search profile. Readiness is local preflight, not proof of remote reachability. See the [getting-started guide](docs/guides/getting-started.en.md) for the full path.

## Capability map

See the [CLI reference](docs/reference/cli.en.md) for research workflows, documentation, websites, repository commands, and the provider-direct catalog.

## Configuration

Configuration defaults to `~/.config/onesearch/config.json`; override its directory with `ONESEARCH_CONFIG_DIR`. See [configuration](docs/reference/configuration.en.md) and [config.example.json](config.example.json) for credentials, routes, profiles, and adapter limits.

## Connect onesearch to an agent

Follow [agent integration](docs/guides/agent-integration.en.md) to install a thin host Skill that loads version-matched workflows from the actual binary instead of copying provider contracts.

## Development

Build and test with the Go version pinned by `mise.toml` and `go.mod`, from the repository root:

```powershell
mise install
mise exec -- go test ./...
mise exec -- go build -o ./bin/onesearch.exe ./cmd/onesearch
./bin/onesearch.exe --version
```

On macOS/Linux use `./bin/onesearch` as the output and executable path. See the [development guide](docs/guides/development.md) and [architecture overview](docs/architecture/README.md) (Chinese) for module boundaries, cache behavior, and release verification.

## Documentation

The [documentation hub](docs/README.md) links current user guides in Chinese and English. [Design decision records](docs/decisions/README.md) preserve rationale and superseded proposals separately; they are not the current operating manual.

Relative documentation links target a source checkout. Release archives include only this README and the license alongside the executable; visit the [GitHub repository](https://github.com/deqiying/onesearch) for the full documentation.

## License

Licensed under the [Apache License 2.0](LICENSE).
