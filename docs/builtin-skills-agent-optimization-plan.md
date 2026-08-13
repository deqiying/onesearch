# Onesearch 内置 Skill 面向 Agent 优化方案

## 实施状态

状态：已按方案实施。主 Skill 的 canonical ID、asset folder、frontmatter name 与 metadata prompt 已统一为 `onesearch`，未保留旧名称作为 CLI alias。

后续 command manifest 已升级到 V2；targeted schema 的 command entry 使用 `name`、`description` 和 `input_schema`，并继续以 `path` 与 `x-cli-binding` 作为 argv 真源。详见 [CLI Command Manifest 与 Agent Tool 声明字段对齐技术方案](cli-command-manifest-tool-declaration-alignment-technical-design.md)。

已落地静态 `skills list/show`、`skills show --file`、targeted schema 驱动的 14 个 Skill、共享 agent execution reference、统一 metadata/drift tests、README/manifest 同步与发布后二进制 asset smoke。下文的问题分析与分阶段方案保留为实施前决策依据。

实施后已通过 `mise exec -- go test -count=1 ./...`、`mise exec -- go vet ./...`、`smoke --mock`、14 个 `quick_validate.py`、隔离配置的静态 discovery 测试，以及独立构建二进制的 targeted schema、Skill inventory、主 Skill、reference 和旧名称拒绝 smoke。真实 provider、网络、凭据、远端 MCP tool 与 npm registry 发布仍不在本次验证范围内。

## 推荐结论

本次优化不应继续把最新命令参数复制到 14 个 `SKILL.md` 中，而应明确以下职责分工：

1. `onesearch schema <canonical-path...>` 是命令路径、输入类型、默认值、约束、argv binding、副作用和 availability preflight 的机器真源。
2. `onesearch status --format json` 是当前 runtime capability/provider 可用性的真源；`doctor` 只用于整体配置异常诊断，不应成为每次调用的固定前置步骤。
3. 内置 Skill 只保留模型无法从 schema 推导的内容：触发条件、意图路由、证据策略、输出选择、字段读取方式、失败恢复和 provider 特有语义。
4. Agent 默认使用 compact JSON，不传 `--pretty`；需要正文时选择 `--format content`，需要完整诊断或未裁剪 payload 时才使用 `--verbose --format json`。
5. 普通调用禁止先读 full manifest。已知命令时使用 targeted schema；只有做 CLI 集成、合同审计或全量命令发现时才读取 full schema。
6. `skills list/show` 应改为与 `schema` 相同的静态发现路径，在 `config.Load()` 前执行，保证读取内置 Skill 不会创建用户配置。
7. `skills show` 当前只返回 `SKILL.md`。若保留 references 的渐进加载设计，应增加单文件读取能力；否则不得在公开 Skill 中依赖 agent 无法读取的 reference。
8. 用 command registry 驱动的合同测试替代当前以字符串包含为主的 Skill 测试，防止命令、flag、输出字段和文案再次漂移。

## 需求理解

本需求不是简单地在现有文案中补充 `schema` 和 `--pretty` 两段说明，而是要把内置 Skill 从“手写 CLI 使用手册”调整为“面向 agent 的决策与执行协议”。

优化后的 Skill 应解决四类问题：

- **选择什么**：根据用户意图选择 workflow 或 provider-direct command。
- **如何构造**：通过 targeted schema 获取当前 canonical path、输入约束和 argv binding，而不是依赖可能过期的参数列表。
- **如何读取**：区分 JSON 物理 compact、quiet 语义裁剪、verbose 完整诊断和 content/markdown 正文输出。
- **失败后做什么**：根据退出码、`error_type` 和 schema/status 信息采取最小恢复动作，避免无条件重跑 `doctor` 或盲目切换 provider。

## 当前事实基线

### CLI command schema 已具备机器发现能力

当前 `internal/commandcontract` 是公共命令合同的共同来源：

- `internal/commandcontract/registry.go` 聚合 workflow、provider 和 utility definitions。
- `internal/commandcontract/definition.go` 生成 manifest envelope、Draft 2020-12 input schema、`x-cli-binding`、constraints、availability、side effects 和 output contract。
- `internal/cli/schema_commands.go` 支持 full manifest 和 canonical path targeted query。
- `internal/cli/cli.go` 在 `config.Load()` 前分派 `schema`，因此 schema/help 不创建配置、不读 provider key、不调用网络。

当前 manifest 包含 44 个 public executable commands：6 个 workflow、27 个 provider-direct、11 个 utility。Targeted query 只返回一个与 full manifest 对应 entry 深度相等的 command definition。

Schema 的公共行为为：

| 场景 | 当前合同 |
| --- | --- |
| `schema` | 返回 full CLI command manifest |
| `schema <canonical-path...>` | 返回一个 targeted command entry |
| alias 作为 targeted path | 返回 `parameter_error`，退出码 2 |
| 非 JSON format、未知 flag、多余参数 | 返回结构化 `parameter_error`，退出码 2 |
| 默认 JSON | 单行 compact，并保留一个结尾 LF |
| `--pretty` | 两空格缩进，不改变 JSON 语义 |
| `--output` | 写入与 stdout 完全相同的 bytes，同时仍会输出 stdout |

一次性测量当前版本时，full schema compact 约 105 KB，pretty 约 191 KB；`schema search` 约 5.3 KB，`schema exa web-search` 约 4.0 KB。该测量只用于说明 targeted query 的上下文收益，不作为固定验收阈值。

### JSON compact 包含两种不同层次

当前实现必须继续区分：

