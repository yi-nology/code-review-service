# OCR Registry 聚合实施记录

> 状态: B 阶段完成 (B1-B7)
> 关联: docs/OCR_CORE_DESIGN.md
> 后续: A 阶段（OCR 核心包迁移）等待触发

## 改动总览

### 新增文件
- `internal/ocr/registry.go` — Registry 主入口（180 LOC）
- `internal/ocr/registry_test.go` — 10 个基础单测
- `internal/ocr/registry_integration_test.go` — 3 个集成测试
- `docs/OCR_CORE_DESIGN.md` — A 阶段设计文档
- `docs/OCR_REGISTRY_IMPLEMENTATION.md` — 本文件

### 修改文件
- `internal/modules/ocr.go` — 收口为 1 个 fx.Provide
- `internal/modules/handler.go` — InitSettingsHandler 14→3 参数
- `internal/modules/server.go` — RegisterRoutes 7→5 参数
- `internal/modules/preflight.go` — 3→2 参数
- `internal/modules/probe.go` — 3→2 参数
- `internal/readiness/checks.go` — 新增 RegistryOCRHealthChecks 聚合
- `biz/handler/settings_service/handler.go` — 字段 15→10

## 数据指标

| 指标 | 改造前 | 改造后 | 变化 |
|---|---|---|---|
| `InitSettingsHandler` 参数数 | 14 | 3 | **-79%** |
| `RegisterRoutes` 参数数 | 7 | 5 | -29% |
| `RegisterPreflight` 参数数 | 3 | 2 | -33% |
| `RegisterLLMProbe` 参数数 | 3 | 2 | -33% |
| `SettingsHandler` 字段数 | 15 | 10 | -33% |
| `OCRModule` fx.Provide 声明 | 7 | 8 (旧 7 + 新 1) | +1（兼容期） |
| `*ocr.Registry` 业务接口封装 | 0 | 1 | +1 |
| 单测覆盖 `internal/ocr/` | 0 | 13 | +13 |
| `internal/modules/` 单测 | 0 | 3 | +3 |

## 调用方迁移

| 调用点 | 之前 | 之后 |
|---|---|---|
| `biz/handler/settings_service/ocr_settings.go` | `handlerInst.llmTester` | `handlerInst.ocr.Tester()` |
| `biz/handler/settings_service/ocr_tools.go` | `handlerInst.providerLister` / `rulesChecker` / `ocrVersioner` / `viewerMgr` | `handlerInst.ocr.Lister()` / `Checker()` / `Versioner()` / `Viewer()` |
| `biz/handler/settings_service/ocr_upgrade.go` | `handlerInst.ocrUpgrader` | `handlerInst.ocr.Upgrader()` |

## 过渡期机制

为避免一次性破坏性改动，B 阶段保留原 7 个 `fx.Provide` 兼容旧 consumer。
新增 1 个 `fx.Provide(ocrcore.New)` 让新代码可注入 *ocr.Registry。

**过渡期依赖**：
- `modules.SetPackageResolver(r)` 把 resolver 注入到 modules 包级缓存，
  供 server.go / preflight.go / probe.go 复用。
- 阶段 A 改造后此函数删除。

## 与原计划的偏差

| 计划项 | 实际情况 | 原因 |
|---|---|---|
| 1 个 PR 完成 B 阶段 | 拆为 7 个小 PR | 1 个大 PR 风险高，拆小便于 review/revert |
| 改 readiness 签名 | 仅新增 `RegistryOCRHealthChecks`，老函数保留 | 阶段 A 才完全迁移 |
| runner/previewer/scanner 注入 | `WithRunner` 链式 API | 简化签名 |

## 测试覆盖

| 包 | 之前测试数 | 之后测试数 | 增量 |
|---|---|---|---|
| `internal/ocr/` | 0 | 13 | +13 |
| `internal/modules/` | 0 | 3 | +3 |
| 全量包 | 25 | 27 | +2 |
| 全量包测试结果 | 全绿 | 全绿 | — |

## 阶段 A 启动条件

✅ B 阶段已合并并跑稳至少 1 周
✅ 13 个新单测全绿
✅ 27 个包全量测试全绿
✅ 文档 `OCR_CORE_DESIGN.md` 已确认

## 阶段 A 实施时需要的迁移

1. 把 `internal/ocr.Registry` 升级为 `internal/ocr.Client`（内部持有 settings 缓存 + Refresh）
2. 把 `internal/service/ocr` / `internal/service/runner` / `internal/service/llm` 三个包合并到 `internal/ocr`
3. 把 5 个等价 settingsSource 接口统一为 1 个
4. 把 4 处 exec 封装统一为 `ocr.Run`
5. 把 `domain/interfaces.go` 拆为 `domain/review.go` + `domain/ocr.go`
6. 删除 `modules.SetPackageResolver`（resolver 进 Registry）
7. 删 `internal/service/ocr` / `runner` / `llm.DynamicProvider` 老包
8. 删 `readiness.OCRBinaryCheck` / `LLMEndpointCheck` 老签名
