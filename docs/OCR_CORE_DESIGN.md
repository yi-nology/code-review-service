# OCR Core 整合设计方案

> 状态: **设计文档**，待方案 B（Registry 聚合）落地后启动方案 A 实施
> 创建: PR-B0
> 关联: 22-PR 改造系列 (PR-01 ~ PR-22) 之后的下一步

## 1. 背景

PR-01 ~ PR-22 完成了 review-service 的系统性重构（写回错误、TOCTOU、ctx 传播、redact、ULID 等），
但「open-code-review 能力」相关的代码仍散落在 3 个包内，存在大量重复：

### 1.1 现状盘点

| 包 | 文件数 | LOC | 职责 |
|---|---|---|---|
| `internal/service/runner` | 13 | ~2200 | 子进程封装 + config 合并 + 文本解析 |
| `internal/service/ocr` | 7 | ~1100 | 7 个 manager（tester / versioner / upgrader / viewer / lister / checker） |
| `internal/service/llm` | 7 | ~700 | HTTP 客户端 + provider + metrics |

### 1.2 核心症状

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

| 目标 | 衡量 |
|---|---|
| 单包拥有所有 OCR 子能力 | `internal/ocr` 0 依赖循环，所有 getter 可独立测试 |
| 消除 4 处 exec 重复 | `runner` / `ocr` / `biz/handler` 三处都调 `ocr.Run` |
| 消除 5 个等价 settingsSource | 1 个 `ocr.Settings` 接口 |
| 消除 7 个 manager 重复 read | 由 `ocr.Client.Config()` 统一读 + 缓存 |
| handler 参数从 14 降到 2 | 阶段 B 立即见效，阶段 A 最终收敛 |
| 启动期 fx Provide 从 12 降到 1 | `fx.Provide(ocr.New)` 单点 |

## 3. 架构

### 3.1 目标结构

```
internal/ocr/                          # 核心包（方案 A 最终落点）
├── doc.go                            # 包级 godoc
├── client.go                         # Client 主入口
├── config.go                         # Config + BuildConfig（吸收 runner.BuildConfig + ocr.ConfigResolver）
├── settings.go                       # Settings 接口（消除 5 个等价接口）
├── runner.go                         # Runner（吸收 runner/{runner,preview,scan,dynamic}）
├── command.go                        # exec 统一封装（吸收 ocr.command + runner.internal.command）
├── env.go                            # AppendEnv / MergeLLMEnv（吸收 runner.AppendLLMEnv）
├── llm.go                            # LLM Provider（吸收 llm/{openai,presets} + llm.NewDynamicProvider）
├── metrics.go                        # LLM Metrics（吸收 llm/metrics.go）
├── tester.go                         # TestLLM（吸收 ocr/tester.go）
├── versioner.go                      # OCRVersion（吸收 ocr/versioner.go）
├── lister.go                         # ListLLMProviders（吸收 ocr/provider_lister.go）
├── checker.go                        # RulesCheck（吸收 ocr/rules_checker.go）
├── viewer.go                         # Viewer（吸收 ocr/viewer.go + PR-05 修复保留）
├── upgrader.go                       # Upgrader（吸收 ocr/upgrader.go）
├── factory.go                        # RunnerFactory（吸收 agent.OCRRunnerFactory）
├── textparse.go                      # 文本解析（吸收 runner/textparse.go）
├── reasoning.go                      # ResolveExtraBody（吸收 runner/reasoning.go）
├── progress.go                       # OCR 进度 reporter（吸收 runner/progress.go）
└── *_test.go                         # 全部单测聚合

internal/domain/review/       # domain 拆分为两个文件
├── review.go                         # Provider / Runner / PreviewRunner / ScanRunner / WritebackPublisher
└── ocr.go                            # LLMTester / LLMProviderLister / RulesChecker / OCRVersioner / ViewerManager / OCRUpgrader
```

### 3.2 Client 主入口

