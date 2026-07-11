<script setup lang="ts">
/**
 * ToolPackageDrawer — 工具包管理抽屉 + 版本历史对话框。
 * 从 ExecutionParamsTab 抽出，完全自包含（通过 API 直接操作工具包 CRUD）。
 */
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'
import {
  deleteToolPackage,
  listToolPackageVersions,
  listToolPackages,
  restoreToolPackageVersion,
  saveToolPackage,
  validateToolPackage,
} from '@/api/config'
import type { OCRAssetVersion, OCRToolPackage } from '@/types/config'

const model = defineModel<boolean>({ default: false })

const emit = defineEmits<{
  (e: 'select', pkg: OCRToolPackage): void
}>()

const toolPackages = ref<OCRToolPackage[]>([])
const loading = ref(false)
const savingPkg = ref(false)
const validatingDraft = ref(false)
const historyDialog = ref(false)
const historyLoading = ref(false)
const historyRestoring = ref(false)
const historyPkg = ref<OCRToolPackage | null>(null)
const toolPackageVersions = ref<OCRAssetVersion<OCRToolPackage>[]>([])

const builtinTools = [
  { name: 'file_read', label: '文件读取' },
  { name: 'file_read_diff', label: 'Diff 读取' },
  { name: 'code_search', label: '代码搜索' },
  { name: 'file_find', label: '文件查找' },
  { name: 'code_comment', label: '代码评论' },
]

const toolDraft = ref({
  id: undefined as number | undefined,
  name: '',
  description: '',
  scan_type: 'all',
  enabled: true,
  projectRefsText: '',
  configText: '',
  builtInSource: false,
  selectedTools: {} as Record<string, boolean>,
})

async function refresh() {
  loading.value = true
  try {
    toolPackages.value = await listToolPackages()
  } catch (e: any) {
    ElMessage.error(e?.message || '加载工具包失败')
  } finally {
    loading.value = false
  }
}

function newPackage() {
  toolDraft.value = {
    id: undefined, name: '', description: '', scan_type: 'all', enabled: true,
    projectRefsText: '',
    configText: JSON.stringify(makeToolConfig(['file_read', 'code_search', 'file_find', 'code_comment']), null, 2),
    builtInSource: false,
    selectedTools: { file_read: true, code_search: true, file_find: true, code_comment: true },
  }
}

function editPackage(pkg: OCRToolPackage) {
  toolDraft.value = {
    id: pkg.built_in ? undefined : pkg.id,
    name: pkg.built_in ? `${pkg.name} 副本` : pkg.name,
    description: pkg.description || '',
    scan_type: pkg.scan_type || 'all',
    enabled: pkg.enabled,
    projectRefsText: (pkg.project_refs || []).join('\n'),
    configText: JSON.stringify(pkg.config || [], null, 2),
    builtInSource: Boolean(pkg.built_in),
    selectedTools: inferSelectedTools(pkg.config),
  }
}

async function togglePackage(pkg: OCRToolPackage) {
  try {
    await saveToolPackage({ ...pkg, enabled: !pkg.enabled })
    await refresh()
  } catch (e: any) { ElMessage.error(e.message || '更新失败') }
}

async function removePackage(pkg: OCRToolPackage) {
  if (!pkg.id || pkg.built_in) return
  try {
    await ElMessageBox.confirm(`确定删除工具包「${pkg.name}」？`, '删除', { type: 'warning' })
  } catch { return }
  try {
    await deleteToolPackage(pkg.id)
    await refresh()
    ElMessage.success('已删除')
  } catch (e: any) { ElMessage.error(e.message || '删除失败') }
}

async function showHistory(pkg: OCRToolPackage) {
  if (!pkg.id) return
  historyPkg.value = pkg
  historyDialog.value = true
  historyLoading.value = true
  try {
    toolPackageVersions.value = await listToolPackageVersions(pkg.id)
  } catch (e: any) { ElMessage.error(e.message || '获取历史失败')
  } finally { historyLoading.value = false }
}

async function restoreVersion(version: number) {
  if (!historyPkg.value?.id) return
  try {
    await ElMessageBox.confirm(`恢复到版本 ${version}？`, '恢复', { type: 'warning' })
  } catch { return }
  historyRestoring.value = true
  try {
    const restored = await restoreToolPackageVersion(historyPkg.value.id, version)
    historyPkg.value = restored
    await refresh()
    toolPackageVersions.value = await listToolPackageVersions(restored.id!)
    ElMessage.success(`已恢复到版本 ${version}`)
  } catch (e: any) { ElMessage.error(e.message || '恢复失败')
  } finally { historyRestoring.value = false }
}

