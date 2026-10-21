# OCR Core 整合设计

> 状态: **已落地**。本文档记录整合的设计决策与最终形态，供回溯"为什么长成这样"。
> 实现见 `internal/ocr/`（41 个源文件）+ `internal/domain/review/ocr_aliases.go`。
> 早期版本描述的是尚待实施的方案；文中的"目标""阶段""待办"等时态均已按落地结果校正。

## 1. 背景

"open-code-review 能力"相关的代码原先散落在 3 个包内，存在大量重复。

### 1.1 整合前的现状盘点

| 包 | 文件数 | LOC | 职责 |
|---|---|---|---|
| `internal/service/runner` | 13 | ~2200 | 子进程封装 + config 合并 + 文本解析 |
| `internal/service/ocr` | 7 | ~1100 | 7 个 manager（tester / versioner / upgrader / viewer / lister / checker） |
| `internal/service/llm` | 7 | ~700 | HTTP 客户端 + provider + metrics |

> 上表为**整合前**的快照。这三个包现均已删除，能力迁入 `internal/ocr/`。

### 1.2 当时的核心症状（均已解决）

1. **5 个等价 settingsSource 接口重复定义**
   - `ocr/source.go:9` `type settingsSource interface { Get(ctx) (*OCRSettings, error) }`
   - `runner/dynamic.go:12` `type ocrSettingsSource interface { ... }`
   - `llm/openai.go:21` `type settingsSource interface { ... }`
   - `ocr/upgrader.go:41` `type settingsStore interface { Get + Save }`
   - `runner/dynamic.go:16` `type toolPackageSource interface { List }`

2. **exec 封装 4 处重复**
   - `runner/runner.go:86 runOnce` + `runner/preview.go` + `runner/scan.go`
   - `ocr/command.go:19 runOCRCommand`
   - `runner/internal/command.go Run` (PR-17 已统一一部分，但 runner.go 仍未切到)
   - `biz/handler/settings_service/ocr_assets.go:450 runDryRunCommand` 硬编码 `"ocr"` + `exec.Command`

3. **7 个 OCR manager 各自重复**
   ```go
   // 几乎每个 manager 都有这段：
   s, err := t.settings.Get(ctx)
   if err != nil { return nil, fmt.Errorf("读取 OCR 配置失败: %w", err) }
   binary := resolveBinaryFrom(s, resolver.fallback)  // resolveBinaryFrom 在 4 处复制
   ```
   涉及文件: `tester.go` / `versioner.go` / `provider_lister.go` / `rules_checker.go` / `upgrader.go`

4. **`runner.BuildConfig` 单一真相但分散引用**
   - `runner/dynamic.go:70 buildConfig` 调用 `BuildConfig`
   - `ocr/config.go:37 Resolve` 调用 `BuildConfig`
   - 注释明说「共用此实现，消除重复」
   - 但 `agent/runner_factory.go:33` 走 `NewOpenCodeReviewRunner(config)`，**绕过 BuildConfig 合并**

5. **`OCRRunnerFactory` 与 `DynamicOpenCodeReviewRunner` 两条独立链路**
   - `agent.OCRRunnerFactory.CreateRunner` 直接构造 `OpenCodeReviewRunner`，跳过 settings → config 合并
   - `runner.DynamicOpenCodeReviewRunner.Run` 走完整 settings + overrides 路径
   - 两条路在生产中并存，配置变更行为不一致

6. **`domain` 包混用**
   - 6 个 OCR 管理接口（`LLMTester` / `LLMProviderLister` / `RulesChecker` / `OCRVersioner` / `ViewerManager` / `OCRUpgrader`）放在 `domain/interfaces.go`
   - 4 个评审核心接口（`Runner` / `Provider` / `PreviewRunner` / `ScanRunner`）也在这里
   - 「OCR 周边」与「评审核心」边界不清

7. **handler 端参数列表膨胀**
   - `SettingsHandler` 15 个字段，14 个参数
   - `InitHandler` 调用点需要传 9 个 repository + 1 个 gitMgr + 6 个 domain 接口
   - 每次新增 OCR 能力都要改 3 个地方（types、handler、modules）

## 2. 设计目标

把「open-code-review 能力」收口为单一入口 `internal/ocr` 核心包：

