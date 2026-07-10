<template>
  <AppLayout>
    <div class="ms-page">
      <header class="ms-header">
        <div>
          <div class="eyebrow">OCR SETTINGS</div>
          <h1>OCR 设置</h1>
          <p>配置代码审查引擎的执行参数、工具、遥测与版本升级</p>
        </div>
        <div class="ms-actions">
          <el-button @click="fetchSettings" :loading="loading">
            <el-icon><Refresh /></el-icon>
            <span>刷新</span>
          </el-button>
        </div>
      </header>

      <!-- 版本与升级 -->
      <section class="version-card">
        <div class="vc-main">
          <div class="vc-icon"><el-icon><Cpu /></el-icon></div>
          <div class="vc-info">
            <div class="vc-title">
              <strong>OCR 引擎</strong>
              <el-tag v-if="version" size="small" effect="plain" type="info">{{ version.version }}</el-tag>
              <el-tag v-if="version?.platform" size="small" effect="plain">{{ version.platform }}</el-tag>
              <el-tag v-if="version?.commit" size="small" effect="plain" class="mono">{{ version.commit.slice(0, 8) }}</el-tag>
            </div>
            <div class="vc-meta">
              <span v-if="version?.build_time">构建于 {{ formatTime(version.build_time) }}</span>
              <span v-if="updateInfo" class="sep">·</span>
              <span v-if="updateInfo?.update_available" class="vc-up">
                发现新版本 {{ updateInfo.latest_version }}
              </span>
              <span v-else-if="updateInfo && !updateInfo.update_available" class="vc-ok">
                已是最新版本
              </span>
            </div>
          </div>
        </div>
        <div class="vc-actions">
          <el-button @click="checkUpdate" :loading="checkingUpdate">
            <el-icon><RefreshRight /></el-icon>
            <span>检查更新</span>
          </el-button>
          <el-button
            v-if="updateInfo?.update_available"
            type="primary"
            :disabled="upgrading"
            @click="startUpgrade"
          >
            <el-icon><Upload /></el-icon>
            <span>升级到 {{ updateInfo.latest_version }}</span>
          </el-button>
          <el-button
            v-if="upgradeStatus && upgradeStatus.state !== 'idle' && upgradeStatus.state !== 'done'"
            type="danger"
            plain
            :loading="upgrading"
            @click="refreshUpgradeStatus"
          >
            刷新进度
          </el-button>
        </div>

        <!-- 升级进度 -->
        <div v-if="upgradeStatus && upgradeStatus.state !== 'idle'" class="vc-progress">
          <div class="vc-progress-head">
            <el-tag :type="upgradeTagType" size="small">{{ upgradeStateLabel }}</el-tag>
            <span v-if="upgradeStatus.message" class="vc-progress-msg">{{ upgradeStatus.message }}</span>
            <span v-if="upgradeStatus.version" class="mono">{{ upgradeStatus.version }}</span>
          </div>
          <el-progress
            v-if="typeof upgradeStatus.progress === 'number'"
            :percentage="upgradeStatus.progress"
            :status="upgradeStatus.state === 'done' ? 'success' : upgradeStatus.state === 'failed' ? 'exception' : undefined"
          />
          <div v-if="updateInfo?.release_notes" class="vc-notes">
            <strong>更新说明</strong>
            <pre>{{ updateInfo.release_notes }}</pre>
          </div>
          <el-button
            v-if="updateInfo?.release_url"
            text
            type="primary"
            @click="openReleaseUrl"
          >
            查看完整发布说明
          </el-button>
        </div>
      </section>

      <!-- 配置卡片 -->
      <section class="config-card">
        <el-tabs v-model="activeTab" class="ocr-tabs">
          <el-tab-pane label="执行参数" name="execution">
            <ExecutionParamsTab />
          </el-tab-pane>
          <el-tab-pane label="审查背景" name="review">
            <ReviewFocusTab />
          </el-tab-pane>
          <el-tab-pane label="MCP 服务器" name="mcp">
            <McpServersTab />
          </el-tab-pane>
        </el-tabs>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, provide, onMounted, onBeforeUnmount } from 'vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import { Refresh, RefreshRight, Upload, Cpu } from '@element-plus/icons-vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import ExecutionParamsTab from '@/components/model-settings/ExecutionParamsTab.vue'
