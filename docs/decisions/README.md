# 设计记录

[文档导航](../README.md) · [当前架构](../architecture/README.md)

这里保存原技术方案的理由、约束、兼容取舍和验证背景，**不是当前使用手册**。各文档沿用原有格式和状态；背景中的“当前”、版本快照及测试结果按该方案的历史语境阅读。目录迁移不代表重新实施或重新执行其中的验证。

## 记录与状态

| 记录 | 状态与阅读边界 | 当前说明 |
| --- | --- | --- |
| [内置 Skill 与 Agent 优化](builtin-skills-agent-optimization-plan.md) | 已实施；保留薄 router、canonical `onesearch` ID、按需加载与 readiness 约束 | [Agent 接入](../guides/agent-integration.md) |
| [CLI command schema V1](cli-command-schema-technical-design.md) | V1 已实施；公开 command entry 字段已被 V2 部分替代，解析、绑定与静态发现的设计理由保留 | [CLI 参考](../reference/cli.md) |
| [CLI manifest / tool declaration V2 对齐](cli-command-manifest-tool-declaration-alignment-technical-design.md) | 已实施并记录本地验证；替代 V1 的 `id` / `summary`，使用 `name` / `description` | [CLI 参考](../reference/cli.md) |
| [JSON compact / pretty 输出](cli-json-output-format-technical-design.md) | 已实施；保留排版与 payload 语义分离的决定 | [CLI 输出](../reference/cli.md#输出与错误) |
| [Provider 安全配置](cli-provider-setup-technical-design.md) | 已实施；保留凭据输入、配置写入与脱敏约束 | [配置参考](../reference/configuration.md) |
| [OpenAI Responses adapter → Alpha Search](openai-responses-alpha-search-technical-design.md) | 已实施，本地验证完成；记录未验证真实 provider 可用性的边界 | [Adapter 限制](../reference/configuration.md#adapter-限制) |
| [MCP stdio bridge 与本地 provider](mcp-stdio-bridge-local-providers-plan.md) | superseded；transport、协商、discovery 与版本冻结由下方协议对齐方案接替，历史理由保留 | [架构概览](../architecture/README.md)、[配置参考](../reference/configuration.md) |
| [Provider 上游协议对齐](provider-upstream-protocol-alignment-technical-design.md) | 实施中；首轮代码与合同对齐不等于全部验收，真实 provider/MCP live smoke 仍未执行 | [CLI 参考](../reference/cli.md)、[配置参考](../reference/configuration.md) |
| [MCP Server 模式](mcp-server-mode-technical-design.md) | Implemented (local validation passed; release/host/live pending)；顶层 `onesearch mcp` 与 5 工具已落地，原未来式正文保留为批准设计，不替代上游 client 设计 | [MCP 接入](../guides/mcp.md)、[当前架构](../architecture/README.md)；npm `1.0.6` 不含新功能 |
| [DeepSeek Search / Fetch provider](deepseek-search-fetch-provider-technical-design.md) | 已实施，本地验证完成，真实 Search / Fetch live 调用已验证（402、429 未触发）；`deepseek` 默认禁用 | [CLI 参考](../reference/cli.md)、[配置参考](../reference/configuration.md) |

## 维护规则

- 使用或接入 CLI 时先读当前说明，完整参数以实际 binary 的 targeted `schema` 为准。
- 修改路径、命令或配置事实时同步相关记录；不覆盖原有决策理由，不把计划项直接改写为已完成能力。
- 新决定真正替代旧决定时说明替代范围并互链；不要因文档搬迁而创造新的实施日期或测试结论。
- 本目录不采用技能原生 Agent Note schema，不运行其专用格式校验器。文档链接、可移植路径和变更范围按[开发指南](../guides/development.md#文档维护)验证。
