# review-service 诊断报告

- 日期:2026-06-22
- 范围:LLM 连通性/可观测性、OCR viewer(:5483)接入、服务层历史 bug 根因
- 方式:只读排查(git log / 代码审阅 / OCR viewer session 取证)

## 一、概念澄清:localhost:5483 与 review-service 的关系

`localhost:5483`(Open Code Review Viewer)不是基于 review-service 构建的,而是相反 ——
review-service 构建在 OCR(Open Code Review 引擎)之上,通过 `ocr viewer` 子命令拉起 viewer
作为被管理的附属进程。

- review-service:HTTP API `:8080` + Vue 控制台 `:3000`
- OCR viewer:引擎自带,默认 `:5483`
- 证据:`internal/service/ocr/viewer.go:95-99` 执行 `ocr viewer`,默认地址 `localhost:5483`;
  `biz/router/settings_service/settings_service.go:41-43` 暴露 `/ocr/viewer` 系列管理 API

层级关系:
```
OCR 引擎 + 内置 viewer (:5483)        底层 AI 评审引擎(独立二进制)
      ▲ 被 wrap
review-service (:8080 / :3000)        网关:多平台 webhook / Agent / 质量门禁 / 回写
```

## 二、本次 LLM DNS 事故的"信息黑洞"根因链

近期对 review-service 自身的评审(6/22)在 OCR viewer 中可见全部失败:
```
Post "https://<llm-gateway>/v1/chat/completions":
dial tcp: lookup <llm-gateway>: no such host
```
但 review-service 自身可观测性看不到此细节,由 5 个缺陷叠加造成:

| 环节 | 缺陷 | 证据 |
|---|---|---|
| 无预检 | 启动不查 LLM 可达、保存配置不强制 TestLLM、运行期无定时探活 | `init.go` 无 preflight 模块;`biz/handler/settings_service/ocr_settings.go:32-57` 不调 TestLLM;grep 定时探活 0 命中 |
| /health 失明 | 唯一 checker 是 DB,OCR/LLM 挂了仍返回 200 | `internal/modules/server.go:58-62` |
| stderr 丢弃 | scanner 默认 64KB 限制 + `scanner.Err()` 被吞 + Wait 后不等 stderr goroutine | `internal/service/runner/runner.go:141-186` |
| 失败回写扁平 | recordFailure 只塞 `Error="open-code-review failed: exit status 1"`,不写 ocr/failed history,不存 stderr | `internal/service/executor/service.go:283-285,340-351` |
| 指标失明 | `/debug/llm/metrics` 永远 0,因 runner 路径绕过 MetricsProvider | `internal/modules/llm.go:19-23` vs `executor/service.go:165-169` |

## 三、合并问题清单(已去重排序)

> 类别:A = LLM 连通性/可观测性,B = OCR viewer 接入,C = 历史 bug 根因。★ = 安全/数据级。

### P0 必修(本次事故级 / 生产事故级)

| # | 类 | 问题 | 证据 file:line |
|---|---|---|---|
| 1 | A | /health 加 OCR 二进制 + LLM endpoint checker | `internal/modules/server.go:58-62` |
| 2 | A | 启动 preflight(exec.LookPath + ocr llm test 短超时) | `init.go` 无 |
| 3 | A | UpdateOCRSettings 保存前强制 TestLLM,失败拒绝 | `biz/handler/settings_service/ocr_settings.go:32-57`;前端 `frontend/src/views/ModelSettings.vue:454-484` |
| 4 | A | recordFailure 写 stage=ocr/status=failed history,details 带 exit_code + stderr 尾部 | `internal/service/executor/service.go:283-285,340-351` |
| 5 | C★ | webhook 静态 DeliveryID 兜底 → 第二条起全部判重静默丢弃 | `internal/service/webhook/gitea.go:174`、`gitcode.go:154`、`tencent.go:154` |
| 6 | C★ | webhook 签名策略不一致:Gitea/GitCode/Tencent 未配 secret 时裸跑放行 | `internal/service/webhook/{github,gitlab,gitea,gitcode,tencent}.go` VerifySignature |

### P1 高优先(可观测性闭环 / 数据丢失)