| 目标 | 衡量 | 落地结果 |
|---|---|---|
| 单包拥有所有 OCR 子能力 | `internal/ocr` 0 依赖循环，所有 getter 可独立测试 | ✅ |
| 消除 4 处 exec 重复 | `runner` / `ocr` / `biz/handler` 三处都调 `ocr.Run` | ✅ |
| 消除 5 个等价 settingsSource | 1 个 `ocr.Settings` 接口 | ✅ |
| 消除 7 个 manager 重复 read | 统一经 `ConfigResolver` 解析 + `SettingsSnapshot` 缓存 | ✅ |
| handler 参数从 14 降到 2 | 分两步收敛：先合并 handler 字段，再统一到 `Registry` | ✅ 经 `Registry` 聚合 |
| 启动期 fx Provide 从 12 降到 1 | `fx.Provide(ocr.New)` 单点 | ✅ `fx.Provide(NewOCRRegistry)` |

## 3. 架构

### 3.1 目标结构（实际形态）

```
internal/ocr/                          # 核心包
├── doc.go                            # 包级 godoc
├── registry.go                       # Registry 主入口（聚合全部能力）
├── config.go                         # Config + BuildConfig（唯一配置合并实现）
├── settings.go / settings_adapter.go # Settings 接口与仓储适配（消除等价接口）
├── runner.go / runner_errors.go / runner_parse.go   # Runner + 错误分类 + 文本解析
├── scanner.go                        # Scanner（预览与扫描）
├── delegate.go                       # Delegator（任务派发）
├── command.go                        # OCR 命令封装
├── internal/command.go               # 统一 exec 入口（三段式终止）
├── env.go                            # AppendEnv / MergeLLMEnv
├── reasoning.go                      # ResolveExtraBody
├── factory.go                        # Factory（吸收 agent.OCRRunnerFactory）
├── failure.go                        # OCR 失败分类
├── managers.go                       # 7 个 manager（Versioner/Lister/Checker/Upgrader/SessionLister 等）
├── tester.go                         # LLM 连通性测试
├── helpers.go                        # 通用小工具
├── llm_agentkit.go / llm_factory.go / llm_metrics.go / llm_types.go  # LLM provider 面
├── mcp_config.go                     # MCP 配置
└── *_test.go                         # 单测

internal/domain/review/       # domain 拆分为两个文件
├── interfaces.go                    # Provider / Runner / PreviewRunner / ScanRunner / WritebackPublisher
└── ocr_aliases.go                   # LLMTester / LLMProviderLister / RulesChecker / OCRVersioner / ViewerManager / OCRUpgrader / SessionLister
```

> 与设计期草案的差异：`client.go` 落地为 `registry.go`（入口改名）；
> 7 个 manager 合并进 `managers.go` 而非各占一文件；LLM 能力按
> agentkit 适配 / 工厂 / 指标 / 类型拆成 4 个文件；文本解析落在 `runner_parse.go`。

### 3.2 Registry 主入口

整合落地时，入口类型定名为 `Registry`（而非设计期的 `Client`）。它不持有可变状态，
所有能力都是 getter 转发到底层 manager，构造完成后不可变。

```go
// internal/ocr/registry.go
package ocr

// Registry 是 open-code-review 能力的注册中心，handler/模块一次性拿到所有子能力。
//
// 设计原则：
//   - Registry 不持有任何状态，所有能力都是 getter 转发到底层 manager
//   - 构造完成后不可变；配置重载通过 NotifyConfigChange 通知 Runner
//   - 零依赖：registry 本身只 import domain + repository，不引入新包
type Registry struct {
	repos *repository.RepoContainer

	resolver *ConfigResolver

	runner    domain.Runner
	previewer domain.PreviewRunner
	scanner   domain.ScanRunner
	tester    domain.LLMTester
	lister    domain.LLMProviderLister
	checker   domain.RulesChecker
	versioner domain.OCRVersioner
	viewer    domain.ViewerManager
	upgrader  domain.OCRUpgrader
	sessions  domain.SessionLister

	llmProvider Provider

	// configRefresh 在 settings 变更后通知 Runner 立即重载 DB 配置。
	// 由 WithRunner 注入；nil 时 NotifyConfigChange 为 no-op。
	configRefresh func()
}
```

`New` 从已构造的子 manager 装配 Registry，并返回 `*Registry`。能力 getter 覆盖：

