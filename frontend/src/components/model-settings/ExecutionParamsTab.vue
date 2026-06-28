<template>
  <el-form :model="form" label-width="120px" :disabled="saving">
    <el-form-item label="OCR 二进制">
      <el-input v-model="form.binary" placeholder="ocr 或 /usr/local/bin/ocr" />
      <div class="form-tip">只配置可执行文件路径，具体 review 参数由服务端按白名单拼装。</div>
    </el-form-item>

    <el-form-item label="输出受众">
      <el-segmented
        v-model="form.audience"
        :options="[
          { label: '人工可读', value: 'human' },
          { label: 'Agent 精简', value: 'agent' },
        ]"
      />
    </el-form-item>

    <el-form-item label="默认严重级别">
      <el-select v-model="form.default_severity">
        <el-option label="严重" value="critical" />
        <el-option label="高危" value="major" />
        <el-option label="中危" value="minor" />
        <el-option label="低危" value="info" />
      </el-select>
      <div class="form-tip">当 OCR 输出未携带严重级别时使用该值。</div>
    </el-form-item>

    <el-form-item label="模型并发数">
      <el-input-number v-model="form.concurrency" :min="1" :max="32" />
    </el-form-item>

    <el-form-item label="Git 并发数">
      <el-input-number v-model="form.max_git_procs" :min="1" :max="64" />
    </el-form-item>

    <el-form-item label="单文件超时（分钟）">
      <el-input-number v-model="form.timeout_minutes" :min="1" :max="120" />
    </el-form-item>

    <el-form-item label="最大工具轮次">
      <el-input-number v-model="form.max_tools" :min="0" :max="100" />
      <div class="form-tip">0 表示使用 OCR 内置默认值</div>
    </el-form-item>

    <el-form-item label="工具白名单">
      <el-input v-model="form.tools_path" placeholder="/path/to/tools.json" />
      <div class="form-tip">可选。JSON 文件路径，限制 OCR 可用的工具集。</div>
    </el-form-item>

    <el-form-item label="工具配置 JSON">
      <div class="tool-package-row">
        <el-select v-model="selectedToolPackage" placeholder="选择托管工具包" clearable @change="applyToolPackage">
          <el-option
            v-for="pkg in toolPackages"
            :key="pkg.id || pkg.name"
            :label="`${pkg.name}${pkg.scan_type ? ' · ' + pkg.scan_type : ''}`"
            :value="pkg.id || pkg.name"
          />
        </el-select>
        <el-button @click="refreshToolPackages" :loading="toolPackageLoading">刷新</el-button>
        <el-button plain @click="openToolDrawer">管理</el-button>
        <el-button type="primary" plain @click="handleValidateTools(false)" :loading="validatingTools">JSON 校验</el-button>
        <el-button type="success" plain @click="handleValidateTools(true)" :loading="validatingTools">OCR dry-run</el-button>
      </div>
      <el-input
        v-model="form.tools_config"
        type="textarea"
        :rows="6"
        placeholder='[{"name":"code_search","main_task":true,"plan_task":true,"definition":{"name":"code_search","description":"Search code","parameters":{"type":"object","properties":{}}}}]'
      />
      <div class="form-tip">可选。未设置工具白名单路径时生效，服务会临时写入 tools.json 并传给 OCR。</div>
    </el-form-item>

    <el-form-item label="额外请求体">
      <el-input
        v-model="extraBodyText"
        type="textarea"
        :rows="4"
        placeholder='{"temperature":0.2}'
        @blur="syncExtraBody(false)"
      />
      <div class="form-tip">可选。必须是 JSON 对象字符串，会透传给 OCR_LLM_EXTRA_BODY。</div>
    </el-form-item>

    <el-form-item label="额外请求头">
      <el-input
        v-model="form.extra_headers"
        type="textarea"
        :rows="2"
        placeholder="X-Org-ID=org-123,X-Trace-ID=review-service"
      />
      <div class="form-tip">可选。逗号分隔的 key=value，会透传给 OCR_LLM_EXTRA_HEADERS。</div>
    </el-form-item>

    <el-divider content-position="left">遥测 (Telemetry)</el-divider>

    <el-form-item label="启用遥测">
      <el-switch v-model="form.telemetry_enabled" />
    </el-form-item>

    <el-form-item label="导出器" v-if="form.telemetry_enabled">
      <el-select v-model="form.telemetry_exporter" clearable>
        <el-option label="OTLP" value="otlp" />
        <el-option label="Stdout" value="stdout" />
      </el-select>
    </el-form-item>

    <el-form-item label="OTLP 端点" v-if="form.telemetry_enabled && form.telemetry_exporter === 'otlp'">
      <el-input v-model="form.telemetry_otlp_endpoint" placeholder="http://localhost:4317" />
    </el-form-item>

    <el-form-item label="内容日志" v-if="form.telemetry_enabled">
      <el-switch v-model="form.telemetry_content_logging" />
      <div class="form-tip">启用后会记录 OCR 交互的完整内容（可能包含敏感信息）。</div>
    </el-form-item>

    <el-divider content-position="left">Session Viewer</el-divider>

    <el-form-item label="Viewer 状态">
      <el-tag :type="viewerStatus?.running ? 'success' : 'info'">
        {{ viewerStatus?.running ? '运行中' : '未运行' }}
      </el-tag>
      <span v-if="viewerStatus?.addr" style="margin-left: 8px; color: #909399;">{{ viewerStatus.addr }}</span>
    </el-form-item>

    <el-form-item>
      <el-button
        v-if="!viewerStatus?.running"
        type="success"
        @click="handleStartViewer"
        :loading="viewerLoading"
      >启动 Viewer</el-button>
      <el-button
        v-else
        type="danger"
        @click="handleStopViewer"
        :loading="viewerLoading"
      >停止 Viewer</el-button>
      <el-button @click="refreshViewerStatus" :loading="viewerLoading">刷新状态</el-button>
      <el-button
        v-if="viewerStatus?.running && viewerStatus?.addr"
        type="primary"
        @click="embedViewer = !embedViewer"
      >{{ embedViewer ? '收起 Viewer' : '嵌入 Viewer' }}</el-button>
    </el-form-item>

    <div v-if="embedViewer && viewerStatus?.running" class="viewer-frame">
      <iframe title="OCR Viewer" :src="viewerProxyUrl" />
    </div>

    <el-divider content-position="left">执行预览</el-divider>

    <div class="command-preview">
      <div class="preview-title">服务端将按受控参数组装命令</div>
      <code>{{ commandPreview }}</code>
    </div>

    <el-form-item>
      <el-button type="primary" @click="save" :loading="saving">保存配置</el-button>
    </el-form-item>

    <el-drawer v-model="toolDrawer" title="工具包管理" size="720px">
      <div class="tool-drawer">
        <section class="tool-list">
          <div class="tool-list-head">
            <strong>托管工具包</strong>
            <el-button size="small" type="primary" @click="newToolPackage">新建</el-button>
          </div>
          <div v-for="pkg in toolPackages" :key="pkg.id || pkg.name" class="tool-package-card">
            <div>
              <strong>{{ pkg.name }}</strong>
              <small>{{ pkg.description || '暂无描述' }}</small>
              <em>{{ pkg.built_in ? '内置' : '自定义' }} · {{ pkg.scan_type || 'all' }} · {{ pkg.enabled ? '启用' : '停用' }}</em>
            </div>
            <div class="tool-package-actions">
              <el-switch size="small" :model-value="pkg.enabled" @change="toggleToolPackage(pkg)" />
              <el-button size="small" text @click="editToolPackage(pkg)">{{ pkg.built_in ? '复制' : '编辑' }}</el-button>
              <el-button v-if="pkg.id" size="small" text @click="showToolPackageHistory(pkg)">历史</el-button>
              <el-button v-if="!pkg.built_in" size="small" text type="danger" @click="removeToolPackage(pkg)">删除</el-button>
            </div>
          </div>
        </section>

        <section class="tool-editor">
          <div class="tool-list-head">
            <strong>{{ toolDraft.id ? '编辑工具包' : '新建工具包' }}</strong>
            <span>{{ toolDraft.builtInSource ? '来自内置包副本' : '自定义' }}</span>
          </div>
          <el-form label-position="top">
            <div class="tool-editor-grid">
              <el-form-item label="名称">
                <el-input v-model="toolDraft.name" />
              </el-form-item>
              <el-form-item label="适用类型">
                <el-select v-model="toolDraft.scan_type">
                  <el-option label="全部" value="all" />
                  <el-option label="Diff Review" value="diff" />
                  <el-option label="全量 Scan" value="scan" />
                </el-select>
              </el-form-item>
            </div>
            <el-form-item label="描述">
              <el-input v-model="toolDraft.description" type="textarea" :rows="2" />
            </el-form-item>
            <el-form-item label="项目绑定">
              <el-input v-model="toolDraft.projectRefsText" type="textarea" :rows="2" placeholder="每行一个 provider/repo；留空表示全部项目" />
            </el-form-item>
            <el-form-item label="内置工具">
              <div class="tool-switch-grid">
                <el-checkbox v-for="tool in builtinTools" :key="tool.name" v-model="toolDraft.selectedTools[tool.name]" @change="syncToolDraftJSON">
                  {{ tool.label }}
                </el-checkbox>
              </div>
            </el-form-item>
            <el-form-item label="工具配置 JSON">
              <el-input v-model="toolDraft.configText" type="textarea" :rows="9" @blur="syncToolDraftSelection" />
            </el-form-item>
            <div class="tool-editor-actions">
              <el-button @click="validateToolDraft(false)" :loading="validatingDraft">JSON 校验</el-button>
              <el-button type="success" plain @click="validateToolDraft(true)" :loading="validatingDraft">OCR dry-run</el-button>
              <el-button type="primary" @click="saveToolDraft" :loading="savingToolPackage">保存工具包</el-button>
            </div>
          </el-form>
        </section>
      </div>
    </el-drawer>

    <el-dialog v-model="toolPackageHistoryDialog" title="工具包版本历史" width="680px">
      <div class="history-title">
        <strong>{{ historyToolPackage?.name || '工具包' }}</strong>
        <span>保存工具包时会自动记录修改前的版本</span>
      </div>
      <div v-loading="toolPackageHistoryLoading" class="version-list">
        <div
          v-for="version in toolPackageVersions"
          :key="version.version"
          class="version-item"
        >
          <div>
            <strong>版本 {{ version.version }}</strong>
            <span>{{ formatVersionTime(version.created_at) }}</span>
            <small>{{ version.snapshot?.name || '-' }} · {{ countTools(version.snapshot?.config) }} 个工具</small>
          </div>
          <el-button
            size="small"
            type="primary"
            plain
            :loading="toolPackageHistoryRestoring"
            @click="restoreToolPackage(version.version)"
          >
            恢复
          </el-button>
        </div>
        <el-empty v-if="!toolPackageVersions.length" description="暂无历史版本" :image-size="56" />
      </div>
    </el-dialog>
  </el-form>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'