| 层次 | 控制内容 | 当前实现 |
| --- | --- | --- |
| 物理排版 | JSON 是否缩进、换行 | 默认 compact；`--pretty` 才缩进 |
| 语义裁剪 | 返回哪些字段、正文是否变成 preview、诊断是否完整 | 默认 quiet；`--verbose` 保留完整诊断/payload |

`internal/output/output.go` 的真实处理顺序是：

1. 对结构化数据脱敏。
2. 非 verbose 时执行 error/search/provider 的语义裁剪。
3. 按 `Pretty` 决定 JSON 是否缩进。
4. 对最终文本再次脱敏。

因此：

- `--pretty` 只改变空白，不能用于获取更多字段。
- `--quiet` 与默认正常日志级别下的 JSON 都可能只保留 preview。
- `--verbose` 默认仍输出单行 compact JSON。
- provider-direct 结果只要带有 `tool`，默认 JSON 就可能把 `content` 转成 `content_preview` 和 `content_length`。
- `--format content` 或 `--format markdown` 用于正文消费，不应通过 `--pretty` 解决正文缺失。
- `--output` 只是额外持久化副本；因为 stdout 仍会输出相同内容，它本身不是 agent 上下文节省开关。

### 内置 Skill 的实际分发边界

`internal/skills/skills.go` 使用 `//go:embed assets/**` 嵌入 14 个 Skill 目录。当前公开 CLI 行为为：

- `skills list` 从手写 `definitions` 返回 ID、aliases、capabilities 和 description。
- `skills show` 通过 `ReadMarkdown()` 只返回目标 `SKILL.md`。
- `LoadFiles()` 可以读取 `agents/openai.yaml` 和 references，但仓库生产代码没有调用它。
- npm 和 Release 流程只分发二进制，没有独立 Skill package 安装或导出链。

因此，实施前真正可由 CLI agent 消费的只有 `SKILL.md`。主 Skill 末尾要求读取一个未被 `skills show` 暴露的长 reference，这不是有效的渐进加载路径。

### `skills list/show` 的副作用与文档冲突

当前 `internal/cli/cli.go` 仅对 `schema` 做 pre-config 静态分派；`skills list/show` 会继续执行 `config.Load()`。`config.Load()` 在配置不存在时调用 `EnsureInitialized()` 写入初始 `config.json`。

这与以下公开描述冲突：

- README 将 `skills show` 描述为不联网、不写文件。
- router reference 将 `skills show` 描述为不读用户 provider config、不写配置。
- Skill 内容实际来自 `embed.FS`，本身不依赖 runtime config。

当前 manifest 反而如实声明了 `skills.list` 与 `skills.show` 的 `config_initialize_when_missing`。这说明问题不是 schema 不准确，而是公开 Skill/README 与实现语义不一致。

推荐修正实现，让纯 Skill 发现保持静态只读，而不是把不必要的配置写入补写进更多文案。

## 现存问题

### 1. 主路由把条件步骤写成近似固定流水线

`onesearch/SKILL.md` 当前依次建议 `skills list`、`skills show`、`doctor`、`status`、`schema`。如果 agent 机械执行，会产生重复调用和大量上下文：

- 已加载 router 后通常不需要再次 `skills list`。
- 已知具体 Skill 后不需要再次 `skills show` 同一内容。
- 健康状态明确时不需要先跑 `doctor`。
- full `schema` 对单条命令远大于 targeted query。
- 同一任务内 runtime config 未改变时，不应在每个 provider command 前重复读取完整 status。

### 2. 子 Skill 仍把参数表当主要合同

provider Skill 中存在大量手写 Options 列表；workflow Skill 又重复 provider command 示例。当前内容大体与源码一致，但这类文案会随着 command contract 演进而产生第三份或第四份事实来源。

Schema 已经能够提供：

- canonical path 与 aliases；
- positional、required、variadic、enum、min/max；
- flag token、repeatable/list encoding、override；
- mutual exclusion constraints；
- side effects；
- availability check command 和 JSON pointer；
- output formats、variants 和 provider payload 是否 opaque。

这些字段不应继续由每个 Skill 手工完整复述。

### 3. 输出读取规则不一致

当前各 Skill 对默认 JSON 的说明不统一：

- `search` 已说明 `used.*`、`content_preview` 和 `--verbose`。
- Exa、DeepWiki 已部分说明 quiet preview。
- Context7 仍无条件描述输出包含 `content`，但默认 provider-direct JSON 会被 renderer 裁剪为 preview。
- Tavily、Firecrawl、Zhipu 等未统一说明何时选择 JSON、content、markdown 或 verbose。

这会让 agent 把“字段不存在”误判为 provider 无结果，或为了取正文无条件使用 `--verbose`，重新引入高 token payload。

### 4. 错误恢复没有形成可执行决策表

当前 compact error 普遍给出“加 `--verbose` 或运行 doctor”的通用 hint，但不同错误需要不同恢复方式：

- 参数错误通常应读取 targeted schema 或 help，不需要 doctor。
- 配置错误需要 doctor/status，并且只有用户授权时才能写配置。
- 网络/证据错误才需要一次 verbose 诊断、fallback 或补抓取。
- 本地/render/write 错误不应自动无限重试。

Skill 应把退出码与动作绑定，而不是只解释错误字段。

### 5. Reference 与 metadata 没有可验证的消费链