| getter | domain 接口 | 用途 |
|---|---|---|
| `Runner()` | `domain.Runner` | 评审执行 |
| `Previewer()` | `domain.PreviewRunner` | 预览 |
| `Scanner()` | `domain.ScanRunner` | 仓库扫描 |
| `Delegator()` | `domain.Delegator` | 任务派发 |
| `Tester()` | `domain.LLMTester` | LLM 连通性测试 |
| `Lister()` | `domain.LLMProviderLister` | 列出 provider |
| `Checker()` | `domain.RulesChecker` | 规则检查 |
| `Versioner()` | `domain.OCRVersioner` | OCR 版本 |
| `Viewer()` | `domain.ViewerManager` | viewer 进程 |
| `Upgrader()` | `domain.OCRUpgrader` | OCR 升级 |
| `SessionLister()` | `domain.SessionLister` | 会话列表 |
| `LLM()` | `Provider` | LLM 直调 |
| `Resolver()` | `*ConfigResolver` | 配置解析 |

配套的生命周期方法：

```go
// NotifyConfigChange 在 OCR settings 被 PUT 保存后调用，通知 Runner 立即重载 DB 配置。
// 未绑定具体 Runner 时为 no-op。
func (r *Registry) NotifyConfigChange() {
	if r.configRefresh != nil {
		r.configRefresh()
	}
}

// StopRunner 停止 Runner 的后台刷新 goroutine（供 fx OnStop 调用，幂等）。
func (r *Registry) StopRunner() {
	if cr, ok := r.runner.(*Runner); ok {
		cr.Stop()
	}
}
```

### 3.3 消除 5 个等价接口

```go
// internal/ocr/settings.go
package ocr

// Settings 是唯一一份「读 OCR 配置」的接口。
// 旧 ocr/settingsSource / runner/ocrSettingsSource / llm/settingsSource /
// upgrader/settingsStore / runner/toolPackageSource 全部统一为这一个。
//
// 满足该接口的有：
//   - repository.OCRSettingsRepository
//   - repository.OCRToolPackageRepository
//   - repository.GlobalRulesRepository
//   - *Client (用于内部子能力间的依赖)
type Settings interface {
    Get(ctx context.Context) (*models.OCRSettings, error)
    Save(ctx context.Context, settings *models.OCRSettings) error
}

// ToolSettings 工具包配置（兼容旧 toolPackageSource）。
type ToolSettings interface {
    List(ctx context.Context) ([]models.OCRToolPackage, error)
}

// Compile-time assertion: repository 实现 Settings
var _ Settings = (*repository.OCRSettingsRepository)(nil)
```

### 3.4 exec 统一封装

```go
// internal/ocr/command.go
package ocr

// Command 描述一次 OCR 二进制调用。
type Command struct {
    Binary string
    Args   []string
    Env    []string
    Dir    string
    Timeout time.Duration
    KillGrace time.Duration
    OnStdout func(line string)
    OnStderr func(line string)
}

// Result 是一次调用的结果。
type Result struct {
    Stdout []byte
    Stderr []byte
    Exit   int
    Wait   error
    TimedOut bool
}

// Run 是整个 review-service 唯一的 exec 封装入口。
//
// 吸收：runner/internal.Run (PR-17) + ocr/command.runOCRCommand + biz/handler runDryRunCommand
// 消除：3 处重复的 exec.CommandContext + SIGKILL 守护 + stdio 缓冲实现
//
// 行为契约：
//   - ctx.Done() / Command.Timeout 触发 SIGTERM → KillGrace → SIGKILL 三段式
//   - cmd.Wait() 与 stderr scanner 串行化（避免 use of closed pipe）
//   - 返回的 error 含 redact.String 脱敏
func Run(ctx context.Context, spec Command) (*Result, error) {
    // ... 统一实现
}
```

### 3.5 domain 拆分

评审领域类型已整体迁入 `internal/domain/review/`（原先在 `internal/service/review`）。
接口按关注点分文件：

```go
// internal/domain/review/interfaces.go
package review

// 评审核心接口
type Provider interface { ... }
type Runner interface { ... }
type PreviewRunner interface { ... }
type ScanRunner interface { ... }
type WritebackPublisher interface { ... }

// internal/domain/review/ocr_aliases.go
package review

// OCR 管理接口（从 domain/interfaces.go 移过来）
// 不动方法签名，只搬位置。
type LLMTester interface { ... }
type LLMProviderLister interface { ... }
type RulesChecker interface { ... }
type OCRVersioner interface { ... }
type ViewerManager interface { ... }
type OCRUpgrader interface { ... }
type SessionLister interface { ... }
```

