# onesearch

`onesearch` is a CLI-first multi-source web research tool for AI agents and terminal users.

This npm package installs a small JavaScript launcher and downloads the matching prebuilt binary for your platform through optional dependencies.

```powershell
npm install -g onesearch
onesearch --version
onesearch search "query" --format json
onesearch tavily search "query" --format json
onesearch exa web-search "query" --format json
onesearch mcp web_search_exa "query" --format json
onesearch skills list --format json
onesearch skills show exa --format content
onesearch skills show tavily --format content
```

Supported npm binary packages:

- `@deqiying/onesearch-win32-x64`
- `@deqiying/onesearch-linux-x64`
- `@deqiying/onesearch-darwin-arm64`
