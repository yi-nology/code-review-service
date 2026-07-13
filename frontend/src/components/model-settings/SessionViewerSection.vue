<script setup lang="ts">
/**
 * SessionViewerSection — OCR Session Viewer 启停/状态/嵌入。
 * 从 ExecutionParamsTab 抽出，完全自包含。
 */
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import { useAuthStore } from '@/stores/auth'
import { getViewerStatus, startViewer, stopViewer } from '@/api/config'
import type { ViewerStatus } from '@/types/config'

const auth = useAuthStore()
const viewerStatus = ref<ViewerStatus | null>(null)
const viewerLoading = ref(false)
const embedViewer = ref(false)

const viewerProxyUrl = computed(() => {
  let url = '/api/v1/settings/ocr/viewer/proxy/'
  if (auth.token) url += `?token=${encodeURIComponent(auth.token)}`
  return url
})

async function refresh() {
  try { viewerStatus.value = await getViewerStatus() } catch { viewerStatus.value = null }
}

async function handleStart() {
  viewerLoading.value = true
  try {
    const status = await startViewer()
    viewerStatus.value = status
    if (!status.running) ElMessage.error(status.message || '启动失败')
    else ElMessage.success('Viewer 已启动')
  } catch (e: any) { ElMessage.error(e?.message || '启动失败')
  } finally { viewerLoading.value = false }
}

async function handleStop() {
  viewerLoading.value = true
  try {
    await stopViewer()
    viewerStatus.value = await getViewerStatus()
    embedViewer.value = false
    ElMessage.success('Viewer 已停止')
  } catch (e: any) { ElMessage.error(e?.message || '停止失败')
  } finally { viewerLoading.value = false }
}

onMounted(refresh)
</script>

<template>
  <div class="viewer-section">
    <div class="viewer-status-row">
      <el-tag :type="viewerStatus?.running ? 'success' : 'info'">
        {{ viewerStatus?.running ? '运行中' : '未运行' }}
      </el-tag>
      <span v-if="viewerStatus?.addr" class="viewer-addr">{{ viewerStatus.addr }}</span>
      <div class="viewer-actions">
        <el-button v-if="!viewerStatus?.running" type="success" size="small" :loading="viewerLoading" @click="handleStart">启动</el-button>
        <el-button v-else type="danger" size="small" :loading="viewerLoading" @click="handleStop">停止</el-button>
        <el-button size="small" :loading="viewerLoading" @click="refresh">刷新</el-button>
        <el-button v-if="viewerStatus?.running && viewerStatus?.addr" type="primary" size="small" @click="embedViewer = !embedViewer">
          {{ embedViewer ? '收起' : '嵌入' }}
        </el-button>
      </div>
    </div>
    <div v-if="embedViewer && viewerStatus?.running" class="viewer-frame">
      <iframe title="Session Viewer" :src="viewerProxyUrl" />
    </div>
  </div>
</template>

<style scoped>
.viewer-status-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}
.viewer-addr { font-size: 12px; color: #909399; font-family: monospace; }
.viewer-actions { display: flex; gap: 6px; margin-left: auto; }
.viewer-frame { margin-top: 12px; }
.viewer-frame iframe { width: 100%; height: 480px; border: 1px solid var(--app-border); border-radius: 8px; }
</style>
