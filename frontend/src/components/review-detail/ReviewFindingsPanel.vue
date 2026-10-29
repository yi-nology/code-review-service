<template>
  <el-card class="findings-card" v-if="findings?.length">
    <template #header>
      <div class="card-header">
        <span>问题报告</span>
        <div class="finding-header-actions">
          <template v-if="patchCapable">
            <el-tag>{{ findings.length }} 个问题</el-tag>
            <el-tooltip
              :disabled="Boolean(patchableFindingIndexes.length)"
              content="当前没有满足文件路径、起始行号、现有代码和建议代码的问题"
              placement="top"
            >
              <el-button size="small" plain :disabled="!patchableFindingIndexes.length" @click="selectPatchableFindings">
                全选可生成
              </el-button>
            </el-tooltip>
          </template>
          <el-tag v-else>{{ findings.length }} 个问题</el-tag>
        </div>
      </div>
    </template>

    <!-- 筛选条：严重度 / 处置状态 / 可生成 Patch / 分组开关 / 隐藏已处置 -->
    <div class="findings-filter-bar">
      <el-select
        v-model="severityFilter"
        multiple
        collapse-tags
        collapse-tags-tooltip
        placeholder="全部严重度"
        size="small"
        class="filter-severity"
      >
        <el-option label="严重" value="critical" />
        <el-option label="高" value="major" />
        <el-option label="中" value="minor" />
        <el-option label="提示" value="info" />
      </el-select>
      <el-select v-model="statusFilter" placeholder="全部状态" size="small" clearable class="filter-status">
        <el-option label="未处置" value="untouched" />
        <el-option label="误报" value="false_positive" />
        <el-option label="接受风险" value="accepted_risk" />
        <el-option label="暂不处理" value="deferred" />
        <el-option label="已修复" value="fixed" />
      </el-select>
      <el-checkbox v-if="patchCapable" v-model="onlyPatchable" size="small">仅可生成 Patch</el-checkbox>
      <el-checkbox v-model="hideDisposed" size="small">隐藏已处置</el-checkbox>
      <div class="filter-spacer" />
      <span class="filter-count">显示 {{ visibleCount }} / {{ findings.length }}</span>
      <el-radio-group v-model="groupBy" size="small" class="filter-group">
        <el-radio-button value="file">按文件</el-radio-button>
        <el-radio-button value="severity">按严重度</el-radio-button>
        <el-radio-button value="none">平铺</el-radio-button>
      </el-radio-group>
      <el-button size="small" plain @click="toggleAllGroups">{{ allCollapsed ? '展开全部' : '折叠全部' }}</el-button>
    </div>

    <!-- Patch 工作流：默认折叠，降低噪音；带步骤引导（argus 模式无建议代码，整块隐藏） -->
    <details v-if="patchCapable" class="patch-panel-wrap">
      <summary class="patch-panel-summary">
        <strong>修复 Patch 工作流</strong>
        <span class="patch-panel-hint">已选 {{ patchSelectionIndexes.length }} / 可生成 {{ patchableFindingIndexes.length }} · 点击展开</span>
      </summary>
      <div class="suggestion-patch-panel">
        <ol class="patch-steps">
          <li :class="{ active: patchSelectionIndexes.length === 0 }">① 勾选问题（或「全选可生成」）</li>
          <li :class="{ done: Boolean(patchPreview?.patch) }">② 生成预览</li>
          <li :class="{ done: Boolean(patchCheck?.applicable) }">③ 检查可应用</li>
          <li :class="{ active: canApplyPatch }">④ 创建修复提交</li>
        </ol>
        <div class="patch-summary">
          <div class="patch-actions">
            <el-input
              v-model="patchBranch"
              class="patch-branch-input"
              size="small"
              placeholder="修复分支名"
              clearable
            />
            <el-input
              v-if="patchCreateCR"
              v-model="patchTargetBranch"
              class="patch-branch-input"
              size="small"
              placeholder="目标分支"
              clearable
            />
            <el-checkbox v-model="patchCreateCR" size="small">创建 PR/MR</el-checkbox>
            <el-button size="small" @click="clearPatchSelection">清空</el-button>
            <el-button size="small" type="primary" plain :disabled="!patchSelectionIndexes.length" :loading="patchLoading" @click="handlePreviewPatch">
              ② 生成预览
            </el-button>
            <el-button size="small" plain :disabled="!patchPreview?.patch" :loading="patchChecking" @click="checkPatch">
              ③ 检查可应用
            </el-button>
            <el-button size="small" plain :disabled="!patchPreview?.patch" @click="copyPatch">复制</el-button>
            <el-button size="small" plain :disabled="!patchPreview?.patch" :loading="patchDownloading" @click="$emit('download-patch', patchSelectionIndexes)">
              下载 .diff
            </el-button>
            <el-tooltip
              :disabled="canApplyPatch"
              content="需先勾选问题 → 生成预览 → 检查可应用，三步完成后才可创建提交"
              placement="top"
            >
              <span>
                <el-button size="small" type="success" plain :disabled="!canApplyPatch" :loading="patchApplying" @click="applyPatch">
                  ④ 创建修复提交
                </el-button>
              </span>
            </el-tooltip>
          </div>
        </div>
        <el-alert
          v-if="!patchableFindingIndexes.length"
          class="patch-empty-alert"
          title="当前问题缺少可安全生成 Patch 的建议代码"
          description="修复 Patch 需要问题同时包含文件路径、起始行号、现有代码和建议代码。只有满足这些条件的问题才能勾选并生成 diff。"
          type="info"
          :closable="false"
          show-icon
        />
        <div v-if="patchPreview" class="patch-result">
          <div class="patch-result-meta">
            <el-tag type="success" size="small">生成 {{ patchPreview.generated_count }}</el-tag>
            <el-tag v-if="patchPreview.skipped_count" type="warning" size="small">跳过 {{ patchPreview.skipped_count }}</el-tag>
            <el-tag v-if="patchCheck" :type="patchCheck.applicable ? 'success' : 'danger'" size="small">
              {{ patchCheck.applicable ? '可应用' : '不可应用' }}
            </el-tag>
            <el-tag v-else-if="patchPreview?.patch" type="info" size="small">待检查</el-tag>
          </div>
          <div v-if="patchCheck && (!patchCheck.applicable || patchCheck.output)" class="patch-check-output">
            {{ patchCheck.error || 'git apply --check' }}<template v-if="patchCheck.output">：{{ patchCheck.output }}</template>
          </div>
          <div v-if="patchApplyResult" class="patch-apply-result" :class="{ failed: !patchApplyResult.applied }">
            <template v-if="patchApplyResult.applied">
              已创建 {{ patchApplyResult.branch }} · {{ patchApplyResult.commit_sha || '本地提交' }}
              <a v-if="patchApplyResult.cr_url" :href="patchApplyResult.cr_url" target="_blank" rel="noreferrer">
                PR/MR #{{ patchApplyResult.cr_number || '' }}
              </a>
              <span v-if="patchApplyResult.workspace">工作区：{{ patchApplyResult.workspace }}</span>
            </template>
            <template v-else>
              创建失败：{{ patchApplyResult.error || 'Patch 无法应用' }}
            </template>
          </div>
          <div v-if="patchPreview.skipped?.length" class="patch-skipped">
            <span v-for="item in patchPreview.skipped" :key="`${item.index}-${item.reason}`">
              #{{ item.index + 1 }} {{ item.path || item.title || '未命名问题' }}：{{ item.reason }}
            </span>
          </div>
          <pre v-if="patchPreview.patch" class="patch-preview">{{ patchPreview.patch }}</pre>
          <el-empty v-else description="当前选择没有可生成的 Patch" :image-size="48" />
        </div>
      </div>
    </details>

    <!-- 问题列表：按分组渲染，可折叠；每个 finding 带 id 锚点 -->
    <div class="finding-list">
      <template v-if="groups.length">
        <div v-for="group in groups" :key="group.key" class="finding-group">
          <button class="group-header" type="button" @click="toggleGroup(group.key)">
            <el-icon class="group-caret" :class="{ collapsed: collapsedGroups.has(group.key) }"><ArrowRight /></el-icon>
            <span class="group-title">{{ group.title }}</span>
            <span class="group-count">{{ group.items.length }} 个</span>
            <span v-if="group.severityCounts.critical" class="sev-badge critical">严重×{{ group.severityCounts.critical }}</span>
            <span v-if="group.severityCounts.major" class="sev-badge major">高×{{ group.severityCounts.major }}</span>
            <span v-if="group.severityCounts.minor" class="sev-badge minor">中×{{ group.severityCounts.minor }}</span>
          </button>
          <div v-show="!collapsedGroups.has(group.key)" class="group-body">
            <FindingItem
              v-for="entry in group.items"
              :key="entry.index"
              :finding="entry.finding"
              :index="entry.index"
              :selected="selectedPatchIndexes.includes(entry.index)"
              :patchable="isPatchableFinding(entry.finding)"
              :patch-block-reason="patchBlockReason(entry.finding)"
              :show-patch="patchCapable"
              :feedback="feedbackByIndex[entry.index]"
              :historical-fp="Boolean(entry.finding.fingerprint && historicalFalsePositiveByFp[entry.finding.fingerprint])"
              :highlight="highlightIndex === entry.index"
              @toggle-patch="(checked: boolean) => togglePatchFinding(entry.index, checked)"
              @feedback="(status: string) => handleFeedback(entry.index, status)"
            />
          </div>
        </div>
      </template>
      <el-empty v-else description="当前筛选条件下没有匹配的问题" :image-size="64">
        <el-button size="small" @click="clearFilters">清除筛选</el-button>
      </el-empty>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { ArrowRight } from '@element-plus/icons-vue'