| # | 类 | 问题 | 证据 |
|---|---|---|---|
| 7 | A | runner 路径也写 llm.Metrics,区分 DNS/5xx/timeout/parse | `internal/modules/llm.go:19-23` vs `executor/service.go:165-169` |
| 8 | A | LLM 定时探活(30s)→ metrics + /health | 无 |
| 9 | A | stderr scanner buffer 扩到 1MB + WaitGroup 等 goroutine | `internal/service/runner/runner.go:141-186` |
| 10 | B | 捕获 OCR session id 入库 + review 详情返回 viewer_url | `internal/service/runner/output.go:80-86` 无 session_id;`internal/repository/postgres/models/review.go` 无列;`biz/handler/review_service/review_service.go:416-432` 不返回链接 |
| 11 | B | 前端 review 详情页加"在 OCR Viewer 看完整 Trace"入口 | `frontend/src/views/ReviewDetail.vue:137-161` |
| 12 | C★ | 平台 Delete/Get 不带 installation_id → 多 installation 互删 | `internal/repository/postgres/git_platform.go:27,45`;`biz/handler/settings_service/platform.go:90-107` |
| 13 | C★ | GIT_SSL_NO_VERIFY 全局污染(请求期 os.Setenv 永不恢复) | `internal/service/provider/service.go:270` |
| 14 | C | 软删除 + 唯一索引未恢复(4 处) | `internal/repository/postgres/custom_provider.go:43-55`、`project_policy.go:42-52`、`reviewer_agent.go:43-53`、`review_map.go:24-26`(与 commit cf5945f 同根) |

### P2 中(健壮性 / 体验)

| # | 类 | 问题 | 证据 |
|---|---|---|---|
| 15 | A | runner 返回结构化 error(%w + ErrOCRTimeout/ErrOCRBinaryMissing…),当前全 %s | `internal/service/runner/runner.go:135,182`;`preview.go:81`;`internal/service/ocr/command.go` |
| 16 | A | 不可重试错误(DNS/配置)不重试,默认 MaxRetries 1→0 | `internal/service/runner/runner.go:30,42-78` |
| 17 | A | RequeueStale 不带 retry_count → 重启即雪崩重跑 | `internal/service/review/worker.go:27-33` |
| 18 | C | context.Background() 漏修 3 处(webhook MarkProcessed ×3、ocr/config) | `internal/service/webhook/service.go:102,111,116` |
| 19 | C | ocr/command.go 的 runOCRCommand 超时不杀进程组(孤儿) | `internal/service/ocr/command.go:18-34` |
| 20 | B | stats 聚合 token/成本(ContributorSummary/ProjectSummary 加字段,jsonb SUM) | `internal/service/stats/types.go:5-91` |
| 21 | B | workflow 路径 OCRMetadata 硬编码,token 全零 | `internal/service/executor/service.go:234-243` |

### P3 低

| # | 类 | 问题 |
|---|---|---|
| 22 | A | /healthz /readyz 路由根本没注册(auth skip 白名单却包含)→ k8s probe 404 |
| 23 | A | ProviderService.TestConnection 是永久成功的桩函数(`internal/service/llm/provider_service.go:124-131`) |
| 24 | C | TOCTOU:findActiveReview → Create 非原子(`internal/service/review/service.go:131-188`) |
| 25 | C | 超时跨层不一致(用户改 OCRSettings.TimeoutMinutes 不传播到 AgentManager) |
| 26 | C | PutPlatformResp 不回填 InstallationID(`biz/handler/settings_service/platform.go:81-87`) |

## 四、三大方向详细发现

### 4.1 LLM 连通性与可观测性(A 类)

**OCR 子进程调用链**:
- `runner.go:141-156` stderr 用 `bufio.Scanner` 默认缓冲(64KB),长 JSON/堆栈会丢;`scanner.Err()` 被 `_ =` 吞掉
- `runner.go:158-186` `close(done)` 后立即读 `stderrLines`,stderr goroutine 可能未结束 → 拿到截断 stderr;应用 WaitGroup 同步
- `runner.go:135,182` 启动失败 / 退出非零都用 `%s` 扁平化错误,丢失 wrap 链,上层无法 errors.As 分类
- `runner.go:95-96` 每个 OCR 子进程自己带 `--concurrency 2 --max-git-procs 4`,worker N × OCR 2 = 实际并发无全局约束
- `runner.go:159-167` Setpgid + kill -PGID 已做进程组清理(良好);但 `syscall.Kill` 返回值被忽略
- `runner.go:30,42-78` 默认 MaxRetries=1,DNS 类错误必然双倍超时(默认 2 × 10.5min = 21min)
- OCR 真正的 LLM 错误可能写在 viewer session 而非 stderr,需确认 OCR 二进制行为

