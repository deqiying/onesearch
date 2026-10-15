# Onesearch MCP Server 模式技术方案

[设计记录导航](README.md) · [当前架构](../architecture/README.md) · [CLI 参考](../reference/cli.md)

> **实施记录（本机日期：2026-10-15）**：本方案已获批准并落地。下文原有未来式、基线分析与实施计划作为批准设计保留，不表示功能仍未实现；当前使用方式以 [MCP 指南](../guides/mcp.md)为准。原方案的代码基线与 npm 版本是设计时快照，不是发布完成声明。

## 1. 文档信息与推荐结论

| 项目 | 内容 |
| --- | --- |
| 状态 | **Implemented (local validation passed; release/host/live pending)** |
| 修订日期 | 2026-10-15 |
| 方案修订 | 第二版：直接 npx 启动与精简 catalog |
| 代码基线 | `main`，提交 `a12e6e3` |
| 本次产物 | 已实施的批准设计与验收边界；原设计正文保留 |

**推荐方案：新增顶层命令 `onesearch mcp`，支持通过 `npx onesearch mcp` 启动本地 stdio MCP Server；首期 catalog 从 9 个工具合并为 5 个。** MCP adapter 使用官方 Go SDK，复用现有 command contract、service 和 provider，不执行 shell，也不反向启动 CLI 处理工具调用。

正式 host 配置推荐 `npx -y onesearch mcp`；可复现环境应使用发布后实际存在的固定包版本。省略 `-y` 的命令仍是支持入口，但 npx 在需要下载包时可能等待安装确认，不能将该等待归因于 MCP 握手失败。[R10]

### 1.1 本轮需求与决定

| 补充需求 | 修订决定 | 理由 |
| --- | --- | --- |
| 希望使用 `npx onesearch mcp` 启动 | `mcp` 本身是 canonical leaf command，进入服务模式，不再要求额外子命令 | npm 已有同名 bin 和参数转发，主要改动在 Go 命令注册 |
| 原 catalog 工具偏多 | 按业务意图合并为 search、fetch、site、repo_wiki、status | 减少顶层选择，保留可描述的输入、输出和风险边界 |
| 避免合并后语义含混 | search 使用显式 `mode`，site 使用显式 `action`；分支参数互斥 | 不通过 query 猜测模式，不自动串行执行全部分支 |

首版的 `mcp serve` 和 9 工具设计从未实施或发布，本修订直接替换其拟议合同，**不新增旧入口兼容 alias 或旧工具名**。首版仍成立的协议兼容、只读配置快照、脱敏、取消与资源限额设计继续保留。

源码已包含本方案功能；当前仓库 npm 包清单版本仍为 `1.0.6`，不含新功能，版本号未因本地实现而变更。公开包需要在发布包含新 binary 的版本后才能用于新入口验收。

### 1.2 实际落地与验收边界

- [CLI 分流](../../internal/cli/cli.go)及[命令定义](../../internal/commandcontract/utilities.go)：顶层 `onesearch mcp`，无运行 flags / `serve` 子命令，早于普通配置加载与输出。
- [Server](../../internal/mcpserver/server.go)、[transport](../../internal/mcpserver/transport.go)、[catalog](../../internal/mcpserver/catalog.go)、[dispatch](../../internal/mcpserver/dispatch.go)、[status](../../internal/mcpserver/status.go)、[result](../../internal/mcpserver/result.go)：固定 5 工具，显式分支、四版本协议、并发/超时/体积限额和脱敏出口；使用固定 SDK `v1.8.0`。现代取消响应在 transport 最终编码出口抑制，legacy 取消行为保持不变。
- [共享执行](../../internal/execution/execution.go)及[只读快照](../../internal/config/snapshot.go)：复用 service，不反向运行 CLI，不通过 MCP 写配置；发现不读配置、不联网、不创建配置。
- 测试证据路径：[catalog_test.go](../../internal/mcpserver/catalog_test.go)、[server_test.go](../../internal/mcpserver/server_test.go)、[wire_test.go](../../internal/mcpserver/wire_test.go)、[execution_test.go](../../internal/execution/execution_test.go)、[snapshot_test.go](../../internal/config/snapshot_test.go)、[CLI 测试](../../internal/cli/cli_test.go)及[commandcontract 测试](../../internal/commandcontract/)。覆盖 schema、分支 dispatch、wire、取消、脱敏、资源限额与配置隔离。
- 2026-10-15 本地验证通过：`go test -count=1 ./...`、`go vet ./...`，以及 MCP/config/execution/CLI 包的 `go test -race`；实际命令均通过 `mise exec --` 使用 Go `1.26.5`。四版本 raw-wire fixture、输入/结果 schema、readiness、快照轮换、4 并发、超时、panic、64 KiB 帧/2 MiB 响应和脱敏回归通过。五工具 catalog JSON 实测 **10,183 bytes**（不是 token 估算）。
- [本地集成脚本](../../scripts/test-mcp.mjs)在 Windows x64、Node `v26.11.1`、npm `11.20.0` 上通过：四版本真实 binary；现代与 legacy 两版本的离线本地 tarball `npx → launcher → binary` 链路；独立 Node wire 客户端和 loopback fake Context7 验证发现、docs 两步、错误恢复、取消、EOF 对 HTTP 请求的清理和启动错误。未修改 npm launcher、包版本，也未发布。
- Windows x64 构建及 Linux x64/macOS arm64 交叉构建通过；后两平台未运行进程验收。新增 [transport 回归](../../internal/mcpserver/transport_test.go)覆盖取消响应过滤、相对配置路径不破坏正文及 `stderr_summary` 不泄漏。
- npm 发布、真实 host（包括 Windows）、live provider、launcher 信号转发及其他平台进程验收仍未执行；本地 EOF 和取消通过不能替代这些验收。正式配置应固定实际已发布且含新功能的版本，不编造可用版本。

## 2. 需求范围与非目标