import { useSettingsForm } from './context'
import { useAuthStore } from '@/stores/auth'
import {
  deleteToolPackage,
  getViewerStatus,
  listToolPackageVersions,
  listToolPackages,
  restoreToolPackageVersion,
  saveToolPackage,
  startViewer,
  stopViewer,
  validateToolPackage,
} from '@/api/config'
import type { OCRAssetVersion, OCRToolPackage, ViewerStatus } from '@/types/config'

const { form, saving, extraBodyText, save, syncExtraBody } = useSettingsForm()
const auth = useAuthStore()

const viewerProxyUrl = computed(() => {
  const base = '/api/v1/settings/ocr/viewer/proxy/'
  return auth.token ? `${base}?token=${encodeURIComponent(auth.token)}` : base
})

const viewerStatus = ref<ViewerStatus | null>(null)
const viewerLoading = ref(false)
const embedViewer = ref(false)
const toolPackages = ref<OCRToolPackage[]>([])
const selectedToolPackage = ref<number | string | undefined>()
const toolPackageLoading = ref(false)
const validatingTools = ref(false)
const toolDrawer = ref(false)
const savingToolPackage = ref(false)
const validatingDraft = ref(false)
const toolPackageHistoryDialog = ref(false)
const toolPackageHistoryLoading = ref(false)
const toolPackageHistoryRestoring = ref(false)
const historyToolPackage = ref<OCRToolPackage | null>(null)
const toolPackageVersions = ref<OCRAssetVersion<OCRToolPackage>[]>([])

