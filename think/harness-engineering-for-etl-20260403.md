# Harness Engineering 在 ETL 工作流中的实践

> 原文：https://www.anthropic.com/engineering/harness-design-long-running-apps
> 作者：Anthropic Engineering
> 收录时间：2026-04-03 CST

---

## 什么是 Harness

Harness 是套在模型外面的「驾驭层」。模型能力是固定的，harness 决定了这些能力能被发挥到什么程度。

Harness 的每个组件都编码了一个假设：**模型自己做不到什么**。

## 原文核心架构：Generator-Evaluator

原文描述了一个三 Agent 系统：

| Agent | 职责 | 关键设计 |
|-------|------|---------|
| Planner | 将 1-4 句用户需求扩展为完整产品 spec | 只做产品级规划，不写技术细节 |
| Generator | 按 spec 逐 feature 实现 | 每轮实现后自检，再交 Evaluator |
| Evaluator | 用 Playwright 真实操作 app，按标准打分 | 只能判断，不能改代码 |

核心原则：

- **分离生成与评估** — 模型自我评价总是偏高，分开后评估更客观
- **Sprint Contract** — 写代码前先协商"什么算完成"，由 Generator 和 Evaluator 两个 Agent 协商达成一致，全程无人参与
- **文件通信** — Agent 间通过读写文件传递状态，可审计、可调试
- **Context Reset** — 上下文满了就交接给新 Agent（Opus 4.6 后可省略）
- **渐进简化** — 模型能力提升后，去掉不再必要的组件（如 sprint 拆分）

关键数据对比：

| 方式 | 时间 | 成本 | 质量 |
|------|------|------|------|
| 单 Agent 直接跑 | 20 分钟 | $9 | 表面能跑，深层 bug 多 |
| 三 Agent Harness | 6 小时 | $200 | 明显更好，功能完整、细节打磨 |

## 我的场景：ETL 工作流开发

### 项目背景

`analytic-framework-workflows` 是一个 Airflow DAG 编排的 ETL 数据分析框架：

- 覆盖 US + 14 个 SEA 国家，共用表结构，按国家前缀区分
- 数据链路：S3 原始日志 → MLSQL/Spark 处理 → Hive → ES/StarRocks
- 典型变更涉及：workflow YAML → MLSQL → Hive DDL → ES mapping → DATA_CATALOG.md

### 当前痛点

尽管已经在用 Claude Code，但工作模式是：

```
我描述需求 → Claude 写代码 → 我补上下文 → Claude 改
→ 我指出问题 → Claude 改 → 我 review → 完成
```

**我就是那个 Evaluator + Context Provider，Claude 只是 Generator。** 效率比以前高，但不够 harness — 我的脑力仍然是瓶颈。

### 我想达到的状态

Claude 自己知道去哪找上下文、自己知道什么时候该跑检查、自己能判断产出质量。我只做两件事：**提需求** 和 **确认结果**。

## Harness 设计：三步建设

### 第一步：Context 层 — 替代"我口头补上下文"

编码我脑子里的隐性知识，让 Agent 自己能找到所需信息。

**市场映射** `harness/context/market_mapping.yaml`

```yaml
markets:
  us:
    hive_db: nebula_entity
    es_index_prefix: "pre-tiktok"
    workflow_dir: us_entity_daily_analysis
  sea:
    countries: [th, my, vn, mx, jp, br, de, fr, it, es, gb, sg, id, ph]
    hive_db_pattern: "{country}_pre_nebula_entity"
    es_index_prefix: "{country}-pre-tiktok"
    workflow_dir: sea_pre_entity_daily_analysis
shared_entities: [product, creator, seller, video, shop]
```

**文件索引** `harness/context/file_index.yaml`

实体 → 对应的 MLSQL 文件、Hive 表名、ES 索引的映射关系，让 Agent 拿到需求后自动定位文件，而不需要我说"去看看 US 那边怎么做的"。

**变更 Checklist** `harness/context/etl_change_checklist.yaml`

标准化变更链路（如"新增字段"需要改哪些东西）+ 常见踩坑（如 MLSQL SELECT 字段顺序必须与 Hive DDL 一致）。

### 第二步：Evaluation 层 — 替代"我肉眼 review"

把"我 review 时看什么"外化为 Agent 可执行的规则。

**可脚本化的检查** `harness/validators/`

| 检查项 | 方式 | 说明 |
|--------|------|------|
| 字段顺序 | `check_field_order.py` | DDL 字段顺序 vs MLSQL SELECT 顺序 |
| 国家覆盖 | `check_market_consistency.py` | 14 SEA 国家是否全覆盖，前缀是否正确 |

**需要 Agent 判断的检查**（写在 Evaluator prompt 里）

| 检查项 | 说明 |
|--------|------|
| 全链路传递 | 新字段从 source MLSQL → snapshot → feature → ES sink 是否完整 |
| 命名一致性 | 字段名/类型与同类现有字段风格一致 |
| 文档更新 | DATA_CATALOG.md 是否记录了新字段 |

**评估标准**：任何一项 FAIL 则整体打回，附具体文件路径和原因。

### 第三步：Orchestration 层 — 替代"我手动推进对话"

不需要外部脚本或 SDK，直接在 CLAUDE.md 中定义规则。

**关键设计决策：工具箱模式 vs 流水线模式**

流水线模式是按需求类型写死流程（"新增字段走流程 A，修 bug 走流程 B"），但需求类型不固定，写死流程迟早遇到不在清单里的情况。