### 2.1 角色与目标

- **下游 MCP Server**：本方案新增，供 agent/host 调用 onesearch。
- **上游 MCP Client**：仓库已有，供 onesearch 调用 DeepWiki、AnySearch 或本地 stdio provider。
- 两个方向的协议版本、请求 ID、配置和取消状态独立；不因下游使用现代协议就强制所有上游升级。

| 编号 | 首期目标 | 验收标准 |
| --- | --- | --- |
| G1 | npx 可直接启动服务 | 在对应 npm 支持平台完成 npx → launcher → binary 的发现、调用及 EOF 关闭 |
| G2 | 精简 catalog | `tools/list` 固定提供 5 个工具，覆盖原提案的全部业务入口 |
| G3 | 业务复用 | 同一分支命中现有 service；不更改 CLI 路由、minimum profile 和 fallback 语义 |
| G4 | 兼容明确 | 本文声明的 4 个协议版本分别通过 wire fixture |
| G5 | 长期运行安全 | stdout 纯协议；单次调用失败隔离；取消、超时、并发和体积受限 |
| G6 | 配置可恢复 | 缺配置仍可发现工具，状态按操作分支报告；不通过 MCP 写配置 |

### 2.2 非目标

- 首期只支持本地 stdio，不监听 HTTP 端口；不提供远程多租户、OAuth、旧 HTTP+SSE 或系统服务部署。
- 不提供 resources、prompts、sampling、elicitation、subscriptions、tasks、token 级输出流。
- 不新增 provider，不改造成通用 MCP proxy；不暴露任意 shell、argv、上游 tool name 或配置修改工具。
- 不自动安装 provider runtime、浏览器、uv 或其他依赖。用户主动使用 npx 下载 onesearch 包是另一条启动边界。
- 不把 `deep` 的 CLI 计划当作 MCP 原生编排，不将候选来源、库解析或 crawl 提交当作研究完成。
- 不承诺工具数量下降就必然降低总 schema token 或调用次数；需要通过生成的 catalog 和任务测试实际评估。

## 3. 现状分析与代码依据

### 3.1 启动与业务执行链

```text
拟议的 npx 启动链：
MCP host → npx/npm → onesearch/bin/onesearch.js → 平台 Go binary → cli.Execute(["mcp"])

现有业务执行链：
cli.Execute → commandcontract / parser → config.Load → service.New
  → Search / Fetch / Map / Crawl / RepoWiki / provider-direct
  → providers → HTTP / MCP client
  → output.RenderWithOptions → 文件 / stdout
```

| 已检查对象 | 当前行为 | 对实现的影响 |
| --- | --- | --- |
| [npm package.json](../../npm/onesearch/package.json) | `name=onesearch`，bin 指向 `bin/onesearch.js`，平台包为同版本 optional dependencies | 不需要另建 MCP npm 包；发布时必须包含新 binary，并保持版本一致 |
| [npm launcher](../../npm/onesearch/bin/onesearch.js) | `spawnSync(binary, process.argv.slice(2), {stdio: "inherit"})` | `mcp` 参数可原样转发；实际 EOF、信号和进程树行为仍需端到端验证 |
| [发布 workflow](../../.github/workflows/npm-publish.yml) | npm 构建矩阵为 Windows x64、Linux x64、macOS arm64；Release binary 矩阵更广 | 不把 binary 支持平台自动等同于 npx 支持平台 |
| [命令路由](../../internal/cli/command_routes.go) | registry 最长路径匹配；namespace 可以设置默认命令 | 单个顶层 leaf 足够，不需要为本需求增加 namespace 默认路由 |
| [CLI 入口](../../internal/cli/cli.go) | 静态命令早于配置加载；普通命令使用 `context.Background()`，错误可能打印 stdout JSON | `mcp` 在通用配置/输出前分流，使用可取消 context，启动错误只写 stderr |
| [命令绑定](../../internal/cli/command_bindings.go) | service 参数、timeout 与 `printCommand` 混合 | 提取无 I/O 的执行绑定，不能把现有 CLI handler 直接注册为 MCP handler |
| [参数与定义](../../internal/cli/command_parser.go)、[contract](../../internal/commandcontract/definition.go) | `Values` / `Present`、类型和约束已有；ValueType 以标量/字符串数组为主 | 复用叶子字段事实；MCP 的嵌套聚合 schema 留在 adapter，不为此扩展整个 CLI 类型系统 |
| [Search 与 direct 方法](../../internal/service/service.go) | `Search` 校验 minimum profile；`ExaSearch`、`Context7Library`、`Context7Docs` 可独立调用 | 搜索模式分别 dispatch；不能先统一执行 `Search` 再假装是 sources/docs 模式 |
| [文档 fallback](../../internal/service/service.go) | `runDocsSearchFallback` 返回候选来源，Context7 路径只解析 library ID | 不是“解析库并返回文档正文”的现成实现，不能误复用 |
| [Context7 归一化](../../internal/providers/context7.go) | Library 返回 `results` 中的 `id/title/description`；Docs 接受明确 library ID | 同一个 MCP 工具可分两次完成解析与查询，无需自动猜选候选 |
| [Map/Crawl](../../internal/service/service.go)、[provider](../../internal/providers/web.go) | 两者各有方法和默认值；Crawl 可能返回 submitted job | 合并工具名但保留 action、默认值、readiness、结果完成度差异 |
| [配置](../../internal/config/config.go)、[runtime](../../internal/config/runtime.go) | `Load()` 会初始化文件；`Service.runtime()` 重复读文件；普通读失败可能折叠为空 map | MCP 新增只读请求快照，坏配置不得静默降级 |
| [输出](../../internal/output/output.go)、[secret 收集](../../internal/service/config_service.go)、[redact](../../internal/redact/redact.go) | 脱敏主要在 CLI 渲染阶段，quiet 可能裁剪正文 | MCP 另设 DTO 脱敏出口，不能直接返回原始 map 或 quiet preview |
| [状态注解](../../internal/cli/provider_commands.go) | `direct_endpoints` 由 CLI 补充，不是 `Service.Status()` 原生字段 | 新状态投影按 MCP operation 检查，不依赖 CLI presentation |