```go
// internal/ocr/client.go
package ocr

import (
    "context"
    "sync"
    "sync/atomic"

    "github.com/yi-nology/code-review-service/internal/repository"
    "github.com/yi-nology/code-review-service/internal/domain/review"
)

// Client 是 open-code-review 能力的统一入口。
//
// 所有子能力通过 Get 方法获取（domain 接口类型）：
//   - Client.Runner() / Previewer() / Scanner()    → 评审执行
//   - Client.LLM()                                  → LLM 直接调用
//   - Client.Tester() / Lister() / Checker() ...   → OCR 管理
//
// Client 内部持有一个动态配置缓存，调用 Settings.Get() 后立即 refresh。
// 这消除 7 个 manager 各自调 Get 的重复。
type Client struct {
    settings   repository.OCRSettingsRepository
    tools      repository.OCRToolPackageRepository
    rules      repository.GlobalRulesRepository
    fallback   Config                  // YAML 兜底

    // 动态配置缓存：每次 settings 变更后 refresh
    configCache atomic.Pointer[Config]

    // 子能力 manager
    runner    *Runner
    previewer *Previewer
    scanner   *Scanner
    tester    *Tester
    lister    *Lister
    checker   *Checker
    versioner *Versioner
    viewer    *Viewer
    upgrader  *Upgrader
    llm       *LLMProvider
    factory   *Factory

    refreshMu sync.Mutex  // 保护 refresh 串行
}

// New 构造 Client。
//
// 自动装配：
//   1. 构造 ConfigResolver (settings + fallback + tools)
//   2. 构造 Runner/Previewer/Scanner (动态合并)
//   3. 构造 Tester/Lister/Checker/Versioner (共享 settings 缓存)
//   4. 构造 Upgrader (依赖 Versioner)
//   5. 构造 Viewer (独立进程组)
//   6. 构造 LLMProvider (复用同一 Config)
//   7. 构造 Factory (供 agent 包注入)
func New(cfg Config, repos *repository.RepoContainer) *Client {
    resolver := NewConfigResolver(repos.OCRSettings, cfg, repos.OCRToolPackage)
    llm := NewLLMProvider(repos.OCRSettings)
    
    c := &Client{
        settings: repos.OCRSettings,
        tools:    repos.OCRToolPackage,
        rules:    repos.GlobalRules,
        fallback: cfg,
    }
    c.configCache.Store(&cfg)
    
    // 子能力（共享 resolver 和 llm）
    c.runner    = NewRunner(resolver)
    c.previewer = NewPreviewer(resolver)
    c.scanner   = NewScanner(resolver)
    c.tester    = NewTester(resolver, llm)
    c.lister    = NewLister(resolver)
    c.checker   = NewChecker(resolver, repos.GlobalRules)
    c.versioner = NewVersioner(resolver)
    c.viewer    = NewViewer(resolver)
    c.upgrader  = NewUpgrader(repos.OCRSettings, resolver, c.versioner, cfg.InstallDir, cfg.GitHubRepo)
    c.llm       = llm
    c.factory   = NewFactory(c.runner)
    
    return c
}

// -------- 能力 getter --------

func (c *Client) Runner() domain.Runner             { return c.runner }
func (c *Client) Previewer() domain.PreviewRunner   { return c.previewer }
func (c *Client) Scanner() domain.ScanRunner       { return c.scanner }
func (c *Client) Tester() domain.LLMTester         { return c.tester }
func (c *Client) Lister() domain.LLMProviderLister  { return c.lister }
func (c *Client) Checker() domain.RulesChecker      { return c.checker }
func (c *Client) Versioner() domain.OCRVersioner    { return c.versioner }
func (c *Client) Viewer() domain.ViewerManager     { return c.viewer }
func (c *Client) Upgrader() domain.OCRUpgrader      { return c.upgrader }
func (c *Client) LLM() domain.LLMProvider           { return c.llm }  // 注意：domain.LLMProvider 是新接口
func (c *Client) Factory() domain.RunnerFactory     { return c.factory }

// Refresh 重新从 settings 拉取最新配置，更新内部缓存。
// 用于 settings API 修改后无需重启服务即可生效。
func (c *Client) Refresh(ctx context.Context) error {
    c.refreshMu.Lock()
    defer c.refreshMu.Unlock()
    
    s, err := c.settings.Get(ctx)
    if err != nil {
        return fmt.Errorf("refresh: %w", err)
    }
    cfg := BuildConfig(s, domain.OCROverrides{}, c.fallback, c.tools)
    c.configCache.Store(&cfg)
    c.runner.refresh(cfg)
    c.previewer.refresh(cfg)
    c.scanner.refresh(cfg)
    c.llm.refresh(cfg)
    return nil
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

```go
// internal/domain/review/review.go
package domain

