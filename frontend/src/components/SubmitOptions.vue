<template>
  <el-form-item label="提交方式">
    <el-radio-group
      :model-value="mode"
      @update:model-value="$emit('update:mode', $event)"
    >
      <el-radio value="async">
        异步提交
        <span class="radio-hint">提交后立即返回，后台执行</span>
      </el-radio>
      <el-radio value="sync">
        同步等待
        <span class="radio-hint">等待审查完成后跳转详情</span>
      </el-radio>
    </el-radio-group>
  </el-form-item>
  <el-collapse
    :model-value="advanced"
    @update:model-value="$emit('update:advanced', $event)"
  >
    <el-collapse-item name="advanced">
      <template #title>
        <span class="advanced-title">高级选项</span>
        <el-tag
          v-if="requirements.trim() || ephemeralReviewers.length"
          size="small"
          type="success"
          class="adv-badge"
        >
          已配置补充
        </el-tag>
      </template>
      <el-form-item label="审查需求">
        <el-input
          :model-value="requirements"
          type="textarea"
          :rows="3"
          placeholder="逐行填写审查需求，例如：&#10;支持分页查询&#10;所有错误需返回统一格式"
          @update:model-value="$emit('update:requirements', $event)"
        />
        <span class="field-hint">每行一条，会作为本次审查的额外关注点</span>
      </el-form-item>
      <el-form-item label="临时审查员">
        <div class="ephemeral-list">
          <div
            v-for="(rev, idx) in ephemeralReviewers"
            :key="idx"
            class="ephemeral-item"
          >
            <el-input
              :model-value="rev.name"
              placeholder="名称（可选）"
              class="ephemeral-name"
              @update:model-value="updateReviewer(idx, 'name', $event)"
            />
            <el-input
              :model-value="rev.background"
              type="textarea"
              :rows="2"
              placeholder="审查关注背景，例如：重点关注并发安全"
              class="ephemeral-bg"
              @update:model-value="updateReviewer(idx, 'background', $event)"
            />
            <el-button
              :icon="Delete"
              circle
              plain
              @click="$emit('removeReviewer', idx)"
            />
          </div>
          <el-button
            :icon="Plus"
            plain
            size="small"
            @click="$emit('addReviewer')"
          >
            添加临时审查员
          </el-button>
        </div>
      </el-form-item>
      <el-form-item label="幂等键">
        <el-input
          :model-value="idempotencyKey"
          placeholder="pipeline:job:sha (可选)"
          @update:model-value="$emit('update:idempotencyKey', $event)"
        />
      </el-form-item>
      <el-form-item label="回调模式">
        <el-select
          :model-value="callbackMode"
          style="width: 100%"
          @update:model-value="$emit('update:callbackMode', $event)"
        >
          <el-option
            label="默认 (comment)"
            value="comment"
          />
          <el-option
            label="无回调"
            value="none"
          />
        </el-select>
      </el-form-item>
    </el-collapse-item>
  </el-collapse>
</template>

<script setup lang="ts">
import { Delete, Plus } from '@element-plus/icons-vue'
import type { EphemeralReviewer } from '@/types/review'

const props = defineProps<{
  mode: 'async' | 'sync'
  advanced: string[]
  idempotencyKey: string
  callbackMode: string
  requirements: string
  ephemeralReviewers: EphemeralReviewer[]
}>()

const emit = defineEmits<{
  (e: 'update:mode', v: 'async' | 'sync'): void
  (e: 'update:advanced', v: string[]): void
  (e: 'update:idempotencyKey', v: string): void
  (e: 'update:callbackMode', v: string): void
  (e: 'update:requirements', v: string): void
  (e: 'update:ephemeralReviewers', v: EphemeralReviewer[]): void
  (e: 'addReviewer'): void
  (e: 'removeReviewer', idx: number): void
}>()

function updateReviewer(idx: number, field: 'name' | 'background', value: string) {
  const next = props.ephemeralReviewers.map((r, i) => (i === idx ? { ...r, [field]: value } : r))
  emit('update:ephemeralReviewers', next)
}
</script>

<style scoped>
.radio-hint {
  font-size: 12px;
  color: #909399;
  margin-left: 4px;
}
.advanced-title {
  font-size: 14px;
  color: var(--app-primary, #5b5bd6);
}
.adv-badge {
  margin-left: 8px;
}
.field-hint {
  display: block;
  margin-top: 4px;
  color: #909399;
  font-size: 11px;
}
.ephemeral-list {
  width: 100%;
}
.ephemeral-item {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 8px;
  margin-bottom: 8px;
}
.ephemeral-name {
  margin-bottom: 4px;
}
:deep(.el-collapse-item__header) {
  font-size: 14px;
}
:deep(.el-collapse-item__content) {
  padding-bottom: 0;
}
</style>