### 3.2 与原设计的关系

本方案增加 manifest 的 MCP 消费方，不替代 [Manifest V2](cli-command-manifest-tool-declaration-alignment-technical-design.md)。它也不替代 [上游协议对齐方案](provider-upstream-protocol-alignment-technical-design.md) 或其未完成的 live 验收；已有 [stdio bridge 方案](mcp-stdio-bridge-local-providers-plan.md)不是 Server 实现。

本节保留批准前的现状分析；实施后 README、架构与内置 Skill 已同步下游 Server 入口，上游 client 的职责与未完成 live 验收边界不变。

## 4. 协议与依赖方案

### 4.1 兼容范围

| 协议版本 | 生命周期 | 结果合同 |
| --- | --- | --- |
| `2026-07-28` | 逐请求 `_meta`，支持 `server/discover`，不要求 initialize | SDK 现代结果外壳、文本及结构化结果 |
| `2025-11-25` | initialize → initialized → 调用 | 文本及结构化结果 |
| `2025-06-18` | 按协商版本的 legacy 生命周期 | 文本及结构化结果 |
| `2025-03-26` | 按协商版本的 legacy 生命周期 | 仅文本 JSON，不发送该版本未定义的 outputSchema / structuredContent |

现代协议的版本和能力元数据、discovery 及 legacy 初始化分别以官方规范为准。[R1][R2][R5] 结构化工具输出在 `2025-06-18` 引入，旧版结果过滤必须以 wire fixture 证实。[R9]

只声明已验收版本和 `tools` 能力；不照搬 SDK 的全部版本/扩展列表。现代 metadata 使用完整命名空间 key；下游服务身份复用 `internal/app` 的 Name 和 Version，不复制上游 client 的 map 组装逻辑。[R2]

### 4.2 SDK 选择

沿用官方 `github.com/modelcontextprotocol/go-sdk/mcp`，候选版本固定为 `v1.8.0`。上轮已核对官方兼容性表、该版发布记录和 `go.mod`：SDK 的最低 Go 版本要求可由本项目工具链满足；发布记录包含 stdio 帧长度及服务端版本白名单控制。[R6][R7][R8]

这是选型依据，不是本项目已成功集成的证明。P0 仍须验证具体 API、版本过滤、取消和 EOF，检查实际依赖图与平台构建。拒绝自行重写协议栈，也不以启动 CLI 子进程代替业务 adapter。本次不执行 `go get`。

## 5. npx 启动合同

### 5.1 命令与命名

拟支持：

```text
npx onesearch mcp
npx -y onesearch mcp
onesearch mcp
onesearch mcp --help
onesearch schema mcp --format json
```

固定版本的部署模板：

```text
npx -y onesearch@<release-version> mcp
```

`<release-version>` 必须替换为已发布且包含 MCP 功能的实际版本，不能直接复制占位符执行。npx 参数放在包名之前；包名后的 `mcp` 传给 onesearch。[R10]

具体注册规则：

- `CommandDefinition.ID = "mcp"`，`Path = ["mcp"]`，类型为 utility；无 positionals，无 alias，无独立 namespace。
- `mcp` 直接启动 stdio，不注册 `serve` 子命令。首期没有第二种 transport，不增加仅有一个合法值的 `--transport` 开关。
- 不套用 `withOutput()`。拒绝 `--format`、`--pretty`、`--output`、`--verbose`、`--quiet`、`--port` 和额外位置参数。
- `mcp --help` / `schema mcp` 仍为普通静态 CLI 输出，不进入协议模式，不读取或创建 runtime 配置。
- 服务分支必须早于通用 `config.Load()` 和通用错误打印；启动参数错误只写 stderr，返回 `2`。
- 正常 EOF/受控关闭返回 `0`，binary 的不可恢复启动/transport 故障返回 `5`；MCP 工具错误不导致进程退出。
- npm 下载、平台包缺失或 launcher 自身错误属于外层启动错误，不承诺与 Go binary 共用退出码；必须保留其 stderr。

CLI manifest 为 `mcp` 新增长运行协议流 output contract，不将其声明为单个普通 CLI JSON envelope。同步 binding 数量校验、help、output contract 校验和 golden；保留 V2 字段，不因新增命令自动提升 manifest_version。

### 5.2 npm 分发与进程边界

复用现有主包、平台包和 JS launcher，不增加 `onesearch-mcp` 包，不在 Node 中实现第二套 MCP Server。

| 场景 | 设计要求 |
| --- | --- |
| 首次 npx 下载 | host 配置使用 `-y`，初始化超时需覆盖包获取；下载失败应可诊断，不吞 stderr |
| 本地旧版本/缓存 | 无版本号可能解析到本地已安装包；生产建议固定版本，并核对最终 binary 版本 |
| 平台 optional dependency 缺失 | 明确报缺包，不动态编译 Go、不自动降级为其他服务 |
| npm 输出 | 对完整启动链捕获 stdout；启动后的 stdout 只能是协议帧，不能把 npm 诊断混入协议 |
| stdin/stdout | npm、launcher、binary 必须保持双向管道及 EOF 语义；不要用 shell 文本转发协议 |
| 关闭 | 验证正常 EOF、host 强制终止、异常 binary 退出，避免遗留 launcher、binary 或上游测试进程 |

现有 `spawnSync` 已提供参数及 stdio 转发，不因新增命令就强制重写。它的长期运行和信号处理是否需要改为异步 spawn，以真实进程测试为依据；如失败，局部修复 launcher，不建立通用进程管理框架。

npx 支持声明限于现有 npm 平台矩阵。其他已构建平台可使用直接 binary，但扩大 npm 平台包属于独立发布范围。

