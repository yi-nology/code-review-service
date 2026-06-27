<template>
  <div class="debug-panels">
    <el-card class="debug-card" v-if="debugMode && rawOcrInput">
      <template #header>
        <div class="card-header">
          <span>
            <el-tag type="warning" size="small" style="margin-right: 8px;">DEBUG</el-tag>
            OCR 原始输入
          </span>
          <el-button size="small" @click="showOcrInput = !showOcrInput">
            {{ showOcrInput ? '收起' : '展开' }}
          </el-button>
        </div>
      </template>
      <el-collapse v-model="ocrInputActive">
        <el-collapse-item title="查看完整 JSON" name="input">
          <pre class="debug-json">{{ formatJson(rawOcrInput) }}</pre>
        </el-collapse-item>
      </el-collapse>
    </el-card>

    <el-card class="debug-card" v-if="debugMode && rawOcr">
      <template #header>
        <div class="card-header">
          <span>
            <el-tag type="warning" size="small" style="margin-right: 8px;">DEBUG</el-tag>
            OCR 原始输出
          </span>
          <el-button size="small" @click="showOcrOutput = !showOcrOutput">
            {{ showOcrOutput ? '收起' : '展开' }}
          </el-button>
        </div>
      </template>
      <el-collapse v-model="ocrOutputActive">
        <el-collapse-item title="查看完整 JSON" name="output">
          <pre class="debug-json">{{ formatJson(rawOcr) }}</pre>
        </el-collapse-item>
      </el-collapse>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'

defineProps<{
  debugMode: boolean
  rawOcrInput?: unknown
  rawOcr?: unknown
}>()

const showOcrInput = ref(false)
const showOcrOutput = ref(false)
const ocrInputActive = ref<string[]>([])
const ocrOutputActive = ref<string[]>([])

function formatJson(data: unknown): string {
  if (!data) return ''
  try {
    if (typeof data === 'string') {
      return JSON.stringify(JSON.parse(data), null, 2)
    }
    return JSON.stringify(data, null, 2)
  } catch {
    return String(data)
  }
}
</script>

<style scoped>
.debug-panels {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.debug-json {
  max-height: 400px;
  overflow: auto;
  padding: 12px;
  background: #1e1e2e;
  color: #cdd6f4;
  border-radius: 8px;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