- 实施前的长 CLI reference 超过 400 行，与 router、README 和 schema 存在大量重复，但当时 CLI 无法按需返回它。
- 14 个 `agents/openai.yaml` 的结构不一致：router 与其余 Skill 使用不同字段形态。
- 当前仓库代码不解析这些 YAML，也没有发布后二进制的 asset smoke。
- `skills.go` description、`SKILL.md` frontmatter description、`agents/openai.yaml` UI metadata 是三套手写文本，没有一致性校验。

## 目标

1. 建立 schema-first、targeted-first、compact-first 的 agent 执行协议。
2. 让每个 Skill 只拥有其不可由 schema/status 推导的语义。
3. 减少正常调用链中的 CLI 往返、重复 status、full manifest 和 verbose 输出。
4. 统一所有 workflow/provider Skill 的输出读取与失败恢复规则。
5. 让 `skills list/show` 成为真正静态、无配置初始化的发现命令。
6. 让 references 具备真实的按需可达路径，或移除公开 Skill 对不可达 reference 的依赖。
7. 统一 Skill frontmatter、UI metadata 和 CLI inventory 的责任边界。
8. 建立 schema/registry 驱动的自动化 drift 检查和发布后二进制 smoke。

## 非目标

1. 不修改 provider 网络协议、runtime routes、凭据读取或 provider payload。
2. 不修改 JSON compact/pretty 的既有物理格式合同。
3. 不把 provider opaque payload 伪装成完整稳定 output schema。
4. 不新增自动业务垂直意图识别；垂直意图仍由调用 agent 判断。
5. 不把 `status.available` 描述为网络、凭据或远端 MCP tool 已验证。
6. 不新增 Skill 安装器、插件市场或独立 npm Skill package。
7. 不为减少上下文而默认启用 `--verbose`、`--pretty` 或 full schema。
8. 核心方案不新增 `status --capability`、`status --provider` 等 CLI filter；先通过条件预检和一次性 status 快照优化，若仍有真实上下文瓶颈再单独设计。
9. 不把 `--output` 描述成静默输出；当前合同仍是文件与 stdout 同时写入。

## 目标责任模型

| 信息类型 | 唯一权威 | Skill 如何使用 |
| --- | --- | --- |
| canonical command path | command registry / targeted schema | Skill 只列稳定入口，不维护 alias 变体表 |
| positionals、flags、default、enum、约束 | targeted schema `input_schema` / `constraints` | 非平凡 argv 构造前读取，不复制完整 Options 表 |
| argv token 绑定 | `x-cli-binding` / `command_argv` | 生成 token 数组，不按空格拆 display command |
| 命令副作用 | targeted schema `side_effects` | 写配置、写文件、网络前据此判断是否需授权/提示 |
| 静态/动态可用性 | targeted schema `availability` | 仅 dynamic command 进入 status preflight |
| 当前 provider/capability 状态 | `status` | 同一任务复用一次快照；配置改变后再刷新 |
| runtime routes/config | `config list` | 仅在需要解释或修改运行时路由时读取 |
| JSON 排版 | output contract | Agent 默认 compact；人工才 pretty |
| quiet/verbose/content 选择 | output renderer + Skill | Skill 给出任务导向的最小 payload 选择规则 |
| 证据与 provider 业务语义 | workflow/provider Skill | 保留在 Skill 中 |
| 失败恢复 | router Skill + command error envelope | 按 exit code/error_type 决策 |
| 触发与 UI 文案 | `SKILL.md` frontmatter、`agents/openai.yaml` | 使用合法且一致的 skill metadata |

## 目标 Agent 执行流程

### 条件式流程

```text
用户意图
  -> 已知 workflow/provider Skill？
       是：直接进入该 Skill
       否：skills list --capability；再 show 一个最具体 Skill
  -> 已知 canonical path 且只用基础 recipe？
       是：不读取 full schema
       否：schema <canonical-path...>
  -> availability.dynamic == true？
       是：读取或复用 status 中对应 JSON pointer
       否：跳过 status
  -> 选择最小输出形态并执行
  -> 先检查进程退出码
  -> format=json：将 stdout 解析为一个 JSON document，再检查 ok/error_type
  -> format=content|markdown：将 stdout 作为文本消费；失败时按退出码处理，必要时用 JSON 重取一次诊断
  -> 搜索候选需要形成结论时，再 fetch 关键 URL
```

### 场景矩阵

| 场景 | 推荐动作 | 明确不做 |
| --- | --- | --- |
| Skill 已加载、命令与基础参数明确 | 必要时复用 status，然后直接执行 compact JSON recipe | 不重复 list/show，不读 full schema |
| 已知命令，但需要可选 flag 或约束 | `schema <canonical-path...> --format json` | 不 grep help 文本反推参数 |
| 只知道 capability | `skills list --capability ...`，选择一个最具体 Skill | 不加载全部 Skill 正文 |
| 需要确认 provider | 读取/复用 status 对应 capability/direct endpoint | 不把 Skill 存在当作可用 |
| 参数错误/exit 2 | targeted schema；修正 canonical path/flag/constraint | 不先跑 doctor |
| 配置错误/exit 3 | doctor；读取相关 status；需要写配置时请求用户授权 | 不自动写 key 或路由 |
| 网络/证据错误/exit 4 | 一次 `--verbose` 获取诊断；按 status/fallback 选择替代；必要时补 fetch | 不无限重试同一 provider |
| 其他/exit 5 | JSON 可用时先按 `error_type` 区分 auth、rate limit、timeout、provider、empty result 与 local failure | 不把所有 code 5 都当作本地 I/O 错误 |
| 人工检查 JSON | 显式 `--pretty` | Agent 默认不使用 pretty |
| 需要正文 | `--format content` 或适用时 markdown | 不把 `--pretty` 当正文开关 |
| 需要完整结构化诊断 | `--verbose --format json` | 不为普通成功结果默认 verbose |
| 全量 CLI 集成/审计 | full schema；必要时持久化并由宿主控制 stdout | 不把 full schema 作为普通调用前置 |