### 5.3 Host 配置

常见 host 的示意配置如下；具体配置文件格式以目标客户端为准：

```json
{
  "mcpServers": {
    "onesearch": {
      "command": "npx",
      "args": ["-y", "onesearch", "mcp"]
    }
  }
}
```

正式部署将 `onesearch` 替换成固定版本包 spec；首次先在终端完成 provider 配置，再让 host 继承 `ONESEARCH_CONFIG_DIR` 和相关凭据环境。`-y` 仅免除 npm 安装确认，不是 provider 访问或付费调用授权。[R10]

Windows host 可能需要按其启动 API 使用 `npx.cmd` 或平台专用启动配置；不能假设 `.cmd` 可被所有无 shell API 直接执行。该差异必须在选定 host 上验收。缺少 Node/npm 时报告前提，不静默安装。

## 6. Catalog 精简设计

### 6.1 推荐 5 个工具

| MCP name | 职责 | 合并来源 |
| --- | --- | --- |
| `onesearch_search` | 答案搜索、来源发现、库文档解析/检索 | 原 Search、Exa web-search、Context7 resolve、Context7 query-docs |
| `onesearch_fetch` | 单个已知 URL 的正文抓取 | 原 Fetch |
| `onesearch_site` | 站点链接发现或多页抓取，显式选择 action | 原 Map、Crawl |
| `onesearch_repo_wiki` | 仓库问答、结构和内容读取 | 原 RepoWiki |
| `onesearch_status` | 配置与各操作分支的本机 readiness | 原 Status |

工具名按 name 排序并固定，不随凭据增删，不注册原 proposal 中被合并的独立工具名。

### 6.2 为什么不是 1 个或 4 个

| 选项 | 评价 | 决策 |
| --- | --- | --- |
| 保持 9 个 | 单工具简单，但将 provider 细节和中间步骤暴露为过多顶层选项 | 被本修订替代 |
| 合并为 5 个 | 领域相近的入口合并；单页、站点、仓库和诊断边界清晰 | 推荐 |
| 再把 fetch 合入 site，降为 4 个 | 技术可行，但单页读取与站点遍历的输入、规模和成本混在一起 | 首期不选 |
| 一个 `onesearch_execute` + 任意 action | 数量最少，却变成难描述、难验证的工具集合，容易退化成命令透传 | 不选 |

合并只减少公开工具数，内部仍复用 8 个业务命令绑定和单独状态投影；不会因工具名合并而自动合并网络调用。status 单独保留，避免每次业务请求都返回大块诊断，或让错误恢复依赖 resources/shell。

## 7. 分支输入与执行合同

### 7.1 onesearch_search

根输入为 object，字段只允许 `mode`、`query`、`timeout`、`answer_options`、`sources_options`、`docs_options`。

- `mode` 必填，枚举 `answer | sources | docs`；不设置默认模式，避免省略时触发答案生成费用。
- `query` 必填非空字符串；`timeout` 可选，单位秒，默认 90，必须大于 0 且不超过 300。
- 只能提供所选 mode 对应的 options；其他分支 options 即使是空 object 也拒绝。
- `answer_options` / `sources_options` 可省略，应用各自叶子默认值；docs 必须提供 `docs_options`。

| mode / 条件 | 映射 command ID | Service 调用 | readiness |
| --- | --- | --- | --- |
| `answer` | `search` | `Search` | 保留 minimum profile 及本次 provider filter 校验 |
| `sources` | `exa.web-search` | `ExaSearch` | Exa direct 可用，不要求答案搜索 profile |
| `docs` + library_name | `context7.resolve-library-id` | `Context7Library` | Context7 direct 可用 |
| `docs` + library_id | `context7.query-docs` | `Context7Docs` | Context7 direct 可用 |

**首期 sources 明确由 Exa 提供，docs 明确由 Context7 提供。** 这是对原提案工具的聚合，不声称新增了 provider-neutral 来源/文档 workflow。相应 provider 不可用时返回明确错误，不偷偷切换成 answer；若未来要求 sources 多 provider fallback，应另行定义公开 service 合同。

#### 参数白名单

| options | 合法字段与映射 |
| --- | --- |
| `answer_options` | `platform`、`model`、`extra_sources`、`fetch_sources`、`validation`、`fallback`、`providers`、`answer_providers`、`source_providers`、`docs_providers`、`fetch_providers`、`repo_providers`、`repo_wiki`、`repo_wiki_mode`、`repo_wiki_query`；映射到原 Search 参数 |
| `sources_options` | `num_results`、`search_type`、`include_text`、`include_highlights`、`start_published_date`、`category`、`include_domains`、`exclude_domains`；映射到 Exa 参数 |
| `docs_options` | 恰好提供一个非空 `library_name` 或 `library_id`；library_name 映射为 resolve 的 `name`，library_id 映射为 query-docs 的同名字段 |

query/timeout 只存在于根对象，不允许在 options 中重复。数组使用 JSON string array，不采用 CLI 逗号编码；不暴露 CLI 输出选项、stream/no_stream 或 Exa deprecated max_results。保留未显式提供的 Search validation/fallback 由 runtime 配置决定的语义。

#### 文档检索不隐式选库

1. 客户端不知道 ID：提供 `mode=docs`、query 和 library_name，调用一次 `Context7Library`。
2. 返回 `operation=docs.resolve`，`data.kind=library_candidates`、归一化候选、`needs_selection`。返回成功仅代表解析完成；零候选时 `candidates=[]`、`needs_selection=false`，给出调整库名的提示，不假装检索到文档。
3. 有候选时由客户端选择 ID；即使只有一个候选也不自动追加正文请求，保证调用次数可预测。
4. 再调用同一工具，改为 library_id，执行 `Context7Docs`，返回 `operation=docs.query`、`data.kind=documentation`。

不从名称猜造 ID，不在 server 保存“上次选中的库”，不自动取第一候选；客户端已有可信 ID 时可直接查询。候选 ID 仍需基本格式/长度校验，不能变成任意请求 URL。

