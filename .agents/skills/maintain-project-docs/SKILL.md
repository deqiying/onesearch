---
name: maintain-project-docs
description: >-
  Use when creating, updating, or auditing project documentation; when code, configuration, interfaces, architecture, development, or deployment changes may make docs stale; or when recording technical decisions, constraints, rejected alternatives, and postmortems. Maintains two connected layers: current-state docs and decision notes, including small frontend/backend repositories and larger monorepos. Mechanical changes may skip new notes, but never skip checking affected docs.
---

# Maintain Project Docs

维护两类互相链接、职责不同的项目知识：

- **docs：项目现在是什么、如何工作、如何使用和维护。** 以当前实现、配置和验证证据核对，原地更新，不写开发流水账。
- **notes：为什么这样决定、受什么约束、放弃了什么、何时需要重访。** 保留决策理由与复盘经验，同步仍成立的事实，不把旧决定改写成相反结论。

**不需要新建 Note，不代表不需要更新 docs。** 不为每次提交写笔记，也不为了目录完整创建空文档。

## 1. 先识别宿主项目

1. 读取宿主的 `AGENTS.md`、`CLAUDE.md`、贡献指南、README 和已有文档入口；明确本次任务是分析、补文档还是随代码同步。
2. 查看实际目录、构建/工作区声明、包清单和入口，判断单项目或 Monorepo；不能只凭存在 `packages/` 就下结论。
3. 定位现有 docs 与决策记录。**已有结构、语言、格式和生成流程优先**；没有约定时采用本技能的默认结构。不要未经授权搬迁旧文档。
4. 找到本次涉及模块的当前说明与历史决定。默认 notes 位于 `.agents/notes/`，检索隐藏目录时包含 `--hidden`，日常检索排除 `archived/`；需要追溯历史时再读归档。
5. 从实际加载的 `SKILL.md` 路径确定脚本与模板位置；宿主目录和技能安装目录不是一回事。

宿主指令和用户授权优先。本技能不授权提交、发布、批量删文档、搬迁文档树或自动安装依赖。事实自己查；只对无法从仓库确定、会改变范围或兼容性的重要取舍提问，已有授权不重复确认。

## 2. 选择维护对象

| 情况 | docs | notes |
|---|---|---|
| 路径、命令、配置默认值、接口或部署方式变化 | 更新受影响的现状说明、示例和链接 | 同步已有笔记的事实；理由不变不新增 |
| 新能力、架构边界、跨模块契约或重要技术取舍 | 落地后更新相关现状说明 | 有长期参考价值时新增或演进决定 |
| 方案尚未实施 | 不写成当前能力；现有规划区可链接提案 | 记录为 proposed |
| 故障修复或复盘 | 更新受影响的操作说明、限制和排障步骤 | 记录非显然根因、取舍、防复发约束和证据 |
| 排版、局部显然修复、机械重命名 | 检查文档是否被影响，无影响不改 | 通常无需新增；修正已有失效引用 |
| 只补充现状文档，没有新决定 | 直接补充或纠正 | 不强制新建 Note |
| 旧决定被替代或功能下线 | 更新为仍支持的现状，修正导航 | 保留仍有效约束，处理替代关系与生命周期 |

新增前先找归属文档；一个事实只在一处详细维护，其余位置链接过去。README 是入口，不是 docs 的重复副本；notes 不充当使用手册。

## 3. 核心能力一：维护 docs

详细目录规则见 [docs 结构](references/docs-structure.md)，Monorepo 另读 [Monorepo 文档分层](references/monorepo.md)，操作步骤见 [docs 维护流程](references/docs-maintenance.md)。

执行顺序：

1. **定位影响**：从本次代码/配置/接口变化出发，列出实际受影响的文档，不做无关全库重写。
2. **核对事实**：读取实现、公开类型、配置默认值、脚本和测试；不把设计愿望或通过编译等同于行为已验证。
3. **原地更新**：修改现有归属文档；确实没有归属且内容值得长期维护时才新建。新页面可参考 [现状文档模板](templates/current-doc.md)，按题材删减，不强制所有小节。
4. **同步导航与引用**：新增、迁移或删除页面时更新 `docs/README.md`、模块 README 及已知入站链接；跨模块事实同时检查提供方、消费方和根级说明。
5. **验证**：使用项目已有文档构建/链接检查；核对命令、路径、默认值和示例。只能在安全且获授权的环境执行；不能运行的明确标记未验证。

