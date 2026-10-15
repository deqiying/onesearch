# 架构概览

[文档导航](../README.md)

## 系统边界

onesearch 是单一 Go module 的 CLI，同时通过顶层 `onesearch mcp` 提供本地 stdio MCP Server。npm launcher 和平台包分发同一 binary，不构成独立业务服务；现有 MCP HTTP 与 stdio client 仍用于连接上游 provider，与下游 Server 的协议和生命周期独立。

```text
cmd/onesearch → cli + commandcontract
                 ├─ 静态发现：help / schema / skills
                 ├─ CLI → config → execution → service → providers
                 │                           → sources → output + redact
                 └─ mcp → mcpserver → 只读配置快照 → execution → service
                              └─ 脱敏结果 → MCP JSON-RPC stdout
```

## 模块职责

| 目录 | 职责 |
| --- | --- |
| [cmd/onesearch](../../cmd/onesearch/) | 可执行程序入口 |
| [internal/app](../../internal/app/) | 应用标识、构建版本与版本文件查找 |
| [internal/commandcontract](../../internal/commandcontract/) | 命令注册、输入 schema、argv 绑定、preflight 与输出合同 |
| [internal/cli](../../internal/cli/) | 严格解析、help/schema、命令分发与退出码 |
| [internal/config](../../internal/config/) | 配置初始化、合并、provider 定义、凭据来源与 readiness |
| [internal/mcpserver](../../internal/mcpserver/) | 下游 stdio MCP Server、5 工具 catalog/schema、分支绑定、readiness、脱敏和资源限额 |
| [internal/execution](../../internal/execution/) | CLI/MCP 共用的无输出业务执行绑定，不反向启动 CLI |
| [internal/service](../../internal/service/) | 能力路由、fallback、provider-direct、诊断与离线研究规划 |
| [internal/providers](../../internal/providers/) | 上游 adapter、协议请求与结果归一化 |
| [internal/mcpclient](../../internal/mcpclient/) | HTTP MCP 客户端 |
| [internal/mcpstdio](../../internal/mcpstdio/) | 本地进程 stdio MCP 客户端 |
| [internal/sources](../../internal/sources/) | 来源解析、去重与结果角色 |
| [internal/output](../../internal/output/) / [internal/redact](../../internal/redact/) | JSON/Markdown/content 渲染、输出与凭据脱敏 |
| [internal/skills](../../internal/skills/) | 嵌入式 Skill 资源、发现和安全读取 |
| [npm](../../npm/) | JS launcher、兼容入口包及平台二进制包 |

## 关键执行边界

[CLI 入口](../../internal/cli/cli.go)先完成 help 和命令解析。`schema`、`skills list/show` 在 `config.Load()` 与 `service.New()` 之前返回，因此静态发现不初始化 provider 配置、不调用远端。显式 `--output` 的文件写入仍由用户请求决定。

普通执行加载 runtime 配置，由 service 按 capability route 和 provider 可用性选择 adapter。Provider-direct 使用显式端点；本机 readiness 不是联网可达性检测。协议约束及输出限制见 [CLI 参考](../reference/cli.md)和[配置参考](../reference/configuration.md)。

MCP 入口在普通 CLI 配置加载与输出前分流；发现不读配置、不联网、不创建配置。每个工具请求通过 [snapshot.go](../../internal/config/snapshot.go)加载只读快照，复用 execution/service，不暴露 shell、文件写入或配置修改。`onesearch_status` 按 operation 报告 readiness，整体 `ready` 只对应 minimum profile。SDK 固定 `v1.8.0`，仅 stdio/tools；输入 64 KiB、完整响应 2 MiB、最多 4 个并发调用。完整工具分支、协议兼容、超时和发布/host 验收边界见 [MCP 指南](../guides/mcp.md)与[批准设计](../decisions/mcp-server-mode-technical-design.md)。

[离线 planner](../../internal/service/deep.go)只生成计划。Sources 表示候选发现，只有执行抓取后才形成页面证据，不把计划或候选 URL 当成已完成的研究。

[Skill 资源](../../internal/skills/skills.go)通过 `go:embed assets/**` 随 binary 分发；文档目录迁移不改变这一路径。host 薄入口与 binary 工作流的关系见 [Agent 接入](../guides/agent-integration.md)。

## 设计理由

- [Command manifest V2](../decisions/cli-command-manifest-tool-declaration-alignment-technical-design.md)：CLI 合同与平台工具声明的边界。
- [内置 Skill](../decisions/builtin-skills-agent-optimization-plan.md)：薄 router、按需读取与 readiness gate。
- [Provider 协议对齐](../decisions/provider-upstream-protocol-alignment-technical-design.md)：协议选择、legacy fallback 与错误语义；原方案的计划项不自动等于当前能力。

构建与测试见[开发指南](../guides/development.md)。