// 评审核心接口（不变）
type Provider interface { ... }
type Runner interface { ... }
type PreviewRunner interface { ... }
type ScanRunner interface { ... }
type WritebackPublisher interface { ... }

// internal/domain/review/ocr.go
package domain

// OCR 管理接口（从 interfaces.go 移过来）
// 不动方法签名，只搬位置。
type LLMTester interface { ... }
type LLMProviderLister interface { ... }
type RulesChecker interface { ... }
type OCRVersioner interface { ... }
type ViewerManager interface { ... }
type OCRUpgrader interface { ... }

// 新增：抽象 LLM 抽象（让 ocr 包可以注入非 LLM 场景）
type LLMProvider interface {
    Complete(ctx context.Context, req CompleteRequest) (CompleteResponse, error)
}

// 新增：runner factory 抽象（让 agent 包依赖此接口而非具体 factory）
type RunnerFactory interface {
    CreateRunner(background, ruleContent string) Runner
}
```

### 3.6 fx 装配

```go
// internal/modules/ocr.go 改造后
package modules

var OCRModule = fx.Options(
    fx.Provide(ocr.New),
    fx.Invoke(registerOCRHealthChecks),
)

func New(
    cfg *config.Config,
    repos *repository.RepoContainer,
) *ocr.Client {
    fallback := ocr.Config{
        Binary:          cfg.OCR.Binary,
        Audience:        "human",
        RulePath:        cfg.OCR.Rule,
        DefaultSeverity: "major",
        Concurrency:     2,
        MaxGitProcs:     4,
        TimeoutMinutes:  10,
        InstallDir:      cfg.OCR.UpgradeInstallDir,
        GitHubRepo:      cfg.OCR.GitHubRepo,
    }
    return ocr.New(fallback, repos)
}

func registerOCRHealthChecks(lc fx.Lifecycle, c *ocr.Client) {
    // 把 OCR 健康检查挂到 /healthz
    // ...
}
```

### 3.7 旧包处理

| 旧包 | 处置 |
|---|---|
| `internal/service/ocr` | 删，新代码用 `internal/ocr` |
| `internal/service/runner` | 删，逻辑搬到 `internal/ocr/runner.go` + `textparse.go` |
| `internal/service/llm` | 保留为子包，仅 `presets.go` + `metrics_test.go` 留下；`openai.go` 搬到 `internal/ocr/llm.go` |
| `internal/service/agent/runner_factory.go` | 删，改用 `ocr.Client.Factory()` |
| `biz/handler/settings_service/ocr_assets.go` 里 `runDryRunCommand` + `exec.Command("ocr", ...)` | 删，改用 `ocr.Client.Run(...)` |

### 3.8 迁移兼容期

为避免一次性大改，方案 A 实施时提供 3 个月的兼容窗口：

```go
// internal/service/runner/alias.go (过渡期保留)
package runner

// Deprecated: 改用 internal/ocr.Config
type OpenCodeReviewConfig = ocr.Config

// Deprecated: 改用 internal/ocr.Runner
type OpenCodeReviewRunner = ocr.Runner
```

```go
// internal/service/llm/alias.go
package llm