import { previewSuggestionPatch, checkSuggestionPatch, applySuggestionPatch, getFindingFeedback, saveFindingFeedback, lookupFeedbackByFingerprints } from '@/api/review'
import type { Finding, FindingFeedback, SuggestionPatchApplyResult, SuggestionPatchCheck, SuggestionPatchPreview } from '@/types/review'
import { ElMessage } from 'element-plus/es/components/message/index'
import FindingItem from './FindingItem.vue'

const props = defineProps<{
  reviewId: string
  findings: Finding[]
  // mode 审查模式（ocr | argus | scan）：argus 无建议代码，隐藏 Patch 工作流与占位标签
  mode?: 'ocr' | 'argus' | 'scan'
}>()

const emit = defineEmits<{
  'download-patch': [indexes: number[]]
}>()

// argus 模式的发现没有 existing_code/suggestion_code，Patch 全链路不适用
const patchCapable = computed(() => (props.mode ?? 'ocr') !== 'argus')

// ---- Patch 相关状态（与原实现一致） ----
const selectedPatchIndexes = ref<number[]>([])
const patchPreview = ref<SuggestionPatchPreview | null>(null)
const patchCheck = ref<SuggestionPatchCheck | null>(null)
const patchApplyResult = ref<SuggestionPatchApplyResult | null>(null)
const patchBranch = ref('')
const patchTargetBranch = ref('')
const patchCreateCR = ref(false)
const patchLoading = ref(false)
const patchChecking = ref(false)
const patchDownloading = ref(false)
const patchApplying = ref(false)
const patchCheckKey = ref('')
const feedbackByIndex = ref<Record<number, FindingFeedback>>({})
const historicalFalsePositiveByFp = ref<Record<string, boolean>>({})

