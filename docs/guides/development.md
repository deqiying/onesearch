# 开发与验证

[文档导航](../README.md) · [架构概览](../architecture/README.md)

本文面向仓库检出，以下命令从仓库根目录执行。项目是单一 Go module；npm 目录负责二进制分发，不另建业务工作区文档。

## 工具链与构建

以 [mise.toml](../../mise.toml) 的 Go `1.26.5` 和 [go.mod](../../go.mod) 的 Go `1.26` 要求为准。先检查命令解析结果，避免调用其他 Go 版本。`mise install` 会安装缺失的工具链，需要允许安装和下载。

PowerShell：

```powershell
Get-Command mise, go -All
mise install
mise exec -- go version
mise exec -- go build -o ./bin/onesearch.exe ./cmd/onesearch
./bin/onesearch.exe --version
```

macOS/Linux Bash：

```bash
command -v mise
command -v go
mise install
mise exec -- go version
mise exec -- go build -o ./bin/onesearch ./cmd/onesearch
./bin/onesearch --version
```

mise 将 module/build cache 放在 `.cache/go-mod` 与 `.cache/go-build`。若不通过 mise 运行，必须自行满足 Go 版本要求；缓存无写权限时可在 PowerShell 中设置 `$env:GOCACHE = Join-Path (Get-Location) '.gocache'` 后运行 Go 命令。

## 测试与静态发现

```powershell
mise exec -- go test ./...

# 按影响范围运行
mise exec -- go test ./internal/cli ./internal/commandcontract ./internal/skills
mise exec -- go test ./internal/config ./internal/providers ./internal/output
mise exec -- go test ./internal/mcpserver ./internal/execution

# 检查构建后的静态发现，不使用个人 provider 配置
./bin/onesearch.exe schema search --format json
./bin/onesearch.exe skills show onesearch --format content
```

macOS/Linux 将 `.exe` 路径替换为 `./bin/onesearch`。测试可能使用本机 HTTP 测试服务或测试子进程；首次构建可能下载 Go module。它们不等同于真实 provider 的端到端验证。

- CLI manifest golden 位于 [cli-command-manifest-v2.golden.json](../../internal/cli/testdata/cli-command-manifest-v2.golden.json)，更新合同后同步对应[测试](../../internal/cli/schema_commands_test.go)。
- 内置 Skill 的命令路径与 metadata 由 [skills_test.go](../../internal/skills/skills_test.go)检查。
- `smoke` 默认 mock，`regression` 固定 mock；当前 `smoke --live` 也仅调用本机 `doctor` 检查 minimum profile，不验证远端。它们会加载 runtime 配置，不能替代 `go test`，也不能当作真实 provider 端到端验证。模式依据见 [Smoke 实现](../../internal/service/service.go)与[命令绑定](../../internal/cli/command_bindings.go)。
- MCP 的 [catalog](../../internal/mcpserver/catalog_test.go)、[handler](../../internal/mcpserver/server_test.go)、[transport](../../internal/mcpserver/transport_test.go)及[wire](../../internal/mcpserver/wire_test.go)测试覆盖 schema/分支、readiness、配置快照、取消、脱敏、限额和四个协议版本；真实 binary/npm 链路使用下述独立脚本检查。当前验收与发布边界见 [MCP 指南](mcp.md)。

### MCP 本地进程与 npm 集成

[scripts/test-mcp.mjs](../../scripts/test-mcp.mjs)需要 Node 18+ 和 npm。使用本地 binary 与复制的 npm 包，离线打包/安装到指定临时目录；仅访问 loopback fake provider，不发布包、不读取个人 provider 配置。Windows PowerShell 示例：

```powershell
$scratch = Join-Path ([System.IO.Path]::GetTempPath()) ('onesearch-mcp-' + [guid]::NewGuid())
New-Item -ItemType Directory -Path $scratch | Out-Null
mise exec -- go build -o (Join-Path $scratch 'onesearch.exe') ./cmd/onesearch
node scripts/test-mcp.mjs --binary (Join-Path $scratch 'onesearch.exe') --scratch $scratch
```

macOS/Linux Bash：

```bash
scratch="$(mktemp -d)"
mise exec -- go build -o "$scratch/onesearch" ./cmd/onesearch
node scripts/test-mcp.mjs --binary "$scratch/onesearch" --scratch "$scratch"
```

脚本验证四版本 binary、现代/legacy npx 链路、docs 两步、错误恢复、取消与 EOF；当前实测平台为 Windows x64，其他平台需在对应环境运行。若无法自动定位 npm JS 入口，传入 `--npm-cli <npm-cli.js>`。临时目录保留供诊断，检查后由调用者清理；这不是公共 npm 下载、具体 MCP host 或 launcher 信号转发验收。

## 文档维护

[文档导航](../README.md)定义当前页面归属；[设计记录](../decisions/README.md)保存理由与替代关系。修改命令、配置、内置 Skill 时，同步直接受影响的中英文指南、参考页、npm README 和示例，不在根 README 重复完整合同。

变更后核对：

1. 新页面从入口可达，相对链接、标题锚点、图片和源码路径存在，大小写一致。
2. 扫描所有文档的 Windows drive path、UNC path、Unix user-directory path，排除 URL、API path 和 Context7 ID；禁止写入开发机绝对路径。
3. 从仓库范围搜索旧文档路径，包含隐藏协作目录，但排除 Git、缓存和索引产物。
4. 执行 `git diff --check` 并检查实际 diff；涉及 Go 行为时运行对应测试，全局行为变化运行 `go test ./...`。

仓库没有独立文档站构建或专用链接检查命令；不要把 Go 测试通过当成文档链接已验证。设计记录沿用项目技术方案格式，不使用原生 Agent Note 格式校验器。

## 发布边界

[Release workflow](../../.github/workflows/npm-publish.yml)由 `v*.*.*` tag 或手动触发，校验 [.deploy/version](../../.deploy/version) 与 npm package 版本，运行测试、构建压缩包和 checksums，再发布 GitHub Release 与 npm 包。构建通过 `BuildVersion` 注入版本，逻辑见 [app.go](../../internal/app/app.go)。

- Release archive matrix 覆盖 Windows、Linux、macOS 的 amd64/arm64；npm 平台包为 Windows x64、Linux x64、macOS arm64。
- Release 压缩包附带英文根 README 和 LICENSE，不包含整棵文档与源码；README 提供仓库入口。
- npm 包的 README 必须脱离仓库可读，不能改成指向包外文件的相对链接。host Skill 模板需与[仓库模板](../assets/onesearch-host-skill.md)同步。
- [本地 npm 打包脚本](../../scripts/package-npm.ps1)仍使用旧的源码版本正则及 manifest `id` 检查，与当前 `BuildVersion` / V2 `name` 合同不一致；不能作为已验证的发布入口。本文不改变该脚本，发布流程以 workflow 为准。

推送 tag、手动触发发布与 `npm publish` 都会改变外部状态，不属于普通文档或构建验证。
