# OCR 设计思想解析与 review-service 借鉴规划

> 参考项目：[spencermarx/open-code-review](https://github.com/spencermarx/open-code-review)（以下简称 OCR）
> 适用项目：`review-service`（Go 代码评审网关）
> 文档目的：(1) 系统拆解 OCR 的设计思想；(2) 对照 review-service 当前实现，定位真实差距；(3) 给出分阶段、可落地的借鉴规划。

---

## 目录

- [第一部分：OCR 设计思想解析](#第一部分ocr-设计思想解析)
- [第二部分：review-service 现状对照与差距分析](#第二部分review-service-现状对照与差距分析)
  - [2.5 与 alibaba OCR 二进制的深度结合（补充）](#25-与-alibaba-ocr-二进制的深度结合补充)
- [第三部分：分阶段借鉴规划（A + B 叠加）](#第三部分分阶段借鉴规划)
- [附录：术语与映射表](#附录术语与映射表)

---

## 第一部分：OCR 设计思想解析

### 1.1 一句话概括

> **用真实工程团队的工作流（角色分工、独立审查、辩论质疑、置信度加权、需求对齐）作为多智能体编排的蓝图，用纯 Markdown + 标准化接口实现跨工具可移植，用"不替代人类"的克制定位守住产品边界。**

OCR 最大的创新点不在于"AI 审代码"，而在于把 **"群体智能 > 个体智能"** 这一工程经验，系统化地翻译成了可配置、可复现的多智能体工作流。

### 1.2 三大核心哲学

传统 AI 代码审查是"一次提问 → 一次回答 → 单一视角"。OCR 彻底改变了这一点，借鉴真实工程实践的三个关键机制：

#### 哲学一：多智能体冗余（Multi-Agent Redundancy）

多个 reviewer 实例**独立**审查同一段代码。不同"注意力模式"发现不同问题，一个漏掉的另一个补上。这本质上是用 **分布式一致性** 提升召回率。

- 同一 persona 可配置运行 N 次（redundancy）。
- 两个实例都发现 = "Confirmed by redundancy"（极高置信度）。
- 只有一个发现 = "Single observation"（较低置信度）。

#### 哲学二：先辩论、后综合（Discourse before Synthesis）

reviewer 之间不是简单汇总，而是用**固定四类动作**辩论：

| 动作 | 含义 | 作用 |
|------|------|------|
| `AGREE` | 背书并给出证据 | 提升置信度 |
| `CHALLENGE` | 用推理反驳 | 降低误报（这是关键，去伪存真） |
| `CONNECT` | 跨 reviewer 关联发现 | 发现系统性问题 |
| `SURFACE` | 从讨论中浮现新问题 | 单一 reviewer 发现不了的洞察 |

这模拟了真实评审会议：评审的价值不在"每个人列一遍问题"，而在"彼此质询"。

#### 哲学三：四级置信度加权

最终发现按以下优先级排序，而非简单罗列：

1. **冗余共识**（同一 persona 多次发现）
2. **跨 reviewer 共识**（不同 persona 都发现）
3. **讨论确认**（discourse 阶段被 AGREE）
4. **严重程度**（critical > major > minor > info）

### 1.3 八阶段工作流（由 Tech Lead 编排）

整体由一个 **Tech Lead** 智能体编排，模仿工程团队的技术负责人：

| 阶段 | 作用 | OCR 的真实做法 |
|------|------|---------------|
| 1. Context Discovery | 加载配置、发现项目标准 | 读取 `CLAUDE.md` / `.cursorrules` / OpenSpec 配置，自动注入项目约定 |
| 2. Change Analysis | 分析 `git diff` | 理解改了什么、为什么 |
| 3. **Tech Lead Assessment** | 总结风险、**选派 reviewer 团队**、生成动态指导 | 评估改动 → 识别风险 → 动态选择 reviewer → 为每个 reviewer 生成专属 guidance（含需求上下文） |
| 4. Parallel Reviews | 用 Task 工具**并行**派生独立 reviewer | reviewer 彼此**不可见**，保证独立性 |
| 5. Aggregation | 聚合同类 reviewer 的冗余结果 | 冗余聚合 + 置信度标记 |
| 6. Discourse | reviewer 互相辩论发现 | AGREE/CHALLENGE/CONNECT/SURFACE（`--quick` 可跳过） |
| 7. Synthesis | 生成去重、优先级排序的最终评审 | 输出 Verdict（APPROVE / REQUEST CHANGES / NEEDS DISCUSSION） |
| 8. Presentation | 展示结果，可选发布到 GitHub PR | — |

**两个关键设计点：**

- **Reviewer 独立性**：并行 reviewer 在审查期间看不到彼此输出。
- **Reviewer 自主权（Agency）**：可以像真实工程师一样自主探索 diff 之外的文件、追踪上下游调用、查阅测试与配置，并输出 "What I Explored" 章节。这是把 reviewer 从"被动的模式匹配器"变成"主动的代码库调查者"。

### 1.4 架构设计思想

#### (1) 关注点分离的三包 Monorepo（Nx）
- `packages/agents/` —— **纯 Markdown**（skills、commands、reviewer personas），零运行时依赖
- `packages/cli/` —— TypeScript CLI（安装、会话管理、托管 dashboard）
- `packages/dashboard/` —— React + Socket.IO 交互式界面

#### (2) 纯 Markdown + Agent Skills 标准 = 跨工具可移植性
智能体资产全部是 Markdown，遵循 Agent Skills 标准（`SKILL.md`）。一套资产能配给 14 种 AI 工具（Claude Code / Cursor / Windsurf / OpenCode / Copilot 等）。`ocr init` 检测已装工具，自动生成各自的 slash 命令，但**单一事实来源**仍是 `.ocr/commands/`。

#### (3) SQLite 作为唯一事实来源
取代了 `state.json`，解决 CLI、agents、dashboard **并发访问**状态的问题。采用**三层 schema**：工作流状态 / 制品 / 用户交互。

#### (4) AI CLI 适配器策略
对多个 AI CLI（Claude Code、OpenCode）做抽象，归一化为统一事件流，从而统一追踪所有 AI 操作。

#### (5) 分层能力模型
`IDE-only → Dashboard Viewer → Dashboard AI → GitHub-integrated`，根据环境渐进解锁能力。

#### (6) 配置与资产分离的更新策略
升级时**绝不覆盖用户定制**：config、自定义 reviewer、会话历史全部保留；只更新 skills/commands 中 OCR 托管的代码块。

### 1.5 需求驱动（Requirements-Aware）

- 可传入 spec / proposal / 验收标准 / ticket / 内联描述，任何形式都行（借助 LLM 的"需求发现"能力）。
- Tech Lead 把需求**传播给所有 reviewer**，最终综合里包含一张 **Requirements Verification 表**（已满足 / 有缺口 / 含糊）。
- Reviewer 主动**提出澄清问题**（需求歧义、范围边界、缺失验收标准、边缘场景），像真实工程师一样，并在 synthesis 阶段独立成"Clarifying Questions"章节。

### 1.6 可定制的"评审团队"模型

四类 reviewer 分层：通用型 / 专家型 / 名工程师人格（Martin Fowler、Kent Beck、Sandi Metz 等）/ 自定义。三种组队方式：

| 模式 | 方式 | 是否持久 |
|------|------|---------|
| 库内 reviewer | Team 页面或 `--team` 选择 | 是 |
| 自定义持久 reviewer | `/ocr:create-reviewer` 生成 markdown | 是 |
| 临时 reviewer（Ephemeral） | `--reviewer "..."` 内联描述 | 否，仅本次会话，但完整参与所有阶段 |

### 1.7 其他值得借鉴的设计

- **Code Review Map**：针对 20+ 文件大变更，生成结构化导航文档（分节 + Mermaid 依赖图），由 Map Architect / Flow Analyst / Requirements Mapper 三个专用 agent 生成。
- **多轮审查（Multi-Round）**：`round-1/`、`round-2/`... 迭代式审查，共享上下文跨轮复用。
- **自然语言激活**：说一句"review my code"就能触发，但有"不误触发"的设计。
- **Human Review Translation**：把多 agent 的机器口吻重写成第一人称自然语言，遵循 Google 代码审查指南，再发布到 PR——优雅解决"AI 味太重"的问题。

### 1.8 产品边界：不替代人类

OCR 官方定位很克制：**不替代人类审查**，而是提前拦截机器能发现的问题，让人类评审更快、更聚焦于机器无法判断的部分。这一边界同样适用于 review-service。

---

## 第二部分：review-service 现状对照与差距分析

### 2.1 定位差异（互补而非重复）

| 维度 | OCR（参考） | review-service（本项目） |
|------|-----------|----------------------|
| 形态 | 本地 CLI + Agent Skills | **服务端网关**（Hertz + uber/fx） |
| 触发 | 开发者手动 `/ocr:review` | **Webhook 驱动**（GitHub/GitLab/Gitea/GitCode/工蜂） |
| 执行引擎 | AI 编码助手（Claude/Cursor）作为子 agent | 调用 OCR **二进制**（`OCR_BINARY`） |
| 多 agent | 原生 Task 子 agent，可自主探索代码库 | 多次调用 OCR 二进制 + 不同 `--background` 模拟 |
| 状态存储 | 本地 SQLite | 持久化 **PostgreSQL** |
| 产物 | 本地 `.ocr/sessions/` markdown | DB + Web Console（Vue） |
| 集成 | 本地仓库 | **CI/CD 质量门禁**、行内评论回写 |
| 并发 | 单机 | Worker 队列 + 并发控制（生产级） |

**结论**：review-service 已经是 OCR 思想的**服务端、平台集成、CI 门禁化**版本。结构骨架完整，差距在于**"灵魂"实现的真实程度**。

### 2.2 已实现且做得不错的部分

对照 `internal/service/workflow/workflow.go`、`steps.go`、`agent/manager.go`、`aggregator.go`：

- ✅ **八阶段管道骨架**（`buildPipeline`）完整复刻 OCR 的 8 phase，基于 `ekit/workflow` PipeFlow。
- ✅ **并行多 agent**（`AgentManager.runParallel`）真正的并发 + 信号量限流 + 超时控制。
- ✅ **聚合去重**（`DefaultAggregator`）按 `path:line:endLine` 分组合并，保留 agent 归属，取最高 severity、合并 body、取最长 suggestion——这部分是真实可用逻辑。
- ✅ **置信度加权**（`calculateConfidence`）按 agentCount/severity/category 加权，有真实计算。
- ✅ **会话/轮次/进度/需求/review map/回写** 模块齐全，前端组件完备（据 `REVIEW_SERVICE_ENHANCEMENT_PLAN.md` 实现状态 ~95%）。

### 2.3 关键差距：骨架已就位，灵魂是桩实现

> 这是本规划的核心发现。下列差距按"对评审质量的影响"排序。

#### 🔴 差距 1：Discourse 阶段是模板化的假实现（最高优先级）

OCR 的核心价值是真实的多 agent **辩论**。但当前 `internal/service/discussion/orchestrator.go` 是**规则模板**，没有调用任何 LLM：

```go
// 实际代码（orchestrator.go）—— 全部是硬编码字符串
func generateAgreeReason(finding Finding) string {
    switch finding.Severity {
    case "critical": return "This is a critical issue that needs immediate attention."
    case "major":    return "This is an important issue that should be addressed."
    ...
    }
}
```

问题清单：
- `generateResponseContent` 按 severity/category 返回固定话术，**不是 LLM 推理**。
- `confidence` 固定写死 `0.8`（`discussionEntry{Confidence: 0.8}`）。
- `NewFindings` 永远是空切片（`[]Finding{}`，注释写明 "No new findings in basic implementation"）。
- AGREE/CHALLENGE 的判定完全靠计数比较，没有任何语义质询。

**影响**：这是 OCR 与"普通单次 AI 审查"的根本区别，当前形同虚设，无法去伪存真。

#### 🔴 差距 2：Phase 2 / Phase 3 是空壳（没有真正的 Tech Lead）

`changeAnalysisStep` 和 `analysisStep`（`steps.go:50-88`）只做了两件事：更新 phase 进度、写 DB 状态，然后立即标"completed"。

**缺失的 OCR 能力**：
- 没有"总结改了什么、为什么"。
- 没有"识别风险点"。
- 没有 **动态选派 reviewer 团队**（OCR 会根据改动类型决定是否加入 security/testing reviewer）。
- 没有 **为每个 reviewer 生成专属动态 guidance**（含需求上下文）。当前团队是静态配置的，guidance 缺失。

**影响**：reviewer 团队无法按改动自适应，每次都用固定配置；Tech Lead 的"风险预警"能力缺失。

#### 🟠 差距 3：没有 Reviewer 冗余（Redundancy）

`AgentManager.runParallel`（`manager.go:201`）对每个注册 agent 只运行**一次**。

OCR 的 `principal:2, security:1` 语义是"principal 这个 persona 跑 2 次，结果聚合后标记 Confirmed/Single observation"。当前 `ReviewerAgent` 模型没有 redundancy 字段，manager 也没有"同一 agent 跑 N 次"的逻辑。

**影响**：丢失了 OCR 提升召回与置信度的关键机制；"Confirmed by redundancy" 这类高价值标记无法产生。

#### ✅ ~~差距 4~~（修正）：Reviewer 自主权（Agency）实为二进制内置

> **修正**：初版误判为差距。阅读 alibaba OCR 二进制源码（`internal/tool/` 的 `file_read` / `file_read_diff` / `code_search` / `file_find` / `code_comment`）后发现，**reviewer agency 是二进制自身能力**——其内置 agent 可自主读 diff 之外的文件、追踪上下游、查阅测试。review-service 通过 `--max-tools` / `--tools` 配置即可控制探索轮数与工具集，无需自建。

当前 `ocrAgent.Review`（`ocr_agent.go:81`）已把 `Dir/BaseSHA/HeadSHA` 传给二进制，二进制在其工作区内自主探索。真正待做的是：**按 persona 配置不同的 `--rule`/`--tools` 让探索各有侧重**（见路径 A1），而非重建探索能力。

**结论**：此项从差距中移除，并入路径 A 的优化点。

#### 🟡 差距 5：没有澄清问题（Clarifying Questions）

OCR 要求 reviewer 主动提出需求歧义、范围边界、缺失验收标准、边缘场景的澄清问题，并在 synthesis 独立成章。当前 `requirements` 模块只有 parser/evaluator，没有"生成澄清问题"的能力。

#### 🟡 差距 6：没有 Human Review Translation

OCR 的"Generate Human Review"会把多 agent 机器口吻重写成第一人称自然语言（遵循 Google 代码审查指南）。当前 `writeback/formatter.go` 是结构化格式化，没有"拟人化改写"。PR 评论会有明显"AI 味"。

#### 🟡 差距 7：没有临时 Reviewer（Ephemeral）

OCR 支持 `--reviewer "..."` 内联描述一次性 reviewer，完整参与各阶段。当前团队只能从 DB 配置，无法在单次评审时临时加视角。

#### 🟡 差距 8：Context Discovery 较浅

OCR 的 Phase 1 会发现 `CLAUDE.md` / `.cursorrules` / OpenSpec / `CONTRIBUTING.md` 等项目约定并注入。当前 `contextDiscoveryStep`（`steps.go:27`）只解析 requirements，没有项目标准的自动发现与注入。

### 2.4 差距总览矩阵

| OCR 能力 | review-service 现状 | 优先级 | 实现难度 |
|---------|-------------------|--------|---------|
| 真实 LLM Discourse | ❌ 模板桩 | 🔴 高 | 中（需 LLM 调用层） |
| Tech Lead 推理 + 动态选派 | ❌ 空壳 | 🔴 高 | 中 |
| Reviewer 冗余聚合 | ❌ 单次 | 🟠 中 | 低（多调用编排即可） |
| Reviewer 自主探索 | ✅ **二进制内置** | 🟢 已具备 | 0（见 2.5） |
| persona 差异化 | 🔶 仅 background 文本 | 🟠 中 | 低（`--rule` 按文件模式，见 2.5） |
| `thinking` 推理利用 | ❌ 主动删除 | 🟠 中 | 低（见 2.5） |
| `--preview` 驱动范围分析 | 🔶 已实现未接入 | 🟠 中 | 低（见 2.5） |
| 澄清问题 | ❌ 无 | 🟡 中 | 低（依赖 LLM） |
| Human Review Translation | ❌ 无 | 🟡 中 | 低（依赖 LLM） |
| Ephemeral Reviewer | ❌ 无 | 🟡 低 | 低 |
| 项目标准 Context Discovery | 🔶 部分 | 🟡 低 | 低 |
| 并行多 agent / 聚合去重 / 置信度 | ✅ 真实可用 | — | — |
| 八阶段管道骨架 | ✅ 完整 | — | — |

> **重要修正**：上表初版误判 "Reviewer 自主探索 = ❌ 未建模"。实际阅读 alibaba OCR 二进制源码后发现，**reviewer agency（自主读 diff 外文件、追踪上下游）是二进制自身能力**（`internal/tool/` 的 file_read / code_search / file_find 等），review-service 通过 `--background`/`--rule` 配置即可受益，无需自建。详见 **[2.5 节](#25-与-alibaba-ocr-二进制的深度结合补充)**。

---

### 2.5 与 alibaba OCR 二进制的深度结合（补充）

> 本节基于对 `alibaba/open-code-review` 源码（`cmd/opencodereview/flags.go`、`internal/agent/`、`internal/tool/`、`skills/open-code-review/SKILL.md`）与 review-service `runner/` 包的实际阅读撰写。

#### 2.5.1 二进制真实能力清单

alibaba OCR 二进制一次调用内做的事：

- 按 `--from/--to`（或 `--commit`、或工作区模式）取 diff → 对每个改动文件**并发**跑一个 agent。
- 该 agent **自带工具调用**：`file_read` / `file_read_diff` / `code_search` / `file_find` / `code_comment` → **这就是 reviewer 自主权（agency），已内置**，能读 diff 之外的文件、追踪上下游。
- 改动超 50 行触发内置 **plan/风险分析阶段**（per-file）。
- 产出结构化 JSON：`comments[]`（含 `content` / `suggestion_code` / `existing_code` / `start_line` / `end_line` / **`thinking`（LLM 推理过程）**）+ `summary`（files_reviewed / input_tokens / output_tokens / cache_read/write / elapsed）。

CLI 关键 flag（`ocr review`）：

| flag | 作用 | review-service 利用情况 |
|------|------|----------------------|
| `--rule <path>` | 按 **glob 文件模式**的规则 JSON（`{rules:[{path,rule}]}`） | ⚠️ 仅全局一个，**未按 persona 区分** |
| `-b/--background` | 业务/需求上下文 | ✅ 已用（persona 唯一区分手段） |
| `--tools <path>` | 自定义工具配置 | ⚠️ 全局，未按 persona 区分 |
| `--max-tools N` | 每 file 工具调用轮数（min 10） | ✅ 已透传 |
| `--preview` / `-p` | 列出待评审文件，不跑 LLM | 🔶 preview.go 已实现**但未接入工作流** |
| `--audience human\|agent` | human=stderr 进度流，agent=仅摘要 | ✅ 用 human + 解析 stderr 进度 |
| `--concurrency` / `--max-git-procs` | 并发控制 | ✅ 已透传 |
| `ocr rules check <file>` | 预览某文件命中哪条规则 | ❌ 未用 |

#### 2.5.2 结合是不对称的（核心结论）

二进制是**单 agent、per-file、带工具调用**的评审器。因此：

- **能深度结合**：persona 差异化（`--rule`）、保留 `thinking`、`--preview` 驱动范围分析、token 遥测、**冗余靠多调用编排**。
- **二进制做不到、必须 service 层上 LLM**：discourse（辩论）、跨文件 Tech Lead 推理、跨 persona 综合、拟人化翻译。单遍评审器天生无法自我质询。

> 一句话：**"深度结合二进制" 让单次评审质量最大化；"多 agent 辩论" 必须在 service 层。两者叠加（B 消费 A 的产出）是最佳路径。**

#### 2.5.3 persona = background + 规则文件（设计决策）

当前 persona 仅靠 `--background` 文本区分（`loader.go:51` `CreateRunner(agentModel.Background)`）。升级为 **background + 规则文件**：

```go
// ReviewerAgent 模型新增字段
RuleFile   string // 按 glob 模式的 rule JSON，透传 --rule
Redundancy int    // 冗余次数（多调用编排用）
```

规则文件示例（security persona）：

```json
{
  "rules": [
    {"path": "**/*.go",       "rule": "检查 SQL 注入、未授权访问、敏感信息日志、goroutine 泄漏"},
    {"path": "**/*mapper*.xml","rule": "检查 SQL 注入风险与缺失闭合标签"},
    {"path": "**/auth/**",     "rule": "重点审查鉴权与权限校验逻辑"}
  ]
}
```

工厂签名 `CreateRunner(background)` → `CreateRunner(background, rulePath)`，runner 双透传 `--background` + `--rule`。persona 区分度从"一段散文"升级为"按文件类型命中不同规则集"。

#### 2.5.4 配置复用与降级策略

- **LLM 配置复用**：路径 B 的 LLM 编排层直接复用 OCR 已有配置（`OCRSettings` 的 URL/Model/AuthToken），不引入新凭证。
- **Noop 降级**：任何 LLM 增强都必须有非 LLM 降级路径（LLM 不可用 → 退回当前模板逻辑），保证服务可用。

---

## 第三部分：分阶段借鉴规划

### 3.0 总体策略（A + B 叠加）

采用 **A+B 叠加** 路径：先做低成本、榨干二进制的 A，再叠加补 multi-agent 灵魂的 B，**B 消费 A 的产出**。

- **路径 A（榨干二进制）**：利用 alibaba OCR 二进制已有但被忽略的能力——per-agent 规则文件、保留 `thinking`、`--preview` 驱动范围分析、冗余多跑。让**单次评审质量**最大化，改动集中在 `runner/`/`agent/` 层。
- **路径 B（补 multi-agent 灵魂）**：在 service 层自建轻量 LLM 编排层，把 discourse / 跨文件 Tech Lead / 综合从桩实现做成真智能。

**A 如何喂给 B（叠加点）**：
- A1 规则 → persona 产出真正不同的 findings → B 的 discourse 有东西可辩
- A2 `thinking` → B 的 discourse 能引用/质疑**真实推理过程**，而非只看 finding 文本
- A3 `--preview` → B 的 Tech Lead 有范围可推理、按文件类型动态选团队
- A4 冗余 → B 的置信度加权拿到 "Confirmed by redundancy" 信号

**关键约束**：
- 不破坏现有 `workflow` 管道与 `AgentManager` 接口，所有增强以**新增 step / 新增字段 / 新增 provider**的方式接入。
- **LLM 配置复用**：路径 B 的 LLM 编排层直接复用 OCR 已有配置（`OCRSettings` 的 URL/Model/AuthToken），不引入新凭证。
- **可降级**：任何 LLM 增强都必须有非 LLM 降级路径（LLM 不可用 → 退回当前模板逻辑），保证服务可用。
- 复用项目已有依赖（ekit、Hertz、GORM、fx），不引入重型框架。

### 3.1 阶段 0：地基（Persona 模型升级 + LLM 编排层）

**目标**：为路径 A 和 B 打好两块地基——persona 配置模型升级（background + 规则文件）、统一 LLM 调用层。

**任务拆解：**

1. **`ReviewerAgent` 模型升级**（A/B 共用地基）
   - `internal/repository/postgres/models/reviewer_agent.go` 增加 `RuleFile string`（按 glob 模式的 rule JSON）、`Redundancy int`（默认 1）。
   - GORM AutoMigrate；前端 AgentSettings 增加"规则文件"编辑器（JSON）与"冗余次数"输入。
   - 规则文件可存 DB（文本字段）或落盘到 `<repo>/.opencodereview/rule-<name>.json` 由 `--rule` 引用。

2. **runner 工厂支持双透传**（A 地基）
   - `agent/runner_factory.go` 的 `CreateRunner(background)` → `CreateRunner(background, rulePath, toolsPath)`。
   - `runner/runner.go` 的 `runOnce` 已有 `--background`/`--rule` 逻辑，确保按 agent 传入各自的 rule/tools 而非全局。

3. **新建 `internal/service/llm/` 包**（B 地基，统一 LLM 抽象）
   - `provider.go`：定义 `LLMProvider` 接口（`Complete(ctx, prompt) (string, error)`）。
   - **复用 OCR 已有模型配置**：从 `OCRSettings`（URL/Model/AuthToken/AuthHeader/UseAnthropic）构造客户端，对接 OpenAI 兼容或 Anthropic endpoint，不引入新凭证。
   - `openai.go`：`OpenAICompatibleProvider`；`noop.go`：`NoopProvider`（降级用，返回空/错误让上层退回模板）。
   - 通过 uber/fx 注入，单例。

**涉及文件**：
- 修改：`internal/repository/postgres/models/reviewer_agent.go`
- 修改：`internal/service/agent/runner_factory.go`、`internal/service/agent/loader.go`、`internal/service/runner/runner.go`
- 新增：`internal/service/llm/provider.go`、`internal/service/llm/openai.go`、`internal/service/llm/noop.go`、`internal/service/llm/module.go`
- 修改：`frontend/src/views/AgentSettings.vue`

**验收标准**：
- [ ] `ReviewerAgent` 支持 RuleFile + Redundancy，前端可编辑。
- [ ] 每个 agent 调用 OCR 时带上各自 `--background` + `--rule`。
- [ ] `LLMProvider` 接口可用，复用 OCR 配置，NoopProvider 降级路径有测试。
- [ ] `go build ./...` 通过。

---

### 3.2 路径 A：榨干二进制（低成本，先做）

**目标**：利用 alibaba OCR 二进制已有但被忽略的能力，最大化单次评审质量。

#### A1. per-agent 规则文件（最高杠杆）

- **当前**：persona 仅靠 `--background` 文本区分（`loader.go:51`）。
- **改法**：每个 persona 配独立 rule JSON，`--background` + `--rule` 双透传。二进制原生按 glob 文件模式匹配规则，"security" persona 对 `**/*.go` 命中安全规则集——区分度从"散文"升级为"按文件类型命中不同规则"。
- **依赖**：阶段 0 的模型升级 + 工厂双透传。

#### A2. 保留 `thinking`，喂给 discourse

- **当前**：`runner/output.go:163 sanitizeRawOCR` **主动删除** `thinking` 字段。
- **改法**：停止删除；在 `review.Finding` / `agent.Finding` 新增 `Thinking string` 字段保留 LLM 推理。把 `summary` 的 token 用量（input/output/cache）接入 `stats`，做成本与质量观测。
- **B 的消费点**：discourse prompt 注入各 reviewer 的 `thinking`，让 AGREE/CHALLENGE 能引用真实推理。

#### A3. `--preview` 驱动 Tech Lead 范围分析

- **当前**：`runner/preview.go` 已实现 `--preview`（列出待评审文件）**但未接入工作流**。
- **改法**：接入当前空壳的 `changeAnalysisStep`/`analysisStep`，用 preview 产出的文件列表驱动团队选派（如 `*.vue` 出现 → 加 frontend；auth 相关路径 → 加 security）。
- **B 的消费点**：真 Tech Lead（B3）在 preview 范围上调 LLM 做跨文件风险推理。

#### A4. 冗余多跑（multi-call 编排）

- **当前**：每个 agent 跑 1 次（`manager.go:201 runParallel`）。
- **改法**：`Redundancy=N`（阶段 0 字段）→ `AgentLoader` 注册 N 个实例（name 后缀 `-1`..`-N`，相同 background+rule）。`DefaultAggregator` 聚合时记录"同一 persona 多实例是否都发现"，Finding 增加 `RedundancyStatus`（`confirmed` / `single`）。
- **B 的消费点**：置信度加权拿到 "Confirmed by redundancy" 高置信信号。

**涉及文件**：
- A1：`internal/service/agent/loader.go`、`runner_factory.go`、`runner/runner.go`
- A2：`internal/service/runner/output.go`、`internal/service/agent/types.go`、`internal/service/review`（Finding）
- A3：`internal/service/workflow/steps.go`
- A4：`internal/service/agent/loader.go`、`aggregator.go`、`types.go`（Finding）

**验收标准**：
- [ ] A1：不同 persona 的 rule 文件实际生效，产出差异化 findings。
- [ ] A2：`Finding.Thinking` 非空且可被 discourse 消费；stats 可见 token 用量。
- [ ] A3：preview 文件列表驱动了团队选派（含 `*.vue` 时加入对应 agent）。
- [ ] A4：redundancy=2 的 agent 执行 2 次，共同发现标记 "Confirmed by redundancy"。

---

### 3.3 路径 B：补 multi-agent 灵魂（叠加，消费 A 的产出）

**目标**：把 discourse / 跨文件 Tech Lead 从模板桩做成真智能。**二进制天生单 agent，这些只能在 service 层。**

#### B1. 真 Discourse（消费 A1 差异化 findings + A2 thinking）

- **当前**：`discussion/orchestrator.go` 的 `generateResponseContent` 按 severity 返回固定话术；confidence 固定 `0.8`；`NewFindings` 永远空。
- **改法**：重写为调用 `LLMProvider`，prompt 含各 reviewer 的 finding + `thinking`（A2）+ 其他 reviewer 发现，让 agent 以自己 persona 发表 AGREE/CHALLENGE/CONNECT/SURFACE。解析返回的结构化 JSON（response_type + content + evidence + confidence）。
- 显式要求"辩论中出现新问题用 SURFACE 提出" → 注入 `DiscussionResult.NewFindings`。
- **降级**：LLM 失败退回当前模板逻辑。

#### B2. 真 Tech Lead（消费 A3 preview 范围）

- **当前**：`analysisStep` 只更新进度。
- **改法**：调用 `LLMProvider`，输入 A3 的 preview 文件列表 + diff + 需求，产出：改动摘要、风险点清单、推荐 reviewer 团队、每个 reviewer 的动态 guidance。写入 `ReviewSession`。
- `AgentManager` 新增 `RunAgentsByNames(ctx, workspace, names, guidance)`；`parallelReviewsStep` 优先用 Tech Lead 推荐团队，无推荐退回 DB 配置。

#### B3. 置信度动态化 + 综合

- discourse 结果按 AGREE 增 / CHALLENGE 减动态调整 confidence（替换固定 0.8）。
- 综合阶段按 OCR-spencermarx 四级权重排序：冗余共识（A4）→ 跨 reviewer 共识 → 讨论确认（B1）→ 严重程度。

**涉及文件**：
- B1：`internal/service/discussion/orchestrator.go`、`prompts.go`
- B2：`internal/service/workflow/steps.go`、`internal/service/agent/manager.go`、`interface.go`、`internal/service/workflow/session.go`
- B3：`internal/service/discussion/orchestrator.go`、`internal/service/workflow/steps.go`（generateFinalReport）

**验收标准**：
- [ ] B1：discourse 产出真实 AGREE/CHALLENGE 文本（引用 thinking），`NewFindings` 可非空；LLM 不可用时自动降级。
- [ ] B2：Phase 3 产出真实改动摘要 + 风险清单 + 动态团队选派（可在前端查看）。
- [ ] B3：finding 置信度按讨论结果动态变化，综合排序体现四级权重。
- [ ] 单测覆盖 prompt 构造、JSON 解析、降级路径。

### 3.4 阶段四：澄清问题 + Human Review Translation（🟡 中优先级）

**目标**：补齐需求对齐与拟人化发布两个差异化能力。

**任务拆解：**

1. **澄清问题（Clarifying Questions）**
   - 在 `requirements` 模块新增 `Questioner`：调用 LLM，基于需求歧义/范围/边缘场景生成结构化问题清单。
   - synthesis 阶段汇入 `FinalReport.ClarifyingQuestions`。
   - 前端 SessionDetail 增加"澄清问题"区块。

2. **Human Review Translation**
   - 新增 `internal/service/writeback/translator.go`：调用 LLM，把多 agent findings 改写成第一人称自然语言，遵循 Google 代码审查指南（保持简洁、聚焦代码而非作者、给出可操作建议）。
   - 发布前支持"预览 → 编辑 → 发布"。
   - 在 publisher handler 增加 `mode=human` 参数。

**涉及文件**：
- 新增：`internal/service/requirements/questioner.go`
- 新增：`internal/service/writeback/translator.go`
- 修改：`internal/service/workflow/steps.go`（synthesisStep）
- 修改：`biz/handler/review_service/publisher.go`、前端

**验收标准**：
- [ ] FinalReport 含 ClarifyingQuestions，前端可展示。
- [ ] human 模式发布前可预览，文本为第一人称、无机器味。

### 3.5 阶段五：Ephemeral Reviewer + Context Discovery 增强（🟡 低优先级）

**目标**：补齐灵活性与上下文深度。

**任务拆解：**

1. **Ephemeral Reviewer**
   - Review 请求体增加 `ephemeral_reviewers []string`（内联描述）。
   - `parallelReviewsStep` 用这些描述即时构造临时 agent（不经 DB），与库内 agent 一起并行，但标记为 ephemeral，不持久化。

2. **Context Discovery 增强**
   - `contextDiscoveryStep` 增加项目约定文件发现：检测仓库内 `AGENTS.md` / `CLAUDE.md` / `.cursorrules` / `CONTRIBUTING.md` / `CONVENTION.md`，读取后注入 `Workspace.ProjectStandards`。
   - 拼入 OCR `--background` 或新增 `--project-context` 透传。

**涉及文件**：
- 修改：`biz/model/review.go`、`internal/service/workflow/steps.go`
- 修改：`internal/service/agent/types.go`（Workspace）

**验收标准**：
- [ ] 单次评审可加临时 reviewer，参与全流程但不入库。
- [ ] 项目约定文件被自动发现并注入评审上下文。

### 3.6 阶段六：验证与可观测（贯穿全程）

- 为每个新增 LLM 调用点加 **降级单测**（mock provider 返回错误，断言退回模板逻辑）。
- 为 redundancy 展开、动态选派加**集成测试**。
- 在 `stats` 模块增加：LLM 调用次数/耗时/失败率、discourse 产出的新发现数、redundancy 确认率——量化"做实"后的收益。
- 运行 `go build ./...`、`go test ./...`、`golangci-lint run ./...` 确保零回归。

---

## 附录：术语与映射表

### A.1 OCR → review-service 概念映射

| OCR 概念 | review-service 对应 | 文件位置 |
|---------|---------------------|---------|
| Tech Lead 编排 | `ReviewWorkflow` | `internal/service/workflow/workflow.go` |
| 8 阶段管道 | `buildPipeline()` + 各 `*Step` | `internal/service/workflow/steps.go` |
| Reviewer 子 agent | `ocrAgent`（= OCR 二进制 + background） | `internal/service/agent/ocr_agent.go` |
| 并行执行 | `AgentManager.runParallel` | `internal/service/agent/manager.go` |
| 聚合去重 | `DefaultAggregator` | `internal/service/agent/aggregator.go` |
| Discourse | `discussion.Orchestrator`（⚠️ 当前为桩） | `internal/service/discussion/orchestrator.go` |
| Requirements-Aware | `requirements.Parser/Evaluator` | `internal/service/requirements/` |
| Code Review Map | `reviewmap.Generator` | `internal/service/reviewmap/` |
| Session（多轮） | `session.Service` + `models.ReviewSession` | `internal/service/session/` |
| SQLite 三层 schema | PostgreSQL（review/session/settings/stats） | `internal/repository/postgres/` |
| 进度实时 | `progress.ProgressManager` + WebSocket | `internal/service/progress/` |
| Human Review Translation | `writeback`（⚠️ 待增强） | `internal/service/writeback/` |

### A.2 本规划阶段总览（A + B 叠加）

| 阶段 | 主题 | 路径 | 核心交付 |
|------|------|------|---------|
| 0 | 地基：Persona 模型升级 + LLM 编排层 | A+B 共用 | RuleFile/Redundancy 字段、`--background`+`--rule` 双透传、复用 OCR 配置的 `LLMProvider` |
| A1 | per-agent 规则文件 | A | persona 按 glob 文件模式差异化 |
| A2 | 保留 `thinking` + token 遥测 | A | discourse 有推理可消费、成本可观测 |
| A3 | `--preview` 驱动 Tech Lead 范围 | A | 按文件类型动态选团队 |
| A4 | 冗余多跑 | A | Confirmed by redundancy |
| B1 | 真 Discourse | B | 真实辩论，去伪存真（消费 A1+A2） |
| B2 | 真 Tech Lead 跨文件推理 | B | 风险预警（消费 A3） |
| B3 | 置信度动态化 + 四级加权综合 | B | 体现冗余/共识/讨论/严重度（消费 A4+B1） |
| 4 | 澄清问题 + Human Translation | B | 需求对齐，拟人化发布 |
| 5 | Ephemeral Reviewer + Context Discovery | — | 灵活性与上下文深度 |
| 6 | 验证与可观测 | 贯穿 | 零回归 + 收益量化 |

### A.3 二进制 flag → review-service 利用情况

| 二进制 flag / 能力 | 当前利用 | 目标状态 | 所属阶段 |
|-------------------|---------|---------|---------|
| `--background` | ✅ persona 唯一区分 | 配合 `--rule` 双透传 | 0 / A1 |
| `--rule`（glob 模式规则） | ⚠️ 全局一个 | per-agent 规则文件 | A1 |
| `--tools`（自定义工具） | ⚠️ 全局 | per-agent（可选） | A1+ |
| `thinking` 字段 | ❌ `sanitizeRawOCR` 删除 | 保留，喂给 discourse | A2 |
| `summary` token 用量 | 🔶 解析未利用 | 接入 stats | A2 |
| `--preview`（列文件） | 🔶 已实现未接入 | 驱动 Tech Lead 选团队 | A3 |
| `ocr rules check` | ❌ 未用 | UI 规则预览 | 5 |
| 内置 50 行 plan 风险阶段 | 未显式利用 | 不需改造，受益即可 | — |
| `--audience human` + stderr 解析 | ✅ 已用 | 保持（注意与 agent 模式不混用） | — |

### A.4 关键原则（贯穿实施）

1. **可降级**：任何 LLM 增强都必须有非 LLM 降级路径，保证服务可用。
2. **不破坏接口**：以新增字段/step/provider 方式接入，避免大面积重构。
3. **复用现有依赖**：ekit、Hertz、GORM、fx，不引入重型框架。
4. **克制边界**：始终定位为"辅助人类评审"，不追求完全替代。
5. **量化验证**：每个阶段都要有可观测指标证明收益。

---

*本文档基于对 spencermarx/open-code-review 仓库（README / spec / openspec config / review-orchestration spec）、alibaba/open-code-review 仓库（`cmd/opencodereview/flags.go` / `internal/agent/` / `internal/tool/` / `skills/open-code-review/SKILL.md`）与 review-service 源码（runner / workflow / agent / discussion / aggregator）的实际阅读撰写。*
