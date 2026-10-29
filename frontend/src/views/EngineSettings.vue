<template>
  <AppLayout>
    <PageShell>
      <PageHeader
        eyebrow="REVIEW ENGINE"
        title="审查引擎"
        description="配置全局默认的服务商、模型、执行参数、工具与版本升级。项目策略可逐项覆盖这些默认值。"
      >
        <template #actions>
          <el-button @click="fetchSettings" :loading="loading">
            <el-icon><Refresh /></el-icon>
            <span>刷新</span>
          </el-button>
        </template>
      </PageHeader>

      <!-- 顶部状态条：当前配置概览 + 启用开关 + 测试 -->
      <section class="status-strip" :class="{ off: !form.enabled }">
        <div class="ss-icon" :class="currentIconClass">{{ currentInitial }}</div>
        <div class="ss-main">
          <div class="ss-title">
            <strong>{{ currentDisplayName }}</strong>
            <el-tag :type="form.enabled ? 'success' : 'info'" size="small" effect="light">
              {{ form.enabled ? '已启用' : '未启用' }}
            </el-tag>
            <el-tag size="small" effect="plain" :type="form.use_anthropic ? 'warning' : 'info'">
              {{ form.use_anthropic ? 'Anthropic' : 'OpenAI 兼容' }}
            </el-tag>
          </div>
          <div class="ss-meta">
            <span class="mono">{{ form.model || '未设置模型' }}</span>
            <span class="sep">·</span>
            <span class="mono url">{{ form.url || '未配置地址' }}</span>
            <span class="sep">·</span>
            <span>{{ form.language === '中文' ? '中文' : 'English' }}</span>
            <span class="sep">·</span>
            <span>并发 {{ form.concurrency || 2 }}</span>
          </div>
        </div>
        <div class="ss-controls">
          <label class="enable-toggle">
            <span>启用</span>
            <el-switch v-model="form.enabled" />
          </label>
        </div>
      </section>

      <!-- 版本与升级（折叠，默认收起；有更新时高亮） -->
      <el-collapse v-model="upgradeCollapse" class="upgrade-collapse">
        <el-collapse-item name="upgrade">
          <template #title>
            <div class="upgrade-title">
              <el-icon class="uc-ico"><Cpu /></el-icon>
              <strong>引擎版本</strong>
              <el-tag v-if="version" size="small" effect="plain" type="info">{{ version.version }}</el-tag>
              <el-tag v-if="version?.platform" size="small" effect="plain">{{ version.platform }}</el-tag>
              <el-tag
                v-if="updateInfo?.update_available"
                size="small"
                type="warning"
                effect="light"
              >
                发现新版本 {{ updateInfo.latest_version }}
              </el-tag>
              <el-tag v-else-if="updateInfo && !updateInfo.update_available" size="small" type="success" effect="light">
                已是最新
              </el-tag>
              <el-tag v-else-if="!version" size="small" type="info" effect="plain">未获取</el-tag>
            </div>
          </template>

          <div class="upgrade-body">
            <div class="ub-meta">
              <span v-if="version?.build_time">构建于 {{ formatTime(version.build_time) }}</span>
              <span v-if="version?.commit" class="mono">{{ version.commit.slice(0, 8) }}</span>
            </div>
            <div class="ub-actions">
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
            <div v-if="upgradeStatus && upgradeStatus.state !== 'idle'" class="ub-progress">
              <div class="ub-progress-head">
                <el-tag :type="upgradeTagType" size="small">{{ upgradeStateLabel }}</el-tag>
                <span v-if="upgradeStatus.message" class="ub-progress-msg">{{ upgradeStatus.message }}</span>
                <span v-if="upgradeStatus.version" class="mono">{{ upgradeStatus.version }}</span>
              </div>
              <el-progress
                v-if="typeof upgradeStatus.progress === 'number'"
                :percentage="upgradeStatus.progress"
                :status="upgradeStatus.state === 'done' ? 'success' : upgradeStatus.state === 'failed' ? 'exception' : undefined"
              />
              <div v-if="updateInfo?.release_notes" class="ub-notes">
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
          </div>
        </el-collapse-item>
      </el-collapse>

      <!-- 配置卡片 -->
      <section class="config-card">
        <el-tabs v-model="activeTab" class="engine-tabs">
          <el-tab-pane label="模型与服务商" name="model">
            <ModelProviderSection ref="modelSectionRef" />
          </el-tab-pane>
          <el-tab-pane label="执行参数" name="execution">
            <ExecutionParamsTab />
          </el-tab-pane>
          <el-tab-pane label="MCP 服务器" name="mcp">
            <McpServersTab />
          </el-tab-pane>
        </el-tabs>
      </section>

      <!-- 底部 sticky 保存条（跨 tab 始终可见） -->
      <div class="sticky-save-bar">
        <span class="save-hint">修改后请点击保存，配置将作为全局默认立即生效。</span>
        <div class="save-buttons">
          <el-button @click="handleTestLLM" :loading="testingLLM">
            <el-icon><Connection /></el-icon>
            <span>测试连接</span>
          </el-button>
          <el-button type="primary" @click="handleSave" :loading="saving">
            <el-icon><Check /></el-icon>
            <span>保存配置</span>
          </el-button>
        </div>
      </div>
    </PageShell>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, provide, onMounted, onBeforeUnmount } from 'vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import { Refresh, RefreshRight, Upload, Cpu, Connection, Check } from '@element-plus/icons-vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import PageShell from '@/components/common/PageShell.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import ModelProviderSection from '@/components/model-settings/ModelProviderSection.vue'
