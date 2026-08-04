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
onesearch status --format json
onesearch skills list --format json
onesearch skills show exa --format content
onesearch skills show tavily --format content
```

`schema` is the static CLI command manifest; runtime configuration remains available through `config list --format json`. Command `--help` and schema queries do not create config files or call providers; `schema --output` only writes the explicitly requested manifest file. Invalid flags or extra positional arguments return exit code 2.

This package delegates to the unscoped `onesearch` package.