const builtinTools = [
  { name: 'file_read', label: 'file_read' },
  { name: 'file_read_diff', label: 'file_read_diff' },
  { name: 'code_search', label: 'code_search' },
  { name: 'file_find', label: 'file_find' },
  { name: 'code_comment', label: 'code_comment' },
]

const toolDraft = ref({
  id: undefined as number | undefined,
  name: '',
  description: '',
  scan_type: 'all' as 'all' | 'diff' | 'scan',
  enabled: true,
  projectRefsText: '',
  configText: '',
  builtInSource: false,
  selectedTools: {} as Record<string, boolean>,
})

async function refreshViewerStatus() {
  viewerLoading.value = true
  try {
    viewerStatus.value = await getViewerStatus()
  } catch {
    viewerStatus.value = { running: false, message: '获取状态失败' }
  } finally {
    viewerLoading.value = false
  }
}

async function handleStartViewer() {
  viewerLoading.value = true
  try {
    viewerStatus.value = await startViewer()
  } catch (e: any) {
    viewerStatus.value = { running: false, message: e.message || '启动失败' }
  } finally {
    viewerLoading.value = false
  }
}

async function handleStopViewer() {
  viewerLoading.value = true
  try {
    viewerStatus.value = await stopViewer()
  } catch (e: any) {
    viewerStatus.value = { running: false, message: e.message || '停止失败' }
  } finally {
    viewerLoading.value = false
  }
}