## Skill 内容重构方案

### `onesearch`：从长清单改为最小路由协议

保留：

- frontmatter 中完整、可触发的 web/current/docs/fetch/repo research 描述；
- workflow 与 provider-direct 的选择规则；
- capability 到具体 Skill 的映射；
- targeted schema、conditional status、compact output 和 error recovery；
- retired public contract：禁止 `onesearch mcp <tool>`、flat provider command 和 provider snake_case subcommand；结果中的上游 `tool` 字段不能反向当作 CLI path；
- 凭据只能通过 hidden TTY 或显式 stdin，不能出现在 argv、日志或回答中。

删除或收敛：

- 把 `skills list -> skills show -> doctor -> status -> schema` 写成固定编号流程；
- 重复列出每个 Skill 的多组调用示例；
- schema 已能表达的完整 flag/default/constraint 表；
- 与 reference 重复的 public command surface 大清单；
- “如果命令可能变化，再 show 当前 Skill”这类自引用调用。

新增简短硬规则：

1. 已知 path 时优先 targeted schema，禁止普通任务默认读取 full schema。
2. `schema` 是 CLI manifest，`config list` 是 runtime schema，二者不可混用。
3. `--format json` 时将 stdout 作为一个 JSON document 解析，不按行当作 JSONL；content/markdown 直接按文本消费。
4. 默认不传 `--pretty`。
5. 先检查进程 exit code；JSON 模式再检查 `ok` 和 `error_type`，文本模式失败时不得尝试 JSON decode。
6. status 快照在同一任务、配置未改变时可以复用。

### Workflow Skill：保留任务策略，减少 provider 细节重复

#### `onesearch-search`

保留：

- current/latest/hot-list/ranking 等触发语义；
- `answer_search` 是综合回答、`source_search` 是候选来源、`page_fetch` 是正文证据；
- 默认读取 `used.<capability>.providers.<provider>.result`；
- `content_preview/content_length`、`sources`、`pages` 的读取方法；
- 高风险结论必须 fetch 关键 URL。

优化：

- 基础 recipe 保留 2 至 3 个；其他 flags 统一指向 `schema search`。
- 不在 workflow Skill 中重复维护 Tavily/Exa/Zhipu 的 option 列表。
- 明确完整答案正文优先 `--format content`；只有需要 routing decision/provider attempts 时使用 `--verbose`。

#### `onesearch-docs`

保留：

- Context7 先 resolve library ID，再 query focused docs；
- Context7 缺覆盖时使用 Exa 找官方 docs；
- exact API claim 必须基于返回片段或官方页面。

优化：

- 为 `context7 resolve-library-id`、`context7 query-docs`、`exa web-search` 提供 targeted schema 入口。
- 修正默认 provider-direct JSON 不保证保留完整 `content` 的描述。
- 明确 snippets/list 用 compact JSON，正文用 content，调试才 verbose。

#### `onesearch-fetch`

保留：

- search result 是候选，fetch/extract 才是页面证据；
- fetch/map/crawl 的意图区分；
- crawl 必须设置有界 limit/depth。

优化：

- 将 provider-specific flags 移回对应 provider Skill 或 targeted schema。
- 明确 page body 应使用 content/markdown；compact JSON 用于 URL、provider、状态和 preview。
- config error 才进入 doctor/status，不为普通参数错误运行 doctor。

#### `onesearch-deep-research`

保留：

- `deep` 是离线 planner，不执行 provider；
- `preflight[].command_argv` 与 `steps[].command_argv` 是机器合同；
- `command` 只是 PowerShell display string，禁止按空格拆分；
- 执行前检查 availability，执行后做 gap check。

优化：

- 对 planner 返回的每个非基础 command，按 canonical path 读取 targeted schema 或由 registry parse-only 校验。
- status 只在开始执行动态 steps 前读取一次，配置未变时复用。
- 不把 planner 输出中的 placeholder 直接执行。

### Provider Skill：只保留 provider 特有语义

所有 9 个 provider Skill 使用同一骨架：

1. **何时选择该 provider**：只写 provider 特有优势和显式触发词。
2. **Canonical commands**：只列叶子 path，不列旧 alias。
3. **Contract discovery**：非基础调用前执行 targeted schema。
4. **Availability**：读取 manifest 中的 status JSON pointer；同一 status 快照可复用。
5. **Output choice**：说明 compact JSON、content/markdown、verbose 的选择。
6. **Provider-specific semantics**：只保留 schema 无法表达的异步 job、证据等级、direct-only、远端工具边界等内容。
7. **Failure recovery**：config、network、remote tool missing 分开处理。

建议的 targeted schema 映射：

| Skill | Canonical paths |
| --- | --- |
| `exa` | `exa web-search`、`exa web-fetch`、`exa similar` |
| `tavily` | `tavily search`、`tavily extract`、`tavily map`、`tavily crawl` |
| `firecrawl` | `firecrawl search`、`firecrawl scrape`、`firecrawl map`、`firecrawl crawl` |
| `context7` | `context7 resolve-library-id`、`context7 query-docs` |
| `deepwiki` | `deepwiki ask-question`、`deepwiki read-wiki-structure`、`deepwiki read-wiki-contents` |
| `anysearch` | `anysearch domains`、`anysearch search`、`anysearch extract`、`anysearch batch` |
| `zhipu` | `zhipu search` |
| `ddg` | `ddg search`、`ddg fetch-content` |
| `freecrawl` | `freecrawl search`、`freecrawl scrape`、`freecrawl crawl`、`freecrawl deep-research` |