// ---- 筛选 / 分组状态 ----
const severityFilter = ref<string[]>([])
const statusFilter = ref('')
const onlyPatchable = ref(false)
const hideDisposed = ref(false)
const groupBy = ref<'file' | 'severity' | 'none'>('file')
const collapsedGroups = ref<Set<string>>(new Set())
const highlightIndex = ref<number | null>(null)
let highlightTimer: ReturnType<typeof setTimeout> | null = null

interface FindingEntry { finding: Finding; index: number }

const patchableFindingIndexes = computed(() =>
  props.findings
    .map((finding, index) => ({ finding, index }))
    .filter(({ finding }) => isPatchableFinding(finding))
    .map(({ index }) => index),
)

const patchSelectionIndexes = computed(() => {
  const allowed = new Set(patchableFindingIndexes.value)
  return selectedPatchIndexes.value.filter((index) => allowed.has(index))
})

const patchSelectionKey = computed(() => patchSelectionIndexes.value.join(','))
const canApplyPatch = computed(() =>
  Boolean(patchSelectionIndexes.value.length && patchCheck.value?.applicable && patchCheckKey.value === patchSelectionKey.value),
)

function isPatchableFinding(finding: Finding) {
  const line = finding.start_line || finding.line || 0
  return Boolean(finding.path?.trim() && finding.existing_code?.trim() && finding.suggestion_code?.trim() && line > 0)
}