async function refreshToolPackages() {
  toolPackageLoading.value = true
  try {
    toolPackages.value = await listToolPackages()
  } finally {
    toolPackageLoading.value = false
  }
}

async function openToolDrawer() {
  toolDrawer.value = true
  await refreshToolPackages()
  if (!toolDraft.value.name) newToolPackage()
}

function applyToolPackage(value?: number | string) {
  const pkg = toolPackages.value.find((item) => (item.id || item.name) === value)
  if (!pkg) return
  form.tools_config = JSON.stringify(pkg.config, null, 2)
}

async function handleValidateTools(dryRun = false) {
  if (!form.tools_config?.trim()) return
  validatingTools.value = true
  try {
    const result = await validateToolPackage(form.tools_config, { dry_run: dryRun })
    if (result.valid) {
      ElMessage.success(result.message || (dryRun ? 'OCR dry-run 通过' : '工具配置校验通过'))
    }
  } catch (e: any) {
    ElMessage.error(e.message || '工具配置校验失败')
  } finally {
    validatingTools.value = false
  }
}

function newToolPackage() {
  toolDraft.value = {
    id: undefined,
    name: '',
    description: '',
    scan_type: 'all',
    enabled: true,
    projectRefsText: '',
    configText: JSON.stringify(makeToolConfig(['file_read', 'code_search', 'file_find', 'code_comment']), null, 2),
    builtInSource: false,
    selectedTools: { file_read: true, code_search: true, file_find: true, code_comment: true },
  }
}

function editToolPackage(pkg: OCRToolPackage) {
  const configText = JSON.stringify(pkg.config || [], null, 2)
  toolDraft.value = {
    id: pkg.built_in ? undefined : pkg.id,
    name: pkg.built_in ? `${pkg.name} 副本` : pkg.name,
    description: pkg.description || '',
    scan_type: pkg.scan_type || 'all',
    enabled: pkg.enabled,
    projectRefsText: (pkg.project_refs || []).join('\n'),
    configText,
    builtInSource: Boolean(pkg.built_in),
    selectedTools: inferSelectedTools(pkg.config),
  }
}

async function toggleToolPackage(pkg: OCRToolPackage) {
  try {
    await saveToolPackage({ ...pkg, enabled: !pkg.enabled })
    await refreshToolPackages()
  } catch (e: any) {
    ElMessage.error(e.message || '更新工具包失败')
  }
}

async function removeToolPackage(pkg: OCRToolPackage) {
  if (!pkg.id || pkg.built_in) return
  try {
    await ElMessageBox.confirm(`确定删除工具包「${pkg.name}」？`, '删除工具包', {
      type: 'warning',
      confirmButtonText: '删除',
      cancelButtonText: '取消',
    })
  } catch {
    return
  }
  try {
    await deleteToolPackage(pkg.id)
    await refreshToolPackages()
    ElMessage.success('工具包已删除')
  } catch (e: any) {
    ElMessage.error(e.message || '删除工具包失败')
  }
}

async function showToolPackageHistory(pkg: OCRToolPackage) {
  if (!pkg.id) return
  historyToolPackage.value = pkg
  toolPackageHistoryDialog.value = true
  toolPackageHistoryLoading.value = true
  try {
    toolPackageVersions.value = await listToolPackageVersions(pkg.id)
  } catch (e: any) {
    ElMessage.error(e.message || '获取工具包历史失败')
  } finally {
    toolPackageHistoryLoading.value = false
  }
}