Provider Skill 中保留的非 schema 语义示例：

- Firecrawl crawl 首次返回异步 job envelope，不承诺立即返回页面正文。
- DDG/Freecrawl 默认 direct-only，不能因为 Skill 存在就进入普通 workflow route。
- MCP stdio status 只证明本地配置与命令查找，不证明上游 package、浏览器依赖、network 或 remote tool 可用。
- DeepWiki 内容属于生成的 repository documentation；本地源码可用时仍以源码为准。
- AnySearch 保持显式 vertical/experimental provider，不自动等同于默认 `source_search`。
- Search result 一律是发现候选，关键事实仍需 fetch 正文。

## 统一输出协议模板

每个会执行 CLI 的 Skill 都应包含一个短版输出协议，不再各自自由描述：

```text
- Agent 默认使用 `--format json`，得到单个 compact JSON document。
- 不传 `--pretty`；pretty 只供人类查看，不增加字段。
- JSON 模式先检查退出码，再读取 `ok`、`error_type` 和命令上下文字段。
- provider/search 正文可能只有 `content_preview` 和 `content_length`。
- 需要正文时使用 `--format content` 或适用的 `--format markdown`。
- content/markdown stdout 是文本而不是 JSON；非零退出时按失败处理，必要时用 `--verbose --format json` 重取一次结构化诊断。
- 需要完整结构化诊断、raw provider fields 或 routing attempts 时使用 `--verbose --format json`。
- `--output` 会额外写文件，但不会抑制 stdout。
```

该模板只在 router 中完整出现；子 Skill 使用 2 至 4 行与本命令相关的精简版本，避免重复占用上下文。

## 统一错误恢复协议

退出码只提供粗分类，Agent 必须在 JSON 可用时优先依据 `error_type` 决策。当前 `ExitCode()` 只对 `parameter_error`、`config_error`、`network_error` 和 `evidence_error` 做专门映射，其他 provider/local 类型都会落入退出码 5，因此不能把 5 等同于本地 I/O 失败。

| Exit code | `error_type` | Agent 动作 |
| --- | --- | --- |
| 0 | success | JSON 模式解析一个 document 并检查 `ok`；content/markdown 模式直接消费文本 |
| 2 | `parameter_error` | 读取 targeted schema；改用 canonical path；修正 required/enum/constraint；不运行 doctor |
| 3 | `config_error` | 运行 doctor；读取/刷新相关 status；需要 `config setup` 或改 route 时先取得用户授权 |
| 4 | `network_error` | 最多用 verbose 重取一次诊断；依据 status/fallback 换可用 provider |
| 4 | `evidence_error` | 增加或更换 source，fetch 关键页面，或降低无法证明的结论 |
| 5 | `auth_error` | 不盲目重试；检查 doctor/status 的凭据状态，只有用户授权时更新配置 |
| 5 | `rate_limited` | 遵守上游等待语义；减少请求或切换可用 provider，不做紧密循环重试 |
| 5 | `timeout` | 最多一次调整有界 timeout 或切换 provider；保留超时上下文 |
| 5 | `provider_error`、`upstream_error`、`protocol_error` | 用 verbose 获取一次 provider 诊断，然后 fallback 或报告上游失败 |
| 5 | `empty_result` | 视为没有可用证据；调整 query/source/provider，不按本地故障排查 |
| 5 | `local_error` | 检查本地输入、路径、权限、embedded asset、render/write；不伪装成功 |
| 5 | 未知或没有 JSON envelope | 保留 stdout/stderr 和命令上下文，停止自动重试并报告未分类失败 |

需要同步修正文案：普通 compact error 的通用 hint 可以继续存在，但 Skill 的决策表优先于“任何错误都 doctor”的模糊建议。

## Skill 发现与渐进加载设计

### 让 `skills list/show` 静态执行

推荐在 `internal/cli/cli.go` 中于 parse 成功后、`config.Load()` 前增加静态分派：

- `skills.list`：调用 `skillsListData()`，使用 `printCommand(nil, ...)`。
- `skills.show`：调用 `skillShowData()`，使用 `printCommand(nil, ...)`。
- schema 保持独立 static encoder，不强行合并到普通 renderer。
- 默认 verbosity 固定为 quiet，显式 `--quiet`/`--verbose` 仍按 command contract 生效，不再受 runtime `defaults.log_level` 影响。

同时：

- `internal/commandcontract/utilities.go` 不再通过自动追加 `config_initialize_when_missing` 的通用 helper 定义这两个命令。
- manifest side effects 保留真正存在的 embedded asset read，以及显式 `--output` 时的文件写入；移除 config initialization。
- README、router Skill 和 reference 统一声明它们不读 runtime config、不创建 config、不联网。
- 隔离 `ONESEARCH_CONFIG_DIR` 测试必须证明 success、unknown skill 和参数错误路径都不创建 `config.json`。

### 给 reference 一个真实的读取入口

推荐扩展现有命令，而不是新增安装系统：

```powershell
# 目标接口，当前尚未实现
onesearch skills show onesearch --file references/agent-execution-contract.md --format content
```

设计约束：

