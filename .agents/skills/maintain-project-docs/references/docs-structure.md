# docs 目录与页面职责

## 适用原则

本结构是没有既有约定时的默认值，不是迁移命令。已有文档站、目录、语言或命名规范优先；先将现有页面映射到以下职责，缺什么补什么，不再并行建设第二套文档。

目录树表示可用位置，不要求一次建齐。开始维护 docs 时建立 `docs/README.md` 入口；其余页面只有存在真实内容和读者需求时才创建。小项目可以只有入口和一两篇文档。

## 单项目默认结构

```text
README.md                         # 项目简介、最短上手路径、docs 入口
CONTRIBUTING.md                    # 已有贡献约定；不复制开发指南
docs/                             # 项目现状文档
  README.md                       # 文档导航与阅读路径
  overview.md                     # 项目目标、能力范围、术语和限制
  architecture/
    README.md                     # 系统边界、模块职责与依赖概览
    components.md                 # 复杂模块的职责与连接关系，按需
    data-flow.md                  # 关键数据流、状态流或生命周期，按需
  guides/
    getting-started.md            # 完成一次最小可用操作
    development.md                # 开发环境、构建、调试
    testing.md                    # 测试运行方法与范围
  reference/
    configuration.md              # 配置项、默认值、优先级、生效条件
    interfaces.md                 # API、事件、协议或对外接口
    cli.md                        # 命令与参数；没有 CLI 不创建
  operations/
    deployment.md                 # 部署、升级、回滚与前置条件
    troubleshooting.md            # 症状、诊断、处理、验证恢复
  assets/                         # 文档引用的图片与图表源文件
.agents/notes/                    # 决策记录，独立于现状说明
```

允许按主题拆分，例如将较大的 `reference/interfaces.md` 拆为 `reference/interfaces/README.md` 与具体接口页；拆分后只保留一个详细事实来源。不要保留内容相同的总页和子页。

## 页面归属与命名

- 文件名默认使用英文小写 kebab-case；正文语言遵循宿主。目录入口统一用 `README.md`，不额外建同义 `index.md`，文档站有要求时例外。
- 根 README 保留简介和最短上手入口；`docs/README.md` 按阅读目标导航，只链接实际存在的页面，不列待办空链接。
- `overview` 说明能力范围；`architecture` 解释当前结构；`guides` 按任务给步骤；`reference` 给可查询的事实；`operations` 描述运行维护。一个主题不必在每类目录各写一遍。
- 本技能允许 docs 导航页；原生 notes 禁止集中 `INDEX.md` 的规则不适用于 docs。
- 图片存放在所属 docs 的 `assets/`，保留可维护的源文件。优先采用项目已支持的图表方式，不为文档新增渲染依赖。
- 只有长期使用指南需要按版本维护时才沿用或建立版本分区；不按每次提交日期复制现状文档。

## 文档最小信息

按题材选择必要内容，不强制套满模板：

1. **范围**：适用项目、模块、版本或运行模式；不确定的边界要标明。
2. **当前事实**：行为、职责、前置条件、命令、参数或接口约定。
3. **限制与失败**：不支持什么，出错时如何识别和处理。
4. **事实来源**：链接关键实现、配置定义、schema 或测试；不是罗列所有文件。
5. **相关决定**：有重要取舍时链接 notes；没有则不创建占位小节。

使用相对链接，按当前文件位置计算，不把本机绝对路径写入产物。命令注明工作目录与前提；示例不能包含真实凭据或内部地址。

## 与其他记录分开

- docs 描述当前支持的行为，计划放提案，发布历史放项目已有 changelog。
- 决策说明可以包含一句理由摘要，但完整备选与取舍归 notes；notes 链回现状说明，不复制整份手册。
- `docs/adr/` 或 `docs/decisions/` 若已存在，属于决策记录区，不受现状文档的“只写已落地事实”约束；必须明确状态。
- `AGENTS.md`、`CLAUDE.md` 负责协作指令，不堆接口明细；必要时链接 docs 或 notes。
- Monorepo 使用 [分层规范](monorepo.md)，而不是将每个包的详细说明都堆到根级 docs。

## 常见目录命名表

新建目录使用下表的标准名，根级和模块级 docs 遵循同一套词汇。下列别名只用于识别已有内容，不是可任选的新名称；已有布局不自动重命名，也不并建标准名的重复目录。

| 含义 | 标准目录 | 路径实例 | 不再新建的同义名 |
|---|---|---|---|
| 系统架构、职责、数据流 | `architecture/` | `docs/architecture/data-flow.md` | `arch/`、用 `design/` 泛指架构 |
| 按任务操作的指南 | `guides/` | `docs/guides/development.md` | `guide/`、`how-to/`、`manuals/` |
| 参数、契约等查询资料 | `reference/` | `docs/reference/configuration.md` | `references/`、`specs/` |
| 部署、运行、排障 | `operations/` | `docs/operations/deployment.md` | `ops/`、`runbooks/`、`devops/` |
| 集中式模块文档 | `modules/` | `docs/modules/frontend/README.md` | `components/`、同层混用 apps/packages |
| 文档图片、图表源文件 | `assets/` | `docs/assets/system-overview.svg` | `images/`、`img/`、`pictures/` |

该表约束宿主项目的 docs，不要求把 Skill 自身的标准资源目录 `references/` 改名。`design/` 若实际存放 UI 设计稿，不等于 architecture，不按名称强行合并。

模块目录名沿用源码中的实际名称：源码叫 `frontend/` 就用 `modules/frontend/`，叫 `web/` 就用 `modules/web/`；不要擅自把所有项目的模块名改成 frontend。统一的是文档分类词，不是业务模块名。

### 常见细分主题

内容少时用 `<主题>.md`；需要多篇时扩为 `<主题>/README.md` 加主题页，名称词干保持一致。例如 `reference/api.md` 扩为 `reference/api/README.md`，不改叫 endpoints，也不保留两份完整内容。

| 含义 | 统一路径（拆分后） | 子页实例 |
|---|---|---|
| HTTP/RPC 等对外 API | `reference/api/` | `authentication.md`、`errors.md`、`devices.md` |
| 消息和事件契约 | `reference/events/` | `device-status-changed.md` |
| 网络/设备/文件协议 | `reference/protocols/` | `snmp.md`、`modbus.md` |
| 配置项与环境变量 | `reference/configuration/` | `environment-variables.md`、`profiles.md` |
| 命令行参数 | `reference/cli/` | `serve.md`、`export.md` |
| 数据模型与字段语义 | `reference/data-model/` | `device.md`、`event.md` |
| 本地开发与调试 | `guides/development/` | `setup.md`、`debugging.md` |
| 测试执行与范围 | `guides/testing/` | `unit.md`、`integration.md`、`end-to-end.md` |
| 部署、升级、回滚 | `operations/deployment/` | `container.md`、`upgrade.md`、`rollback.md` |
| 排障与恢复 | `operations/troubleshooting/` | `connectivity.md`、`startup.md` |

单一接口类型优先使用 api、events 或 protocols；`reference/interfaces.md` 仅用于多种接口的总览与导航，不重复定义它们。配置不另起 config/settings，协议不另起 protocol，排障不另起 debug；按内容归属，不能因为名字相近就合并不同含义。

初次采用时，将实际使用的目录和阅读路径写进 `docs/README.md`。未列出的业务主题使用准确的小写 kebab-case；如果后续发现属于已有标准类别，就归入该类别，不新增近义顶层目录。