async function restoreToolPackage(version: number) {
  if (!historyToolPackage.value?.id) return
  try {
    await ElMessageBox.confirm(`确定将「${historyToolPackage.value.name}」恢复到版本 ${version}？`, '恢复工具包', {
      type: 'warning',
      confirmButtonText: '恢复',
      cancelButtonText: '取消',
    })
  } catch {
    return
  }
  toolPackageHistoryRestoring.value = true
  try {
    const restored = await restoreToolPackageVersion(historyToolPackage.value.id, version)
    historyToolPackage.value = restored
    await refreshToolPackages()
    toolPackageVersions.value = await listToolPackageVersions(restored.id!)
    ElMessage.success(`已恢复到版本 ${version}`)
  } catch (e: any) {
    ElMessage.error(e.message || '恢复工具包失败')
  } finally {
    toolPackageHistoryRestoring.value = false
  }
}

async function validateToolDraft(dryRun = false) {
  validatingDraft.value = true
  try {
    const result = await validateToolPackage(toolDraft.value.configText, { dry_run: dryRun })
    ElMessage.success(result.message || (dryRun ? 'OCR dry-run 通过' : '工具包配置校验通过'))
  } catch (e: any) {
    ElMessage.error(e.message || '工具包配置校验失败')
  } finally {
    validatingDraft.value = false
  }
}

async function saveToolDraft() {
  const draft = toolDraft.value
  if (!draft.name.trim()) {
    ElMessage.warning('请填写工具包名称')
    return
  }
  let config: any
  try {
    config = JSON.parse(draft.configText)
  } catch {
    ElMessage.error('工具配置 JSON 不合法')
    return
  }
  savingToolPackage.value = true
  try {
    await validateToolPackage(config)
    await saveToolPackage({
      id: draft.id,
      name: draft.name.trim(),
      description: draft.description.trim(),
      scan_type: draft.scan_type,
      enabled: draft.enabled,
      project_refs: splitLines(draft.projectRefsText),
      config,
    })
    await refreshToolPackages()
    ElMessage.success('工具包已保存')
  } catch (e: any) {
    ElMessage.error(e.message || '保存工具包失败')
  } finally {
    savingToolPackage.value = false
  }
}

function syncToolDraftJSON() {
  const names = Object.entries(toolDraft.value.selectedTools).filter(([, enabled]) => enabled).map(([name]) => name)
  toolDraft.value.configText = JSON.stringify(makeToolConfig(names), null, 2)
}

function syncToolDraftSelection() {
  try {
    toolDraft.value.selectedTools = inferSelectedTools(JSON.parse(toolDraft.value.configText))
  } catch {
    // JSON 校验按钮负责提示，这里只避免编辑时打断输入。
  }
}

function inferSelectedTools(config: any) {
  const selected: Record<string, boolean> = {}
  const list = Array.isArray(config) ? config : Array.isArray(config?.tools) ? config.tools : []
  list.forEach((item: any) => {
    if (item?.name) selected[item.name] = true
  })
  return selected
}

function countTools(config: any) {
  const list = Array.isArray(config) ? config : Array.isArray(config?.tools) ? config.tools : []
  return list.length
}

function makeToolConfig(names: string[]) {
  return names.map((name) => ({
    name,
    main_task: true,
    plan_task: name !== 'code_comment',
    definition: {
      name,
      description: toolDescription(name),
      parameters: { type: 'object', properties: {} },
    },
  }))
}

function toolDescription(name: string) {
  const map: Record<string, string> = {
    file_read: 'Read repository files',
    file_read_diff: 'Read diff hunks',
    code_search: 'Search code by keyword',
    file_find: 'Find files by pattern',
    code_comment: 'Create code review comments',
  }
  return map[name] || name
}

function splitLines(value: string) {
  return value.split('\n').map((item) => item.trim()).filter(Boolean)
}

function formatVersionTime(value: string) {
  if (!value) return '-'
  return new Date(value).toLocaleString('zh-CN')
}

onMounted(() => {
  refreshViewerStatus()
  refreshToolPackages()
})