工具箱模式更好：只定义 **context 在哪里、什么时候该读、什么时候该跑 validator、什么时候该启 review**，让模型自己判断如何组合。模型擅长这种灵活调度，不擅长的是知道你的项目里有哪些检查工具可用 — 后者才需要写进 CLAUDE.md。

```markdown
## Harness 基础设施

### Context（涉及 ETL 时必须先读）
- harness/context/market_mapping.yaml — 市场 ↔ 库表映射
- harness/context/file_index.yaml — 实体 ↔ 文件索引
- harness/context/etl_change_checklist.yaml — 变更链路 + 踩坑

### Validation（改了 MLSQL 或 DDL 时必须执行）
- harness/validators/check_field_order.py — DDL vs MLSQL 字段顺序
- harness/validators/check_market_consistency.py — 15 国家覆盖检查

### Review（变更超过 3 个文件时启动 sub-agent）
用 code-analyzer sub-agent 检查全链路字段传递、命名一致性、文档更新。
不合格则自行修复，最多 3 轮。
```

触发条件是规则，不是建议 — "改了 MLSQL 就必须跑 check_field_order.py"，不是"建议跑一下"。

这样不管需求是新增字段、修 bug、梳理血缘、还是其他任何类型，Claude 都会根据触发条件自动决定用哪些 harness 组件。

日常使用效果：

```
# 新增字段
我：给 seller 表新增 gmv_growth 字段
Claude：[读 context] → [输出 spec] → 我确认 → [写代码] → [自动跑 validator + sub-agent review] → done

# 梳理血缘
我：梳理 product_basic_info 的数据血缘
Claude：[读 context 定位文件] → [追踪 MLSQL 表引用链] → 输出血缘图

# 修 bug
我：br 市场 seller_daily_metrics 字段错位了
Claude：[读 context 定位文件] → [排查原因] → [修复] → [自动跑 validator 验证] → done
```

## 对照原文的映射关系

| 原文概念 | 我的实现 | 实现载体 |
|----------|---------|---------|
| Planner Agent | 读 context，理解需求范围 | context 文件 + 模型自行规划 |
| Generator Agent | 写代码 | Claude Code 本身 |
| Evaluator Agent | 检查产出质量 | sub-agent + validator 脚本 |
| Sprint Contract | 复用 openspec 流程 | 高风险由人确认方向，低风险跳过 |
| 失败重试循环 | 自行修复再 review | CLAUDE.md 触发规则 |

与原文的关键差异：

- **Orchestration**：原文按固定流水线编排（Plan → Generate → Evaluate），我采用工具箱模式 — 只定义触发规则，让模型自行组合。原因是需求类型不固定。
- **Sprint Contract**：原文由两个 Agent 自动协商，全程无人。我的场景中高风险变更（跨市场 DDL、新数据链路）由人确认方向，低风险变更（单文件修复）跳过。这是有意的设计选择 — ETL 变更直接影响线上数据，方向错了不可逆。
- **Review 分层**：人只 review 方向（spec/openspec），Agent 自己 review 质量（validator + sub-agent）。

## 编码的假设

每个 harness 组件都对应一个"模型做不到"的假设：

| 假设 | 组件 |
|------|------|
| 模型不知道我的市场映射和文件位置 | context 文件 |
| 模型不会主动检查字段顺序和国家覆盖 | validator 脚本 |
| 模型自我评价偏高 | 分离 sub-agent 做 evaluator |
| 模型不知道有哪些检查工具可用 | CLAUDE.md 触发规则 |

这些假设需要随模型能力提升持续验证 — 原文的教训："strip away pieces that are no longer load-bearing to performance"。

---

## 附录：自我思考

### Harness 的本质是知识外化

写 context 文件 = 外化领域知识，写 evaluator 规则 = 外化质量标准，写触发规则 = 外化"什么时候该做什么"的判断。这三样东西以前全在我脑子里，每次对话都要重新灌进去。Harness 就是把它们固化下来，一次投入，反复复用。

### 从"干活的人"到"写规则的人"

传统工作：我写 MLSQL、我检查字段顺序、我复制 15 个国家的 DDL。
用 Claude Code：我告诉 Claude 写、我检查、我指出问题。
有 Harness：我定义规则，Claude 自己写、自己检查、自己改。

杠杆层级在提升 — 但前提是规则写得足够好。低质量的 harness 比没有 harness 更危险，因为它给你一个"已经检查过了"的虚假安全感。

### 渐进式建设的务实性

不需要一步到位。先把 context 文件写好，就算不改 CLAUDE.md 流程，日常对话效率也会提升 — 因为 Claude 能自己从文件里找到上下文。这是投入产出比最高的一步。

### 与原文场景的差异

原文是全自动跑 benchmark（无人值守 6 小时），Sprint Contract 由两个 Agent 自动协商。我的场景复用 openspec 流程，高风险变更由人确认方向。这不是退步 — 原文的 Evaluator 能用 Playwright 真实操作 app 来验证，我的 ETL 场景没有等价的端到端验证手段，错误的 DDL 直接影响线上数据且不可逆。在验证能力不够强的地方，人工节点是合理的兜底。

Harness 的目标不是消除人，而是把人从"逐行 review"拉到"确认方向"的层级。

### 需要持续验证的假设

随着 Claude 能力提升，有些组件可能变成多余开销。比如如果未来模型能自己识别"这个 MLSQL 字段顺序和 DDL 不一致"，那 `check_field_order.py` 就可以退役。定期 stress test 每个组件是否仍然 load-bearing。
