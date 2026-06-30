# @deqiying/onesearch

Scoped entry point for the `onesearch` CLI.

```powershell
npm install -g @deqiying/onesearch
onesearch --version
onesearch tavily search "query" --format json
onesearch exa web-search "query" --format json
onesearch skills list --format json
onesearch skills show exa --format content
onesearch skills show tavily --format content
```

This package delegates to the unscoped `onesearch` package.