function patchBlockReason(finding: Finding) {
  const line = finding.start_line || finding.line || 0
  if (!finding.path?.trim()) return '缺少文件路径'
  if (line <= 0) return '缺少起始行号'
  if (!finding.existing_code?.trim()) return '缺少现有代码'
  if (!finding.suggestion_code?.trim()) return '缺少建议代码'
  return '可生成 Patch'
}

// 过滤后的 finding 条目（带原始 index）。
const filteredEntries = computed<FindingEntry[]>(() => {
  return props.findings
    .map((finding, index) => ({ finding, index }))
    .filter(({ finding, index }) => {
      if (severityFilter.value.length && !severityFilter.value.includes(finding.severity)) return false
      if (onlyPatchable.value && !isPatchableFinding(finding)) return false
      const fb = feedbackByIndex.value[index]
      const hasFeedback = Boolean(fb)
      if (hideDisposed.value && hasFeedback) return false
      if (statusFilter.value) {
        if (statusFilter.value === 'untouched') {
          if (hasFeedback) return false
        } else if (!fb || fb.status !== statusFilter.value) {
          return false
        }
      }
      return true
    })
})

const visibleCount = computed(() => filteredEntries.value.length)

interface FindingGroup {
  key: string
  title: string
  items: FindingEntry[]
  severityCounts: Record<string, number>
}

// 按所选维度分组。severity 分组保持 critical→info 固定顺序。
const groups = computed<FindingGroup[]>(() => {
  const entries = filteredEntries.value
  if (groupBy.value === 'none') {
    return entries.length
      ? [{ key: 'all', title: '全部问题', items: entries, severityCounts: countSeverities(entries) }]
      : []
  }

  const map = new Map<string, FindingEntry[]>()
  for (const entry of entries) {
    let key: string
    if (groupBy.value === 'file') {
      key = entry.finding.path?.trim() || '（未关联文件）'
    } else {
      key = entry.finding.severity
    }
    if (!map.has(key)) map.set(key, [])
    map.get(key)!.push(entry)
  }

  const buildTitle = (key: string) => {
    if (groupBy.value === 'severity') return severityLabel(key)
    return key
  }

  let result = Array.from(map.entries()).map(([key, items]) => ({
    key,
    title: buildTitle(key),
    items,
    severityCounts: countSeverities(items),
  }))

  // severity 分组固定排序：严重→高→中→提示。
  if (groupBy.value === 'severity') {
    const order = ['critical', 'major', 'minor', 'info']
    result.sort((a, b) => order.indexOf(a.key) - order.indexOf(b.key))
  }
  return result
})