**失败回写**:
- `executor/service.go:340-351` recordFailure 只塞 Error 字符串,RawOCR 永远 nil,stderr 完全不保存
- `executor/service.go:281-287` 失败分支只调 recordFailure,无 `appendHistory(stage="ocr", status="failed")`,前端 timeline 视觉上 OCR 永远卡 running
- `executor/persistence.go:88-95` 状态变化事件 message 固定为"review execution failed",无诊断价值
- `review/worker.go:135-143` tracker.SetError 只推一次 SSE,新连客户端拿不到

**健康检查**:
- `modules/server.go:58-62` 只注册 `/ping` `/health` `/buildVersion`(经 ekit `RegisterBasicRoutes`),`/healthz` `/readyz` 根本不存在但 auth skip 白名单包含 → 配置成 k8s probe 会一直 404
- `/health` 唯一 checker 是 DBChecker,无 OCR 二进制 / LLM endpoint / Git provider 探活
- 启动无 preflight(`init.go` 模块清单无),migrate.go 只同步 config 不验证 OCR/LLM
- `llm/provider_service.go:124-131` TestConnection 是永久成功的桩(目前无引用,隐患)

**LLM 配置/预检**:
- `ocr_settings.go:32-57` UpdateOCRSettings 只做字段范围校验,不调 TestLLM
- `ModelSettings.vue:454-484` 前端保存按钮不等 testLLM,testLLM 是独立按钮
- 全局无定时探活 LLM endpoint 机制
- `llm.Metrics` 只包 workflow 路径,runner 路径(默认)绕过 → `/debug/llm/metrics` 永远 0
- `models/ocr_settings.go` 无 LastTestedAt / LastTestSuccess 字段

**超时/并发**:
- `runner.go:56` 超时后错误是 `signal: killed`,用户无从判断"超时" vs "OCR 卡死"
- `executor/service.go:140-158` cleanup 失败只 hlog.Warn,磁盘残留静默
- `review/worker.go:36-46` worker 池 spawn goroutine 后立即再 claim,实际并发 = 派生速度
- `review/worker.go:27-33` RequeueStale 重启即重排全部 running,LLM 故障期雪崩重跑
- `ocr/tester.go:22-51` TestLLM 无独立 timeout,依赖外层 ctx

### 4.2 OCR viewer 接入(B 类)

**端到端链路断点**(用户在控制台点 review 看 OCR trace,需要 4 环,目前仅环 2 部分):

| 环 | 状态 | 说明 |
|---|---|---|
| ① 捕获 OCR session id 入库 | 缺失 | runner/output.go:80-86 不解析 session_id;models/review.go 无列 |
| ② viewer 可启动 | 部分 | viewer.go 能拉起子进程,但端口硬编码、无探活、无生命周期 hook |
| ③ 详情 API 返回 viewer 链接 | 缺失 | review_service.go:416-432 convertDetail 无 viewer_url |
| ④ 前端入口 | 缺失 | ReviewDetail.vue:137-161 只显示 token 数字,无跳转 |

**viewer 管理**(`internal/service/ocr/viewer.go` 全 161 行):
- `viewer.go:99` 端口硬编码 `localhost:5483`,config/DB/IDL 全无 viewer 配置项
- `viewer.go:54-63` Status 只看内存态 `process == nil`,不 TCP/HTTP 探活,假启动风险
- `viewer.go:102-107` cmd.Start 后立即返回 success,不等端口就绪;5483 被占时 OCR 启动即挂,review-service 仍报"已启动"
- `modules/ocr.go:40-42` 仅 fx.Provide,**无 lifecycle hook**,review-service 退出时 viewer 子进程变孤儿(Setpgid 独立进程组更不会连带回收)
- `viewer.go:16-23` pid/addr 纯内存,重启后不知道旧 viewer 端口