当前文档应说明适用范围、真实行为、限制和可执行操作，不堆“本次新增”“以前使用”“下一步计划”。确实存在多版本支持时明确版本和适用边界，不能为了现在时抹去必要的迁移说明。

生成文档修改其源文件并按项目流程再生成，不手改产物。代码与既定契约矛盾时先报告差异，判断实现缺陷还是文档过时，不能一律将错误行为写成规范。

## 4. 核心能力二：维护 notes

完整结构、分类和状态规则见 [notes 规范](references/notes.md)。

- **检索在先**：读相关 implemented 和 rejected；考虑中的方案看 proposed。新建前检查同主题笔记，避免重复决定。
- **记录门槛**：需要保留非显然理由、架构/接口约束、真实备选、兼容取舍或复盘经验时记录。改动规模、文件数量和是否 CRUD 都不是唯一标准。
- **事实同步**：决定未变时，就地修正路径、符号和参数；理由改变时另建笔记，明确替代范围并互链。
- **状态真实**：未落地是 proposed；落地后同批转为 implemented，并同步 docs；被否决且值得防重犯时保留 rejected。
- **不伪造备选**：只写实际考虑过的方案。若没有比较记录，明确说明证据缺失，不编造论证填模板。
- **谨慎归档**：仍约束当前行为的内容留在活跃区。归档前检查替代关系及 docs/README/notes/代码中的引用；冻结后不改正文。

默认目录为 `.agents/notes/{proposed,implemented,rejected,archived}/<class>/YYYY-MM-DD-topic.md`，按需创建。使用原生格式时可参考 [提案](templates/proposed.md)、[已实施](templates/implemented.md)、[已否决](templates/rejected.md) 模板。

已有 ADR 或其他决策格式时沿用宿主规范，**不强制迁移，也不拿本技能的原生格式校验器检查它们**。自定义 notes 根目录只是路径变化，不等于任意格式兼容。

## 5. 交叉同步与收尾

每次收尾分别回答：

1. docs 是否准确描述已落地现状？导航、示例和真实路径是否一致？
2. notes 是否保留当前决定的理由？状态是否真实？是否与 docs 冲突或重复？
3. Monorepo 是否覆盖变更提供方、受影响消费方及必要的根级文档？
4. 哪些检查实际执行并通过，哪些只是人工核对，哪些尚未验证？

执行方式和工具边界见 [验证说明](references/verification.md)。只有使用本技能原生 notes 格式时才运行附带脚本；docs 的正确性不能由 notes 检查通过来证明。

交付简要列出：更新了哪些 docs/notes、保留或替代了什么决定、验证结果和剩余缺口。未执行的操作不能写成已完成；没有理由新增 Note 时说明即可，不为证明工作量制造文件。

## 按需阅读

- [docs 结构](references/docs-structure.md)：单项目默认目录、页面职责与命名。
- [Monorepo 文档分层](references/monorepo.md)：根级与工作区局部文档、入口及跨模块变更。
- [docs 维护流程](references/docs-maintenance.md)：影响定位、事实来源、同步和场景验收。
- [notes 规范](references/notes.md)：分类、格式、生命周期、替代与归档。
- [验证说明](references/verification.md)：notes 脚本、docs 检查、运行前提与回归测试。

## 来源与范围

notes 方法及核心脚本参考本技能库中的 `write-notes-like-deepseek/skills/write-notes-like-deepseek/`；本技能重新定义了 docs/notes 分工、Monorepo 文档结构、触发条件与授权边界，并调整了原生 notes 的通用性和归档失败行为。原项目的演示看板、语料导入和宣传素材不属于本技能运行资源，安装后不依赖原项目目录。
