# 架构演进：executor.Service → eino → 引擎适配层

> 状态: **演进史**。本文档记录评审执行架构的三次形态变迁与各自的决策依据，
> 供回溯"为什么长成这样"。文中"旧架构/新架构"的对比轴是**历史切片**，
> 不是当前形态。
>
> 当前形态见文末「当前形态」一节。

## 0. 三次形态变迁

| 阶段 | 形态 | 状态 |
|---|---|---|
| 一 | `executor.Service`（单体执行器） | 已被替换 |
| 二 | `eino.Executor`（eino 工作流编排） | 已被再次收敛 |
| 三 | 引擎适配层 `internal/eino` + 编排端口 `review.PipelineRunner` | **当前** |

- 阶段一 → 阶段二：把单体执行器换成 eino 工作流，接口与依赖保持不变。
- 阶段二 → 阶段三：把**前置编排**从引擎适配层下沉到 service 层。原因是这段流程
  属于评审领域（克隆工作区 → 项目策略 → 文件过滤 → 规则合并 → 记忆召回 →
  OCR 执行参数），与"用哪个引擎跑"无关。此前它住在引擎适配层，导致该层直接持
  6 个仓储、绕过 service 层直读 5 张业务表，又反向调用 service/review 的 13 个
  导出函数——两套编排互相调用，是耦合的主要来源。

---

## 1. 整体架构

### 旧架构（executor.Service，已移除）

```
┌─────────────────────────────────────────────────────────────┐
│                    executor.Service                          │
│  ┌───────────────────────────────────────────────────────┐  │
│  │  Execute() - 200+ 行大方法                              │  │
│  │  ├── 预算检查                                           │  │
│  │  ├── workspace 准备（clone + policy + rules）            │  │
│  │  ├── OCR resume 检测                                    │  │
│  │  ├── diff 恢复                                          │  │
│  │  └── 分支判断：shouldUseWorkflow()                      │  │
│  │      ├── true  → executeWithWorkflow()                  │  │
│  │      └── false → executeWithRunner()                    │  │
│  └───────────────────────────────────────────────────────┘  │
│                                                             │
│  executeWithRunner()         executeWithWorkflow()          │
│  ├── runner.Run()            ├── workflow.RunReview()       │
│  ├── BuildReport()           ├── findings 聚合               │
│  ├── policy.Evaluate()       ├── BuildReport()              │
│  └── publishWriteback()      └── publishWriteback()         │
└─────────────────────────────────────────────────────────────┘
```

**特点：**
- 单一大类，职责混合
- 手写调用链，流程硬编码
- 难以独立测试各阶段
- 代码重复度高

---

### 新架构（eino.Executor + compose.Chain）