const allCollapsed = computed(() =>
  groups.value.length > 0 && groups.value.every((g) => collapsedGroups.value.has(g.key)),
)

function countSeverities(entries: FindingEntry[]): Record<string, number> {
  const counts: Record<string, number> = { critical: 0, major: 0, minor: 0, info: 0 }
  for (const e of entries) {
    counts[e.finding.severity] = (counts[e.finding.severity] || 0) + 1
  }
  return counts
}

function severityLabel(severity: string) {
  const map: Record<string, string> = { critical: '严重', major: '高风险', minor: '中风险', info: '提示' }
  return map[severity] || '未知'
}

function toggleGroup(key: string) {
  const next = new Set(collapsedGroups.value)
  if (next.has(key)) next.delete(key)
  else next.add(key)
  collapsedGroups.value = next
}

function toggleAllGroups() {
  if (allCollapsed.value) {
    collapsedGroups.value = new Set()
  } else {
    collapsedGroups.value = new Set(groups.value.map((g) => g.key))
  }
}

// 分组变化时，清理已失效的折叠 key，避免残留。
watch(groups, (gs) => {
  const valid = new Set(gs.map((g) => g.key))
  const next = new Set<string>()
  for (const k of collapsedGroups.value) if (valid.has(k)) next.add(k)
  if (next.size !== collapsedGroups.value.size) collapsedGroups.value = next
})

function clearFilters() {
  severityFilter.value = []
  statusFilter.value = ''
  onlyPatchable.value = false
  hideDisposed.value = false
}

// 外部跳转到指定 finding（来自侧栏「优先修复」锚点）：滚动定位 + 短暂高亮。
function scrollToFinding(index: number) {
  // 若被筛选/折叠隐藏，先清除筛选并展开所在分组。
  severityFilter.value = []
  statusFilter.value = ''
  onlyPatchable.value = false
  hideDisposed.value = false
  const entry = props.findings[index]
  const groupKey = groupBy.value === 'none' ? 'all' : (groupBy.value === 'file' ? (entry?.path?.trim() || '（未关联文件）') : entry?.severity)
  if (collapsedGroups.value.has(groupKey)) {
    const next = new Set(collapsedGroups.value)
    next.delete(groupKey)
    collapsedGroups.value = next
  }
  nextTick(() => {
    const el = document.getElementById(`finding-${index}`)
    if (el) {
      el.scrollIntoView({ behavior: 'smooth', block: 'center' })
      highlightIndex.value = index
      if (highlightTimer) clearTimeout(highlightTimer)
      highlightTimer = setTimeout(() => { highlightIndex.value = null }, 2200)
    }
  })
}

defineExpose({ resetPatchState, scrollToFinding })

async function loadFeedback() {
  const items = await getFindingFeedback(props.reviewId)
  feedbackByIndex.value = Object.fromEntries(items.map((item) => [item.finding_index, item]))

  // 跨 review 误报抑制：查询本 review 所有 finding 指纹的历史反馈。
  const fps = props.findings.map((f) => f.fingerprint).filter((fp): fp is string => Boolean(fp))
  if (fps.length) {
    try {
      const historical = await lookupFeedbackByFingerprints(fps)
      const map: Record<string, boolean> = {}
      for (const h of historical) {
        if (h.status === 'false_positive' && h.review_id !== props.reviewId && h.fingerprint) {
          map[h.fingerprint] = true
        }
      }
      historicalFalsePositiveByFp.value = map
    } catch {
      // 历史查询失败不影响主流程，静默降级。
    }
  }
}

async function handleFeedback(index: number, status: string) {
  const fingerprint = props.findings[index]?.fingerprint
  const item = await saveFindingFeedback(props.reviewId, index, { status, fingerprint })
  feedbackByIndex.value = { ...feedbackByIndex.value, [index]: item }
  ElMessage.success(`已标记为${feedbackLabel(status)}`)
}