**session 数据**(最致命断点):
- `models/review.go:8-24` Review 表无 ocr_session_id / viewer_url 列
- `models/session.go` ReviewSession 是 review-service 自己的编排 session(`types.GenerateID()` 生成),与 OCR 无关,名字误导
- `runner/runner.go:89-99` 拼的 args 无 `--session-id`,grep 全仓 0 命中
- `executor/service.go:431-451` RawOCRInput 是命令参数快照,不是 session

**前端**:
- `ExecutionParamsTab.vue:83-106` 是唯一 viewer 操作入口(启/停/刷新),无"打开 Viewer"外链
- `ReviewDetail.vue:137-161` OCR 卡片无 viewer 跳转
- `types/config.ts:171-175` ViewerStatus 类型只有 running/addr/message
- `DiscussionViewer.vue` / `ReviewMap.vue` 名字带 Viewer 但是 review-service 自己的组件,与 :5483 无关

**token/成本**:
- `runner/output.go:67-75` 单次 review 解析 5 个 token 字段
- `executor/service.go:287-292` total_tokens 写 history;BuildReport 整体塞 Report.OCR
- `ReviewDetail.vue:152-154` 展示 total/input/output,**不展示** CacheRead/CacheWrite(数据有,前端没读)
- `stats/types.go:5-91` 完全无 token/cost 聚合,无法回答"本月花了多少 token"
- `executor/service.go:234-243` workflow 路径 OCRMetadata 硬编码 Engine:workflow,token 全零
- `llm/metrics.go` 是另一套(review-service 自连 LLM 的统计),与 OCR token 未合并

**配置层**:
- `config/yaml.go:40-48` OCRConfig 无 viewer 字段
- `etc/config-local.yaml:23-28` 无 viewer 配置
- `idl/settings.thrift:31-58` OCRSettings struct 26 字段无 viewer;仅 `idl/settings.thrift:157-171` 有运行态 ViewerStatus/StartViewer/StopViewer

**端到端修复依赖**:第 2 批 viewer 接入(B 类)技术上最硬的骨头是需要先确认 OCR `review --format json` 是否输出 session id(建议先做 spike)。

### 4.3 历史 bug 根因(C 类)

**历史 fix 概览**(后端 14 个关键修复):cf5945f(软删除)、4be3c98(installation_id 归一化)、f522835(ConfigFileWriter omitempty + 工厂超时)、48a7f34(超时跨层)、b6fd559(13 项 P0/P1/P2 含 context.Background 误用)、b7e97b8(worker 并发安全)、bc514fc(Requeue 事务 + defer git.Close)、8335826(stderr Mutex + scanner.Err)、810ab76(OriginalHeadSHA)、fbed825(merged PR base SHA)、9b2e59d(Gitea 单行评论)、8205ef1(Gitea HMAC 签名)、6d41722(skipped 状态)、3a5bc73(JSON 数组)。

**根因模式表**:

| 模式 | 历史证据 | 当前仍存的同类隐患 | 风险 |
|---|---|---|---|
| M1 软删除+唯一索引不恢复 deleted_at | cf5945f | custom_provider/project_policy/reviewer_agent/review_map 4 处 | 高 |
| M2 context.Background() 误用 | b6fd559 修了 2 处 | webhook/service.go:102,111,116 漏修 3 处;ocr/viewer.go:35;ocr/config.go:23;provider/service.go:270 | 中 |
| M3 exec 超时不杀进程组 | runner.go 已修(模板) | ocr/command.go:18-34 runOCRCommand 没复制该模式 | 中 |
| M4 webhook 静态 DeliveryID 兜底 | 8205ef1 暴露幂等框架 | gitea/gitcode/tencent "xxx-unknown" 静态 | 极高 |
| M5 签名策略跨平台不一致 | 8205ef1 引入 secret==""→放行 | github/gitlab 拒绝,gitea/gitcode/tencent 放行 | 高(安全) |
| M6 TOCTOU 读改写非原子 | bc514fc | review/service.go:131-188;executor/persistence.go;ocr_settings/project_policy/global_rules | 中 |
| M7 全局可变环境(GIT_SSL_NO_VERIFY) | 同 b6fd559 安全类 | provider/service.go:270 请求期永不恢复 | 高(安全) |
| M8 吞错 | b6fd559/bc514fc | webhook/service.go:99 解析吞错;runner.go:153 注释与代码不符;webhook.go:129,134,135 落库全吞 | 中 |
| M9 Delete/Get 不带全键 | 4be3c98 同根 | git_platform.go:27,45 只按 provider,多 installation 互删 | 高(数据) |
| M10 超时跨层不一致 | 48a7f34+f522835 手动对齐过默认值 | 用户改 OCRSettings.TimeoutMinutes 不传播到 AgentManager(仍 35min 杀) | 中 |
| M11 bool+omitempty 吞 false | f522835 修了 UseAnthropic | 当前 safe,建议加 lint 防回归 | 低 |
| M12 缺幂等键的手动 API | b6fd559 用 crypto/rand 防碰撞 | review/service.go:113-188 手动 API 不传幂等键时仅靠 findActiveReview | 低 |
| M13 PutPlatform 响应丢字段 | 4be3c98 相关 | platform.go:81-87 不回填 InstallationID | 低 |