- `--file` 默认 `SKILL.md`，保持现有调用兼容。
- 只允许读取目标 Skill embed root 下已存在的相对文件；拒绝绝对路径、`..` 和未知文件。
- `--format content` 返回原始文本；JSON 返回 metadata、relative path 和 content。
- 读取仍在 config 之前完成，不联网、不写文件；只有显式 `--output` 产生写入。
- 不提供 `--all-files` 默认大对象，避免一次加载 UI metadata 和全部 references。

将当前 412 行 `cli-contract.md` 收敛为按需的 `agent-execution-contract.md`：

- 删除 schema 已拥有的完整 command/flag/default 列表。
- 保留 output envelope、error recovery、runtime/status 边界、evidence policy 和 `command_argv` 规则。
- 如果仍超过 100 行，在文件开头增加目录。
- router `SKILL.md` 只说明何时读取该 reference，不重复正文。

若不接受新增 `--file`，则本阶段必须改用备选方案：把执行关键规则全部放回 `SKILL.md`，并删除公开文案对 reference 的依赖。不能继续保留“要求读取但无入口”的状态。

## Metadata 优化

按 Codex Skill 结构约束处理 14 个 Skill：

- `SKILL.md` frontmatter 只保留 `name` 和 `description`。
- description 同时描述能力和触发场景；正文不再重复大段 “Use When”。
- `agents/openai.yaml` 统一使用合法的 `interface.display_name`、`interface.short_description`、`interface.default_prompt` 结构。
- UI metadata 的字符串值统一加引号，`short_description` 控制在 25 至 64 个字符。
- `default_prompt` 必须显式提及 `$<frontmatter.name>`。
- 不在没有消费方证据时添加 icon、brand color、MCP dependency 或 invocation policy。
- 使用确定性生成器更新 metadata，并在 Skill 内容变化后重新生成。

`internal/skills/skills.go` 继续只拥有 CLI inventory 需要的 folder、aliases 和 capabilities；description 与 frontmatter 的语义一致性通过测试锁定。第一阶段不为此引入新的 YAML runtime dependency，也不在每次 `skills list` 时动态解析 YAML。

## 文件改动规划

| 文件/目录 | 规划改动 |
| --- | --- |
| `internal/skills/assets/onesearch/SKILL.md` | 重构为条件式 router、targeted schema、compact/output、error recovery 与 retired command 规则 |
| `internal/skills/assets/onesearch/references/agent-execution-contract.md` | 收敛 agent execution reference，删除 schema 可推导的重复事实 |
| `internal/skills/assets/onesearch-{search,docs,fetch,deep-research}/SKILL.md` | 统一 workflow 结构、输出选择、targeted schema 与证据策略 |
| `internal/skills/assets/onesearch-{exa,tavily,firecrawl,context7,deepwiki,anysearch,zhipu,ddg,freecrawl}/SKILL.md` | 统一 provider 骨架，删除完整手写 options 表，修正 compact/正文语义 |
| `internal/skills/assets/*/agents/openai.yaml` | 按统一 UI metadata schema 重新生成 |
| `internal/skills/skills.go` | 增加按相对路径读取单个嵌入文件的安全 API；保持 aliases/capabilities inventory |
| `internal/skills/skills_test.go` | 增加目录/definition、frontmatter/metadata、reference、canonical command 与公共规则一致性测试 |
| `internal/cli/skills_commands.go` | 支持目标 Skill 单文件读取；明确 list/show error envelope |
| `internal/cli/cli.go` | 在 `config.Load()` 前静态分派 skills list/show |
| `internal/cli/command_bindings.go` | 移除 list/show 的 runtime binding，避免静态命令回落到 service/config 路径 |
| `internal/commandcontract/utilities.go` | 更新 skills show input schema；修正 list/show side effects |
| `internal/cli/cli_test.go` | 覆盖静态发现无 config、file 读取、compact/pretty/content/error |
| `internal/cli/testdata/cli-command-manifest-v2.golden.json` | 通过显式 update gate 维护当前 skills show option、side effects 与 V2 command fields |
| `internal/output/output.go`、`internal/output/output_test.go` | 按 `error_type` 输出恢复 hint，确保 parameter error 不误导 agent 运行 doctor |
| `README.md` | 更新 agent 最小调用链、Skill 静态发现、reference 读取和输出/恢复规则 |
| `npm/onesearch/README.md` | 同步 npm 用户需要的最小 agent contract |
| `npm/deqiying-onesearch/README.md` | 与 unscoped npm README 保持一致 |
| `.github/workflows/npm-publish.yml`、`scripts/package-npm.ps1` | 对最终二进制增加 schema/skills asset smoke，而不仅验证 `--version` |
| `.gitignore` | 仅放行 `scripts/package-npm.ps1`，使 Windows package smoke 成为可交付文件 |

若实现时选择“不新增 `skills show --file`”备选方案，则删除上表对应 ReadFile/CLI/commandcontract/golden 变更，但必须同步删除不可达 reference 依赖。

## 分阶段实施方案

### 阶段一：冻结责任边界和行为矩阵

1. 以当前 44-command manifest 为事实基线。
2. 为 14 个 Skill 建立“保留的语义 / 交给 schema 的事实 / 交给 status 的动态状态”清单。
3. 固定 agent normal path：route -> targeted schema if needed -> conditional/reused status -> compact execution -> typed recovery。
4. 固定 full schema、doctor、verbose 和 pretty 的例外使用条件。
5. 决定 reference 采用 `--file` 可达方案或 SKILL 自包含备选方案；实现中不得混用。

### 阶段二：修复 Skill 静态发现合同

