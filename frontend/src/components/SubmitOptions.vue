<template>
  <el-form-item label="提交方式">
    <el-radio-group :model-value="mode" @update:model-value="$emit('update:mode', $event)">
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
  <el-collapse :model-value="advanced" @update:model-value="$emit('update:advanced', $event)">
    <el-collapse-item title="高级选项" name="advanced">
      <el-form-item label="幂等键">
        <el-input :model-value="idempotencyKey" @update:model-value="$emit('update:idempotencyKey', $event)" placeholder="pipeline:job:sha (可选)" />
      </el-form-item>
      <el-form-item label="回调模式">
        <el-select :model-value="callbackMode" @update:model-value="$emit('update:callbackMode', $event)" style="width: 100%">
          <el-option label="默认 (comment)" value="comment" />
          <el-option label="无回调" value="none" />
        </el-select>
      </el-form-item>
    </el-collapse-item>
  </el-collapse>
</template>

<script setup lang="ts">
defineProps<{
  mode: 'async' | 'sync'
  advanced: string[]
  idempotencyKey: string
  callbackMode: string
}>()

defineEmits<{
  (e: 'update:mode', v: 'async' | 'sync'): void
  (e: 'update:advanced', v: string[]): void
  (e: 'update:idempotencyKey', v: string): void
  (e: 'update:callbackMode', v: string): void
}>()
</script>

<style scoped>
.radio-hint {
  font-size: 12px;
  color: #909399;
  margin-left: 4px;
}
:deep(.el-collapse-item__header) {
  font-size: 14px;
  color: #409eff;
}
:deep(.el-collapse-item__content) {
  padding-bottom: 0;
}
</style>