import ReviewFocusTab from '@/components/model-settings/ReviewFocusTab.vue'
import McpServersTab from '@/components/model-settings/McpServersTab.vue'
import { settingsFormKey, type SettingsFormContext } from '@/components/model-settings/context'
import { useOcrSettingsForm } from '@/composables/useOcrSettingsForm'
import {
  checkOCRUpdate,
  getOCRVersion,
  getOCRUpgradeStatus,
  startOCRUpgrade,
} from '@/api/config'
import type { OCRUpdateInfo, OCRUpgradeStatus, OCRVersion } from '@/types/config'

const {
  form,
  loading,
  saving,
  testingLLM,
  extraBodyText,
  fetchSettings,
  handleSave,
  handleTestLLM,
  syncExtraBody,
} = useOcrSettingsForm()

const activeTab = ref('execution')

const ctx: SettingsFormContext = {
  form,
  saving,
  testingLLM,
  extraBodyText,
  save: handleSave,
  testLLM: handleTestLLM,
  syncExtraBody,
  selectedProvider: ref(null),
  selectedProviderType: ref(null),
  selectProvider: () => {},
}
provide(settingsFormKey, ctx)

// ========== 版本与升级 ==========
const version = ref<OCRVersion | null>(null)
const updateInfo = ref<OCRUpdateInfo | null>(null)
const upgradeStatus = ref<OCRUpgradeStatus | null>(null)
const checkingUpdate = ref(false)
const upgrading = ref(false)
let pollTimer: ReturnType<typeof setInterval> | null = null

const upgradeStateLabel = computed(() => {
  const map: Record<string, string> = {
    downloading: '下载中',
    verifying: '校验中',
    installing: '安装中',
    done: '升级完成',
    failed: '升级失败',
  }
  return map[upgradeStatus.value?.state || ''] || upgradeStatus.value?.state || ''
})

const upgradeTagType = computed<'success' | 'danger' | 'warning' | 'info'>(() => {
  const state = upgradeStatus.value?.state
  if (state === 'done') return 'success'
  if (state === 'failed') return 'danger'
  if (state === 'idle') return 'info'
  return 'warning'
})

function formatTime(value: string) {
  if (!value) return '-'
  return new Date(value).toLocaleString('zh-CN')
}

async function fetchVersion() {
  try {
    version.value = await getOCRVersion()
  } catch {
    version.value = null
  }
}

async function checkUpdate() {
  checkingUpdate.value = true
  try {
    updateInfo.value = await checkOCRUpdate()
    if (updateInfo.value.update_available) {
      ElMessage.info(`发现新版本 ${updateInfo.value.latest_version}`)
    } else {
      ElMessage.success('已是最新版本')
    }
  } catch (e: any) {
    ElMessage.error(e?.message || '检查更新失败')
  } finally {
    checkingUpdate.value = false
  }
}

async function startUpgrade() {
  if (!updateInfo.value?.latest_version) return
  upgrading.value = true
  try {
    upgradeStatus.value = await startOCRUpgrade(updateInfo.value.latest_version)
    startPolling()
  } catch (e: any) {
    ElMessage.error(e?.message || '启动升级失败')
    upgrading.value = false
  }
}

async function refreshUpgradeStatus() {
  try {
    upgradeStatus.value = await getOCRUpgradeStatus()
    handleUpgradeDone()
  } catch (e: any) {
    ElMessage.error(e?.message || '获取升级状态失败')
  }
}