async function validateDraft(dryRun = false) {
  validatingDraft.value = true
  try {
    const result = await validateToolPackage(toolDraft.value.configText, { dry_run: dryRun })
    ElMessage.success(result.message || (dryRun ? 'dry-run 通过' : '校验通过'))
  } catch (e: any) { ElMessage.error(e.message || '校验失败')
  } finally { validatingDraft.value = false }
}

async function saveDraft() {
  const d = toolDraft.value
  if (!d.name.trim()) { ElMessage.warning('请填写名称'); return }
  let config: any
  try { config = JSON.parse(d.configText) } catch { ElMessage.error('JSON 不合法'); return }
  savingPkg.value = true
  try {
    await validateToolPackage(config)
    await saveToolPackage({
      id: d.id, name: d.name.trim(), description: d.description.trim(),
      scan_type: d.scan_type as 'all' | 'diff' | 'scan', enabled: d.enabled,
      project_refs: d.projectRefsText.split('\n').map(s => s.trim()).filter(Boolean),
      config,
    })
    await refresh()
    ElMessage.success('已保存')
  } catch (e: any) { ElMessage.error(e.message || '保存失败')
  } finally { savingPkg.value = false }
}

function syncDraftJSON() {
  const names = Object.entries(toolDraft.value.selectedTools).filter(([, v]) => v).map(([n]) => n)
  toolDraft.value.configText = JSON.stringify(makeToolConfig(names), null, 2)
}

function syncDraftSelection() {
  try { toolDraft.value.selectedTools = inferSelectedTools(JSON.parse(toolDraft.value.configText)) } catch { /* config 非合法 JSON 时保留当前选择，忽略解析错误 */ }
}

function inferSelectedTools(config: any) {
  const sel: Record<string, boolean> = {}
  const list = Array.isArray(config) ? config : Array.isArray(config?.tools) ? config.tools : []
  list.forEach((item: any) => { if (item?.name) sel[item.name] = true })
  return sel
}

function makeToolConfig(names: string[]) {
  return names.map((name) => ({
    name, main_task: true, plan_task: name !== 'code_comment',
    definition: { name, description: toolDescription(name), parameters: { type: 'object', properties: {} } },
  }))
}

function toolDescription(name: string) {
  const map: Record<string, string> = {
    file_read: 'Read repository files', file_read_diff: 'Read diff hunks',
    code_search: 'Search code by keyword', file_find: 'Find files by pattern',
    code_comment: 'Create code review comments',
  }
  return map[name] || name
}

function formatVersionTime(value: string) {
  return value ? new Date(value).toLocaleString('zh-CN') : '-'
}

onMounted(refresh)
</script>

