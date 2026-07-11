<script setup lang="ts">
/**
 * ModelComparePanel — 评审详情页的模型 A/B 对比面板。
 *
 * 布局：头部（标题 + 开始对比）→ 候选输入卡列表（可增删）→ [结果区]
 *       汇总条（胜出者 + 速度差）→ 结果卡网格（评分环 + 可展开明细）。
 *
 * 设计上贴合 review-detail 最近的重构风格：自定义卡片、评分环、
 * lane 徽章，并把输入和结果拆到独立子组件。
 */
import { computed, reactive, ref, onMounted } from 'vue'
import { Plus } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import { compareReviewModels } from '@/api/review'
import { listCustomProviders } from '@/api/config'
import type { ModelCompareCandidate, ModelCompareResult, ModelCompareCandidateRun } from '@/types/review'
import type { CustomProvider } from '@/types/config'
import ModelCompareCandidateForm from './ModelCompareCandidateForm.vue'
import ModelCompareResultCard from './ModelCompareResultCard.vue'

const props = defineProps<{ reviewId: string }>()

// 模型预设库（供候选下拉引用）
const presets = ref<CustomProvider[]>([])

// 候选输入：默认 A/B 两行，可增删。label 由 index 推导，无需用户手填。
const candidates = reactive<ModelCompareCandidate[]>([
  { label: 'A', model: '' },
  { label: 'B', model: '' },
])

const result = ref<ModelCompareResult | null>(null)
const loading = ref(false)
// 记录展开明细的结果 key（label 或 model）
const expandedKeys = ref<Set<string>>(new Set())

const validCandidates = computed(() =>
  candidates.filter((item) => item.model.trim()).map((item, index) => ({ ...item, label: String.fromCharCode(65 + index) })),
)

// 仅当有效候选时，所有候选都可删除（保证至少能删到 1 个）
const canRemove = computed(() => candidates.length > 1)

function addCandidate() {
  candidates.push({ label: String.fromCharCode(65 + candidates.length), model: '' })
}

function removeCandidate(index: number) {
  candidates.splice(index, 1)
}

// —— 结果分析 ——
const runs = computed<ModelCompareCandidateRun[]>(() => result.value?.candidates || [])
const successfulRuns = computed(() => runs.value.filter((item) => item.success))

// 质量最高（且唯一最高分才算胜出者，并列则不标记）
const winner = computed<ModelCompareCandidateRun | null>(() => {
  if (successfulRuns.value.length < 2) return null
  const top = [...successfulRuns.value].sort((a, b) => (b.score ?? 0) - (a.score ?? 0))
  return (top[0]?.score ?? 0) > (top[1]?.score ?? 0) ? top[0] : null
})

// 最快（最小 duration_ms）
const fastest = computed<ModelCompareCandidateRun | null>(() => {
  if (successfulRuns.value.length < 2) return null
  return [...successfulRuns.value].sort((a, b) => (a.duration_ms || 0) - (b.duration_ms || 0))[0]
})

// 汇总条仅在 ≥2 个成功候选时展示
const showSummary = computed(() => successfulRuns.value.length >= 2)

function isWinner(run: ModelCompareCandidateRun) {
  return Boolean(winner.value && winner.value.label === run.label && winner.value.model === run.model)
}
function isFastest(run: ModelCompareCandidateRun) {
  return Boolean(fastest.value && fastest.value.label === run.label && fastest.value.model === run.model)
}

const scoreGap = computed(() => {
  if (!winner.value) return 0
  const others = successfulRuns.value.filter((item) => !(item.label === winner.value!.label && item.model === winner.value!.model))
  const maxOther = Math.max(...others.map((item) => item.score ?? 0))
  return (winner.value.score ?? 0) - maxOther
})

const speedGap = computed(() => {
  if (!fastest.value) return 0
  const others = successfulRuns.value.filter((item) => !(item.label === fastest.value!.label && item.model === fastest.value!.model))
  const minOther = Math.min(...others.map((item) => item.duration_ms || 0))
  return Math.round(((minOther - (fastest.value.duration_ms || 0))) / 1000)
})

function runKey(run: ModelCompareCandidateRun) {
  return run.label || run.model
}

// 结果在 result.candidates 中的序号，用于结果卡 lane 徽章
function runIndex(run: ModelCompareCandidateRun) {
  return runs.value.findIndex((item) => item.label === run.label && item.model === run.model)
}

function isExpanded(run: ModelCompareCandidateRun) {
  return expandedKeys.value.has(runKey(run))
}

function toggleExpand(run: ModelCompareCandidateRun) {
  const key = runKey(run)
  const next = new Set(expandedKeys.value)
  if (next.has(key)) next.delete(key)
  else next.add(key)
  expandedKeys.value = next
}