```
┌─────────────────────────────────────────────────────────────┐
│                      eino.Executor                           │
│  ┌───────────────────────────────────────────────────────┐  │
│  │  Execute() - 清晰入口                                   │  │
│  │  ├── 预算检查                                           │  │
│  │  ├── workspace 准备（复用 WorkspacePreparer）            │  │
│  │  ├── OCR resume 检测                                    │  │
│  │  └── 分支判断：shouldUseWorkflow()                      │  │
│  │      ├── true  → executeWithWorkflow()                  │  │
│  │      └── false → executeWithRunner()                    │  │
│  └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
           │
           ▼
┌─────────────────────────────────────────────────────────────┐
│              compose.Chain[*ReviewInput, *ReviewOutput]      │
│                                                             │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐  │
│  │   Node 1     │───▶│   Node 2     │───▶│   Node 3     │  │
│  │   prepare    │    │   run_ocr    │    │   build      │  │
│  │   workspace  │    │              │    │   result     │  │
│  └──────────────┘    └──────────────┘    └──────────────┘  │
│         │                                      │           │
│         ▼                                      ▼           │
│  ┌──────────────┐                      ┌──────────────┐   │
│  │   Node 4     │◀─────────────────────│   assemble   │   │
│  │   output     │                      │   output     │   │
│  └──────────────┘                      └──────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

**特点：**
- 模块化设计，职责分离
- 声明式工作流，流程可组合
- 每个节点可独立测试
- 代码复用度高

---

## 2. 代码组织

### 旧架构文件结构

```
internal/service/executor/
├── service.go        # 798 行 - 所有逻辑混在一起
└── persistence.go    # 142 行 - 持久化辅助
```

**问题：**
- `service.go` 包含 30+ 个方法
- `Execute()` 方法超过 200 行
- 业务逻辑、持久化、通知、metrics 混杂

### 新架构文件结构

```
internal/eino/
├── executor.go           # 725 行 - 执行器主逻辑
├── persistence.go        # 141 行 - 持久化辅助
├── tools.go              # 305 行 - eino Tool 封装
├── workflow.go           # 229 行 - compose.Chain 工作流
├── agent.go              # 217 行 - 多 Agent 并行执行
└── executor_integration.go # 194 行 - 集成辅助
```

**优势：**
- 单一职责原则
- 每个文件 < 300 行
- 清晰的依赖关系

---

## 3. 执行流程对比

### 旧架构：executeWithRunner()

```go
func (s *Service) executeWithRunner(...) {
    // 1. 记录 history
    s.appendHistory(ctx, id, "ocr", "running", ...)

    // 2. 执行 OCR（直接调用）
    start := time.Now()
    result, err := s.runner.Run(ctx, workspace)

    // 3. 更新 metrics（手写）
    if s.llmMetrics != nil {
        s.llmMetrics.TotalLatency.Add(...)
        s.llmMetrics.Calls.Add(1)
    }

    // 4. 计算指纹
    types.ComputeFingerprints(result.Findings)

    // 5. 构建报告（手写）
    report := review.BuildReport(&result)
    analysis := review.BuildAnalysis(result)

    // 6. 评估策略（手写）
    gateEngine := s.policy
    gate := gateEngine.Evaluate(toPolicyFindings(result.Findings))

    // 7. 组装响应（手写）
    resp := domain.Response{...}

    // 8. 发布 writeback
    wbSummary, wbErr := s.publishWriteback(...)

    // 9. 更新数据库
    s.updateWithRawOCR(...)

    // 10. 发送通知
    s.notifyCompletion(...)

    // 11. 记录指标
    s.recordReviewMetrics(...)
}
```

**问题：**
- 11 个步骤线性排列
- 错误处理分散
- 难以复用单个步骤

---

### 新架构：compose.Chain 工作流

```go
// 工作流定义（workflow.go）
func BuildReviewWorkflow(runner, policyEngine) *compose.Chain {
    chain := compose.NewChain[*ReviewInput, *ReviewOutput]()

    // Node 1: 准备工作区
    chain.AppendLambda(compose.InvokableLambda(func(ctx, input) {
        input.Workspace.OCR = review.ProjectPolicyToOCROverrides(...)
        return &ReviewContext{Input: *input, Workspace: input.Workspace}
    }), compose.WithNodeName("prepare_workspace"))

    // Node 2: 执行 OCR
    chain.AppendLambda(compose.InvokableLambda(func(ctx, rc) {
        result, err := runner.Run(ctx, rc.Workspace)
        rc.Result = &result
        return rc
    }), compose.WithNodeName("run_ocr"))

    // Node 3: 构建结果
    chain.AppendLambda(compose.InvokableLambda(func(ctx, rc) {
        types.ComputeFingerprints(rc.Result.Findings)
        report := review.BuildReport(rc.Result)
        rc.Report = &report
        // ... 评估策略
        return rc
    }), compose.WithNodeName("build_result"))

    // Node 4: 组装输出
    chain.AppendLambda(compose.InvokableLambda(func(ctx, rc) {
        return &ReviewOutput{Response: resp, Result: *rc.Result}
    }), compose.WithNodeName("assemble_output"))

    return chain
}