<template>
  <el-drawer v-model="model" title="工具包管理" size="720px">
    <div class="tool-drawer">
      <!-- 左栏：工具包列表 -->
      <section class="tool-list" v-loading="loading">
        <div class="tool-list-head">
          <strong>托管工具包</strong>
          <el-button size="small" type="primary" @click="newPackage">新建</el-button>
        </div>
        <div v-for="pkg in toolPackages" :key="pkg.id || pkg.name" class="tool-package-card">
          <div class="pkg-info">
            <strong>{{ pkg.name }}</strong>
            <small>{{ pkg.description || '暂无描述' }}</small>
            <em>{{ pkg.built_in ? '内置' : '自定义' }} · {{ pkg.scan_type || 'all' }} · {{ pkg.enabled ? '启用' : '停用' }}</em>
          </div>
          <div class="pkg-actions">
            <el-switch size="small" :model-value="pkg.enabled" @change="togglePackage(pkg)" />
            <el-button size="small" text @click="editPackage(pkg)">{{ pkg.built_in ? '复制' : '编辑' }}</el-button>
            <el-button v-if="pkg.id" size="small" text @click="showHistory(pkg)">历史</el-button>
            <el-button v-if="!pkg.built_in" size="small" text type="danger" @click="removePackage(pkg)">删除</el-button>
          </div>
        </div>
      </section>

      <!-- 右栏：编辑器 -->
      <section class="tool-editor">
        <div class="tool-list-head">
          <strong>{{ toolDraft.id ? '编辑工具包' : '新建工具包' }}</strong>
          <span>{{ toolDraft.builtInSource ? '来自内置包副本' : '自定义' }}</span>
        </div>
        <el-form label-position="top">
          <div class="tool-editor-grid">
            <el-form-item label="名称"><el-input v-model="toolDraft.name" /></el-form-item>
            <el-form-item label="适用类型">
              <el-select v-model="toolDraft.scan_type">
                <el-option label="全部" value="all" />
                <el-option label="Diff Review" value="diff" />
                <el-option label="全量 Scan" value="scan" />
              </el-select>
            </el-form-item>
          </div>
          <el-form-item label="描述"><el-input v-model="toolDraft.description" type="textarea" :rows="2" /></el-form-item>
          <el-form-item label="项目绑定">
            <el-input v-model="toolDraft.projectRefsText" type="textarea" :rows="2" placeholder="每行一个 provider/repo；留空表示全部项目" />
          </el-form-item>
          <el-form-item label="内置工具">
            <div class="tool-switch-grid">
              <el-checkbox v-for="tool in builtinTools" :key="tool.name" v-model="toolDraft.selectedTools[tool.name]" @change="syncDraftJSON">
                {{ tool.label }}
              </el-checkbox>
            </div>
          </el-form-item>
          <el-form-item label="工具配置 JSON">
            <el-input v-model="toolDraft.configText" type="textarea" :rows="9" @blur="syncDraftSelection" />
          </el-form-item>
          <div class="tool-editor-actions">
            <el-button @click="validateDraft(false)" :loading="validatingDraft">JSON 校验</el-button>
            <el-button type="success" plain @click="validateDraft(true)" :loading="validatingDraft">OCR dry-run</el-button>
            <el-button type="primary" @click="saveDraft" :loading="savingPkg">保存工具包</el-button>
          </div>
        </el-form>
      </section>
    </div>
  </el-drawer>

  <!-- 版本历史 -->
  <el-dialog v-model="historyDialog" title="工具包版本历史" width="680px">
    <div class="history-title">
      <strong>{{ historyPkg?.name || '工具包' }}</strong>
      <span>保存时自动记录修改前的版本</span>
    </div>
    <div v-loading="historyLoading" class="version-list">
      <div v-for="version in toolPackageVersions" :key="version.version" class="version-item">
        <div class="version-head">
          <strong>v{{ version.version }}</strong>
          <span>{{ formatVersionTime(version.created_at) }}</span>
        </div>
        <pre v-if="version.snapshot?.config">{{ JSON.stringify(version.snapshot.config, null, 2) }}</pre>
        <el-button size="small" type="warning" plain :loading="historyRestoring" @click="restoreVersion(version.version)">
          恢复到此版本
        </el-button>
      </div>
      <el-empty v-if="!toolPackageVersions.length && !historyLoading" description="暂无历史版本" />
    </div>
  </el-dialog>
</template>

<style scoped>
.tool-drawer {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
  height: 100%;
}
.tool-list-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}
.tool-list-head strong { font-size: 14px; color: var(--app-text); }
.tool-package-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px;
  margin-bottom: 8px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
}
.pkg-info strong { display: block; font-size: 13px; }
.pkg-info small { display: block; font-size: 11px; color: #999; margin: 2px 0; }
.pkg-info em { font-size: 11px; color: #aaa; font-style: normal; }
.pkg-actions { display: flex; align-items: center; gap: 4px; flex-shrink: 0; }
.tool-editor-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
.tool-switch-grid { display: flex; flex-wrap: wrap; gap: 8px 16px; }
.tool-editor-actions { display: flex; gap: 8px; }
.history-title { margin-bottom: 12px; }
.history-title strong { font-size: 14px; }
.history-title span { margin-left: 8px; font-size: 12px; color: #999; }
.version-item { padding: 10px 0; border-top: 1px solid var(--app-border); }
.version-head { display: flex; gap: 12px; align-items: center; margin-bottom: 6px; font-size: 13px; }
.version-item pre { font-size: 11px; max-height: 120px; overflow: auto; background: #f5f5f5; padding: 8px; border-radius: 4px; }
</style>