示例仅展示调用参数，不代表真实执行；第二步的 library_id 必须取自第一步结果：

```json
{
  "mode": "docs",
  "query": "如何使用路由？",
  "docs_options": {"library_name": "example-library"}
}
```

```json
{
  "mode": "docs",
  "query": "如何使用路由？",
  "docs_options": {"library_id": "/example/library"}
}
```

### 7.2 onesearch_site

根字段只允许 `action`、`url`、`provider`、`max_depth`、`limit`、`timeout`、`map_options`。

- `action` 必填，枚举 `map | crawl`；`url` 必填，HTTP(S) URL；provider 默认 auto。
- `map_options` 只允许 `instructions`、`max_breadth`，仅 map 可用；crawl 请求携带它时直接报 parameter_error。
- defaults 按 action 分支应用，不能先填 map 默认值再运行 crawl。

| action | 业务调用 | 默认值 | readiness / 结果 |
| --- | --- | --- | --- |
| `map` | `Service.Map` | max_depth=1，limit=50，timeout=150，max_breadth=20 | `site_map`；返回链接，不自动抓正文 |
| `crawl` | `Service.Crawl` | max_depth=2，limit=20，timeout=180 | `site_crawl`；保留页面或 submitted/job 状态 |

不能把 map 的结果自动送入 crawl；也不能因为 map 可用就宣称 crawl 可用。保留两条现有 service 的路由行为，不顺手统一它们不同的 fallback 实现。

### 7.3 其余工具

| 工具 | 输入合同 | 行为 |
| --- | --- | --- |
| `onesearch_fetch` | 必填 url；可选 provider（auto）、timeout（90 秒） | 调用 `Service.Fetch`，只处理单个已知 URL |
| `onesearch_repo_wiki` | 必填 repo、mode；mode 为 ask/structure/contents；可选 question、provider、timeout（60 秒） | ask 要求非空 question；structure/contents 不接受 question，分别映射既有 RepoWikiOptions |
| `onesearch_status` | 空 object | 返回配置、minimum profile、5 工具各 operation 的安全 readiness |

Fetch 的 MCP timeout 为 adapter 外层预算，不伪装为现有 CLI 新增参数。repo_wiki 的 mode 在 MCP 中显式必填；不更改 CLI 对 mode 的现有默认处理。

### 7.4 共享校验与 schema

- MCP 根 schema 始终是 object，嵌套 options 也是封闭 object，均 `additionalProperties=false`；不允许 null 或字符串数值替代正确类型。
- 从 commandcontract 的叶子定义选取字段及默认值，构建 MCP 聚合 schema。mode、action、嵌套位置和分支约束由 MCP adapter 定义，**不是 CLI manifest 的一比一导出**。
- 分支约束可用 JSON Schema 的条件约束表达，必须验证所选 SDK/schema validator 的支持；运行时显式验证始终保留，不依赖 client 执行 schema 校验。
- 保留 Values/Present，先确定分支，再校验并填充该分支缺省值，最后映射内部 command ID。不能把“未提供”和“显式 false”混淆。
- 不增加 `ValueTypeObject` 或通用嵌套 CLI 参数来服务 MCP；不从任意 JSON key 构造 handler 名称。
- 原 CLI lexical parsing 留在 CLI；只提取确实复用的值校验和执行绑定。

### 7.5 readiness 与 annotations

`onesearch_status` 的 `ok=true` 表示检查完成，`ready` 继续反映 minimum profile，但客户端选择操作时以 `operations` 中具体分支为准。

- search 分别列出 answer、sources、docs.resolve、docs.query。
- site 分别列出 map、crawl；repo_wiki 分别列出 ask、structure、contents。
- 工具级 `available` 定义为任一操作可用，`fully_available` 为全部操作可用；不可只看一个布尔值就调用所有分支。
- 保留 capability、provider ID、脱敏 reason 和配置分类，不返回配置绝对路径、endpoint、完整 settings 或环境变量值。
- direct 分支在实际调用前也检查 enabled、凭据和配置有效性，不能只在 status 中检查后直接绕过禁用状态；复用现有规则，CLI 行为不变。
- readiness 不证明网络、凭据、远端工具和额度实际可用；不因某分支失败自动切换语义不同的 mode/action。

description 必须写明模式用途、docs 的两步流程、sources/docs 的 provider 边界和 crawl 可能仅提交任务。聚合工具 annotations 按所有分支中较保守的行为声明，不把 map 的只读属性用于整个 site；annotations 不是授权机制。[R4]

## 8. 架构与结果合同

### 8.1 模块边界

```text
CLI argv → internal/cli ─────────────────────────────┐
                                                    │
MCP host → npx/launcher/binary → internal/mcpserver   │
                                  │ 分支参数映射    │
                                  └────────┬────────┘
                                           ▼
                              internal/execution（新增）
                                           ▼
                                  internal/service
                                           ▼
                              providers → 上游 HTTP / MCP

commandcontract → 共享叶子参数事实
config          → MCP 只读请求快照
redact          → MCP DTO 和 stderr 脱敏
```

`internal/mcpserver` 负责 transport、静态 catalog、聚合 schema、受限分支映射、DTO 和资源控制；`internal/execution` 提取 8 个业务入口的无 I/O 参数绑定、执行与原有 timeout 逻辑，供 CLI/MCP 共用。status 单独作安全投影。execution 不依赖 MCP，不读 stdin、不输出、不写文件。

SDK 按请求版本处理协议后，工具调用依次进行：信封校验 → 分支参数校验 → 并发准入 → 配置快照/readiness → 有界 context → service → DTO/脱敏/体积校验 → 协议响应。现代请求不强制先 discovery，legacy 按初始化生命周期进入。合并后的调用仍只选择一个业务分支；既有 Search 内部的补充搜索和 fallback 保持原语义。

### 8.2 结果信封