function startPolling() {
  stopPolling()
  pollTimer = setInterval(async () => {
    try {
      upgradeStatus.value = await getOCRUpgradeStatus()
      handleUpgradeDone()
    } catch {
      // 静默重试
    }
  }, 2000)
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

function handleUpgradeDone() {
  const state = upgradeStatus.value?.state
  if (state === 'done') {
    stopPolling()
    upgrading.value = false
    ElMessage.success('OCR 升级完成')
    fetchVersion()
  } else if (state === 'failed') {
    stopPolling()
    upgrading.value = false
    ElMessage.error(upgradeStatus.value?.message || 'OCR 升级失败')
  }
}

function openReleaseUrl() {
  if (updateInfo.value?.release_url) {
    window.open(updateInfo.value.release_url, '_blank')
  }
}

onMounted(() => {
  fetchSettings()
  fetchVersion()
  refreshUpgradeStatus()
})

onBeforeUnmount(() => {
  stopPolling()
})
</script>

<style scoped>
.ms-page {
  width: min(1180px, 100%);
  margin: 0 auto;
  padding: 30px 32px 56px;
}

.ms-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 22px;
}

.ms-header h1 {
  margin: 4px 0 6px;
  font-size: 26px;
  color: var(--app-text);
}

.ms-header p {
  margin: 0;
  font-size: 13px;
  color: var(--app-muted);
}

.eyebrow {
  color: var(--app-primary);
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 1.3px;
}

.ms-actions {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
}

/* 版本卡片 */
.version-card {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 18px 22px;
  margin-bottom: 20px;
  background: linear-gradient(135deg, #f0f4ff 0%, #e8f0fe 100%);
  border: 1px solid #cdddff;
  border-radius: 14px;
}

.vc-main {
  display: flex;
  align-items: center;
  gap: 14px;
  min-width: 0;
}

.vc-icon {
  width: 48px;
  height: 48px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 22px;
  color: #fff;
  flex-shrink: 0;
  background: linear-gradient(135deg, #6366f1, #4f46e5);
}

.vc-title {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 5px;
}

.vc-title strong {
  font-size: 16px;
  color: var(--app-text);
}

.vc-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--app-muted);
  flex-wrap: wrap;
}

.vc-up {
  color: #e6a23c;
  font-weight: 600;
}

.vc-ok {
  color: #67c23a;
  font-weight: 600;
}

.sep {
  color: #c4ccd9;
}

.mono {
  font-family: ui-monospace, "SFMono-Regular", Menlo, monospace;
}

.vc-actions {
  display: flex;
  gap: 10px;
  flex-shrink: 0;
  flex-wrap: wrap;
}

.vc-progress {
  width: 100%;
  margin-top: 6px;
  padding-top: 16px;
  border-top: 1px solid #d8e3ff;
}

.vc-progress-head {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 10px;
  font-size: 13px;
  color: var(--app-muted);
}

.vc-progress-msg {
  flex: 1;
  min-width: 0;
}

.vc-notes {
  margin-top: 14px;
  padding: 12px 14px;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.7);
  border: 1px solid #e0e8f5;
}

.vc-notes strong {
  display: block;
  font-size: 13px;
  color: var(--app-text);
  margin-bottom: 6px;
}

.vc-notes pre {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 12px;
  line-height: 1.6;
  color: var(--app-muted);
  font-family: inherit;
}

/* 配置卡片 */
.config-card {
  background: var(--app-surface);
  border: 1px solid var(--app-border);
  border-radius: 16px;
  padding: 8px 26px 24px;
  box-shadow: var(--app-shadow);
}

.ocr-tabs :deep(.el-tabs__header) {
  margin-bottom: 18px;
}

@media (max-width: 860px) {
  .ms-page {
    padding: 20px 16px 40px;
  }

  .version-card {
    flex-direction: column;
    align-items: stretch;
  }

  .vc-actions {
    justify-content: flex-end;
  }
}
</style>
