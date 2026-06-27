<template>
  <el-card class="model-compare-card">
    <template #header>
      <div class="card-header">
        <span>模型 A/B</span>
        <el-button size="small" type="primary" plain :loading="loading" :disabled="!validCandidates.length" @click="runCompare">
          开始对比
        </el-button>
      </div>
    </template>

    <div class="candidate-grid">
      <div v-for="(candidate, index) in candidates" :key="index" class="candidate-row">
        <el-input v-model="candidate.label" size="small" placeholder="标签" />
        <el-input v-model="candidate.model" size="small" placeholder="模型名" />
        <el-input v-model="candidate.url" size="small" placeholder="可选 URL" />
      </div>
    </div>

    <el-table v-if="result?.candidates?.length" :data="result.candidates" size="small" class="compare-table">
      <el-table-column label="候选" min-width="130">
        <template #default="{ row }">
          <strong>{{ row.label || row.model }}</strong>
          <small>{{ row.model }}</small>
        </template>
      </el-table-column>
      <el-table-column label="质量" width="90" align="center">
        <template #default="{ row }">
          <span v-if="row.success">{{ row.score ?? 0 }} / {{ row.grade || '-' }}</span>
          <span v-else class="error-text">失败</span>
        </template>
      </el-table-column>
      <el-table-column label="问题" width="70" prop="findings" align="center" />
      <el-table-column label="Tokens" width="100" align="center">
        <template #default="{ row }">{{ formatNumber(row.total_tokens) }}</template>
      </el-table-column>
      <el-table-column label="耗时" width="90" align="center">
        <template #default="{ row }">{{ Math.round((row.duration_ms || 0) / 1000) }}s</template>
      </el-table-column>
    </el-table>

    <div v-if="failedRows.length" class="compare-errors">
      <span v-for="row in failedRows" :key="row.label || row.model">{{ row.label || row.model }}：{{ row.error }}</span>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { compareReviewModels } from '@/api/review'
import type { ModelCompareCandidate, ModelCompareResult } from '@/types/review'
import { ElMessage } from 'element-plus/es/components/message/index'

const props = defineProps<{ reviewId: string }>()

const candidates = reactive<ModelCompareCandidate[]>([
  { label: 'A', model: '' },
  { label: 'B', model: '' },
])
const result = ref<ModelCompareResult | null>(null)
const loading = ref(false)

const validCandidates = computed(() => candidates.filter((item) => item.model.trim()))
const failedRows = computed(() => result.value?.candidates?.filter((item) => !item.success && item.error) || [])

function formatNumber(value?: number) {
  return value ? value.toLocaleString() : '0'
}

async function runCompare() {
  if (!validCandidates.value.length) return
  loading.value = true
  try {
    result.value = await compareReviewModels(props.reviewId, validCandidates.value)
    ElMessage.success('模型对比完成')
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.candidate-grid {
  display: grid;
  gap: 8px;
}

.candidate-row {
  display: grid;
  grid-template-columns: 80px 1fr 1fr;
  gap: 8px;
}

.compare-table {
  margin-top: 12px;
}

.compare-table strong,
.compare-table small {
  display: block;
}

.compare-table small {
  color: var(--app-muted);
  font-size: 11px;
}

.compare-errors {
  display: grid;
  gap: 4px;
  margin-top: 10px;
  color: #b4233c;
  font-size: 12px;
}

.error-text {
  color: #b4233c;
}

@media (max-width: 720px) {
  .candidate-row {
    grid-template-columns: 1fr;
  }
}
</style>