1. 将 skills list/show 移到 config load 前执行。
2. 修正 command definition side effects 和 manifest golden。
3. 增加隔离配置目录测试，覆盖 list/show/unknown skill/parse error。
4. 保持显式 `--output` 的文件写入和 stdout byte equality。
5. 更新 README 与 contract 中的只读语义。

### 阶段三：重构主 router 和 shared reference

1. 先改 `onesearch/SKILL.md`，把固定前置清单改为条件式决策表。
2. 加入 targeted-first、compact-first、typed error recovery 和 retired command 规则。
3. 删除 full command/flag 重复列表。
4. 收敛 shared reference，只保留非 schema 的 agent execution contract。
5. 若采用 `--file`，实现单 reference 读取和路径安全校验。

### 阶段四：按 workflow/provider 批量但逐项迁移

1. 先处理 4 个 workflow Skill，固定输出字段和证据策略。
2. 再处理 9 个 provider Skill，使用统一骨架但保留 provider 特有边界。
3. 删除自引用 `skills show <same-skill>` 和完整手写 Options 表。
4. 每个 Skill 保留少量高价值 recipe，并为非基础 flags 给出 targeted schema command。
5. 修正 Context7、Tavily、Firecrawl、Zhipu 等默认 JSON/正文描述。

### 阶段五：Metadata 与 drift test

1. 重新生成 14 个 `agents/openai.yaml`。
2. 校验 `Definition.Folder == frontmatter.name`；`Definition.ID` 保持独立、唯一的 CLI 短 ID，并能解析到对应 folder。
3. 校验 aliases 和 capabilities 唯一、可归一化。
4. 校验每个 Skill 声明的 canonical path 都存在于 command registry。
5. 校验 agent 示例不使用 `--pretty`，不使用 retired/alias path。
6. 校验 dynamic command 的 Skill 包含 status/availability 边界。
7. 校验所有 provider/workflow Skill 说明默认 compact 与正文/verbose 的选择。

### 阶段六：发布与 forward test

1. 运行定向和全量 Go 测试、vet、mock smoke。
2. 对最终构建二进制执行 `schema`、`skills list`、`skills show` 和 reference 读取 smoke。
3. 默认使用 fake/mock provider 结果与隔离配置验证：普通最新新闻、Context7 文档、Firecrawl crawl、DDG unavailable、参数拼错、config error、evidence error；真实 provider forward test 只在用户明确授权并提供测试环境后执行。
4. Forward test 只提供构建后的 Skill 和用户任务，不泄露预期步骤或本方案结论。
5. 对比调用次数、是否读取 full schema、是否重复 status、是否误用 pretty/verbose、是否正确按错误类型恢复。

## 测试方案

### Skill 资产与 metadata

至少覆盖：

1. 14 个 asset folder 与 `definitions` 双向一一对应，无 orphan 或缺失。
2. 每个 `SKILL.md` frontmatter 仅含合法 `name`、`description`，且 `Definition.Folder == frontmatter.name`；不要求 CLI 短 ID 与 Skill name 字符串相等。
3. 每个 `agents/openai.yaml` 使用统一 interface 结构，default prompt 提及准确 `$<frontmatter.name>`。
4. references 只存在于明确会读取它们的 Skill，并可通过目标接口获取。
5. `LoadFiles`/单文件读取保持稳定排序、LF 和路径隔离。

### Schema 与 Skill drift

至少覆盖：

1. Skill 中声明的 canonical command path 均能由 registry `LookupCanonical()` 解析。
2. Skill 中的 targeted schema path 不使用 alias。
3. 高价值 recipe 的 argv 由 `BuildArgv()` 生成或经过 parser round-trip。
4. 不出现 `onesearch mcp`、flat provider command、provider snake_case subcommand。
5. provider Skill 的 capability 与 `skills.go`、command definitions 一致。
6. 动态 command 的 availability check 仍指向有效 status JSON pointer。
7. intentional command contract 变化必须显式更新 pretty golden 并人工审查。

### Skill 发现 CLI

至少覆盖：

1. 隔离配置目录运行 `skills list` 后没有 `config.json`。
2. `skills show <name>` 的 JSON、content、pretty 与 error 路径都不创建配置。
3. `skills show --file` 只读目标 embed root 内文件，拒绝绝对路径、`..`、未知文件和其他 Skill 文件。
4. `--output` 与 stdout byte-for-byte 相同；只有显式 output path 产生文件写入。
5. 未知 Skill/file 返回 compact `parameter_error` 和退出码 2。

### Output 与恢复文案

至少覆盖：

1. 所有 agent recipe 使用默认 compact JSON，不追加 `--pretty`。
2. router 明确 JSON stdout 是单个 document 而不是 JSONL，并明确 content/markdown stdout 是普通文本。
3. router 明确 pretty 不改变 payload，output 不抑制 stdout。
4. workflow/provider Skill 不再把默认 quiet JSON 描述为必然包含完整 `content`。
5. exit 2/3/4/5 的动作矩阵均存在，参数错误不建议先 doctor；code 5 至少区分 auth、rate limit、timeout、provider/upstream、empty result、local 和未知类型。
6. `command_argv` 与 display `command` 的边界在 deep-research Skill 中保持明确。

### 项目验证命令

实现后至少执行：

```powershell
mise exec -- go test -count=1 ./internal/skills ./internal/commandcontract ./internal/cli ./internal/output
mise exec -- go test -count=1 ./...
mise exec -- go vet ./...
mise exec -- go run .\cmd\onesearch schema skills show --format json
mise exec -- go run .\cmd\onesearch skills list --format json
mise exec -- go run .\cmd\onesearch skills show onesearch --format content
mise exec -- go run .\cmd\onesearch smoke --mock --format json
git diff --check
```