async function runCompare() {
  if (!validCandidates.value.length) return
  loading.value = true
  try {
    result.value = await compareReviewModels(props.reviewId, validCandidates.value)
    expandedKeys.value = new Set()
    ElMessage.success('模型对比完成')
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  try {
    presets.value = await listCustomProviders()
  } catch {
    // 预设库加载失败不阻塞对比（手填仍可用），全局拦截器会提示
  }
})
</script>

<template>
  <div class="compare-panel">
    <!-- 头部 -->
    <div class="panel-header">
      <div class="header-left">
        <h3 class="panel-title">模型 A/B</h3>
        <span class="panel-desc">对比不同模型在本次评审上的质量与开销</span>
      </div>
      <el-button
        size="small"
        type="primary"
        plain
        :loading="loading"
        :disabled="!validCandidates.length"
        @click="runCompare"
      >
        开始对比
      </el-button>
    </div>

    <!-- 候选输入区 -->
    <div class="candidate-list">
      <ModelCompareCandidateForm
        v-for="(candidate, index) in candidates"
        :key="index"
        :candidate="candidate"
        :index="index"
        :presets="presets"
        :removable="canRemove"
        @remove="removeCandidate(index)"
      />
    </div>

    <button type="button" class="add-btn" @click="addCandidate">
      <el-icon :size="14"><Plus /></el-icon>
      添加候选
    </button>

    <!-- 结果区 -->
    <template v-if="runs.length">
      <!-- 汇总条 -->
      <div v-if="showSummary" class="summary-bar">
        <template v-if="winner">
          <span class="summary-item">
            <span class="summary-lane" :class="`lane-${winner.label}`">{{ winner.label }}</span>
            质量最高
            <strong>{{ winner.score }}</strong>
            <span v-if="scoreGap > 0" class="summary-delta">+{{ scoreGap }} 分</span>
          </span>
          <span class="summary-sep">·</span>
        </template>
        <template v-if="fastest">
          <span class="summary-item">
            <span class="summary-lane" :class="`lane-${fastest.label}`">{{ fastest.label }}</span>
            最快
            <span v-if="speedGap > 0" class="summary-delta">快 {{ speedGap }}s</span>
          </span>
        </template>
      </div>

      <!-- 结果卡网格 -->
      <div class="result-grid">
        <ModelCompareResultCard
          v-for="run in runs"
          :key="runKey(run)"
          :run="run"
          :index="runIndex(run)"
          :is-winner="isWinner(run)"
          :is-fastest="isFastest(run)"
          :expanded="isExpanded(run)"
          @toggle="toggleExpand(run)"
        />
      </div>
    </template>
  </div>
</template>

<style scoped>
.compare-panel {
  border: 1px solid var(--app-border);
  border-radius: 14px;
  background: #fff;
  padding: 18px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

/* 头部 */
.panel-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}
.header-left {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.panel-title {
  margin: 0;
  font-size: 16px;
  font-weight: 700;
  color: var(--app-text);
}
.panel-desc {
  font-size: 12px;
  color: var(--app-muted);
}

/* 候选列表 */
.candidate-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

/* 添加按钮 */
.add-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 10px;
  border: 1px dashed var(--app-border);
  border-radius: 12px;
  background: transparent;
  color: var(--app-muted);
  font-size: 13px;
  cursor: pointer;
  transition: all 0.15s;
}
.add-btn:hover {
  border-color: var(--app-primary);
  color: var(--app-primary);
  background: #eff6ff;
}

/* 汇总条 */
.summary-bar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  background: #f0fdf4;
  border: 1px solid #bbf7d0;
  border-radius: 10px;
  font-size: 13px;
  color: #15803d;
}
.summary-item {
  display: inline-flex;
  align-items: center;
  gap: 5px;
}
.summary-item strong {
  font-size: 15px;
  font-weight: 700;
}
.summary-delta {
  padding: 1px 6px;
  background: #fff;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 600;
}
.summary-sep {
  color: #bbf7d0;
}

/* lane 徽章（汇总条用的小尺寸） */
.summary-lane {
  width: 18px;
  height: 18px;
  border-radius: 5px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 11px;
  font-weight: 700;
  color: #fff;
}
.summary-lane.lane-A { background: linear-gradient(135deg, #3b82f6, #2563eb); }
.summary-lane.lane-B { background: linear-gradient(135deg, #8b5cf6, #7c3aed); }
.summary-lane.lane-C { background: linear-gradient(135deg, #10b981, #059669); }
.summary-lane.lane-D { background: linear-gradient(135deg, #f59e0b, #d97706); }
.summary-lane.lane-E { background: linear-gradient(135deg, #ec4899, #db2777); }
.summary-lane.lane-F { background: linear-gradient(135deg, #6b7280, #4b5563); }

/* 结果网格 */
.result-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 12px;
}

@media (max-width: 720px) {
  .result-grid {
    grid-template-columns: 1fr;
  }
}
</style>