import ExecutionParamsTab from '@/components/model-settings/ExecutionParamsTab.vue'
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
  authToken,
  extraBodyText,
  fetchSettings,
  handleSave,
  handleTestLLM,
  syncExtraBody,
} = useOcrSettingsForm()

const activeTab = ref('model')
const modelSectionRef = ref<InstanceType<typeof ModelProviderSection> | null>(null)

// 服务商显示元数据（与 ModelProviderSection 保持一致，用于状态条摘要）
const providerMeta: Record<string, { display: string; icon: string }> = {
  openai: { display: 'OpenAI', icon: 'openai' },
  anthropic: { display: 'Anthropic', icon: 'anthropic' },
  deepseek: { display: 'DeepSeek', icon: 'deepseek' },
  dashscope: { display: '阿里百炼 DashScope', icon: 'dashscope' },
  'dashscope-tokenplan': { display: '阿里百炼 (Token Plan)', icon: 'dashscope' },
  'z-ai': { display: '智谱 AI', icon: 'zhipu' },
  volcengine: { display: '火山引擎 (豆包)', icon: 'volcengine' },
  mimo: { display: '小米 MiMo', icon: 'mimo' },
  minimax: { display: 'MiniMax', icon: 'minimax' },
  kimi: { display: 'Moonshot Kimi', icon: 'kimi' },
  'baidu-qianfan': { display: '百度千帆', icon: 'baidu' },
  'hy-tokenplan': { display: '腾讯混元 (Token Plan)', icon: 'hunyuan' },
  'tencent-tokenhub': { display: '腾讯云 (TokenHub)', icon: 'tencent' },
  // OCR v1.7.7+ EdenAI（聚合 provider）。
  edenai: { display: 'Eden AI', icon: 'edenai' },
  // OCR v1.7.11+ Ollama Cloud、智谱 coding plan。
  'ollama-cloud': { display: 'Ollama Cloud', icon: 'ollama' },
  'z-ai-coding': { display: '智谱 AI (Coding Plan)', icon: 'zhipu' },
  // OCR v1.7.12+ LiteLLM gateway。
  litellm: { display: 'LiteLLM Gateway', icon: 'litellm' },
}

const currentDisplayName = computed(() => {
  const p = form.provider
  if (!p) return '未配置'
  return providerMeta[p]?.display || p
})
const currentInitial = computed(() => currentDisplayName.value.charAt(0))
const currentIconClass = computed(() => (form.provider ? providerMeta[form.provider]?.icon || '' : ''))

const ctx: SettingsFormContext = {
  form,
  saving,
  testingLLM,
  authToken,
  extraBodyText,
  save: handleSave,
  testLLM: handleTestLLM,
  syncExtraBody,
  // 占位：服务商选择由 ModelProviderSection 自行管理（直接写 form.provider），
  // 这里保留接口契约以兼容 context 类型。
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
// 有可用更新时默认展开版本区，否则收起（低频操作降权）。
const upgradeCollapse = ref<string[]>([])
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
      upgradeCollapse.value = ['upgrade']
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
    ElMessage.success('引擎升级完成')
    fetchVersion()
  } else if (state === 'failed') {
    stopPolling()
    upgrading.value = false
    ElMessage.error(upgradeStatus.value?.message || '引擎升级失败')
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
/* 状态条 */
.status-strip {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 16px 20px;
  background: linear-gradient(135deg, #eff4ff 0%, #e8effd 100%);
  border: 1px solid #cdddff;
  border-radius: 14px;
  margin-bottom: 20px;
}

.status-strip.off {
  background: var(--app-surface);
  border-color: var(--app-border);
}

.ss-icon {
  width: 52px;
  height: 52px;
  border-radius: var(--card-radius);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 19px;
  font-weight: 700;
  color: #fff;
  flex-shrink: 0;
  background: var(--app-border);
}

.ss-main {
  flex: 1;
  min-width: 0;
}

.ss-title {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 5px;
}

.ss-title strong {
  font-size: 16px;
  color: var(--app-text);
}

.ss-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--app-muted);
  flex-wrap: wrap;
  min-width: 0;
}