// Deprecated: 改用 internal/ocr.LLMProvider
type DynamicProvider = ocr.LLMProvider
```

## 4. 风险评估

| 风险 | 等级 | 缓解 |
|---|---|---|
| 跨包循环依赖 | 中 | domain 仍然在最底层，ocr → domain 单向 |
| 迁移期 API 断裂 | 高 | 用 type alias 保留旧类型，最小化 caller 改动 |
| 单测全量回归 | 中 | 每 PR 跑 `go test -count=1 -short ./...` |
| fx 启动顺序 | 低 | ocr.Client 在 modules 顶层 Provide，不依赖其他模块 |
| LLM 子能力去留 | 中 | 保留 `internal/service/llm` 子包，**只迁 DynamicProvider** |
| 启动期 async 加载一致性 | 中 | Client.New 只做对象构造，不读 DB；Refresh 走 lifecycle 异步 |

## 5. 实施计划

### 阶段 B（先做，0 风险，2-3 小时）

| PR | 内容 | 工作量 |
|---|---|---|
| **B1** | 新建 `internal/ocr/registry.go` + `registry_test.go`，8 个单测 | 1h |
| **B2** | `internal/modules/ocr.go` 收口为 1 个 fx.Provide | 0.5h |
| **B3** | 5 个 `internal/modules/*.go` 函数签名简化 | 0.5h |
| **B4** | `biz/handler/settings_service/handler.go` 字段合并 | 0.5h |

**总目标**：handler 参数 14 → 2，fx Provider 12 → 1，立即改善维护性。

### 阶段 A（方案 B 合并后启动，1-2 周）

| PR | 内容 | 工作量 |
|---|---|---|
| **A1** | 新建 `internal/ocr/client.go` + `config.go` + `settings.go`，迁 `BuildConfig` | 2d |
| **A2** | 迁 `runner.Runner/Previewer/Scanner` 到 `ocr.runner.go` + `ocr.previewer.go` + `ocr.scanner.go` | 1d |
| **A3** | 统一 `ocr.Run` 替换 4 处 exec 实现 | 1d |
| **A4** | 迁 7 个 manager 到 ocr 包（tester/versioner/lister/checker/upgrader/viewer） | 2d |
| **A5** | 迁 `llm.DynamicProvider` 到 `ocr.llm.go` | 1d |
| **A6** | 迁 `agent.OCRRunnerFactory` 到 `ocr.factory.go` | 0.5d |
| **A7** | 拆 `domain/interfaces.go` 为 `review.go` + `ocr.go` | 0.5d |
| **A8** | 删 `internal/service/ocr` / `internal/service/runner` 包 + alias 兼容 | 1d |
| **A9** | `biz/handler/settings_service/ocr_assets.go` 改用 `ocr.Run` | 0.5d |
| **A10** | 修整 `internal/service/llm` 残留为 presets 子包 | 0.5d |

**总目标**：消灭 5 处 settingsSource 重复 + 4 处 exec 重复 + 2 条 runner 链路。

## 6. 与 PR-22 改造系列的衔接

本方案建立在 PR-01 ~ PR-22 的 22 PR 改造基础上，**不重叠**：

- PR-17 已统一 `runner/internal/Run`，本方案 A3 把剩余 3 处 exec 全部切到该实现
- PR-08 的 redact 工具在 `ocr.Run` 中强制应用
- PR-15 拆分的 `internal/service/review/*.go` 仍保留 review 业务，OCR 部分迁出
- PR-21 的启动期异步加载模式可复用到 `ocr.Client.Refresh`

## 7. 不在本方案范围内

- ❌ review 业务核心（Service / Worker / Executor）继续在 `internal/service/review`
- ❌ git 平台 Provider（internal/service/provider）继续原样
- ❌ writeback 评论回写（internal/service/writeback）继续原样
- ❌ DB schema / 迁移：本方案无任何 SQL 变更

## 8. 验收标准

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
