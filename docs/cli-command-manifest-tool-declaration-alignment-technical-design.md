# Onesearch CLI Command Manifest 与 Agent Tool 声明字段对齐技术方案

## 实施状态

状态：已实施并完成本地验证（2026-08-13）。

本文档先基于 `main` 分支源码、`onesearch schema` 的 V1 输出、现有 command contract 技术方案，以及 OpenAI Function Calling 和 Model Context Protocol（MCP）Tool 声明的公开合同制定，现已按本文合同完成代码、测试、golden、Skill 和公共文档同步。

实施结果：

- `ManifestVersion` 已从 1 提升为 2；公开 command entry 使用 `name`、`description` 和 `input_schema`，不再输出 V1 `id` 和 `summary`。
- 内部 `CommandDefinition` 与 `NamespaceDefinition` 已统一使用 `Description`；内部稳定 `ID`、canonical `path`、alias、参数、binding、availability、side effects 和 output contract 保持不变。
- registry-backed namespace/group/leaf help 已改读同一 `Description`，现有用户可见文案未重写。
- 完整 golden 已迁移为 `internal/cli/testdata/cli-command-manifest-v2.golden.json`，V1 golden 已删除。
- README 中英文镜像、两份 npm README、主 Skill、Agent execution reference，以及直接受影响的历史技术文档已同步 V2 合同。
- `.deploy/version`、npm package versions、Git tag 和发布 workflow 未修改；CLI 正式 2.0.0 发布仍需单独授权。

实施验证：

- `mise exec -- go test -count=1 ./internal/commandcontract ./internal/cli ./internal/skills` 通过。
- `mise exec -- go test -count=1 ./...` 通过。
- `mise exec -- go vet ./...` 通过。
- `mise exec -- go run ./cmd/onesearch smoke --mock --format json` 返回 `ok: true`，13 个 mock cases 全部通过。
- 隔离 smoke 确认 targeted schema 返回 `manifest_version: 2`，包含 `name`、`description`、`input_schema`，不包含 command-level `id`、`summary`；compact 为 1 行，pretty 为多行且数据语义相等。
- 隔离 smoke 确认 targeted schema 与 command help 均未创建 runtime 配置目录。
- V1 golden 经批准字段迁移归一化后与 V2 golden 做排序 JSON 对比，结果一致，未混入参数、默认值、binding、availability、side effect 或 output 语义变化。
- 文档绝对文件系统路径检查和 `git diff --check` 通过。

真实 provider、网络、凭据、远端 MCP tool、OpenAI native tool 注册、MCP client 注册、发布后二进制与外部 V1 consumer 迁移不属于本地验收范围，未执行。

当前基线已确认：

- 项目使用 `mise.toml` 声明的 Go 1.26.5，`go.mod` 声明 Go 1.26。
- 当前源码版本和已安装 CLI 版本均为 1.0.2，targeted `schema search --pretty` 输出一致。
- 当前 command manifest 使用 `manifest_version: 1`，command entry 使用 `id`、`summary` 和 `input_schema`。
- `internal/commandcontract` 是 command path、parser、help、status metadata、Deep planner argv 和 manifest 的共同来源。
- `mise exec -- go test -count=1 ./internal/commandcontract ./internal/cli` 已通过。
- 方案编写前工作树无未提交修改；实施期间只修改了本文定义的直接影响文件。

## 推荐结论

将 `onesearch schema` 的公共 command entry 升级为更接近通用 Agent tool declaration 的 V2 合同：

```text
commands[].id       -> commands[].name
commands[].summary  -> commands[].description
commands[].input_schema 保持不变
manifest_version    1 -> 2
```

推荐的 V2 command entry 核心形状如下；示例只突出字段名称与层级，具体 input properties、availability、side effects 和 output 内容仍由各 command definition 生成：

```json
{
  "name": "exa.web-search",
  "description": "Search the web with Exa.",
  "input_schema": {
    "$schema": "https://json-schema.org/draft/2020-12/schema",
    "type": "object",
    "properties": {},
    "additionalProperties": false
  },
  "path": ["exa", "web-search"],
  "category": "provider",
  "provider": "exa",
  "capabilities": ["docs_search", "source_search"],
  "aliases": [],
  "availability": {},
  "side_effects": [],
  "output": {}
}
```

其中 `name`、`description` 和 `input_schema` 构成面向 Agent 的共同核心；`path`、`x-cli-binding`、`constraints`、`input_channels`、`availability`、`side_effects` 和 `output` 继续表达 CLI 调用所需的扩展合同。

本次不同时输出旧字段和新字段，不增加 V1 compatibility mode，也不把 command entry 伪装成可直接提交给某一模型平台的 native tool declaration。上层适配器仍需根据目标平台完成字段映射、tool name 校验和调用执行。

## 背景

`onesearch schema` 的主要读者是 AI agent、脚本和上层适配器。其价值不仅是让调用方知道 CLI 有哪些参数，还要让 Agent 在不猜测 help 文本的情况下完成以下工作：