聚合工具不能再依赖单个 CLI command 解释结果，业务信封改为以下形态。示例为设计数据，不代表实际执行：

```json
{
  "ok": true,
  "tool": "onesearch_site",
  "operation": "map",
  "data": {
    "url": "https://example.com",
    "results": [{"url": "https://example.com/guide", "title": "示例页面"}]
  }
}
```

- 必需字段为 ok、tool、data；operation 在成功及已确定分支的失败结果中必需。无效 mode/action 的 parameter_error 可省略 operation，不回显未校验字符串作为操作名。
- operation 使用固定值：answer、sources、docs.resolve、docs.query、fetch、map、crawl、ask、structure、contents、status；实际 CLI command ID 仅作内部映射。
- data 保留完整业务正文、来源、provider、证据警告、fallback 及必要 attempt 状态，排除原始配置、provider stderr 和 opaque 调试副本。
- docs.resolve 的候选列表不是文档正文；crawl 的 submitted 不是完成；两者必须保留其业务阶段。
- 失败附 error_type/error，`isError = !ok`；零候选解析可成功返回空列表，不把“无候选”伪造为有文档。
- 不采用 CLI quiet 的正文 preview；完整返回或明确 result_too_large，不静默截断后继续声称证据完整。

outputSchema 描述稳定外壳，对 data 保持可扩展 object。成功/失败均做一致性测试，不为各 provider opaque payload 伪造统一精确 schema。[R4]

### 8.3 wire 与错误

文本 content 为一份紧凑 JSON 信封；支持结构化结果的版本再提供语义相同的 structuredContent，均基于同一个脱敏对象生成。SDK 处理现代 `resultType` 等外壳；旧版字段过滤必须测试。不得对整个 JSON-RPC 字符串替换凭据，以免破坏 request ID。

| 场景 | 处理 |
| --- | --- |
| 非法 JSON/请求、未知 method | SDK 协议错误；未知 method 为 -32601 |
| 未知工具、非法 tools/call 信封 | -32602；未知工具不等于未知 method |
| 已知工具的参数类型、分支混用、非法范围 | tool error：parameter_error，零 provider 调用 |
| 配置缺失/损坏/provider 未就绪 | tool error：config_error；status 返回诊断状态 |
| provider/网络/证据/上游限流失败 | 保留原 error_type 和脱敏上下文，不在 adapter 额外重试 |
| 本地总超时、超额并发、超大结果 | timeout / server_busy / result_too_large，不退出进程 |
| 内部 panic/序列化异常 | 有界隔离、脱敏 internal error；不得返回空成功结果 |
| 客户端取消 | 交给 SDK 对应版本的取消处理；现代 stdio 不再发送该请求的后续消息 |

协议错误和工具错误按官方 Tools 规范区分；不把所有业务失败都变成 -32603。[R4] EOF、取消和 stdout 规则遵循 stdio 规范。[R3]

## 9. 配置、安全与生命周期

### 9.1 只读请求快照

新增内部只读快照入口，例如 `config.LoadReadOnlySnapshot`，复用现有配置路径与优先级，但不调用 EnsureInitialized/SaveFile。

1. 文件不存在时使用内存默认 runtime，不创建目录/文件；已有无需 key 的能力按现有规则判断。
2. 文件损坏、不可读或 schema 不可识别时保留 config_error，不将其折叠成默认配置。
3. 每次工具调用读取一次文件，并捕获本次凭据环境视图。通过既有 Config 边界提供给 service；快照模式的 LoadFile/LoadFileStrict/Get 不再读取实时状态，返回 map 深拷贝。
4. 路由、分支 readiness、默认值和 OutputSecretValues 基于同一快照；在途请求不随配置文件更新而换凭据，新请求可读取更新配置。
5. 快照不可写，不新增 watcher 或跨请求配置缓存；host 启动环境改变后仍需重启进程。

CLI 原有 `config.Load()` 初始化行为保持不变。MCP discovery/tools/list 不读取 runtime 配置，不联网，也不启动上游 provider。

### 9.2 信任与脱敏

首期是本机受信任 host 下运行的用户权限进程，不是匿名远程用户的沙箱。

- tool arguments 不接受 endpoint、API key、配置路径、provider command/env 或任意文件输出。
- 用户本地配置可以启动已有 stdio provider；这属于受信任配置，不是模型可以改写的执行参数。
- URL 只允许 HTTP(S)，拒绝 userinfo 和本地文件 scheme，保留 provider 现有安全限制。scheme 校验不等于完整 SSRF 防护；远程 HTTP 必须另行评审出口、DNS、重定向和认证。
- 使用快照上的 OutputSecretValues、redact.Data 和 redact.Text；正文、嵌套错误、structuredContent、文本 JSON 和 stderr 均需覆盖脱敏测试。
- stderr 仅记录工具/operation、关联 ID、耗时、错误分类与生命周期；不打印完整 query、正文、headers、凭据或 provider 原始 stderr。
- 外部正文与候选数据不得改变 tool allowlist、配置或触发安装/执行。

### 9.3 context 与限额

进程 context 处理适用平台信号和 EOF；request context 贯穿 execution/service/provider。取消后不启动下一轮 fallback/抓取；整体 timeout 覆盖所有上游工作。清理必须有界，不将本地取消等同于撤销已提交的远程 crawl job。[R3]

| 项目 | 首期建议值 / 行为 |
| --- | --- |
| 并发 | 每进程最多 4 个业务调用，超额立即 server_busy；不建立无界等待队列 |
| 输入帧 | 64 KiB，SDK 在完整缓冲前限制；超限按 transport 错误关闭并诊断 |
| 输出 | 最终序列化响应最多 2 MiB，包含文本/结构化双份；超限返回小型错误 |
| timeout | search 各 mode 与 fetch 默认 90 秒；site 按 map=150/crawl=180；repo_wiki=60；显式值大于 0 且不超过 300 |
| Search 数量 | answer 的 extra_sources/fetch_sources 各 0–10；sources num_results 1–20，默认沿用 Exa 的 5 |
| Site 规模 | map limit 1–200、crawl 1–50；max_depth 1–3；map max_breadth 1–50；defaults 见第 7.2 节 |

