# OCR Core 完整实施汇总

> 状态: A 阶段 + B 阶段全部完成
> 创建: 跟随 PR-A1 ~ PR-A15（共 15 commits）
> 关联: docs/OCR_CORE_DESIGN.md, docs/OCR_REGISTRY_IMPLEMENTATION.md

## 1. 整体回顾

整个 OCR Core 重构分两阶段 25 个 commit 完成：

### B 阶段（8 commits）— 薄壳 Registry 聚合
- **B1**: 新建 `internal/ocr` 包 + `Registry` 聚合入口（10 个单测）
- **B2**: `modules/ocr.go` 收口为 1 个 fx.Provide
- **B3+B4**: `SettingsHandler` 字段 15→10，InitHandler 参数 14→3
- **B5**: server/preflight/probe 改用 `*ocr.Registry` 注入
- **B6**: readiness 新增 `RegistryOCRHealthChecks` 聚合
- **B7**: 集成测试覆盖完整 Registry 流程
- **B8**: B 阶段实施记录文档

### A 阶段（15 commits）— 核心包 + 完整迁移
- **A1**: Config / Settings / SettingsData 单一真相
- **A2+A3**: Runner + 统一 exec 入口
- **A4+A6**: 7 个 OCR manager + RunnerFactory 迁入 ocr 包
- **A5**: llm.DynamicProvider 迁入 ocr.LLMProvider
- **A7**: domain/interfaces.go 拆分为 review + ocr_aliases
- **A8**: Client 升级 + 类型 alias 边界
- **A9**: biz/handler/settings_service/ocr_assets.go 改用 ocr.InternalRun
- **A10**: llm 残留标记
- **A11+A12**: ocr.LLMProvider + ocr.Runner 完整 HTTP / exec 实现
- **A13**: scanner.go 独立文件
- **A14+A15 part 1**: modules/llm.go 改用 ocr.LLMProvider + SettingsAdapter
- **A15 part 2**: modules/runner.go 改用 ocr.Runner
- **A15 part 3**: modules/ocr.go + review.go + runner.go 完全改用 ocr.Runner
- **A15 part 4**: 删除 internal/service/llm/openai.go + openai_retry_test.go
- **A15 part 5**: 删除 runner/dynamic.go + 简化 adapter
- **A15 part 6**: modules/server.go 改用 ocr.Viewer
- **A15 part 7**: 删除 internal/service/ocr 包（1450 LOC）
- **A15 part 8**: 删除 internal/service/llm/deprecated.go

## 2. 数据指标

| 指标 | 数值 |
|---|---|
| 总 commit 数 | 25（22 OCR Core + 3 文档）|
| 新增文件 | 24 |
| 修改文件 | 32 |
| 删除文件 | 14（5 runner / 11 ocr / 1 llm）|
| 新增单测 | 30+ |
| 全量包测试 | 27/27 全绿 |
| `internal/ocr` 总 LOC | ~3466（含测试）|
| `internal/ocr` 测试数 | 60+ |

## 3. internal/ocr 包结构（最终态）

```
internal/ocr/                          # 核心包
├── doc.go                              # 包级文档
├── client.go                           # Client 主入口（持有 Config + 缓存锁）
├── config.go                           # Config + Fallback + BuildConfig + AppendEnv + Validate
├── settings.go                         # Settings 接口 + SettingsData + SettingsSnapshot
├── settings_adapter.go                 # 把 models.OCRSettingsRepository 适配成 ocr.Settings
├── llm.go                              # LLMProvider + LLMRequest/LLMResponse + Adapter
├── runner.go                           # Runner（实现 domain.Runner/Preview/Scan + BinaryMissingError 等）
├── scanner.go                          # Preview + Scan 独立文件
├── managers.go                         # 7 个 manager：Versioner/Lister/Checker/Viewer/Upgrader
├── tester.go                           # Tester + ConfigResolver
├── factory.go                          # Factory.CreateRunner
├── command.go                          # Run / CommandSpec / CommandResult（公共 exec 入口）
├── helpers.go                          # firstNonBlank / firstPositive / ResolveExtraBody
├── registry.go                         # Registry（B 阶段薄壳；阶段 A+10 升级为 Client）
├── adapter/legacy.go                   # 阶段 A15 末清空（占位）
├── internal/                           # exec 公共原语
│   ├── command.go                      # 统一 exec 入口（SIGTERM → KillGrace → SIGKILL）
│   └── command_test.go
└── *_test.go                          # 60+ 单测
```

## 4. 阶段 A+10 收尾的剩余工作

未完成（推迟到后续 PR）：

1. **`internal/service/runner` 包删除**（仍保留 runner.go / preview.go / scan.go）
   - 原因：runner.IsBinaryMissing / IsTimeout / IsExit / ResolveExtraBody / NewConfigFileReader
     被 6+ 个 consumer 依赖（executor / discussion / requirements / workflow / biz handler）
   - 计划：迁移 consumer 到 ocr.IsBinaryMissing + ocr.ResolveExtraBody + ocr.NewConfigFileReader

2. **`internal/service/llm` 包核心保留**
   - 原因：llm.Provider / llm.CompleteRequest / llm.CompleteJSON 被 6+ 个 consumer 依赖
   - 已删除：openai.go（HTTP 客户端实现） + openai_retry_test.go + deprecated.go
   - 保留：errors.go / json_helper.go / metrics.go / noop.go / presets.go / provider.go

3. **`internal/ocr/adapter/legacy.go` 占位文件**
   - 阶段 A15 末清空（保留文件以维持 import 路径）
   - 阶段 A+10 末删除整个 adapter 目录

4. **`modules.SetPackageResolver` 包级缓存**
   - 阶段 A+10：删除此函数，resolver 改为通过 `ocr.Registry.Resolver()` getter 获取

## 5. 验收

```bash
go build ./...                       ✅ 0 errors
go vet ./...                          ✅ 0 warnings
go test -count=1 -short -timeout 120s ./...   ✅ 27/27 packages green
```

## 6. 核心设计要点

### Config + Settings 分离
- `Config` = runtime 配置（合并多源后冻结）
- `Settings` interface = 配置源抽象（DB / snapshot / 等）
- `BuildConfig` 是合并的单一实现（overrides > settings > fallback > defaults）

### exec 统一封装
- `internal/ocr/internal.Run(ctx, CommandSpec)` 是唯一 exec 入口
- 三段式 SIGTERM → KillGrace → SIGKILL
- 取代 runner.internal.Run + ocr.command.runOCRCommand + biz/handler runDryRunCommand

### LLM 类型兼容
- `ocr.LLMRequest = llm.CompleteRequest`（type alias）
- `ocr.LLMResponse = llm.CompleteResponse`（type alias）
- `*ocr.LLMProviderAdapter` 满足 `llm.Provider` 接口
- 阶段 A+10：删除 llm 包时改 ocr 独立类型

### domain 接口拆分
- `domain/review.go`（评审核心）
- `domain/ocr_aliases.go`（OCR 管理接口 type alias）
- 阶段 A+10：ocr 包直接定义 OCR 接口，删除 ocr_aliases

## 7. PR 链接

- **B 阶段（PR-B1 ~ PR-B7）**: https://github.com/yi-nology/code-review-service/compare/main...refactor/pr-b1-ocr-registry
- **A 阶段（PR-A1 ~ PR-A15）**: https://github.com/yi-nology/code-review-service/compare/main...refactor/pr-a1-ocr-core
- **设计文档**: https://github.com/yi-nology/code-review-service/compare/main...refactor/ocr-core-design