function feedbackLabel(status?: string) {
  const map: Record<string, string> = {
    false_positive: '误报',
    accepted_risk: '接受风险',
    deferred: '暂不处理',
    fixed: '已修复',
  }
  return status ? map[status] || '未知' : ''
}

function togglePatchFinding(index: number, checked: boolean) {
  const current = new Set(selectedPatchIndexes.value)
  if (checked) current.add(index)
  else current.delete(index)
  selectedPatchIndexes.value = Array.from(current).sort((a, b) => a - b)
  resetPatchPreviewState()
}

function selectPatchableFindings() {
  selectedPatchIndexes.value = patchableFindingIndexes.value
  resetPatchPreviewState()
}

function clearPatchSelection() {
  selectedPatchIndexes.value = []
  resetPatchPreviewState()
}

function resetPatchPreviewState() {
  patchPreview.value = null
  patchCheck.value = null
  patchCheckKey.value = ''
  patchApplyResult.value = null
}

async function handlePreviewPatch() {
  if (!patchSelectionIndexes.value.length) return
  patchLoading.value = true
  try {
    patchPreview.value = await previewSuggestionPatch(props.reviewId, patchSelectionIndexes.value)
    patchCheck.value = null
    patchCheckKey.value = ''
    patchApplyResult.value = null
  } finally {
    patchLoading.value = false
  }
}

async function checkPatch() {
  if (!patchSelectionIndexes.value.length) return
  patchChecking.value = true
  try {
    patchCheck.value = await checkSuggestionPatch(props.reviewId, patchSelectionIndexes.value)
    patchCheckKey.value = patchSelectionKey.value
    if (patchCheck.value.applicable) ElMessage.success('Patch 可应用')
    else ElMessage.warning(patchCheck.value.error || 'Patch 当前不可应用')
  } finally {
    patchChecking.value = false
  }
}

async function applyPatch() {
  if (!canApplyPatch.value) {
    ElMessage.warning('请先完成生成预览 + 检查可应用')
    return
  }
  patchApplying.value = true
  try {
    patchApplyResult.value = await applySuggestionPatch(props.reviewId, {
      indexes: patchSelectionIndexes.value,
      branch: patchBranch.value || undefined,
      push: patchCreateCR.value,
      create_cr: patchCreateCR.value,
      target_branch: patchTargetBranch.value || undefined,
    })
    if (patchApplyResult.value.applied) {
      patchPreview.value = patchApplyResult.value
      patchCheck.value = null
      ElMessage.success(patchApplyResult.value.cr_url ? '修复 PR/MR 已创建' : '修复提交已创建')
    } else {
      ElMessage.warning(patchApplyResult.value.error || 'Patch 当前不可应用')
    }
  } finally {
    patchApplying.value = false
  }
}

async function copyPatch() {
  if (!patchPreview.value?.patch) return
  try {
    await navigator.clipboard.writeText(patchPreview.value.patch)
    ElMessage.success('Patch 已复制')
  } catch {
    ElMessage.error('复制失败，请手动复制预览内容')
  }
}

function resetPatchState() {
  selectedPatchIndexes.value = patchableFindingIndexes.value
  resetPatchPreviewState()
}

onMounted(loadFeedback)
</script>

<style scoped>
.finding-header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

/* 筛选条 */
.findings-filter-bar {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  margin-bottom: 12px;
  padding: 10px 12px;
  border: 1px solid var(--app-border);
  border-radius: 10px;
  background: #fafbfc;
}

.filter-severity {
  width: 170px;
}

.filter-status {
  width: 130px;
}

.filter-spacer {
  flex: 1;
}

.filter-count {
  color: #929aab;
  font-size: 12px;
}

.filter-group {
  flex-shrink: 0;
}