这些是设计初值，不是性能测试结论，不为首期新增通用限额配置系统。下载阶段也需检查实际 HTTP 读取上限，不能以读入无限内容后再限制输出宣称资源安全；次数限制不等于费用硬上限。

## 10. 实施计划与影响范围

### 10.1 分阶段交付

| 阶段 | 工作 | 完成门槛 |
| --- | --- | --- |
| P0：合同与原型 | 固定 SDK、验证四版本/EOF/取消/旧版投影；定义 5 工具 schema 与分支映射 | SDK 与 schema validator 行为有测试证据，依赖范围可审查 |
| P1：共享执行 | 提取 8 个业务绑定和值校验，新增只读配置快照，保留 CLI I/O | 原 CLI 参数、输出、退出码和配置初始化回归通过 |
| P2：Server | 注册顶层 mcp，落实聚合 handlers、分支 readiness、DTO、脱敏和限额 | 5 工具全部分支通过 fake service/provider 测试，非法分支零副作用 |
| P3：npx 交付 | 本地打包链路、实际子进程与 host 联调、必要 launcher 修复、文档同步 | npx 冷/热缓存、版本定位、stdout、EOF 和进程回收通过；明确平台边界 |

不以“工具数降到 5”或“initialize 成功”作为完整交付；功能和安全验收仍覆盖所有底层操作。

### 10.2 预计文件

| 路径 | 预计改动 |
| --- | --- |
| `go.mod`、`go.sum` | 固定 SDK 和实际依赖，实施时审查 |
| `internal/commandcontract/utilities.go` | 新增 mcp leaf，专用 output/side-effects；不新增 mcp namespace |
| `internal/commandcontract/registry.go` 及值校验文件 | 必需的合同校验和叶子值校验复用，不扩展通用嵌套 CLI schema |
| `internal/cli/cli.go`、`command_bindings.go`、相关 parser/help 测试 | 服务早分流、启动错误 stderr、既有绑定迁移 |
| `internal/execution`（新增） | 无 I/O 的业务命令执行与测试 |
| `internal/mcpserver`（新增） | catalog、聚合 schema、mode/action 映射、DTO、SDK transport、安全和进程测试 |
| `internal/config`、必要的 `internal/service` 文件 | 只读快照、取消检查、readiness helper；不重写路由 |
| `internal/cli/testdata/cli-command-manifest-v2.golden.json` | 新增 mcp command，旧命令合同不变 |
| `npm/onesearch/bin/onesearch.js` | 仅在进程验收证实问题时修复；增加 launcher/打包测试 |
| npm 包、平台包、`.deploy/version` | 正式发布阶段统一版本并装入新 binary；不能只发布 README 宣称支持 |
| `.github/workflows/npm-publish.yml` | 若新增发布前打包 smoke，使用隔离配置与 mock，不访问真实 provider |

功能实施时同步根 README 中英文、架构、CLI/配置/Agent 指南、npm README、内置 Skill 和相关测试。Skill 中禁止旧 `mcp <tool>` 的约束应改为明确禁止桥接调用，而不是一概禁止新的 `mcp` 服务入口。

本轮不改上述功能文件、版本、依赖或发布流程，不执行发布；只更新本文与设计导航。

## 11. 验证与验收

### 11.1 重点测试矩阵

| 类别 | 必须验证 |
| --- | --- |
| CLI 路由 | mcp 直接进入服务；help/schema 静态返回；拒绝额外子命令、输出 flags；启动错误不污染 stdout |
| npm 包 | bin 声明、平台依赖和版本一致；可用平台、不支持平台、缺 optional dependency 分别诊断 |
| npx 全链路 | 冷缓存首次获取、热缓存、固定版本、本地旧包遮蔽；记录实际 Node/npm/包/binary 版本 |
| Catalog | 固定 5 工具，旧 proposal 工具名不注册；schema 字段白名单；生成 JSON 体积作为测量数据 |
| Search 分支 | answer/sources/docs 分别映射正确；禁止跨 options；mode 缺失/未知零调用；缺答案配置不阻断 sources/docs |
| Docs 阶段 | 零/一/多候选、ID 缺失、同时传 name/id、直接 ID 查询；不自动选择/二次调用，不把候选当正文 |
| Site 分支 | map/crawl defaults 独立；crawl 拒绝 map_options；map 不触发 crawl；保留 submitted |
| Readiness | 工具 available-any/fully-available 与 operation 状态一致；disabled provider 实际调用也被拒绝 |
| 参数回归 | JSON/CLI 映射后的 service 参数一致；显式 false、默认值、数组、超限、未知参数、null |
| 四版本协议 | discovery/初始化/直接现代调用、未知版本、结果字段过滤、字符串与数字 ID、ping |
| 无配置发现 | discovery/list 不创建配置、不联网；坏配置不阻断发现，status 保留诊断 |
| 结果与脱敏 | 完整正文、业务阶段、错误分类、文本/结构化一致；嵌套凭据、错误和 stderr 不泄漏 |
| 资源与关闭 | 并发超额、timeout、取消、EOF、panic、超长帧/超大结果；无遗留 launcher/binary/上游测试进程 |

不只用同一 SDK 的 client/server 自测：补充原始 wire fixture 或独立实现客户端。至少一个 legacy 客户端和一个现代独立客户端完成发现、调用、错误恢复与取消，记录版本和平台；不能把 Windows 终端能执行 npx 当作 Windows host 已适配。

### 11.2 离线测试与发布测试分层