**子系统重灾区**:
- `internal/service/webhook/`:M4(极高)+ M5(安全)+ M2(漏修)+ M8(吞错)
- `internal/service/provider/`:M7(全局污染)+ M9(数据)+ Reload 没锁 s.configs
- `internal/repository/postgres/`:M1(4 处)+ M6(Get-then-Create 非原子);project_agent.go 是唯一正确处理软删除的(Unscoped)

## 五、建议修复路线图

**第 1 批 P0(#1-6)** — 止血 + 防事故,6 处改动,纯后端,含 2 个安全修复。改完 DNS 类故障 1 分钟内可发现、前端可见根因。
- #1 `/health` 加 OCR+LLM checker
- #2 启动 preflight
- #3 UpdateOCRSettings 强制 TestLLM
- #4 recordFailure 写 ocr/failed history + stderr 尾部
- #5 webhook DeliveryID 动态兜底(参考 gitlab.go:167 `gl-<unixnano>`)
- #6 webhook 签名策略统一(配置项 allow_insecure,默认 false)

**第 2 批 P1(#7-14)** — 可观测性闭环 + viewer 接入 + 数据/安全兜底。含唯一 schema 变更(review 表加 ocr_session_id)。前置:做 OCR `--format json` session id spike。
- #10 #11 是 viewer 端到端打通的核心
- #12 #13 #14 是数据/安全兜底

**第 3 批 P2+P3** — 健壮性与体验,按需推进。

## 附录:关键文件清单

**A 类(LLM 链路)**:
- `internal/service/runner/runner.go`(207 行,核心)、`output.go`、`config.go`、`preview.go`
- `internal/service/ocr/command.go`、`tester.go`、`viewer.go`、`versioner.go`
- `internal/service/executor/service.go`(Execute/recordFailure)、`persistence.go`
- `internal/service/review/worker.go`、`types.go`、`result.go`
- `internal/modules/server.go`(health 注册)、`runner.go`、`llm.go`、`migrate.go`
- `internal/service/llm/provider_service.go:124-131`(桩)、`metrics.go`
- `init.go`(模块清单,无 preflight)

**B 类(viewer 接入)**:
- `internal/service/ocr/viewer.go`(全文)、`internal/modules/ocr.go`
- `internal/service/runner/output.go:80-86`(session id 缺失)
- `internal/repository/postgres/models/review.go`、`session.go`、`ocr_settings.go`
- `biz/handler/review_service/review_service.go:416-457`、`biz/handler/settings_service/ocr_tools.go`
- `idl/settings.thrift:31-58,157-171`、`internal/config/yaml.go:40-48`
- `frontend/src/views/ReviewDetail.vue:137-161`、`components/model-settings/ExecutionParamsTab.vue:83-106`
- `internal/service/stats/types.go`

**C 类(历史 bug)**:
- `internal/service/webhook/{github,gitlab,gitea,gitcode,tencent}.go`(签名 + DeliveryID)
- `internal/service/webhook/service.go:99,102,111,116`(吞错 + ctx)
- `internal/repository/postgres/{custom_provider,project_policy,reviewer_agent,review_map,git_platform}.go`
- `internal/service/provider/service.go:270`、`internal/modules/provider.go:51,82`(GIT_SSL_NO_VERIFY)
- `internal/service/review/service.go:131-188`(TOCTOU)