/* Patch 工作流折叠面板 */
.patch-panel-wrap {
  margin-bottom: 16px;
  border: 1px solid var(--app-border);
  border-radius: 10px;
  background: #fff;
}

.patch-panel-wrap[open] > .patch-panel-summary {
  border-bottom: 1px solid var(--app-border);
}

.patch-panel-summary {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 14px;
  cursor: pointer;
  list-style: none;
  user-select: none;
}

.patch-panel-summary::-webkit-details-marker {
  display: none;
}

.patch-panel-summary strong {
  font-size: 13px;
}

.patch-panel-hint {
  color: #929aab;
  font-size: 12px;
}

.suggestion-patch-panel {
  padding: 12px 14px;
}

.patch-steps {
  display: flex;
  flex-wrap: wrap;
  gap: 14px;
  margin: 0 0 12px;
  padding: 0;
  list-style: none;
}

.patch-steps li {
  color: #929aab;
  font-size: 12px;
}

.patch-steps li.active {
  color: #1d4ed8;
  font-weight: 600;
}

.patch-steps li.done {
  color: #16a34a;
  text-decoration: line-through;
}

.patch-summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.patch-actions {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
  align-items: center;
}

.patch-empty-alert {
  margin-top: 10px;
}

.patch-branch-input {
  width: 190px;
}

.patch-result {
  margin-top: 12px;
}

.patch-result-meta {
  display: flex;
  gap: 6px;
  margin-bottom: 8px;
}

.patch-check-output {
  margin: 8px 0;
  padding: 8px;
  background: #fef0cd;
  border-radius: 6px;
  font-size: 12px;
}

.patch-apply-result {
  display: flex;
  flex-direction: column;
  gap: 3px;
  margin: 8px 0;
  padding: 8px;
  color: #14745a;
  background: #e9f8f1;
  border-radius: 6px;
  font-size: 12px;
  overflow-wrap: anywhere;
}

.patch-apply-result.failed {
  color: #b4233c;
  background: #fff0f2;
}

.patch-apply-result a {
  color: inherit;
  font-weight: 700;
}

.patch-skipped {
  display: flex;
  flex-direction: column;
  gap: 2px;
  margin: 8px 0;
  font-size: 12px;
  color: #b88230;
}

.patch-preview {
  max-height: 260px;
  overflow: auto;
  padding: 12px;
  background: #1e1e2e;
  color: #cdd6f4;
  border-radius: 8px;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre;
}

/* 分组 */
.finding-list {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.finding-group {
  border: 1px solid var(--app-border);
  border-radius: 10px;
  background: #fff;
  overflow: hidden;
}

.group-header {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 10px 14px;
  border: 0;
  background: #fafbfc;
  color: var(--app-text);
  text-align: left;
  cursor: pointer;
  font-size: 13px;
}

.group-header:hover {
  background: #f3f5f9;
}

.group-caret {
  transition: transform 160ms ease;
  color: #929aab;
  font-size: 12px;
}

.group-caret.collapsed {
  transform: rotate(0deg);
}

.group-caret:not(.collapsed) {
  transform: rotate(90deg);
}

.group-title {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  font-weight: 600;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.group-count {
  color: #929aab;
  font-size: 12px;
}

.sev-badge {
  padding: 1px 7px;
  border-radius: 5px;
  font-size: 11px;
  font-weight: 600;
}

.sev-badge.critical {
  color: #dc2626;
  background: #fef2f2;
}

.sev-badge.major {
  color: #d97706;
  background: #fffbeb;
}

.sev-badge.minor {
  color: #2563eb;
  background: #eff6ff;
}

.group-body {
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

/* finding-item 的高亮（来自侧栏跳转）透传 */
.group-body :deep(.finding-item.highlight) {
  animation: finding-flash 1.6s ease;
}

@keyframes finding-flash {
  0% { box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.45); }
  100% { box-shadow: 0 0 0 0 rgba(37, 99, 235, 0); }
}
</style>