// 执行（executor.go）
func (e *Executor) executeWithRunner(...) {
    input := &ReviewInput{...}
    output, err := RunReviewWorkflow(ctx, e.workflowRunner.chain, input)
    // ... writeback, notification, metrics
}
```

**优势：**
- 每个节点职责单一
- 节点可独立测试
- 工作流可复用
- 类型安全（泛型）

---

## 4. 多 Agent 执行对比

### 旧架构：串行执行

```go
func (m *AgentManager) runAgents(ctx, agents, ...) {
    // 串行执行每个 agent
    for _, agent := range agents {
        result, err := agent.Review(ctx, workspace)
        results = append(results, result)
    }

    // 聚合结果
    aggregated := aggregator.Aggregate(results)
    return aggregated
}
```

**问题：**
- Agent 串行执行，耗时 = N × 单 agent 耗时
- 无并发控制
- 错误处理简单

---

### 新架构：并行执行

```go
// agent.go
func BuildMultiAgentWorkflow(agentManager) *compose.Chain {
    chain := compose.NewChain[*AgentReviewInput, *AgentReviewOutput]()

    // Node 1: 解析 agents
    chain.AppendLambda(resolveNode)

    // Node 2: 并行执行
    chain.AppendLambda(compose.InvokableLambda(func(ctx, input) {
        // 使用 AgentManager 的并行能力
        aggregatedResult, err := agentManager.RunAgentsForProject(
            ctx, workspace, provider, repo, nil, nil,
        )

        // 转换结果
        results := make([]AgentResult, 0, len(aggregatedResult.AgentResults))
        for _, ar := range aggregatedResult.AgentResults {
            results = append(results, AgentResult{...})
        }
        return results
    }))

    // Node 3: 聚合去重
    chain.AppendLambda(aggregateNode)

    return chain
}
```

**优势：**
- Agent 并行执行，耗时 = max(单 agent 耗时)
- 自动去重（基于 path+line+title）
- 结构化错误处理

---

## 5. 工具化能力

### 旧架构

```go
// 无法被外部 AI agent 调用
// 必须通过 HTTP API 间接访问
```

### 新架构：eino Tool 封装

```go
// tools.go - OCR 评审工具
func NewOCRReviewTool(runner domain.Runner) *SimpleTool {
    return NewSimpleTool(
        func(ctx) (*schema.ToolInfo, error) {
            return &schema.ToolInfo{
                Name: "ocr_review",
                Desc: "Run OCR on a code workspace",
                ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
                    "dir":      {Type: "string", Required: true},
                    "base_sha": {Type: "string"},
                    "head_sha": {Type: "string"},
                }),
            }, nil
        },
        func(ctx, arguments) (string, error) {
            // 解析参数，执行 OCR，返回 JSON 结果
        },
    )
}
```

**优势：**
- LLM 可通过 function calling 直接调用
- 标准化的工具接口
- 可被 eino ADK agent 使用

---

## 6. 依赖注入对比

### 旧架构：uber/fx 直接注入

```go
// modules/review.go
func NewExecutorService(deps ExecutorDeps) *executor.Service {
    return executor.NewService(executor.Config{
        ReviewRepo:   deps.Repos.Review,
        Runner:       deps.OcrRunner,
        Policy:       deps.PolicyEngine,
        // ... 15+ 个依赖
    })
}
```

### 新架构：相同模式，新类型

```go
// modules/review.go
func NewExecutorService(deps ExecutorDeps) *eino.Executor {
    return eino.NewExecutor(eino.ExecutorConfig{
        ReviewRepo:   deps.Repos.Review,
        Runner:       deps.OcrRunner,
        Policy:       deps.PolicyEngine,
        // ... 相同依赖
    })
}
```

**变化：**
- 接口不变（`review.Executor`）
- 依赖不变
- 仅实现类替换

---

## 7. 测试对比

### 旧架构测试

```go
func TestExecuteWithRunner(t *testing.T) {
    // 需要 mock 10+ 个依赖
    svc := executor.NewService(executor.Config{
        ReviewRepo: mockRepo,
        Runner:     mockRunner,
        Policy:     mockPolicy,
        // ... 更多 mock
    })

    // 测试整个流程
    resp, err := svc.Execute(ctx, id, req, provider, nil)
    // 难以测试单个步骤
}
```

### 新架构测试

```go
func TestBuildResultNode(t *testing.T) {
    // 可以单独测试 Node 3
    node := compose.InvokableLambda(func(ctx, rc *ReviewContext) (*ReviewContext, error) {
        // ... build result logic
        return rc, nil
    })

    // 测试单个节点
    result, err := node.Invoke(ctx, &ReviewContext{Result: &domain.Result{...}})
}

func TestWorkflow(t *testing.T) {
    // 测试整个工作流
    chain, _ := BuildReviewWorkflow(mockRunner, mockPolicy)
    output, err := RunReviewWorkflow(ctx, chain, input)
}
```

**优势：**
- 节点级单元测试
- 工作流级集成测试
- Mock 更简单

---

## 8. 扩展性对比

### 旧架构扩展

```go
// 添加新步骤需要修改 executeWithRunner()
func (s *Service) executeWithRunner(...) {
    // ... 现有步骤

    // 新步骤：插入到中间
    // 需要理解上下文，容易出错

    // ... 后续步骤
}
```

### 新架构扩展

```go
// 添加新节点只需 AppendLambda
chain.AppendLambda(compose.InvokableLambda(func(ctx, rc) {
    // 新步骤逻辑
    return rc, nil
}), compose.WithNodeName("new_step"))