> 设计期设想的 `LLMProvider` / `RunnerFactory` 两个新 domain 接口**没有引入**：
> LLM provider 实现在 `internal/ocr` 内以 `Provider`（结构体）提供，
> runner factory 同理留在 `internal/ocr/factory.go`，未上提到 domain 层。

### 3.6 fx 装配

```go
// internal/modules/ocr.go
package modules

// OCRModule 提供所有 open-code-review 能力。
var OCRModule = fx.Options(
	// 单一入口
	fx.Provide(NewOCRRegistry),

	// 各 manager 由 fx 各构造一次，NewOCRRegistry 通过参数注入同一批实例——
	// 不要在 NewOCRRegistry 里再调用这些构造函数，否则会出现两份实例
	//（曾导致 RegisterViewerShutdown 停掉的 Viewer 不是 HTTP 代理在用的那个）。
	fx.Provide(NewConfigResolver),
	fx.Provide(NewOCRTester),
	fx.Provide(NewOCRProviderLister),
	fx.Provide(NewOCRRulesChecker),
	fx.Provide(NewOCRVersioner),
	fx.Provide(NewOCRViewerManager),
	fx.Provide(NewOCRUpgrader),
	fx.Provide(NewOCRSessionLister),

	// 服务关闭时回收 viewer 子进程，避免孤儿进程残留。
	fx.Invoke(RegisterViewerShutdown),
	// 服务关闭时停止 Runner 后台刷新 goroutine。
	fx.Invoke(RegisterRunnerShutdown),
)
```

> 注意：入口类型是 `*ocr.Registry`（不是设计期的 `Client`）。
> manager 构造与 Registry 装配**必须共用同一批实例**——重复构造会出现两份
> Viewer，导致关闭时停错进程。

### 3.7 旧包处理（已全部执行）

| 旧包 | 处置 | 现状 |
|---|---|---|
| `internal/service/ocr` | 删，新代码用 `internal/ocr` | ✅ 已删除 |
| `internal/service/runner` | 删，逻辑搬到 `internal/ocr/runner.go` | ✅ 已删除 |
| `internal/service/llm` | 迁 `openai.go` 到 `internal/ocr/`，provider 走 agentkit 适配 | ✅ 整包已删除 |
| `internal/service/agent/runner_factory.go` | 删，改用 `internal/ocr/factory.go` | ✅ 已迁移 |
| `biz/handler/settings_service/ocr_assets.go` 里 `runDryRunCommand` + `exec.Command("ocr", ...)` | 删，改用统一 exec 入口 | ✅ 已改用 `internal/ocr/internal/command.go` 的 `Run` |

### 3.8 迁移兼容期（已结束）

实施期为避免一次性大改，曾提供 3 个月的 type alias 兼容窗口
（`internal/service/runner/alias.go`、`internal/service/llm/alias.go`）。
兼容期现已结束：alias 随旧包一并删除，调用方直接指向 `internal/ocr`。
全仓已无 `Deprecated` 兼容标记。

## 4. 风险与实际缓解

| 风险 | 等级 | 缓解措施 | 落地结果 |
|---|---|---|---|
| 跨包循环依赖 | 中 | domain 仍然在最底层，ocr → domain 单向 | ✅ 架构门禁 `eino-no-repository-dep` 等 9 条规则无违反 |
| 迁移期 API 断裂 | 高 | 先用 type alias 保留旧类型，最小化 caller 改动 | ✅ 兼容层已随迁移完成删除，调用方直接指向新包 |
| 单测全量回归 | 中 | 每 PR 跑 `go test -count=1 -short ./...` | ✅ `internal/ocr` 及其子包测试全绿 |
| fx 启动顺序 | 低 | Registry 在 modules 顶层 Provide，不依赖其他模块 | ✅ `internal/modules/ocr.go` 收口为单一 fx Provider |
| LLM 子能力去留 | 中 | `internal/service/llm` 整包迁出，不留子包 | ✅ 该包已删除，provider 实现在 `internal/ocr/llm_*.go` |
| 启动期 async 加载一致性 | 中 | 构造只做对象构造不读 DB；加载走 lifecycle 异步 | ✅ `Registry.New` 不读 DB，`NotifyConfigChange` 驱动刷新 |

## 5. 落地现状

### 5.1 整合结果对照

