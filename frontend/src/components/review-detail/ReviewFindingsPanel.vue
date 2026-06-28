<template>
  <el-card class="findings-card" v-if="findings?.length">
    <template #header>
      <div class="card-header">
        <span>问题报告</span>
        <div class="finding-header-actions">
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
        </div>
      </div>
    </template>
    <div class="suggestion-patch-panel">
      <div class="patch-summary">
        <div>
          <strong>修复 Patch</strong>
          <span>已选 {{ selectedPatchIndexes.length }} / 可生成 {{ patchableFindingIndexes.length }}</span>
        </div>
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
            生成预览
          </el-button>
          <el-button size="small" plain :disabled="!patchPreview?.patch" :loading="patchChecking" @click="checkPatch">
            检查可应用
          </el-button>
          <el-button size="small" plain :disabled="!patchPreview?.patch" @click="copyPatch">复制</el-button>
          <el-button size="small" plain :disabled="!patchPreview?.patch" :loading="patchDownloading" @click="$emit('download-patch', patchSelectionIndexes)">
            下载 .diff
          </el-button>
          <el-button size="small" type="success" plain :disabled="!canApplyPatch" :loading="patchApplying" @click="applyPatch">
            创建修复提交
          </el-button>
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
    <div class="finding-list">
      <div v-for="(finding, index) in findings" :key="index" class="finding-item">
        <div class="finding-header">
          <el-tooltip :disabled="isPatchableFinding(finding)" :content="patchBlockReason(finding)" placement="top">
            <el-checkbox
              class="finding-patch-checkbox"
              :model-value="selectedPatchIndexes.includes(index)"
              :disabled="!isPatchableFinding(finding)"
              @change="(checked: boolean | string | number) => togglePatchFinding(index, Boolean(checked))"
            />
          </el-tooltip>
          <el-tag :type="getSeverityType(finding.severity)" size="small">
            {{ finding.severity.toUpperCase() }}
          </el-tag>
          <span class="finding-title">{{ finding.title }}</span>
          <span class="finding-path" v-if="finding.path">
            {{ finding.path }}{{ finding.line ? `:${finding.line}` : '' }}
          </span>
          <el-tag v-if="!isPatchableFinding(finding)" size="small" type="info">
            {{ patchBlockReason(finding) }}
          </el-tag>
          <el-dropdown trigger="click" @command="(command: string) => handleFeedback(index, command)">
            <el-button size="small" plain>
              {{ feedbackLabel(feedbackByIndex[index]?.status) || '标记' }}
            </el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="false_positive">误报</el-dropdown-item>
                <el-dropdown-item command="accepted_risk">接受风险</el-dropdown-item>
                <el-dropdown-item command="deferred">暂不处理</el-dropdown-item>
                <el-dropdown-item command="fixed">已修复</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
          <el-tag v-if="feedbackByIndex[index]" size="small" type="info">
            {{ feedbackLabel(feedbackByIndex[index].status) }}
          </el-tag>
        </div>
        <div class="finding-body" v-if="finding.body">{{ finding.body }}</div>
        <div class="finding-code" v-if="finding.existing_code">
          <div class="code-label">现有代码</div>
          <pre>{{ finding.existing_code }}</pre>
        </div>
        <div class="finding-code" v-if="finding.suggestion_code">
          <div class="code-label">建议代码</div>
          <pre>{{ finding.suggestion_code }}</pre>
        </div>
      </div>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { getSeverityType } from '@/utils/status'
import { previewSuggestionPatch, checkSuggestionPatch, applySuggestionPatch, getFindingFeedback, saveFindingFeedback } from '@/api/review'
import type { Finding, FindingFeedback, SuggestionPatchApplyResult, SuggestionPatchCheck, SuggestionPatchPreview } from '@/types/review'
import { ElMessage } from 'element-plus/es/components/message/index'

const props = defineProps<{
  reviewId: string
  findings: Finding[]
}>()

const emit = defineEmits<{
  'download-patch': [indexes: number[]]
}>()

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

const patchableFindingIndexes = computed(() => {
  return props.findings
    .map((finding, index) => ({ finding, index }))
    .filter(({ finding }) => isPatchableFinding(finding))
    .map(({ index }) => index)
})

const patchSelectionIndexes = computed(() => {
  const allowed = new Set(patchableFindingIndexes.value)
  return selectedPatchIndexes.value.filter((index) => allowed.has(index))
})

const patchSelectionKey = computed(() => patchSelectionIndexes.value.join(','))
const canApplyPatch = computed(() => {
  return Boolean(patchSelectionIndexes.value.length && patchCheck.value?.applicable && patchCheckKey.value === patchSelectionKey.value)
})

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

function feedbackLabel(status?: string) {
  const map: Record<string, string> = {
    false_positive: '误报',
    accepted_risk: '接受风险',
    deferred: '暂不处理',
    fixed: '已修复',
  }
  return status ? map[status] || status : ''
}

async function loadFeedback() {
  const items = await getFindingFeedback(props.reviewId)
  feedbackByIndex.value = Object.fromEntries(items.map((item) => [item.finding_index, item]))
}

async function handleFeedback(index: number, status: string) {
  const item = await saveFindingFeedback(props.reviewId, index, { status })
  feedbackByIndex.value = { ...feedbackByIndex.value, [index]: item }
  ElMessage.success(`已标记为${feedbackLabel(status)}`)
}

function togglePatchFinding(index: number, checked: boolean) {
  const current = new Set(selectedPatchIndexes.value)
  if (checked) {
    current.add(index)
  } else {
    current.delete(index)
  }
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
    if (patchCheck.value.applicable) {
      ElMessage.success('Patch 可应用')
    } else {
      ElMessage.warning(patchCheck.value.error || 'Patch 当前不可应用')
    }
  } finally {
    patchChecking.value = false
  }
}

async function applyPatch() {
  if (!canApplyPatch.value) {
    ElMessage.warning('请先通过当前选择的 Patch 可应用检查')
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

defineExpose({ resetPatchState })

onMounted(loadFeedback)
</script>

<style scoped>
.finding-header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.suggestion-patch-panel {
  margin-bottom: 16px;
  padding: 12px;
  background: var(--app-surface);
  border: 1px solid var(--app-border);
  border-radius: 10px;
}

.patch-summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.patch-summary strong {
  font-size: 13px;
}

.patch-summary > div:first-child {
  display: flex;
  align-items: center;
  gap: 8px;
}

.patch-actions {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
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

.finding-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.finding-item {
  padding: 14px;
  background: var(--app-surface);
  border: 1px solid var(--app-border);
  border-radius: 10px;
}

.finding-header {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 8px;
}

.finding-patch-checkbox {
  margin-right: 4px;
}

.finding-title {
  font-weight: 600;
  font-size: 14px;
}

.finding-path {
  color: #8892a4;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
}

.finding-body {
  margin-bottom: 10px;
  color: #5d6677;
  font-size: 13px;
  line-height: 1.6;
}

.finding-code {
  margin-top: 8px;
}

.code-label {
  margin-bottom: 4px;
  color: #8892a4;
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.finding-code pre {
  padding: 10px;
  background: #f6f8fa;
  border-radius: 6px;
  font-size: 12px;
  line-height: 1.5;
  overflow-x: auto;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
