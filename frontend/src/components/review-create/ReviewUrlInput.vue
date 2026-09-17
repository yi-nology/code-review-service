<template>
  <section class="create-panel">
    <div class="panel-heading">
      <div>
        <h3>粘贴链接</h3>
        <p>支持 PR/MR/CR 链接和 compare 分支对比链接，粘贴后自动识别平台、仓库、编号或对比分支。</p>
      </div>
      <el-button :icon="DocumentCopy" @click="pasteFromClipboard">读取剪贴板</el-button>
    </div>

    <el-input
      ref="urlInputRef"
      :model-value="urlInput"
      class="url-box"
      type="textarea"
      :rows="3"
      placeholder="例如：https://github.com/owner/repo/pull/42 或 https://github.com/owner/repo/compare/main...feature"
      clearable
      @update:model-value="$emit('update:urlInput', $event)"
      @input="$emit('url-input')"
      @paste="$emit('url-paste')"
    />

    <div v-if="parsedUrl" class="parse-card success">
      <div class="parse-main">
        <el-icon><Select /></el-icon>
        <div>
          <strong>已识别 {{ getProviderLabel(parsedUrl.provider) }} 审查请求</strong>
          <span>{{ parsedUrl.repo }} · #{{ parsedUrl.number }}</span>
        </div>
      </div>
      <div class="parse-actions">
        <el-button text size="small" @click="$emit('apply-url')">从 PR 列表选择</el-button>
        <el-button type="primary" :disabled="submitDisabled" @click="$emit('submit')">
          立即提交
        </el-button>
      </div>
    </div>
    <div v-else-if="parsedCompareUrl" class="parse-card success">
      <div class="parse-main">
        <el-icon><Select /></el-icon>
        <div>
          <strong>已识别 {{ getProviderLabel(parsedCompareUrl.provider) }} 分支对比</strong>
          <span>{{ parsedCompareUrl.repo }} · {{ parsedCompareUrl.head_branch }} → {{ parsedCompareUrl.base_branch }}</span>
        </div>
      </div>
      <div class="parse-actions">
        <el-button text size="small" @click="$emit('apply-compare-url')">查看分支对比</el-button>
        <el-button type="primary" :disabled="submitDisabled" @click="$emit('submit')">
          立即提交
        </el-button>
      </div>
    </div>
    <div v-else-if="urlInput.trim()" class="parse-card danger">
      <div class="parse-main">
        <el-icon><Warning /></el-icon>
        <div>
          <strong>暂时无法解析这个链接</strong>
          <span>请确认链接包含仓库路径、PR/MR/CR 编号，或 compare 的 base...head 分支。</span>
        </div>
      </div>
    </div>

    <slot />
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { DocumentCopy, Select, Warning } from '@element-plus/icons-vue'
import { getProviderLabel } from '@/utils/url-parser'

const props = defineProps<{
  urlInput: string
  parsedUrl: { provider: string; repo: string; number: number } | null
  parsedCompareUrl: { provider: string; repo: string; base_branch: string; head_branch: string } | null
  /** 当前不可提交（如 Argus + 分支对比）时置灰「立即提交」 */
  submitDisabled?: boolean
}>()

const emit = defineEmits<{
  'update:urlInput': [value: string]
  'url-input': []
  'url-paste': []
  'apply-url': []
  'apply-compare-url': []
  submit: []
}>()

const urlInputRef = ref<{ focus: () => void } | null>(null)

// 弹窗打开时聚焦输入框，粘贴动作少一步点击；等 dialog 动画后再聚焦
onMounted(() => {
  setTimeout(() => urlInputRef.value?.focus(), 260)
})

async function pasteFromClipboard() {
  try {
    const text = await navigator.clipboard.readText()
    emit('update:urlInput', text)
    emit('url-input')
    urlInputRef.value?.focus()
  } catch {
    // Silently fail - user can paste manually
  }
}
</script>

<style scoped>
.url-box {
  margin-bottom: 12px;
}

.parse-card {
  align-items: center;
  border-radius: 8px;
  display: flex;
  gap: 12px;
  justify-content: space-between;
  margin: 12px 0 16px;
  padding: 13px 14px;
}

.parse-card.success {
  background: #f0f9eb;
  border: 1px solid #d8efcd;
}

.parse-card.danger {
  background: #fef0f0;
  border: 1px solid #f6d5d5;
}

.parse-main {
  align-items: center;
  display: flex;
  gap: 10px;
  min-width: 0;
}

.parse-actions {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 8px;
}

.parse-main .el-icon {
  flex: 0 0 auto;
  font-size: 20px;
}

.parse-main strong,
.parse-main span {
  display: block;
}

.parse-main strong {
  color: #20242a;
  font-size: 14px;
}

.parse-main span {
  color: #707782;
  font-size: 12px;
  margin-top: 2px;
}

@media (max-width: 760px) {
  .parse-card {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