const commandPreview = computed(() => {
  const args = [
    form.binary || 'ocr',
    'review',
    '--repo <workspace>',
    '--from <base>',
    '--to <head>',
    '--format json',
    `--concurrency ${form.concurrency || 2}`,
    `--max-git-procs ${form.max_git_procs || 4}`,
    `--timeout ${form.timeout_minutes || 10}`,
    `--audience ${form.audience || 'human'}`,
  ]
  if (form.max_tools && form.max_tools > 0) {
    args.push(`--max-tools ${form.max_tools}`)
  }
  if (form.tools_path) {
    args.push(`--tools ${form.tools_path}`)
  } else if (form.tools_config) {
    args.push('--tools <managed tools.json>')
  }
  if (form.background) {
    args.push('--background <configured background>')
  }
  return args.join(' ')
})
</script>

<style scoped>
.form-tip {
  font-size: 12px;
  color: #999;
  margin-top: 4px;
}

.command-preview {
  margin: 0 0 22px 120px;
  padding: 12px 14px;
  border: 1px solid #dcdfe6;
  border-radius: 8px;
  background: #f7f9fc;
}

.preview-title {
  margin-bottom: 8px;
  color: #606266;
  font-size: 12px;
}

.command-preview code {
  display: block;
  white-space: pre-wrap;
  word-break: break-word;
  color: #303133;
  font-size: 12px;
  line-height: 1.6;
}

.tool-package-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto auto auto auto;
  gap: 8px;
  width: 100%;
  margin-bottom: 8px;
}

.tool-drawer {
  display: grid;
  grid-template-columns: minmax(210px, 0.85fr) minmax(0, 1.4fr);
  gap: 16px;
}

.tool-list,
.tool-editor {
  min-width: 0;
}

.tool-list-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 12px;
  color: #303133;
}

.tool-list-head span {
  color: #909399;
  font-size: 12px;
}

.tool-package-card {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 10px;
  align-items: center;
  padding: 12px;
  margin-bottom: 10px;
  border: 1px solid #dcdfe6;
  border-radius: 8px;
  background: #fff;
}

.tool-package-card strong,
.tool-package-card small,
.tool-package-card em {
  display: block;
  min-width: 0;
}

.tool-package-card small {
  margin-top: 4px;
  color: #606266;
  font-size: 12px;
}

.tool-package-card em {
  margin-top: 6px;
  color: #909399;
  font-size: 11px;
  font-style: normal;
}

.tool-package-actions {
  display: flex;
  align-items: center;
  gap: 4px;
  white-space: nowrap;
}

.tool-editor {
  padding-left: 16px;
  border-left: 1px solid #ebeef5;
}

.tool-editor-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 150px;
  gap: 12px;
}

.tool-switch-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px 12px;
}

.tool-editor-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.viewer-frame {
  height: 520px;
  margin: 0 0 22px 120px;
  overflow: hidden;
  border: 1px solid #dcdfe6;
  border-radius: 8px;
  background: #fff;
}

.viewer-frame iframe {
  width: 100%;
  height: 100%;
  border: 0;
}

.history-title {
  margin-bottom: 14px;
}

.history-title strong,
.history-title span {
  display: block;
}

.history-title strong {
  color: #303133;
  font-size: 15px;
}

.history-title span {
  margin-top: 4px;
  color: #909399;
  font-size: 12px;
}

.version-list {
  min-height: 120px;
}

.version-item {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 12px;
  align-items: center;
  padding: 12px;
  border: 1px solid #e6ebf2;
  border-radius: 8px;
  background: #fff;
}

.version-item + .version-item {
  margin-top: 10px;
}

.version-item strong,
.version-item span,
.version-item small {
  display: block;
}

.version-item strong {
  color: #303133;
  font-size: 13px;
}

.version-item span {
  margin-top: 4px;
  color: #909399;
  font-size: 12px;
}

.version-item small {
  margin-top: 4px;
  color: #409eff;
  font-size: 11px;
}

@media (max-width: 720px) {
  .command-preview {
    margin-left: 0;
  }

  .tool-package-row {
    grid-template-columns: 1fr;
  }

  .viewer-frame {
    margin-left: 0;
  }

  .tool-drawer {
    grid-template-columns: 1fr;
  }

  .tool-editor {
    padding-left: 0;
    border-left: 0;
  }

  .tool-editor-grid,
  .tool-switch-grid {
    grid-template-columns: 1fr;
  }
}
</style>