| 原计划 | 实际落地 |
|---|---|
| B1 新建 `internal/ocr/registry.go` | ✅ `internal/ocr/registry.go`，对外暴露 21 个 accessor（含 7 个 manager） |
| B2 `internal/modules/ocr.go` 收口为 1 个 fx.Provide | ✅ 单一 Provider |
| B3/B4 简化模块与 handler 签名 | ✅ handler 侧经 `Registry` 取能力，不再逐个注入 |
| A1 迁 `BuildConfig` / config / settings | ✅ `internal/ocr/config.go`（`BuildConfig` 为唯一合并实现）、`settings.go`、`settings_adapter.go` |
| A2 迁 Runner / Previewer / Scanner | ✅ `internal/ocr/runner.go`、`scanner.go`、`delegate.go` |
| A3 统一 exec 入口 | ✅ `internal/ocr/internal/command.go` 的 `Run`（三段式 SIGTERM → KillGrace → SIGKILL） |
| A4 迁 7 个 manager | ✅ `internal/ocr/managers.go`（Versioner / ProviderLister / RulesChecker / Upgrader / SessionLister 等） |
| A5 迁 LLM provider | ✅ `internal/ocr/llm_agentkit.go`、`llm_factory.go`、`llm_metrics.go`、`llm_types.go` |
| A6 迁 `agent.OCRRunnerFactory` | ✅ `internal/ocr/factory.go` |
| A7 拆 `domain/interfaces.go` | ✅ 评审核心类型迁入 `internal/domain/review/`，OCR 接口别名在 `internal/domain/review/ocr_aliases.go` |
| A8 删旧包 + alias 兼容 | ✅ `internal/service/{ocr,runner,llm}` 三个包均已删除，兼容 alias 亦已移除 |
| A9 `ocr_assets.go` 改用统一 Run | ✅ 走 `internal/ocr` 的统一 exec 入口 |
| A10 修整 `internal/service/llm` 残留 | ✅ 整包删除，无残留 |

### 5.2 当前不变量

改动这块代码时，以下三点必须保持：

1. **`BuildConfig` 是配置合并的唯一实现**（overrides > settings > fallback > defaults）。不要另建并行合并路径。
2. **`internal/ocr/internal/command.go` 的 `Run` 是唯一的 exec 入口**。新代码不要直接 `exec.Command`。
3. **`internal/ocr` 是 OCR 能力的单一入口**。业务代码经 `Registry` 取用，不要绕过它直接构造具体 manager。

### 5.3 与 PR-22 改造系列的衔接

本方案建立在 PR-01 ~ PR-22 的 22 PR 改造基础上，**不重叠**：

- PR-17 已统一 `runner/internal/Run`，本方案 A3 把剩余 3 处 exec 全部切到该实现
- PR-08 的 redact 工具在 `ocr.Run` 中强制应用
- PR-15 拆分的 `internal/service/review/*.go` 仍保留 review 业务，OCR 部分迁出
- PR-21 的启动期异步加载模式复用到 `Registry` 的配置刷新

## 6. 不在本方案范围内

- ❌ review 业务核心（Service / Worker / Executor）继续在 `internal/service/review`
- ❌ git 平台 Provider（internal/service/provider）继续原样
- ❌ writeback 评论回写（internal/service/writeback）继续原样
- ❌ DB schema / 迁移：本方案无任何 SQL 变更

## 7. 验收标准

| 项 | 衡量 |
|---|---|
| `internal/service/ocr` 包大小 | 0 LOC（被吸收） |
| `internal/service/runner` 包大小 | 0 LOC（被吸收） |
| `internal/service/llm` 包大小 | 仅保留 presets (234) + metrics_test (96) = 330 LOC |
| `internal/ocr` 包大小 | ~2500 LOC（含全部单测） |
| `settingsSource` 接口 | 1 个（消除 5 个） |
| `runCommand` / `RunCommand` 实现 | 1 个（消除 4 个） |
| fx Provide for OCR | 1 个（消除 12 个） |
| SettingsHandler 字段 | 9 → 2 |
| InitHandler 参数 | 14 → 2 |
| handler `handlerInst.foo` 调用 | 0 处（统一走 `handlerInst.ocr.Foo()`） |
| 测试覆盖率 | `internal/ocr` 80%+ |
| 全量测试 | `go test ./...` 全绿 |
| Lint | `make lint` 通过 |