若采用 `--file` 方案，再执行：

```powershell
mise exec -- go run .\cmd\onesearch skills show onesearch --file references/agent-execution-contract.md --format content
```

文档验证还必须扫描 README、`docs/**`、npm README 和 `internal/skills/**/*.md` 中的 Windows drive path、UNC path 和 Unix user-directory path，并人工排除 URL、HTTP API path 与 Context7 `/org/project` ID。

## 验收标准

1. 14 个内置 Skill 具有清晰且互不重叠的 router/workflow/provider 职责。
2. 普通 agent happy path 不要求读取 full schema、重复 show 当前 Skill、同时运行 doctor 与 status。
3. 已知命令的非基础 argv 通过 targeted schema 构造；Skill 不再维护完整 Options 表。
4. Agent 示例默认使用 compact JSON，未使用 `--pretty`。
5. 所有 Skill 正确区分 JSON 物理 compact 与 quiet/verbose 语义裁剪。
6. 需要正文、markdown、完整诊断时的格式选择清晰且一致。
7. router 以 `error_type` 为主、exit code 为粗分类提供可执行恢复动作；参数错误不会误导 agent 先运行 doctor，退出码 5 不会被一律误判为本地 I/O 错误。
8. `skills list/show` 不创建或读取 runtime config，不访问网络；manifest side effects 与实现一致。
9. references 要么可以单文件按需读取，要么公开 Skill 不再依赖不可达 reference。
10. Skill 中不存在旧 `mcp`、flat provider 或 snake_case provider CLI 用法。
11. `status.available` 始终被描述为本地 preflight，不被当作远端成功证明。
12. `--output` 不被描述为静默/token-saving 开关。
13. frontmatter、UI metadata、Definition ID、folder、aliases 和 capabilities 通过映射一致性检查，不要求 CLI 短 ID 与 Skill name 字符串相等。
14. command registry/Skill drift、静态发现、output 文案和 deep argv 均有自动化测试。
15. 源码测试、全量测试、vet、mock smoke、发布二进制 asset smoke、文档路径检查和 `git diff --check` 全部通过。

## 风险与取舍

### Targeted schema 仍有数 KB

Targeted schema 比 full manifest 小很多，但仍不应在每个简单 recipe 前机械读取。Skill 应明确“基础 recipe 可直接执行；需要可选参数、约束、副作用或 preflight 细节时再读 targeted schema”。

### status 仍可能较大

当前 status 返回全量 runtime capability/provider 状态。核心方案先通过 conditional status 和同任务快照复用减少重复调用，不立即扩大 CLI 增加 filter。若 forward test 仍显示 status 是主要上下文瓶颈，再单独设计 `status --capability` / `--provider`，并保证 filtered object 与全量 JSON pointer 对应值一致。

### 静态分派改变 manifest side effects

将 skills list/show 移到 config 前会改变两条 command 的 side effects，但这是去除不必要副作用并修复公开合同，不是隐藏兼容变化。需要显式更新 golden、README 和 release note。

### Reference 读取扩展会改变公共 CLI

`skills show --file` 会新增一个 public option，需要同步 schema/golden/help/npm docs。如果维护者不希望扩大命令面，应选择 SKILL 自包含备选方案；不能只实现一半。

### 删除手写 Options 可能降低人类速查性

处理方式是保留少量 canonical recipe 和 provider 特有参数语义，同时把完整机器约束交给 targeted schema。`--help` 继续服务人类临时查看，Skill 不承担完整 CLI manual 职责。

### Metadata 当前没有生产消费方

统一 `agents/openai.yaml` 能修复资产质量，但不会改变当前 `skills show` 行为。该工作应与内容改动一起完成并验证，不应宣称已经提升当前 CLI routing；只有未来完整 Skill package 被安装或消费时，UI metadata 才产生运行时效果。

### 自动解析 Markdown 示例容易过度设计

优先减少示例数量并用 registry/BuildArgv 的表驱动 fixture 验证高价值 recipe。不要为任意 PowerShell Markdown 建立复杂 shell parser；只对标准化的一行 canonical recipe 做轻量提取，其余示例人工评审。

## 可后续独立评估的 CLI 改进

以下问题与 agent 体验有关，但不作为本次 Skill 核心验收条件：

1. 未知顶层命令当前只写 stderr 文本；可另行评估统一为静态 JSON `parameter_error`。
2. `skills list --capability <typo>` 当前成功返回空集合；可另行评估返回 `available_capabilities` 或显式 unmatched 状态。
3. 可基于真实 forward-test 数据评估 focused status，而不是预先新增 filter。
4. 如果确需把大结果写文件且不进入 stdout，可另行设计显式 silent/no-stdout 语义；不能改变现有 `--output` 合同而不说明兼容影响。

## 推荐实施顺序

推荐按“静态发现合同 -> router -> workflow -> provider -> metadata/test -> release smoke”推进。先修复 `skills list/show` 的不必要配置副作用，再让 router 建立正确的 schema/status/output/recovery 协议，随后逐个迁移子 Skill。这样每一阶段都能用当前 schema 和测试验证，不需要在一次提交中同时重写全部 CLI 行为。

本方案的核心不是增加更多说明，而是删除 schema 已经能够可靠提供的说明，把有限的 Skill 上下文留给 agent 真正需要的路由、证据和失败恢复知识。