- 识别一个 command 的用途并选择正确入口。
- 构造符合类型、required、enum、range 和互斥约束的输入对象。
- 根据 `x-cli-binding` 将结构化输入转换为安全的 argv token 数组。
- 在执行前理解配置初始化、网络、文件写入和敏感 stdin 等副作用。
- 根据 `availability` 定位本地 readiness 检查，但不把本地状态误认为远端可用性证明。
- 根据 `output` 判断稳定 envelope、format、variant 和 opaque provider payload 边界。

当前 V1 已经覆盖大部分 CLI 调用合同，但 command-level 用途字段命名为 `summary`。这在内部 help 模型中语义成立，却与主流 Agent tool declaration 使用的 `description` 不一致。Agent 或适配器需要先理解 Onesearch 私有字段语义，再把它翻译为模型平台字段，增加了不必要的认知和适配成本。

如果 Onesearch 将 command manifest 明确定位为 Agent-first contract，公共字段应优先采用跨平台共同语义；CLI 特有信息再通过清晰的扩展字段补充，而不应让最基础的工具名称和用途也依赖私有命名。

## 需求理解

本需求不是只把 JSON tag 从 `summary` 改成 `description`，也不是要求 Onesearch 直接输出某一家模型平台的完整 tool 数组。真正目标是建立以下边界：

1. Agent 首先看到熟悉的 tool 核心字段，减少额外语义映射。
2. command manifest 继续准确描述 CLI，而不是冒充 native function、MCP Server 或远端工具注册结果。
3. OpenAI、MCP 或其他 Agent runtime 可以通过轻量适配消费同一份 manifest。
4. CLI path、argv binding、副作用、动态 availability 和输出 envelope 等 Onesearch 特有合同继续完整保留。
5. 字段重命名作为显式破坏性变更发布，不通过双字段长期兼容制造新的漂移源。

## 术语与边界

| 术语 | 含义 | 本方案中的位置 |
| --- | --- | --- |
| Tool declaration | 提供给 Agent runtime 的工具名称、用途和输入 schema | 本方案只靠近其共同字段，不直接生成某平台请求体 |
| CLI command manifest | Onesearch 公共命令、argv 映射、可用性、副作用和输出合同 | `onesearch schema` 的实际输出 |
| Command input schema | 描述 command 结构化输入的 JSON Schema | `commands[].input_schema` |
| Runtime schema | 描述本机 `config.json` 的 provider 和 route 配置 | `config list --format json`，不属于本方案 |
| Native tool registration | 将工具声明实际提交给模型 API 或 MCP client | 由上层适配器负责，不由 `schema` 命令执行 |

`manifest_version` 是 CLI command manifest 的破坏性合同版本；`cli.version` 是产生该 manifest 的二进制版本；runtime `schema_version` 是用户配置合同版本。三者必须继续独立演进。

## 外部 Tool 声明合同基线

### OpenAI Function Calling

OpenAI Function Calling 的 function tool 使用以下核心字段：

```json
{
  "type": "function",
  "name": "get_weather",
  "description": "Retrieves current weather for the given location.",
  "parameters": {
    "type": "object",
    "properties": {},
    "additionalProperties": false
  },
  "strict": true
}
```