- **默认离线层**：Go 测试使用 httptest/fake MCP、临时配置；launcher 参数测试 mock 子进程；不读取个人凭据。
- **本地打包层**：由待发布包和平台 binary 在隔离目录搭建 package fixture/本地 registry，使用隔离 npm cache 验证真实 npx 进程。全部测试中 stdin/stdout/stderr 分离，捕获从启动到关闭的完整流。
- **发布后确认层**：经授权发布包含 MCP 的真实版本后，验证公共 registry 上固定版本命令。此项不能由当前旧版公开包代替，也不在提案阶段运行。
- **provider live 层**：另行授权网络、凭据和可能的费用，不把本机 readiness 当 live 验证。

实施后执行：

```bash
mise exec -- go test -count=1 ./...
mise exec -- go vet ./...
mise exec -- go test -race ./internal/mcpserver ./internal/execution ./internal/config ./internal/service
```

race 至少在一个支持的工具链平台通过；再按 npm 三个平台验证打包与启动，其他 binary 平台按既有构建矩阵检查。若 host 启动超时不覆盖 npx 冷缓存，应记录预安装/预热方案，不默认为协议兼容失败。

### 11.3 本轮已执行的检查

- 已重新检查 npm 包、launcher、发布矩阵、CLI 路由、service 和 Context7 返回结构。
- Go `1.26.5` 下重新运行以下 4 个模块测试，全部通过：

```bash
mise exec -- go test -count=1 ./internal/commandcontract ./internal/cli ./internal/service ./internal/providers
```

- 使用当前 Node `v26.11.1` 对现有 launcher 执行 6 个隔离模拟用例：三种已声明平台 × mcp/mcp --help；确认 bin、平台依赖、argv 与 stdio 转发。
- launcher 检查 mock 了 spawnSync，不代表在三个操作系统上启动过真实进程，也不证明 Node 16 最低声明版本或 npx 下载链路已通过。
- 本轮未实现 Server、未引入 SDK、未运行公开 npx 下载、未联调 host、未访问真实 provider；本节只记录实际检查。

## 12. 风险、待确认项与回退

| 风险 | 控制措施 |
| --- | --- |
| 少工具却形成巨型 schema | 分支 options 分组、字段白名单、固定 dispatch；测量 schema 大小和代表任务成功率 |
| search 聚合误导为全 provider 自动路由 | 明确 sources=Exa、docs=Context7；不跨 mode fallback，新增多 provider 另立合同 |
| 合并后的 readiness 不准确 | 按 operation 判断，工具总体状态仅为聚合摘要 |
| docs 自动选错库或隐藏额外请求 | name 只解析、ID 才查询，无 server 选库状态 |
| site 合并掩盖 crawl 成本 | action 必填、不同默认值、保守 annotations、保留提交状态 |
| npx 旧版本/下载提示/平台缺包 | 固定版本、-y、隔离包验收、清晰诊断，公共发布后再验证 |
| 进程树退出不完整 | 分层测试 npx/launcher/binary/upstream；依据实测做局部生命周期修复 |
| 配置轮换或输出路径泄漏 | 请求快照、DTO 安全投影、复用脱敏并覆盖所有出口 |

### 12.1 实施前确认

已明确的要求是直接 `npx onesearch mcp` 启动；不再把额外子命令作为必选。建议采用 5 工具方案，仍需确认：

1. 首期本地 stdio 是否满足部署范围；远程 HTTP 若必要需另行安全设计。
2. 是否接受 sources/docs 延续原提案的 Exa/Context7 边界；如必须 provider-neutral，需要增加 service workflow 设计，不能仅改名称。
3. 正式支持的 host、操作系统及 Node/npm 组合，用于真实进程验收。
4. 实施阶段接受固定官方 SDK 及实际依赖图；正式包版本在发布时确定，不在文档中预造版本号。

### 12.2 兼容与回退

新增 MCP 是 opt-in，不迁移用户配置，不改既有 CLI 命令。停止 host 的 MCP 子进程即可继续使用 CLI；首次 npx 获取的软件包/cache 属于 npm 管理，不在工具内部自动删除。

本修订中的未发布工具名可直接调整，正式发布后 tool name、mode/action、输入/输出语义视为公共合同。未来扩展不能通过增加 action 变成任意命令执行；协议版本支持失败时缩小声明，不保留未通过的兼容承诺。

## 13. 官方参考资料

MCP/SDK 来源在本任务前一轮核对，npx 来源在本轮补充核对；核对日期均为 2026-10-15。固定 tag 和版本化规范作为依据，不以滚动页面替代实施期测试。

| 编号 | 来源 | 地址 |
| --- | --- | --- |
| R1 | MCP 2026-07-28 Versioning | `https://modelcontextprotocol.io/specification/2026-07-28/basic/versioning` |
| R2 | MCP 2026-07-28 Discovery | `https://modelcontextprotocol.io/specification/2026-07-28/server/discover` |
| R3 | MCP 2026-07-28 stdio | `https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/stdio` |
| R4 | MCP 2026-07-28 Tools | `https://modelcontextprotocol.io/specification/2026-07-28/server/tools` |
| R5 | MCP 2025-11-25 Lifecycle | `https://modelcontextprotocol.io/specification/2025-11-25/basic/lifecycle` |
| R6 | 官方 Go SDK README | `https://github.com/modelcontextprotocol/go-sdk` |
| R7 | 官方 Go SDK v1.8.0 发布记录 | `https://github.com/modelcontextprotocol/go-sdk/releases/tag/v1.8.0` |
| R8 | 官方 Go SDK v1.8.0 go.mod | `https://github.com/modelcontextprotocol/go-sdk/blob/v1.8.0/go.mod` |
| R9 | MCP 2025-06-18 结构化结果变更 | `https://modelcontextprotocol.io/specification/2025-06-18/changelog` |
| R10 | npm CLI v11 npx 命令、参数位置与安装确认 | `https://docs.npmjs.com/cli/v11/commands/npx/` |

5 工具划分、分支 options、只读快照和限额是本仓库的设计建议，不是 MCP 或 npm 规范强制要求。
