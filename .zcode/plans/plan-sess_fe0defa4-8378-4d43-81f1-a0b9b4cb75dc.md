## 重新设计 模型 A/B 对比面板

### 目标文件
重写 `./review-service/frontend/src/components/review-detail/ModelComparePanel.vue`，并拆出两个子组件（贴合最近 `review-detail`/`ocr-settings` 的组件拆分风格）：
- 新建 `frontend/src/components/review-detail/ModelCompareCandidateForm.vue` — 单个候选输入卡
- 新建 `frontend/src/components/review-detail/ModelCompareResultCard.vue` — 单个对比结果卡（含可展开明细）
- 重写 `ModelComparePanel.vue` — 容器：头部、候选列表、添加按钮、汇总条、结果区

API 与类型不变：`compareReviewModels(reviewId, candidates)` 与 `ModelCompareCandidate` / `ModelCompareCandidateRun` / `ModelCompareResult` 复用现有定义（`ModelCompareCandidate` 已含 provider/auth/headers/body 字段，这次补齐 UI）。

---

### 设计要点（对应你选的三项）

**① 候选输入卡（多候选 2+，可增删）**
```
┌ ● A ────────────────────── [×] ┐
│ 模型  [ gpt-4o              ]   │
│ URL   [ https://… 可选       ]   │
│ [▾ 高级] provider · token · …   │
└─────────────────────────────────┘
┌ ● B ────────────────────── [×] ┐  …
┌─────── [＋ 添加候选] ───────────┐  (虚线 dashed，hover 变色)
```
- 左侧彩色「泳道徽章」（A=蓝/B=紫/C=绿/D=橙…），复用 `PresetSelector` 的渐变方块样式。
- 默认只填「模型名 + URL」；点「高级」折叠展开 provider / auth_token / auth_header / extra_headers / extra_body（当前类型已有、UI 缺失）。
- 每行右侧 `[×]` 删除（≥1 个时显示）；底部 `[＋ 添加候选]` 虚线按钮追加。
- 候选序号删除后自动重排标签（A/B/C…）。

**② 对比结果卡 + 可展开明细（结果呈现核心）**
```
┌ 🏆 A 质量最高 · +8 分 · B 最快 · 快 6s ┐   ← 汇总条(仅≥2成功候选时显示)
├──────────────────┬──────────────────┤
│ ●A 🏆  gpt-4o    │ ●B     claude-3.5│
│      ◯ 88        │      ◯ 80        │   ← 评分环(复用 sidebar SVG)
│      等级 A 🟢   │      等级 B 🟢   │
│ 问题 12  警告 3  │ 问题 9   警告 1  │
│ 1.2k tok · 34s   │ 980 tok · 28s    │
│ 摘要：…（2行截断）│ 摘要：…          │
│ [▾ 展开明细]     │ [▾ 展开明细]     │
└──────────────────┴──────────────────┘
```
- 汇总条：胜出者(最高分)+分差、最快者+速度差，多指标并列；失败时不显示。
- 结果卡：`grid-template-columns: repeat(auto-fit, minmax(240px, 1fr))` 自适应（A/B 两列，加 C/D 自动换行）。
- 评分环：复用 `ReviewSummarySidebar` 的 SVG 圆环（r=34，`scoreColor`：≥85 绿/≥70 琥珀/<70 红）。
- 明细展开区（点「展开明细」切换）：token 拆解(input/output/cache)、provider、URL、摘要全文。
- 失败候选：卡片显示红色错误框（替代当前底部纯文本错误列表）。
- 胜出者：质量最高且成功的候选带 🏆 冠军标记（同分不标记）。

---

### 组件契约

**ModelComparePanel.vue（容器）**
- props: `{ reviewId: string }`
- state: `candidates: ModelCompareCandidate[]`(reactive, 初始 2 个 A/B)、`result`、`loading`、`expandedResult: Set<string>`
- computed: `validCandidates`(model 非空)、`successfulRuns`、`winner`(最高分)、`fastest`(最小 duration_ms)、`failedRows`
- 渲染：头部(标题+开始对比按钮) → 候选输入卡列表 + 添加按钮 → [结果区] 汇总条 + 结果卡网格

**ModelCompareCandidateForm.vue（候选输入卡）**
- props: `{ candidate: ModelCompareCandidate; index: number; removable: boolean }`
- emits: `remove`
- 本地 state: `showAdvanced`(折叠高级配置)
- 字段：label(徽章)、model、url + 高级：provider、auth_token、auth_header、extra_headers、extra_body

**ModelCompareResultCard.vue（结果卡）**
- props: `{ run: ModelCompareCandidateRun; rank: number; isWinner: boolean; isFastest: boolean; expanded: boolean }`
- emits: `toggle`
- 渲染：徽章+模型名+状态 → 评分环(成功) / 错误框(失败) → 指标(问题/警告/tokens/耗时) → 摘要(截断) → 展开明细

---

### 样式规范（复用现有设计系统）
- 颜色 token：`--app-border`、`--app-surface`、`--app-text`、`--app-muted`、`--app-primary`
- 卡片：`border-radius: 10-14px`、`border: 1px solid var(--app-border)`、`background: #fff`
- 评分环色：≥85 `#22c55e` / ≥70 `#e6a23c` / <70 `#f56c6c`（与 sidebar 一致）
- 泳道徽章：复用 `PresetSelector` 的渐变方块（A=蓝/B=紫/C=绿/D=橙/E+=灰）
- 耗时格式化：`Ns`(<60s) 或 `Mm Ss`(≥60s)，比当前固定 `Math.round(/1000)s` 更准
- 图标：`Plus`/`Close`/`ArrowDown`/`ArrowUp` from `@element-plus/icons-vue`；🏆 用 emoji（与汇总条风格一致）

### 不改动
- API 层（`api/review.ts`）、类型（`types/review.ts`）、后端。
- `ReviewContentArea.vue` 中渲染位置与 `v-if` 条件保持不变。

### 验证
- 启动 dev server，访问 `/reviews/:id`（success 状态），确认：候选可增删、高级可展开、对比后显示评分环+汇总条+可展开明细、失败候选显示错误框、响应式换行。