.ss-meta .mono {
  font-family: ui-monospace, "SFMono-Regular", Menlo, monospace;
}

.ss-meta .url {
  color: var(--app-primary);
  word-break: break-all;
}

.sep {
  color: #c4ccd9;
}

.ss-controls {
  display: flex;
  align-items: center;
  gap: 14px;
  flex-shrink: 0;
}

.enable-toggle {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: var(--app-muted);
}

/* 状态条服务商图标配色 */
.ss-icon.openai { background: linear-gradient(135deg, #10a37f, #0d8a6a); }
.ss-icon.anthropic { background: linear-gradient(135deg, #d97706, #b45309); }
.ss-icon.deepseek { background: linear-gradient(135deg, #4d6bfe, #2b4fd6); }
.ss-icon.dashscope { background: linear-gradient(135deg, #6366f1, #4f46e5); }
.ss-icon.zhipu { background: linear-gradient(135deg, #3b5bdb, #2b3fa8); }
.ss-icon.volcengine { background: linear-gradient(135deg, #f59e0b, #d97706); }
.ss-icon.mimo { background: linear-gradient(135deg, #10b981, #059669); }
.ss-icon.minimax { background: linear-gradient(135deg, #ec4899, #db2777); }
.ss-icon.kimi { background: linear-gradient(135deg, #8b5cf6, #6d28d9); }
.ss-icon.baidu { background: linear-gradient(135deg, #2932e1, #1b2bbd); }
.ss-icon.hunyuan { background: linear-gradient(135deg, #0052d9, #003eaa); }
.ss-icon.tencent { background: linear-gradient(135deg, #00a4ff, #0080d6); }

/* 版本/升级折叠 */
.upgrade-collapse {
  margin-bottom: 20px;
  border: 1px solid var(--app-border);
  border-radius: var(--card-radius);
  background: var(--app-surface);
}

.upgrade-collapse :deep(.el-collapse-item__header) {
  padding: 0 18px;
  border-bottom: none;
  border-radius: var(--card-radius);
}

.upgrade-collapse :deep(.el-collapse-item__wrap) {
  border-bottom: none;
}

.upgrade-collapse :deep(.el-collapse-item__content) {
  padding: 0 18px 18px;
}

.upgrade-title {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.upgrade-title .uc-ico {
  color: var(--app-primary);
}

.upgrade-title strong {
  font-size: 15px;
  color: var(--app-text);
}

.upgrade-body {
  padding-top: 8px;
}

.ub-meta {
  display: flex;
  gap: 12px;
  font-size: 12px;
  color: var(--app-muted);
  margin-bottom: 12px;
}

.ub-actions {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.ub-progress {
  margin-top: 14px;
  padding-top: 14px;
  border-top: 1px solid var(--app-border);
}

.ub-progress-head {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 10px;
  font-size: 13px;
  color: var(--app-muted);
}

.ub-progress-msg {
  flex: 1;
  min-width: 0;
}

.ub-notes {
  margin-top: 14px;
  padding: 12px 14px;
  border-radius: 8px;
  background: #f7f9fc;
  border: 1px solid var(--app-border);
}

.ub-notes strong {
  display: block;
  font-size: 13px;
  color: var(--app-text);
  margin-bottom: 6px;
}

.ub-notes pre {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 12px;
  line-height: 1.6;
  color: var(--app-muted);
  font-family: inherit;
}

.mono {
  font-family: ui-monospace, "SFMono-Regular", Menlo, monospace;
}

/* 配置卡片 */
.config-card {
  background: var(--app-surface);
  border: 1px solid var(--app-border);
  border-radius: 16px;
  padding: 8px 26px 24px;
  box-shadow: var(--app-shadow);
}

.engine-tabs :deep(.el-tabs__header) {
  margin-bottom: 18px;
}

/* 底部 sticky 保存条 */
.sticky-save-bar {
  position: sticky;
  bottom: 0;
  z-index: 9;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-top: 16px;
  padding: 14px 20px;
  border: 1px solid var(--app-border);
  border-radius: var(--card-radius);
  background: var(--app-surface);
  box-shadow: var(--shadow-sticky);
}

.save-hint {
  font-size: 12px;
  color: var(--app-muted);
}

.save-buttons {
  display: flex;
  gap: 10px;
  flex-shrink: 0;
}

@media (max-width: 860px) {
  .status-strip {
    flex-wrap: wrap;
  }

  .ss-controls {
    width: 100%;
    justify-content: space-between;
  }

  .sticky-save-bar {
    flex-direction: column;
    align-items: stretch;
  }

  .save-buttons {
    justify-content: flex-end;
  }
}
</style>
