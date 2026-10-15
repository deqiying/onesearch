# notes 维护规范

## 默认目录与适用边界

没有既有决策规范时使用本节的原生格式；已有 ADR、RFC 或项目自定义格式时沿用它们，保留本技能的检索、事实同步、理由保留原则，不强套目录和校验器。

```text
.agents/notes/
  proposed/<class>/YYYY-MM-DD-topic.md
  implemented/<class>/YYYY-MM-DD-topic.md
  rejected/<class>/YYYY-MM-DD-topic.md
  archived/<class>/YYYY-MM-DD-topic.md
  archived/manifest.json
```

- `proposed`：有方案但未落地。
- `implemented`：已落地，仍指导现有实现。
- `rejected`：被否决且有防重犯价值；状态行写原因。
- `archived`：不再指导当前实现的历史 implemented 快照，归档后冻结。
- 文件日期为首次提出日；流转不改日期。只创建用到的目录。
- 原生格式不创建集中 `INDEX.md`；以目录、关键词和相对链接检索。docs 的导航页不受此限制。
- 自定义根目录通过 `AGENT_NOTE_ROOT` 指定，但内部结构仍遵循本规范。脚本不支持任意 ADR 格式。

## 六种 class

| Class | 用途 |
|---|---|
| `feature` | 能力、产品选择、非显然的可观察行为 |
| `bug-fix` | 缺陷根因、修复取舍、复盘后的防复发机制 |
| `simplification` | 不新增能力的删减、收窄或复杂度降低 |
| `architecture` | 模块职责、依赖、数据流、协议和结构性约束 |
| `process` | 构建、发布、协作、工具链和工作流 |
| `testing` | 测试策略、验证边界和测试基础设施 |

按决定的主要目的选一个，不因跨包而增加一层目录。普通重构不自动产生 Note；值得记录的结构调整归 architecture，真正的删减才归 simplification。分类常量与脚本一致，不能只改文档增加第七类。

## 文件格式

原生笔记头部固定为标题、空行、状态、空行：

```markdown
# Agent Note: 标题

Status: implemented

## Problem
```

新笔记使用英文头块和下列节名，正文语言遵循宿主；校验器也接受部分中文节名。为了归档互操作，不将 `Status:` 头块翻译为中文。

| 状态 | 必要正文结构 |
|---|---|
| proposed | Problem → Proposal → Alternatives considered → Acceptance criteria → Risks |
| implemented | Problem → Decision → Alternatives considered → Consequences |
| rejected | 保留提案背景、Proposal、Alternatives considered；状态写 `Status: rejected — 原因` |

Problem 必须是第一个二级节。可在 Proposal/Decision 后加 Scope、接口、Verification 等必要小节。implemented 不留 Proposal、Plan、Migration plan 或 Acceptance criteria 标题；验收结果写入 Verification，收益、代价和残留风险写入 Consequences。

Alternatives considered 必须存在，只记录真实考虑过的方案，先说明其优势，再解释为何不选。确实无备选记录时如实说明缺失，不虚构讨论，也不使用上游特定日期的历史豁免。脚本检查结构，不证明内容真实或完整。

## 更新与流转

1. 新建前按模块和机制检索活跃笔记，确定归属；重叠时明确互链或替代范围，不默认删旧文档。
2. 决定不变：就地更新路径、符号、参数及仍有效的事实，不追加修改日志。
3. 理由或决定翻转：新开一篇，解释取代范围；旧篇不得改成新决定。部分取代时两篇保留，旧篇标明仍有效部分并互链。
4. proposed 落地：移入 implemented，日期不变；同步 Status 和正文骨架，去掉计划口吻，记录实际验证与未验证边界；同批更新 docs 和所有受影响链接。
5. proposed 被否决：转 rejected 并说明原因。未经授权不直接删除重要记录；没有保留价值时先提出删除建议。
6. docs 的操作说明与完整参数表不复制进 Note；用相对链接引用。决定的关键约束必须在 Note 中能独立理解。

代码和文档应在同一批变更交付；用户授权提交时才执行 Git 写操作，不因本技能要求自动提交。

## 归档与替代

- 不按年龄或篇幅归档。仍生效的所有权、安全边界、兼容约束、否定性保证和防重犯理由，应保留在活跃区。
- 仅归档 implemented。完全取代时，新篇先承接仍有效的约束和独特理由，旧篇作为历史快照；部分取代不能整篇归档。
- 执行前列出目标和依据，确认属于本次授权范围；批量清理或删除重要记录另行确认。proposed 转 rejected，而不是塞进 archived。
- 先运行原生 notes 检查，再使用归档脚本。脚本在 `Status: implemented` 后插入 `Archived: YYYY-MM-DD`，移动文件并记录 SHA-256 封印；其余正文保持不变。
- 可用 `--superseded-by` 指向同一 notes 根内另一篇 implemented 笔记，历史快照链接写入新篇，不写进冻结旧篇。
- 归档脚本仅报告活跃 notes 中的常规 Markdown 入站链接。必须另查 docs、README、源码注释和跨根笔记，不能把“无入站链接”当成全仓扫描结果。
- 当前说明指向新决定；明确讨论历史时才指向归档。移动后修复链接并重新校验。
- 冻结文件不得编辑、翻译、格式化、移动或删除；需要纠错时在活跃笔记解释并链接历史快照，不重算旧封印掩盖修改。
- `manifest.json` 损坏或已有封印失配时停止，不自动重建。历史未封印文件经核对后才使用 `--write` 补封印。
- 无 Git 或无可用历史基线时只能校验文件与当前封印一致，不能证明历史只增不改；报告降级，不把封印当作防恶意篡改的签名。

## 写后语义自检

Problem 是否独立成立？Decision 是否与实现相符？Alternatives 是否真实且公平？Consequences 是否同时说明收益、代价和重访条件？Verification 是否区分实际结果与未验证范围？

避免会话流水账、无出处的性能提升和只有当时参与者才懂的编号。保留关键否定条件、失败语义、数据归属及证据，不因精简文字删掉重要约束。格式校验通过不能替代这些判断。

执行命令和工具限制见 [验证说明](verification.md)。
