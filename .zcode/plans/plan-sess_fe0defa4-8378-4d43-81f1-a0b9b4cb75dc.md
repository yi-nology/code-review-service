## 激活后端 Provider 库 + 模型预设库 + A/B 引用预设

彻底解决「割裂」：把闲置的 `custom_providers` 后端库激活，在 `settings/model` 页新增「模型预设库」管理区，A/B 对比改为下拉引用预设并**后端按 id 解析真实凭据**（前端永不接触 key）。

---

### 一、后端改动（让 model-compare 按 id 解析预设凭据）

候选新增 `provider_id`；后端注入 `CustomProviderRepository`，在 `runModelCandidate` 前按 id 查库，用预设的 url/model/key/auth_header/extra_body **作为基底、请求体显式字段覆盖**，再执行。

1. **`internal/service/review/model_compare.go`**
   - `ModelCompareCandidate` 增加 `ProviderID int64 \`json:"provider_id,omitempty"\``。
   - 新增 `resolveCustomProvider(ctx, candidate)`：`ProviderID>0` 时 `repo.GetByID`，把库里 url/model/auth_token/auth_header/extra_body 作基底，请求体非空字段覆盖（用户下拉选预设后仍可临时改 model）；`Protocol`→`Provider`。查不到则该候选 error。
   - `CompareModels` 在取完 review/provider 后、并行执行前遍历候选做解析（解析本身串行、轻量；执行仍并行）。

2. **`internal/service/review/service.go`** — `ServiceConfig` 与 `Service` 各加 `customProviderRepo repository.CustomProviderRepository` 字段；`NewService` 赋值。

3. **`internal/modules/review.go`** — `NewReviewService` 的 `ServiceConfig` 加 `CustomProviderRepo: deps.Repos.CustomProvider`（`ReviewServiceDeps.Repos *RepoContainer` 已在，其 `.CustomProvider` 字段已存在于 `container.go:24`，不加新依赖、不加路由）。

4. **`biz/handler/review_service/model_compare.go`** — 无需改（透传 `ModelCompareOptions`，新字段自动随绑定）。

> 完全复用现有 `RepoContainer.CustomProvider` 与 `CustomProviderRepository.GetByID`，不新建表、不加路由。

---

### 二、前端：settings/model 页新增「模型预设库」区块

**新组件 `frontend/src/components/model-settings/ProviderPresetLibrary.vue`**
- `onMounted` 拉 `listCustomProviders()`；列表展示：图标 + 名称 + model + protocol + [编辑][删除]。
- `[+ 新建预设]` → 行内表单（name 必填、protocol openai/anthropic、url、api_key 密码框、model、auth_header、extra_body），调 `createCustomProvider` 后刷新。
- 编辑：行内表单回填（api_key 留空表示不改），调 `updateCustomProvider(id,...)`。
- 删除：确认后 `deleteCustomProvider(id)`。
- 空列表用 `el-empty`；复用 `ModelSettings.vue` 的 `.pchip` 渐变图标语言。

**`views/ModelSettings.vue`** — 在 `config-card` 下方加 `<ProviderPresetLibrary />`，区块标题「模型预设库」+ 说明"保存多套模型配置，供 A/B 对比快速引用"。

---

### 三、前端：A/B 对比改为引用预设

**`types/review.ts`** — `ModelCompareCandidate` 增加 `provider_id?: number`。

**`ModelComparePanel.vue`（容器）**
- `onMounted` 拉 `listCustomProviders()` 存 `presets`；传给候选卡。
- `validCandidates` 提交体带上各自 `provider_id`。

**`ModelCompareCandidateForm.vue`（候选输入卡）**
- 新增 props `presets: CustomProvider[]`。
- 顶部加预设下拉 `[▾ 从预设库选择]`（可搜索）；选中后回填 model/url 并标记 `provider_id`，显示「已引用：预设名」徽章 + `[×]` 解除引用（清 provider_id，保留已填字段）。
- 现有 model/url 字段保留可编辑（引用预设后仍可微调，微调不影响库）；高级区不变。

---

### 四、不改
- API 层 `api/config.ts`（custom providers CRUD 已封装）、`api/review.ts`。
- 后端 settings CRUD handler、`dbCustomProviderToAPI`（仍屏蔽 key，解析在后端做）。
- `ReviewContentArea.vue` 渲染位置。

---

### 五、验证
- 后端 `go build ./...` + `go test ./internal/service/review/...`。
- 前端 `vue-tsc --noEmit` + `vite build`。
- 手动：settings/model 建两个预设（gpt-4o+key、claude+key）→ A/B 下拉选这两个 → 对比正常（后端用各自 key）；手填（不选预设）仍可用。