其中 `description` 用于说明何时、如何使用 function，`parameters` 是输入 JSON Schema。公开合同参见 [OpenAI Function Calling](https://developers.openai.com/api/docs/guides/function-calling)。

### Model Context Protocol Tool

MCP Tool 使用以下核心字段：

```json
{
  "name": "get_weather",
  "description": "Get current weather information for a location",
  "inputSchema": {
    "type": "object",
    "properties": {}
  }
}
```

MCP 还可以包含 `title`、`outputSchema`、`annotations`、`icons` 和 execution metadata。公开合同参见 [MCP Tools Specification](https://modelcontextprotocol.io/specification/2025-11-25/server/tools)。

### 跨平台共同核心

| 概念 | OpenAI | MCP | Onesearch V2 |
| --- | --- | --- | --- |
| 工具标识 | `name` | `name` | `name` |
| 工具用途 | `description` | `description` | `description` |
| 输入 schema | `parameters` | `inputSchema` | `input_schema` |
| 工具类型 | `type: function` | 由协议位置确定 | 不输出 |
| 严格调用 | `strict` | client/server validation | 不输出平台级开关 |
| CLI 调用路径 | 无 | 无 | `path` |
| argv 映射 | 无 | 无 | `x-cli-binding` |
| 本地 readiness | 无统一字段 | 无统一字段 | `availability` |
| 副作用 | 主要依赖 description | `annotations` 可表达部分提示 | `side_effects` |

因此，`name` 和 `description` 应直接对齐共同语义；输入 schema 应继续使用中立的 `input_schema`，避免把整个 manifest 偏置为 OpenAI `parameters` 或 MCP `inputSchema`。

## 当前代码与数据流

### Typed command registry

`internal/commandcontract/definition.go` 当前定义：

- `CommandDefinition.ID`：内部稳定 command ID。
- `CommandDefinition.Summary`：命令用途短说明。
- `NamespaceDefinition.Summary`：namespace help 说明。
- `ManifestCommand.ID`：序列化为 `id`。
- `ManifestCommand.Summary`：序列化为 `summary`。
- `ManifestCommand.InputSchema`：序列化为 `input_schema`。

`CommandDefinition.Manifest()` 负责复制、排序并组装公开 command entry。`inputSchema()` 从 positional 和 option definition 生成 JSON Schema property、required 和 `x-cli-binding`。

### Registry validation

`internal/commandcontract/registry.go` 对 command 和 namespace 执行以下相关校验：

- command `ID` 非空。
- canonical path、alias 和 ID 唯一。
- public command `Summary` 非空。
- namespace `Summary` 非空。
- positional、option、constraint、input channel、availability 和 output 合同有效。

因此，字段迁移必须从 typed definition 和 validation 开始，不能只在 JSON encoder 外层做字符串替换。

### Human-readable help

`internal/cli/command_help.go` 复用 `Summary`：

- namespace help 首段打印 `NamespaceDefinition.Summary`。
- group command list 在 canonical leaf 后打印 `CommandDefinition.Summary`。
- leaf help 首段打印 `CommandDefinition.Summary`。

字段重命名后应继续由同一个 `Description` 驱动 help，保持用户可见文案不变。

### Schema 输出

`internal/cli/schema_commands.go` 在配置加载前完成 full 或 targeted manifest 组装，并调用独立静态 JSON encoder。该路径具有以下现有合同：

- 不创建或读取 runtime 配置。
- 不读取凭据或访问网络。
- 默认输出一行 compact JSON 和一个结尾 LF。
- 显式 `--pretty` 使用两空格缩进。
- `--output` 与 stdout 使用同一份 bytes。
- full 和 targeted query 复用同一个 `Manifest()` assembler。

V2 字段调整不能改变这些行为。

### Tests 与 golden

当前主要保护边界包括：

- `internal/commandcontract/registry_test.go`：registry 完整性、copy safety、input schema 和敏感输入边界。
- `internal/cli/schema_commands_test.go`：full/targeted 等价、静态执行、compact/pretty、stdout/file equality 和 golden。
- `internal/cli/testdata/cli-command-manifest-v1.golden.json`：完整 V1 command manifest snapshot。

当前仓库内部没有生产代码反序列化 `schema` stdout 中的 `id` 或 `summary`；内部运行链直接消费 typed `CommandDefinition`。外部 Agent、脚本和适配器是否消费旧字段无法由仓库证明，因此仍必须按公共破坏性变更处理。

## 当前问题

### Command 用途字段与 Tool 声明不一致

`summary` 需要调用方知道 Onesearch 的私有字段语义。主流 tool declaration 已使用 `description` 表达同一职责，保留 `summary` 没有提供额外区分价值。

### `id` 仍需要额外映射到 tool `name`

V1 的 `id` 实际承担稳定逻辑标识职责。对 Agent tool 适配器而言，它最终仍会映射为 `name`。改为 `name` 可以让核心 entry 更接近 tool declaration，同时保留 `path` 表达实际 CLI token。

### 只增加同义字段会形成双重真源

如果 V1 同时输出：

```json
{
  "summary": "Search the web.",
  "description": "Search the web."
}
```

则调用方无法判断哪个字段是 canonical，测试必须永久保证两者相等，后续文案调整也更容易发生漂移。既然 command manifest 已有独立的破坏性版本，应通过 V2 完成一次明确迁移。

### 完全复制某一平台字段会损失中立性

把 `input_schema` 改为 OpenAI `parameters` 会偏向 OpenAI；改为 MCP `inputSchema` 会偏向 MCP。加入 `type: function` 或 `strict` 还会暗示 manifest 可以原样注册，而当前 CLI contract 并不满足所有平台的 name 和 strict schema 要求。

## 目标

1. 将 command-level 公共字段调整为 Agent 熟悉的 `name`、`description` 和 `input_schema`。
2. 保持 typed registry 是 parser、help、manifest、status 和 planner 的单一真源。
3. 保留 canonical CLI `path` 和完整 `x-cli-binding`，使结构化输入可逆地构造 argv token。
4. 保留 constraints、input channels、availability、side effects 和 output contract。
5. 将字段重命名发布为 `manifest_version: 2`，提供明确迁移表。
6. 保持 full/targeted schema、compact/pretty、static dispatch 和 stdout/file equality 合同不变。
7. 保持所有命令、参数、默认值、枚举、side effect、readiness 和实际执行行为不变。
8. 不新增第三方依赖，不修改 runtime `schema_version`。

## 非目标

1. 不把 Onesearch 改造成 MCP Server。
2. 不新增直接调用 OpenAI Responses API 的 tool registration 命令。
3. 不为 OpenAI、Anthropic、Gemini 或其他平台各维护一套独立 manifest。
4. 不把 `input_schema` 改为 `parameters` 或 `inputSchema`。
5. 不输出 `type: "function"`、`strict`、`defer_loading` 或其他平台专属字段。
6. 不为全部 provider payload 新增虚假的完整 `output_schema`。
7. 不新增 `summary`/`description` 或 `id`/`name` 双字段兼容。
8. 不增加 `--manifest-version`、`--legacy` 或 V1 fallback 参数。
9. 不修改 command canonical path、alias、flag、默认值、退出码和配置行为。
10. 不在本任务中修改 `.deploy/version`、npm package version、Git tag 或发布 workflow。

## 方案选择

| 方案 | 优点 | 问题 | 结论 |
| --- | --- | --- | --- |
| V1 新增 `description`，保留 `summary` | 表面向后兼容 | 同义双字段；长期漂移；调用方选择不明确 | 不采用 |
| 只把 `summary` 改为 `description` | 改动最小；直接解决当前问题 | `id` 仍需映射为 tool `name`；核心字段仍不完整 | 不推荐 |
| V2 改为 `name`、`description`、`input_schema` | 接近跨平台 tool 核心；CLI 扩展仍完整；迁移边界清楚 | 外部 V1 consumer 必须迁移 | 推荐 |
| 直接输出 OpenAI function tools | 可直接用于一个平台 | `parameters`、`strict`、name 规则和调用语义平台绑定 | 不采用 |
| 直接输出 MCP tools | 接近开放协议 | 无法表达完整 CLI argv、availability 和 output 扩展；并不启动 MCP Server | 不采用 |
| 增加多个 `--format openai|mcp|onesearch` | 可按平台生成 | 多套合同、测试和版本策略；超出当前真实需求 | 不采用 |

推荐采用单一、平台中立的 V2 manifest。它只对齐跨平台共同字段，不承诺“无需适配即可注册”。

## V2 Manifest 合同

### Root envelope

Root envelope 只提升 `manifest_version`，其余结构保持不变：

```json
{
  "ok": true,
  "kind": "onesearch_cli_command_manifest",
  "manifest_version": 2,
  "cli": {
    "name": "onesearch",
    "version": "<runtime-version>"
  },
  "scope": {
    "mode": "command",
    "path": ["search"]
  },
  "commands": []
}
```

以下字段语义不变：

- `kind` 继续区分 CLI command manifest 与 runtime schema。
- `cli.version` 继续取当前二进制版本。
- `scope.mode` 继续使用 `all` 或 `command`。
- `scope.path` 继续记录 targeted canonical path；full query 使用空数组。
- `commands` 继续按 category 和 canonical path 稳定排序。

### Command entry

V2 `ManifestCommand` 推荐按 Agent tool 核心优先的字段顺序编码：

```json
{
  "name": "search",
  "description": "Search the web through configured capability routes.",
  "input_schema": {
    "$schema": "https://json-schema.org/draft/2020-12/schema",
    "type": "object",
    "properties": {
      "query": {
        "type": "string",
        "description": "Search query.",
        "minLength": 1,
        "x-cli-binding": {
          "kind": "positional",
          "index": 0,
          "variadic": false
        }
      }
    },
    "required": ["query"],
    "additionalProperties": false
  },
  "path": ["search"],
  "category": "workflow",
  "capabilities": ["answer_search", "docs_search", "source_search"],
  "preferred_for": ["answer_search", "docs_search", "source_search"],
  "aliases": [["s"]],
  "constraints": [],
  "availability": {},
  "side_effects": [],
  "output": {}
}
```

JSON object 字段顺序不改变语义，但稳定顺序属于当前 golden 和 byte-level contract。将 `name`、`description`、`input_schema` 前置可以让 targeted schema 在流式查看、截断输出和 Agent 初步路由时优先暴露最关键的信息。

### `name`

`name` 取现有 `CommandDefinition.ID`，不是 canonical path 的展示字符串：

| Command | `name` | `path` |
| --- | --- | --- |
| workflow search | `search` | `["search"]` |
| Exa web search | `exa.web-search` | `["exa", "web-search"]` |
| config setup | `config.setup` | `["config", "setup"]` |

约束：

- `name` 在同一 manifest 中唯一且稳定。
- command rename 仍属于破坏性 manifest change。
- Agent 执行 CLI 时必须使用 `path`，不能按点号或其他分隔符拆分 `name`。
- 上层适配器若目标平台限制 tool name 字符集，必须生成平台合法名称并维护回到 `name`/`path` 的映射；Onesearch V2 不声称 `name` 可原样用于所有平台。
- 内部 `CommandDefinition.ID` 可继续保留 `ID` 命名，避免无收益地改动 registry lookup、handler binding、status 和 planner；公开 `ManifestCommand.Name` 从 `ID` 映射生成。

### `description`

`description` 取代 command-level `summary`，承担以下唯一职责：

- 告诉 Agent command 做什么以及适合何时调用。
- 作为 namespace command list 和 leaf `--help` 的首段说明。
- 映射为 OpenAI function `description` 或 MCP tool `description`。

本次只迁移字段名，不要求批量重写 44 个 command 的英文文案。后续如需提升 Agent 选路质量，应以独立任务逐条评估 description，避免把字段迁移扩大为 provider 文案重构。

内部 `CommandDefinition.Summary` 和 `NamespaceDefinition.Summary` 推荐统一改为 `Description`，使 registry validation、help 和 manifest 使用同一术语。Positional 和 Option 已经使用 `Description`，无需改变。

### `input_schema`

`input_schema` 保持当前 Draft 2020-12 受限子集和 `x-cli-binding` 扩展：

- `type`
- `properties`
- `required`
- `default`
- `description`
- `deprecated`
- `enum`
- `minimum`、`maximum`
- `minLength`
- `items`
- `minItems`、`maxItems`
- `additionalProperties: false`
- `x-cli-binding`

参数级 `description` 不受 command-level 字段迁移影响。

### 为什么不输出 `parameters`

`parameters` 是 OpenAI function tool 的字段名，不是跨平台共同名称。Onesearch command input 还包含 `x-cli-binding`，其首要职责是描述 CLI 输入而不是直接提交给 OpenAI。OpenAI adapter 可以执行：

```text
function.parameters <- command.input_schema
```

### 为什么不输出 `inputSchema`

`inputSchema` 是 MCP 的协议字段风格。Onesearch manifest 当前整体使用 snake_case，例如 `manifest_version`、`preferred_for`、`input_channels` 和 `side_effects`。保留 `input_schema` 可维持内部一致性，MCP adapter 可以执行：

```text
tool.inputSchema <- command.input_schema
```

### 为什么不输出 `type: function`

一个 Onesearch command entry 描述的是 CLI 可执行叶子，不代表模型运行时已经注册了 native function。输出 `type: function` 会错误暗示调用方可以把整个 entry 原样提交给 OpenAI，而其中的 CLI 扩展字段、name 规则、可选参数和输出语义仍需适配。

### 为什么不输出 `strict`

OpenAI strict mode 对 JSON Schema 有平台特定要求，例如 object 必须使用 `additionalProperties: false`，所有 properties 需要进入 `required`，可选输入通常通过 nullable 类型表达。当前 Onesearch input schema 直接表达 CLI 可选 flag，并不满足“所有 properties required”的 strict tool schema 形状。

因此，`strict` 应由 OpenAI adapter 在完成 schema 归一化和验证后决定，不能由 CLI manifest 无条件声明。

### 为什么不输出完整 `output_schema`

当前 `OutputDefinition` 只承诺：

- `default_format`
- 支持的 `formats`
- quiet/verbose 等 `variants`
- 稳定 envelope `contract`
- provider payload 是否 `opaque`

workflow 和 provider 的动态 map、verbose diagnostics 与上游 payload 尚不具备完整稳定 schema。V2 不应借字段对齐虚构精确的 tool output schema。

## 上层适配器映射

### OpenAI function tool

概念映射为：

```text
function.name        <- validateAndMap(command.name)
function.description <- command.description
function.parameters  <- normalizeForOpenAIStrictMode(command.input_schema)
function.strict      <- normalizationSucceeded
```

适配器还必须保存：

```text
function.name -> command.name -> command.path
```

模型返回 function arguments 后，适配器使用原始 `input_schema` 中的 `x-cli-binding` 生成 argv token，不能拼接 shell 字符串。

### MCP tool

概念映射为：

```text
tool.name        <- validateAndMap(command.name)
tool.description <- command.description
tool.inputSchema <- removeOrPreserveExtensionsAccordingToClient(command.input_schema)
```

这仍只是客户端适配。Onesearch 本身不会因 V2 manifest 自动成为 MCP Server。

## 兼容性与版本策略

### Manifest version

从 `summary` 删除并改为 `description`、从 `id` 删除并改为 `name` 都会使现有 consumer 读取失败，属于明确的破坏性字段重命名：

```text
manifest_version: 1 -> 2
```

不得在 `manifest_version: 1` 下静默改变字段，也不得只新增新字段后继续宣称 V1 canonical contract 不变。

### CLI version

`manifest_version` 与 `cli.version` 独立，但 `onesearch schema` 已在 README 中作为稳定 Agent 公共合同发布。若本方案随正式版本发布，推荐同步提升 CLI SemVer major 至 2.0.0，并在 release note 提供迁移表。

方案实施阶段不修改版本文件。版本号、tag、npm 发布和 release workflow 应在代码与文档验证通过后作为独立发布步骤执行。

### 不提供双版本输出

不建议增加：

```text
onesearch schema --manifest-version 1
onesearch schema --legacy
```

原因：

- 当前仓库没有必须保留的内置 V1 consumer。
- 双 assembler 或字段转换会增加测试和长期维护成本。
- full/targeted schema 必须继续共享一个 canonical assembler。
- V1 golden 和 V2 golden 并存会让未来合同修改需要维护两套快照。

如外部生态在实施前证明存在无法同步升级的重要 V1 consumer，应重新评估迁移窗口；不能在没有证据时预先加入永久兼容层。

### Rollback

实现尚未发布时，可通过普通 Git revert 恢复 V1。V2 一旦发布并被 consumer 使用，不能在同一 CLI major 中回退到 V1 字段；后续修复应保持 `manifest_version: 2`，只有再次发生破坏性字段语义变化时才提升到 V3。

## 实现设计

### Command definition

修改 `internal/commandcontract/definition.go`：

1. `ManifestVersion` 从 1 改为 2。
2. `CommandDefinition.Summary` 改为 `Description`。
3. `NamespaceDefinition.Summary` 改为 `Description`。
4. `ManifestCommand.ID` 改为 `Name string`，JSON tag 改为 `json:"name"`。
5. `ManifestCommand.Summary` 改为 `Description string`，JSON tag 改为 `json:"description"`。
6. 将 `InputSchema` 移到 `Name` 和 `Description` 后，稳定前置 Agent 核心字段。
7. `CommandDefinition.Manifest()` 使用 `Name: d.ID`、`Description: d.Description` 和现有 `d.inputSchema()`。
8. 其余 copy、sort 和 schema generation 逻辑保持不变。

内部 `CommandDefinition.ID` 不改名。它已经驱动 registry lookup、handler binding、status 和 planner，是稳定内部标识；公开 field 对齐不要求无关重构内部调用链。

### Registry validation

修改 `internal/commandcontract/registry.go`：

- command validation 从检查 `Summary` 改为检查 `Description`。
- namespace validation 同步改为检查 `Description`。
- 错误信息从 `summary is required` 改为 `description is required`。
- 新增或强化 manifest-level 断言，保证公开 `name == definition.ID` 且唯一。
- path、alias、preferred capability 和其余 validation 保持不变。

### Definitions and helpers

修改：

- `internal/commandcontract/workflows.go`
- `internal/commandcontract/providers.go`
- `internal/commandcontract/utilities.go`
- `internal/commandcontract/fragments.go`

执行机械、可审阅的命名迁移：

- struct literal 的 `Summary:` 改为 `Description:`。
- helper 形参 `summary string` 改为 `description string`。
- 不调整现有 44 个 command 文案。
- 不调整 positional/option descriptions。

### Help

修改 `internal/cli/command_help.go`：

- namespace help 使用 `namespace.Description`。
- group command list 使用 `command.Description`。
- leaf help 使用 `definition.Description`。
- usage、arguments、flags、default、enum 和 constraint 展示保持不变。

### Schema assembler

`internal/cli/schema_commands.go` 无需增加 V1/V2 分支。它继续调用 `definition.Manifest()`；字段变化完全由 typed `ManifestCommand` 决定。

只需确认：

- full 和 targeted command 使用同一 V2 entry。
- root `manifest_version` 自动取新常量。
- compact/pretty encoder、static error envelope 和 `--output` 行为不变。

## 影响文件

| 文件 | 计划修改 | 原因 |
| --- | --- | --- |
| `internal/commandcontract/definition.go` | V2 types、字段映射、字段顺序 | manifest 唯一组装源 |
| `internal/commandcontract/registry.go` | description validation | typed contract 自校验 |
| `internal/commandcontract/workflows.go` | definition 字段名迁移 | workflow source definitions |
| `internal/commandcontract/providers.go` | definition 字段名迁移 | provider source definitions |
| `internal/commandcontract/utilities.go` | definition 与 namespace 字段名迁移 | utility source definitions |
| `internal/commandcontract/fragments.go` | helper 形参迁移 | 统一术语 |
| `internal/cli/command_help.go` | 改读 Description | 保持 registry-backed help |
| `internal/commandcontract/registry_test.go` | 新字段与 validation 测试 | 锁定 typed contract |
| `internal/cli/schema_commands_test.go` | V2、旧字段缺失、help 测试 | 锁定公共 manifest |
| `internal/cli/testdata/cli-command-manifest-v2.golden.json` | 新增 V2 golden | 完整合同 snapshot |
| `internal/cli/testdata/cli-command-manifest-v1.golden.json` | 删除 | 避免并存两套 canonical golden |
| `README.md` | V2 字段与迁移说明 | English 公共文档 |
| `README.zh-CN.md` | V2 字段与迁移说明 | 中文镜像 |
| `npm/onesearch/README.md` | 同步 npm 公共合同 | npm 安装入口 |
| `npm/deqiying-onesearch/README.md` | 同步 scoped package | scoped npm 入口 |
| `internal/skills/assets/onesearch/SKILL.md` | 必要时说明 V2 核心字段 | Agent router |
| `internal/skills/assets/onesearch/references/agent-execution-contract.md` | 更新 targeted schema 字段合同 | version-matched Agent reference |
| `docs/cli-command-schema-technical-design.md` | 标注 V1 已由本方案的 V2 supersede | 历史设计边界 |
| `docs/cli-json-output-format-technical-design.md` | 更新当前 manifest version 说明 | 避免状态文档过期 |
| `docs/builtin-skills-agent-optimization-plan.md` | 更新 schema 真源字段说明 | 避免 Agent contract 漂移 |

`.deploy/version`、npm `package.json` 和平台 binary package versions 不在实现阶段修改；它们只在用户另行授权发布时同步。

## 分阶段实施步骤

### 阶段一：冻结 V2 合同

1. 确认 `name` 取内部 `CommandDefinition.ID`，`path` 继续是唯一 CLI 执行入口。
2. 冻结 V1 到 V2 字段迁移表。
3. 明确 `input_schema`、CLI 扩展字段和 root envelope 除版本外不变。
4. 在测试中先表达 V2 预期，避免实现期间临时改变合同。

### 阶段二：迁移 typed definitions

1. 修改 manifest types 和 `ManifestVersion`。
2. 迁移 command/namespace `Summary` 到 `Description`。
3. 更新 workflow、provider、utility definitions 和 helper 参数。
4. 更新 registry validation。
5. 保持 `CommandDefinition.ID`、path、alias 和所有参数定义不变。

### 阶段三：同步 help 与 schema

1. 更新 registry-backed help 读取字段。
2. 验证所有 canonical path 和 alias help 内容不变。
3. 验证 full 和 targeted schema 都输出 V2。
4. 验证 schema 仍在 config/service 初始化前返回。

### 阶段四：更新 tests 与 golden

1. 更新 registry 单元测试的 definition fixtures。
2. 增加 `name`、`description`、旧字段缺失和 `manifest_version: 2` 断言。
3. 将 golden 路径迁移到 `cli-command-manifest-v2.golden.json`。
4. 通过现有显式 update gate 生成 V2 pretty golden。
5. 人工审核完整 golden diff，确认没有参数、默认值、side effect 或 availability 意外变化。

### 阶段五：同步 Agent 与公共文档

1. 更新 root bilingual README 和两份 npm README。
2. 更新主 Skill 与 agent execution reference。
3. 在既有 V1 schema 技术方案顶部追加 superseded 状态和 V2 文档链接，不重写其历史实施记录。
4. 更新 compact JSON 与 Skill 优化方案中对当前 manifest version 和字段的描述。
5. 增加 release note 所需迁移表，但不执行正式发布。

### 阶段六：验证与交付

1. 运行定向 command contract 和 CLI 测试。
2. 运行全量 Go test 和 vet。
3. 执行 full/targeted schema smoke、help smoke 和 mock smoke。
4. 检查目标 diff、文档绝对路径和 `git diff --check`。
5. 报告未验证的外部 consumer、真实 provider 和正式发布边界。

## 测试方案

### Command contract 单元测试

至少覆盖：

1. `ManifestVersion == 2`。
2. 每个 public command 的 `Description` 非空。
3. 每个 manifest entry 的 `Name == CommandDefinition.ID`。
4. command `name` 唯一且不因 alias 变化。
5. namespace description validation 生效。
6. Positionals 和 Options 的参数级 description 不变。
7. input schema、constraint、input channel、availability、side effect 和 output 排序不变。
8. registry copy safety 不因字段迁移退化。

### Schema 单元测试

至少覆盖：

1. full query 返回 `manifest_version: 2` 和全部 public commands。
2. targeted query 返回与 full manifest 同 name entry 深度相等的对象。
3. 每个 entry 都包含非空 `name`、`description` 和 object 类型 `input_schema`。
4. 将输出解码为 `map[string]any` 后，command entry 不存在 `id` 和 `summary`。
5. root `cli.name` 和 `cli.version` 语义不变。
6. canonical path、alias target rejection 和 parameter error 行为不变。
7. compact/pretty 解码后深度相等，各自重复生成 byte-for-byte 稳定。
8. stdout 和 `--output` 在成功、unknown path 和 parse error 路径逐字节一致。
9. schema/help 不创建 config、不读取凭据、不访问网络。
10. V2 golden 仍只归一化动态 `cli.version`，不重新 marshal 整个 manifest。

### Help 单元测试

对全部 public command 和 namespace 覆盖：

- canonical path 与 alias 的 `--help` 返回 0。
- help 包含 definition/namespace `Description`。
- leaf usage、positionals、flags、default 和 enum 与迁移前相同。
- help 不触发配置初始化、doctor、regression 或 provider。

### Golden review

V2 golden 的预期 diff 只能包含：

- `manifest_version` 从 1 变为 2。
- 每个 command 的 `id` 改为 `name`。
- 每个 command 的 `summary` 改为 `description`。
- 为 Agent 核心字段前置而产生的字段顺序调整。
- golden 文件名从 V1 改为 V2。

以下变化均应视为意外并停止更新：

- command 数量变化。
- canonical path 或 alias 变化。
- property 类型、required、default、enum 或 binding 变化。
- availability、side effect 或 output contract 变化。
- provider capability 或 preferred command 变化。

### CLI smoke

使用隔离配置目录执行：

```powershell
$manifestSmokeRoot = Join-Path (Get-Location) ".cache/manifest-v2-smoke"
$env:ONESEARCH_CONFIG_DIR = Join-Path $manifestSmokeRoot "config"

mise exec -- go run ./cmd/onesearch schema search
mise exec -- go run ./cmd/onesearch schema search --pretty
mise exec -- go run ./cmd/onesearch schema --pretty --output .cache/manifest-v2-smoke/manifest.json
mise exec -- go run ./cmd/onesearch search --help
mise exec -- go run ./cmd/onesearch smoke --mock --format json
```

验证：

- targeted compact 输出只有一行并以 LF 结尾。
- targeted pretty 输出多行且数据语义相同。
- full manifest 文件与 stdout bytes 相同。
- 输出包含 `name`、`description`、`input_schema`，不包含 command-level `id`、`summary`。
- 隔离配置目录未创建 `config.json`。
- mock smoke 仍返回 `ok: true`。

### 回归命令

```powershell
mise exec -- go test -count=1 ./internal/commandcontract ./internal/cli ./internal/skills
mise exec -- go test -count=1 ./...
mise exec -- go vet ./...
mise exec -- go run ./cmd/onesearch smoke --mock --format json
git diff --check
```

真实 provider、网络、凭据、远端 MCP tool、OpenAI native tool 注册、MCP client 注册和 npm 发布不属于本方案的本地验收范围。

## 文档与路径检查

实现完成后必须检查全部文档中的 Windows drive path、UNC path 和 Unix user-directory path，并人工排除 URL、HTTP API path 和 Context7 `/org/project` 标识。

文档中的仓库文件使用相对路径；命令从仓库根目录执行；临时目录使用 `Get-Location`、环境变量或 `.cache/...`，不得写入开发机真实绝对路径。

## 验收标准

1. `onesearch schema` 返回 `manifest_version: 2`。
2. 每个 public command entry 使用 `name`、`description`、`input_schema` 作为 Agent 核心字段。
3. command entry 不再输出 `id` 和 `summary`。
4. `name` 稳定映射自内部 command ID，`path` 继续是唯一 canonical CLI 执行路径。
5. `input_schema`、`x-cli-binding`、constraints、input channels、availability、side effects 和 output contract 保持准确。
6. full 与 targeted entry 深度一致。
7. compact/pretty、stdout/file、错误 envelope 和退出码合同保持不变。
8. schema 和 help 仍在配置加载前静态返回。
9. 所有 command 和 namespace help 的用户可见说明不发生无意文案变化。
10. V2 golden 只包含批准的版本、字段名和字段顺序变化。
11. README、npm README、主 Skill、Agent reference 和直接受影响的技术方案已同步。
12. 文档不包含开发机绝对文件系统路径。
13. 定向测试、全量测试、vet、mock smoke 和 `git diff --check` 全部通过。
14. 未执行版本发布、tag、npm publish 或 push，除非用户另行授权。

## 风险与取舍

### 外部 V1 consumer 中断

风险：读取 `commands[].id` 或 `commands[].summary` 的外部脚本会在升级后失败。

处理：提升 `manifest_version`，同步 CLI major release，提供明确迁移表；不通过同义双字段隐藏破坏性变化。

### `name` 并非所有平台都可原样注册

风险：`exa.web-search` 等内部 ID 可能不满足某些模型平台的 tool name 字符限制。

处理：文档明确 `name` 是 manifest 内稳定逻辑名，不承诺跨平台原样注册；适配器负责合法化并保留反向映射。CLI 执行始终使用 `path`。

### Agent 误认为 manifest 是 native tool declaration

风险：字段更接近 tool declaration 后，调用方可能忽略 CLI 扩展和适配步骤。

处理：保留 `kind: "onesearch_cli_command_manifest"`，不输出 `type: function` 或无条件 `strict`，并在 README/Skill 中明确 schema 仍是 CLI input contract。

### 字段前置导致 large golden review 噪音

风险：44 个 command 的字段顺序调整会产生较大 diff。

处理：在同一 V2 破坏性迁移中一次完成；golden 继续使用 pretty JSON；人工按字段迁移表审核，禁止混入参数或行为改动。

### 内部命名迁移范围扩大

风险：将 `Summary` 统一改为 `Description` 会触及 workflow、provider、utility、namespace、help 和 tests。

处理：改动保持机械化，不重写文案、不改变 handler 和 parser；通过 registry-backed help 与 golden 验证没有行为漂移。

### CLI SemVer 与 manifest version 混淆

风险：只提升 `manifest_version` 而继续发布 1.x，可能让依赖 CLI SemVer 的 consumer 低估兼容影响。

处理：推荐正式发布为 CLI 2.0.0；实现与发布分离，未完成全量验证前不修改版本和 tag。

## 推荐实施顺序

按以下顺序推进：

```text
冻结 V2 字段合同
  -> 迁移 typed definitions 与 validation
  -> 同步 registry-backed help
  -> 更新 schema tests 与 V2 golden
  -> 同步 README、Skill 和历史技术方案状态
  -> 定向测试、全量测试、vet、mock smoke、文档检查
  -> 单独决定 CLI 2.0.0 发布
```

实现过程中不重新设计 parser、provider、runtime config 或 output payload。任何超出字段迁移直接影响面的改动都应另立任务，避免一次可审计的 manifest V2 变更演变为广泛 CLI 重构。

## 最终结论

采用 `manifest_version: 2`，将 command-level `id`/`summary` 明确迁移为 `name`/`description`，保留中立的 `input_schema` 与完整 CLI 扩展合同。这一方案让 Agent 首先看到熟悉的 tool 核心字段，同时不牺牲 canonical argv、安全边界、动态 readiness 和输出语义。

V2 仍然是 Onesearch CLI command manifest，而不是某一模型平台的 native tool declaration。上层适配器应负责平台 name 约束、OpenAI strict schema 归一化或 MCP field casing；Onesearch 只维护一份稳定、可版本化、可由 Agent 和适配器共同消费的事实合同。