// 或使用 Branch 实现并行
branch := compose.NewBranch(func(ctx, rc) string {
    if condition {
        return "path_a"
    }
    return "path_b"
})
branch.AddLambda("path_a", nodeA)
branch.AddLambda("path_b", nodeB)
chain.AppendBranch(branch)
```

**优势：**
- 声明式扩展
- 不影响现有节点
- 支持分支、并行、重试

---

## 9. 性能对比

| 指标 | 旧架构 | 新架构 | 改进 |
|------|--------|--------|------|
| 单 Agent 评审 | ~3-7 分钟 | ~3-7 分钟 | 无变化 |
| 多 Agent 评审（串行） | N × 3-7 分钟 | N × 3-7 分钟 | 无变化 |
| 多 Agent 评审（并行） | 不支持 | max(3-7 分钟) | **N 倍提升** |
| 内存占用 | 较低 | 略高（工作流状态） | +5% |
| 启动时间 | 较快 | 略慢（工作流编译） | +10ms |

---

## 10. 迁移路径

### 阶段 1：替换执行器（已完成）

```diff
- executor.Service
+ eino.Executor
```

- 接口不变
- 依赖不变
- 行为不变

### 阶段 2：启用 eino 工作流（可选）

```go
// 在 Execute() 中使用 compose.Chain
if useEinoWorkflow {
    return e.executeWithEinoWorkflow(ctx, id, req, workspace, projectPolicy)
}
return e.executeWithRunner(ctx, id, req, workspace, projectPolicy)
```

### 阶段 3：工具化（可选）

```go
// 注册 eino Tools
tools := []tool.InvokableTool{
    eino.NewOCRReviewTool(runner),
    eino.NewProviderTool(provider),
    eino.NewWritebackTool(writeback),
}

// 可被 LLM agent 调用
```

---

## 总结

| 维度 | 旧架构 | 新架构 | 评价 |
|------|--------|--------|------|
| **代码组织** | 单一大类 | 模块化 | ✅ 新架构更好 |
| **可测试性** | 困难 | 容易 | ✅ 新架构更好 |
| **扩展性** | 修改现有代码 | 声明式扩展 | ✅ 新架构更好 |
| **并行能力** | 不支持 | 原生支持 | ✅ 新架构更好 |
| **工具化** | 不支持 | 原生支持 | ✅ 新架构更好 |
| **学习曲线** | 低 | 中等 | ⚠️ 旧架构更简单 |
| **调试难度** | 中等 | 较高（工作流） | ⚠️ 旧架构更直观 |
| **性能** | 相同 | 相同 | ➖ 无差异 |

**结论：** 新架构在代码质量、可测试性、扩展性方面有显著提升，适合长期维护和功能演进。

---

## 当前形态（阶段三）

### 分层

```
internal/service/review/     # 评审领域
├── pipeline.go              # 前置编排（实现 review.PipelineRunner 端口）
├── worker.go                # worker 生命周期
└── ...                      # create/query/report/rules/state 等

internal/eino/               # 引擎适配层（只认端口，不持仓储）
├── executor.go              # Execute 主流程与两条引擎路径（workflow / 单 runner）
├── sideeffects.go           # 副作用（写回、通知）
└── ...

internal/domain/review/      # 领域词汇与端口
├── interfaces.go            # Executor / PipelineRunner / ReviewStateStore 等
└── ocr_aliases.go           # OCR 相关接口别名
```

### 关键约束

1. **引擎适配层不持仓储。** `internal/eino` 只依赖 `review.PipelineRunner`（前置编排端口）
   与 `review.ReviewStateStore`（窄状态存储接口），不 import 任何 repository 包。
   这条由架构门禁规则 `eino-no-repository-dep` 强制执行。
2. **前置编排在 service 层。** 克隆工作区 → 项目策略 → 文件过滤 → 规则合并 →
   记忆召回 → OCR 执行参数，实现在 `internal/service/review/pipeline.go`。
3. **状态存储用窄接口。** 引擎侧用 `review.ReviewStateStore` 而非完整的
   `repository.ReviewRepository`，避免依赖方向倒置（infra → service）。

### 当前能力

| 维度 | 当前状态 |
|------|----------|
| 代码组织 | 引擎适配层与领域编排分离，依赖方向单向 |
| 可测试性 | 编排可脱离引擎单测（pipeline_test.go） |
| 并行能力 | 多 Agent 并行（workflow 路径） |
| 工具化 | eino Tool 封装 |
| 依赖方向 | 由 `architecture-policy.yaml` + `archcheck` 门禁强制 |

> 修改执行架构时，先看 `AGENTS.md` 的「Architecture & layer rules」与
> `architecture-policy.yaml`——那些是**当前**的规则，本文档只记录演进脉络。
