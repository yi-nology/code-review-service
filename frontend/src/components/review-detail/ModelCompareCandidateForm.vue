<script setup lang="ts">
/**
 * ModelCompareCandidateForm — 模型 A/B 对比的单个候选输入卡。
 * 默认只暴露「模型名 + 可选 URL」，点「高级」展开 provider / 鉴权 / 自定义请求。
 */
import { ref } from 'vue'
import { Close, ArrowDown, ArrowUp } from '@element-plus/icons-vue'
import type { ModelCompareCandidate } from '@/types/review'

const props = defineProps<{
  candidate: ModelCompareCandidate
  index: number
  removable: boolean
}>()

const emit = defineEmits<{
  (e: 'remove'): void
}>()

const showAdvanced = ref(false)

// 判断是否已填写任意高级字段，用于在折叠时给出提示
const hasAdvancedValue = (): boolean =>
  Boolean(
    props.candidate.provider ||
      props.candidate.auth_token ||
      props.candidate.auth_header ||
      props.candidate.extra_headers ||
      props.candidate.extra_body,
  )
</script>

<template>
  <div class="candidate-form">
    <!-- 泳道徽章 -->
    <div class="lane-badge" :class="`lane-${String.fromCharCode(65 + index)}`">
      {{ String.fromCharCode(65 + index) }}
    </div>

    <div class="candidate-body">
      <!-- 基础字段：模型名 + URL -->
      <div class="field-row">
        <label class="field">
          <span class="field-label">模型</span>
          <el-input
            v-model="candidate.model"
            size="small"
            placeholder="模型名，如 gpt-4o"
          />
        </label>
        <label class="field field-url">
          <span class="field-label">URL</span>
          <el-input
            v-model="candidate.url"
            size="small"
            placeholder="可选，自定义推理地址"
          />
        </label>
      </div>

      <!-- 高级折叠 -->
      <button
        type="button"
        class="advanced-toggle"
        :class="{ active: showAdvanced }"
        @click="showAdvanced = !showAdvanced"
      >
        <el-icon :size="12">
          <ArrowDown v-if="!showAdvanced" />
          <ArrowUp v-else />
        </el-icon>
        高级配置
        <span v-if="!showAdvanced && hasAdvancedValue()" class="advanced-dot" />
      </button>

      <div v-show="showAdvanced" class="advanced-fields">
        <div class="field-row">
          <label class="field">
            <span class="field-label">Provider</span>
            <el-input
              v-model="candidate.provider"
              size="small"
              placeholder="可选，如 openai"
            />
          </label>
          <label class="field">
            <span class="field-label">鉴权头</span>
            <el-input
              v-model="candidate.auth_header"
              size="small"
              placeholder="可选，默认 Authorization"
            />
          </label>
        </div>
        <label class="field field-full">
          <span class="field-label">鉴权 Token</span>
          <el-input
            v-model="candidate.auth_token"
            size="small"
            type="password"
            show-password
            placeholder="可选"
          />
        </label>
        <label class="field field-full">
          <span class="field-label">自定义请求头</span>
          <el-input
            v-model="candidate.extra_headers"
            size="small"
            type="textarea"
            :autosize="{ minRows: 2, maxRows: 4 }"
            placeholder='JSON，如 {"X-Custom":"value"}'
          />
        </label>
        <label class="field field-full">
          <span class="field-label">附加 Body</span>
          <el-input
            v-model="candidate.extra_body"
            size="small"
            type="textarea"
            :autosize="{ minRows: 2, maxRows: 4 }"
            placeholder='JSON，如 {"temperature":0.2}'
          />
        </label>
      </div>
    </div>

    <!-- 删除 -->
    <button
      v-if="removable"
      type="button"
      class="remove-btn"
      aria-label="删除候选"
      @click="emit('remove')"
    >
      <el-icon :size="14"><Close /></el-icon>
    </button>
  </div>
</template>

<style scoped>
.candidate-form {
  display: flex;
  gap: 12px;
  padding: 12px;
  border: 1px solid var(--app-border);
  border-radius: 12px;
  background: #fff;
  position: relative;
}

/* 泳道徽章 —— 复用 PresetSelector 的渐变方块语言 */
.lane-badge {
  width: 32px;
  height: 32px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 700;
  font-size: 15px;
  color: #fff;
  flex-shrink: 0;
}
.lane-A { background: linear-gradient(135deg, #3b82f6, #2563eb); }
.lane-B { background: linear-gradient(135deg, #8b5cf6, #7c3aed); }
.lane-C { background: linear-gradient(135deg, #10b981, #059669); }
.lane-D { background: linear-gradient(135deg, #f59e0b, #d97706); }
.lane-E { background: linear-gradient(135deg, #ec4899, #db2777); }
.lane-F { background: linear-gradient(135deg, #6b7280, #4b5563); }

.candidate-body {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.field-row {
  display: flex;
  gap: 10px;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  flex: 1;
  min-width: 0;
}
.field-url { flex: 1.2; }
.field-full { flex-basis: 100%; }

.field-label {
  font-size: 11px;
  font-weight: 600;
  color: #8892a4;
  text-transform: uppercase;
  letter-spacing: 0.3px;
}

/* 高级折叠 */
.advanced-toggle {
  align-self: flex-start;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 6px;
  border: none;
  background: transparent;
  color: var(--app-muted);
  font-size: 12px;
  cursor: pointer;
  border-radius: 6px;
  transition: background 0.15s;
}
.advanced-toggle:hover { background: var(--app-bg); }
.advanced-toggle.active { color: var(--app-primary); }

.advanced-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--app-primary);
  margin-left: 2px;
}

.advanced-fields {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  padding-top: 4px;
  border-top: 1px dashed var(--app-border);
}

/* 删除按钮 */
.remove-btn {
  position: absolute;
  top: 8px;
  right: 8px;
  width: 22px;
  height: 22px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--app-muted);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.15s;
}
.remove-btn:hover {
  background: #fff0f2;
  color: var(--app-danger);
}

@media (max-width: 720px) {
  .field-row { flex-direction: column; }
}
</style>
