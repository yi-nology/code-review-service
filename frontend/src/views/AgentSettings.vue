<template>
  <AppLayout>
    <PageShell>
      <PageHeader
        eyebrow="REVIEWER AGENTS"
        title="审查员"
        description="维护审查员能力库。项目未配置团队时按技术栈自动匹配启用的审查员；可在「全局规则」页给审查员引用规则集。"
      >
        <template #actions>
          <el-dropdown trigger="click" @command="(cmd: string) => applyPreset(presets.find((p) => p.name === cmd)!)">
            <el-button plain>
              <el-icon><MagicStick /></el-icon>从模板创建
              <el-icon class="el-icon--right"><ArrowDown /></el-icon>
            </el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item v-for="p in presets" :key="p.name" :command="p.name">
                  <div class="preset-option">
                    <strong>{{ p.display_name }}</strong>
                    <small>{{ p.description }}</small>
                  </div>
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </template>
      </PageHeader>

      <StatStrip :items="agentStats" />

      <div class="content-grid">
        <!-- 左栏：审查员列表 -->
        <aside class="agent-sidebar panel-shell">
          <div class="panel-heading">
            <div>
              <strong>审查员池</strong>
              <span>按优先级排序</span>
            </div>
            <el-button circle text aria-label="刷新" @click="fetchAgents" :loading="loading">
              <el-icon><Refresh /></el-icon>
            </el-button>
          </div>

          <el-button type="primary" plain class="new-agent-btn" @click="startCreate">
            <el-icon><Plus /></el-icon>新建审查员
          </el-button>

          <el-input v-if="agents.length" v-model="keyword" class="agent-search" clearable placeholder="搜索名称或标签">
            <template #prefix><el-icon><Search /></el-icon></template>
          </el-input>

          <el-empty v-if="!loading && filteredAgents.length === 0 && agents.length === 0" description="暂无审查员">
            <el-button type="primary" @click="applyPreset(presets[0])">从推荐模板开始</el-button>
          </el-empty>

          <div v-else v-loading="loading" class="agent-list">
            <button
              v-for="agent in filteredAgents"
              :key="agent.id"
              class="agent-item"
              :class="{ active: selectedAgent?.id === agent.id, disabled: !agent.enabled }"
              type="button"
              @click="editAgent(agent)"
            >
              <span class="agent-title">
                <strong>{{ agent.display_name }}</strong>
                <span class="priority-badge">P{{ agent.priority ?? 0 }}</span>
                <span v-if="agent.is_default" class="default-badge">默认</span>
              </span>
              <span class="agent-meta">{{ agent.name }} · {{ agent.enabled ? '启用' : '禁用' }}</span>
              <span v-if="agent.tags?.length" class="tag-row">
                <span v-for="tag in agent.tags.slice(0, 3)" :key="tag">{{ tag }}</span>
              </span>
            </button>
          </div>
        </aside>

        <!-- 右栏：编辑器（标签页） -->
        <main class="editor-column panel-shell">
          <div class="editor-header">
            <div class="editor-title">
              <span class="editor-state">{{ editingExisting ? '编辑中' : '新建' }}</span>
              <strong>{{ form.display_name || form.name || '未命名审查员' }}</strong>
              <small v-if="dirty" class="dirty-tag">有未保存修改</small>
            </div>
            <div class="editor-toggles">
              <label class="toggle-chip">
                <span>启用</span>
                <el-switch v-model="form.enabled" size="small" />
              </label>
              <label class="toggle-chip">
                <span>默认</span>
                <el-switch v-model="form.is_default" size="small" />
              </label>
            </div>
          </div>

          <div class="editor-tabs">
            <el-tabs v-model="activeTab">
              <el-tab-pane label="基本信息" name="basic">
                <div class="tab-body">
                  <div class="field-grid two-columns">
                    <label class="field-block">
                      <span>标识名</span>
                      <el-input v-model="form.name" placeholder="security" :disabled="editingExisting" />
                      <small class="field-hint">小写字母开头，创建后不可改</small>
                    </label>
                    <label class="field-block">
                      <span>显示名称</span>
                      <el-input v-model="form.display_name" placeholder="安全审查员" />
                    </label>
                  </div>
                  <label class="field-block">
                    <span>职责描述</span>
                    <el-input v-model="form.description" placeholder="说明这个审查员负责什么风险面" />
                  </label>
                  <div class="field-grid two-columns">
                    <label class="field-block">
                      <span>全局优先级</span>
                      <el-input-number v-model="form.priority" :min="0" :max="100" />
                      <small class="field-hint">数字越小越靠前；项目策略可覆盖</small>
                    </label>
                    <label class="field-block">
                      <span>能力标签</span>
                      <div class="tags-input">
                        <el-tag v-for="tag in form.tags" :key="tag" closable @close="removeTag(tag)">{{ tag }}</el-tag>
                        <el-input
                          v-if="tagInputVisible"
                          ref="tagInputRef"
                          v-model="tagInputValue"
                          size="small"
                          class="tag-input"
                          @keyup.enter="addTag"
                          @blur="addTag"
                        />
                        <el-button v-else size="small" @click="showTagInput">+ 添加</el-button>
                      </div>
                      <small class="field-hint">含 always 标签的审查员在所有项目自动选中</small>
                    </label>
                  </div>
                </div>
              </el-tab-pane>

              <el-tab-pane label="审查侧重点" name="focus">
                <p class="tab-hint">这段内容作为模型审查背景，决定审查员从哪个角度看代码。</p>
                <el-input
                  v-model="form.background"
                  type="textarea"
                  :autosize="{ minRows: 8, maxRows: 18 }"
                  placeholder="例如：重点关注鉴权绕过、越权访问、敏感信息泄露、输入校验和不安全配置。"
                />
              </el-tab-pane>

              <el-tab-pane label="引用规则包" name="rules">
                <p class="tab-hint">勾选要应用的规则集，会与全局规则一起生效。规则集在「全局规则」页集中维护，改了自动同步，无需副本。</p>
                <RulePackagePicker v-model="form.rule_package_ids" />
              </el-tab-pane>
            </el-tabs>

            <!-- 校验 + 危险操作（折叠到标签页下方） -->
            <div class="editor-footer-info">
              <div v-if="validationIssues.length" class="issues-inline">
                <strong>需处理（{{ validationIssues.length }}）：</strong>
                <span v-for="(issue, i) in validationIssues.slice(0, 3)" :key="i" class="issue-chip">{{ issue }}</span>
                <span v-if="validationIssues.length > 3" class="issue-more">等 {{ validationIssues.length }} 项</span>
              </div>
              <div v-else class="issues-inline ok">
                <el-icon><CircleCheckFilled /></el-icon> 校验通过，可保存
              </div>
              <el-button v-if="editingExisting" type="danger" plain size="small" @click="handleDeleteSelected">
                <el-icon><Delete /></el-icon>删除审查员
              </el-button>
            </div>
          </div>
        </main>
      </div>

      <!-- 底部 sticky 保存条 -->
      <div class="sticky-save-bar" :class="{ dirty }">
        <div class="save-bar-info">
          <strong>{{ form.display_name || form.name || '未命名审查员' }}</strong>
          <span>P{{ form.priority }} · {{ form.enabled ? '启用' : '禁用' }}{{ form.is_default ? ' · 默认' : '' }}</span>
        </div>
        <div class="save-bar-actions">
          <el-button v-if="dirty" plain size="small" @click="discardChanges">放弃修改</el-button>
          <el-button type="primary" :loading="submitting" :disabled="validationIssues.length > 0" @click="handleSave">
            <el-icon><CircleCheckFilled /></el-icon>保存审查员
          </el-button>
        </div>
      </div>
    </PageShell>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref } from 'vue'
import { ArrowDown, CircleCheckFilled, Delete, MagicStick, Plus, Refresh, Search, User, Star, WarningFilled } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'
import AppLayout from '@/components/layout/AppLayout.vue'
import PageShell from '@/components/common/PageShell.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import StatStrip from '@/components/common/StatStrip.vue'
import RulePackagePicker from '@/components/RulePackagePicker.vue'
import { listReviewerAgents, createReviewerAgent, updateReviewerAgent, deleteReviewerAgent } from '@/api/config'
import type { ReviewerAgent } from '@/types/config'
import { AGENT_PRESETS, type AgentPreset } from '@/constants/agents'

const presets = AGENT_PRESETS

const loading = ref(true)
const submitting = ref(false)
const agents = ref<ReviewerAgent[]>([])
const selectedAgent = ref<ReviewerAgent | null>(null)
const keyword = ref('')
const originalJson = ref('')
const activeTab = ref('basic')
const tagInputVisible = ref(false)
const tagInputValue = ref('')
const tagInputRef = ref()

const form = reactive({
  name: '',
  display_name: '',
  description: '',
  background: '',
  enabled: true,
  is_default: false,
  priority: 20,
  tags: [] as string[],
  rule_package_ids: [] as number[],
})

const enabledCount = computed(() => agents.value.filter((agent) => agent.enabled).length)
const defaultCount = computed(() => agents.value.filter((agent) => agent.is_default).length)

const agentStats = computed(() => [
  { icon: User, label: '审查员总数', value: agents.value.length, tone: 'primary' as const },
  { icon: CircleCheckFilled, label: '已启用', value: enabledCount.value, tone: 'success' as const },
  { icon: Star, label: '默认审查员', value: defaultCount.value, tone: 'warning' as const },
  { icon: WarningFilled, label: '待处理问题', value: validationIssues.value.length, tone: validationIssues.value.length > 0 ? ('danger' as const) : ('info' as const) },
])
const editingExisting = computed(() => !!selectedAgent.value)
const dirty = computed(() => JSON.stringify(payload.value) !== originalJson.value)

const filteredAgents = computed(() => {
  const value = keyword.value.trim().toLowerCase()
  const sorted = [...agents.value].sort((a, b) => (a.priority ?? 0) - (b.priority ?? 0) || a.id - b.id)
  if (!value) return sorted
  return sorted.filter((agent) => {
    return [
      agent.name,
      agent.display_name,
      agent.description,
      ...(agent.tags || []),
    ].some((part) => String(part || '').toLowerCase().includes(value))
  })
})

const payload = computed(() => ({
  name: form.name.trim(),
  display_name: form.display_name.trim(),
  description: form.description.trim(),
  background: form.background.trim(),
  enabled: form.enabled,
  is_default: form.is_default,
  priority: form.priority,
  tags: form.tags.map((tag) => tag.trim()).filter(Boolean),
  rule_package_ids: [...form.rule_package_ids],
}))

const validationIssues = computed(() => {
  const issues: string[] = []
  if (!payload.value.name) issues.push('请输入标识名')
  if (payload.value.name && !/^[a-z][a-z0-9_-]*$/.test(payload.value.name)) {
    issues.push('标识名只能使用小写字母、数字、下划线和短横线，并以字母开头')
  }
  if (!payload.value.display_name) issues.push('请输入显示名称')
  if (!payload.value.background) issues.push('请输入审查侧重点')
  if (payload.value.background.length > 2000) issues.push('审查侧重点过长，请控制在 2000 字以内')
  const duplicatedName = agents.value.some((agent) => {
    return agent.name === payload.value.name && agent.id !== selectedAgent.value?.id
  })
  if (duplicatedName) issues.push('标识名已存在')
  return issues
})

async function fetchAgents() {
  loading.value = true
  try {
    agents.value = await listReviewerAgents()
    if (selectedAgent.value) {
      const latest = agents.value.find((agent) => agent.id === selectedAgent.value?.id)
      if (latest) editAgent(latest)
    } else if (agents.value.length > 0) {
      editAgent(filteredAgents.value[0])
    } else {
      startCreate()
    }
  } catch (e) {
    console.error('Failed to load agents:', e)
  } finally {
    loading.value = false
  }
}

function editAgent(agent: ReviewerAgent) {
  selectedAgent.value = agent
  form.name = agent.name
  form.display_name = agent.display_name
  form.description = agent.description || ''
  form.background = agent.background || ''
  form.enabled = agent.enabled
  form.is_default = agent.is_default ?? false
  form.priority = agent.priority ?? 20
  form.tags = agent.tags ? [...agent.tags] : []
  form.rule_package_ids = agent.rule_package_ids ? [...agent.rule_package_ids] : []
  activeTab.value = 'basic'
  syncOriginal()
}

function startCreate() {
  selectedAgent.value = null
  form.name = ''
  form.display_name = ''
  form.description = ''
  form.background = ''
  form.enabled = true
  form.is_default = false
  form.priority = 20
  form.tags = []
  form.rule_package_ids = []
  activeTab.value = 'basic'
  syncOriginal()
}

function applyPreset(preset: AgentPreset) {
  selectedAgent.value = null
  form.name = preset.name
  form.display_name = preset.display_name
  form.description = preset.description
  form.background = preset.background
  form.enabled = true
  form.is_default = false
  form.priority = preset.priority
  form.tags = [...preset.tags]
  form.rule_package_ids = []
  activeTab.value = 'basic'
  syncOriginal()
}

// 放弃修改：有选中则回到已保存版本，无选中则重置为空表单。
function discardChanges() {
  if (selectedAgent.value) {
    editAgent(selectedAgent.value)
  } else {
    startCreate()
  }
}

async function handleSave() {
  if (validationIssues.value.length > 0) {
    ElMessage.warning('请先处理校验中的问题')
    return
  }
  submitting.value = true
  try {
    let saved: ReviewerAgent
    if (selectedAgent.value) {
      saved = await updateReviewerAgent(selectedAgent.value.id, {
        display_name: payload.value.display_name,
        description: payload.value.description,
        background: payload.value.background,
        enabled: payload.value.enabled,
        is_default: payload.value.is_default,
        priority: payload.value.priority,
        tags: payload.value.tags,
        rule_package_ids: payload.value.rule_package_ids,
      })
      ElMessage.success('审查员已更新')
    } else {
      saved = await createReviewerAgent(payload.value)
      ElMessage.success('审查员已创建')
    }
    await fetchAgents()
    const latest = agents.value.find((agent) => agent.id === saved.id || agent.name === saved.name)
    if (latest) editAgent(latest)
  } catch (e: any) {
    ElMessage.error(e?.message || '保存失败')
  } finally {
    submitting.value = false
  }
}

async function handleDeleteSelected() {
  if (!selectedAgent.value) return
  try {
    await ElMessageBox.confirm(`确定删除 ${selectedAgent.value.display_name}？`, '确认删除')
    await deleteReviewerAgent(selectedAgent.value.id)
    ElMessage.success('审查员已删除')
    selectedAgent.value = null
    await fetchAgents()
  } catch {
    // 用户取消
  }
}

function removeTag(tag: string) {
  form.tags = form.tags.filter((item) => item !== tag)
}

function showTagInput() {
  tagInputVisible.value = true
  nextTick(() => tagInputRef.value?.focus())
}

function addTag() {
  const value = tagInputValue.value.trim()
  if (value && !form.tags.includes(value)) {
    form.tags.push(value)
  }
  tagInputVisible.value = false
  tagInputValue.value = ''
}

function syncOriginal() {
  originalJson.value = JSON.stringify(payload.value)
}

onMounted(fetchAgents)
</script>

<style scoped>
.preset-option {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.preset-option strong {
  font-size: 13px;
}

.preset-option small {
  color: var(--app-muted);
  font-size: 11px;
}

/* 两栏布局 */
.content-grid {
  display: grid;
  grid-template-columns: 300px minmax(0, 1fr);
  gap: 16px;
  align-items: start;
}

.panel-shell {
  border: 1px solid var(--app-border);
  border-radius: var(--card-radius-lg);
  background: var(--app-surface);
  box-shadow: var(--app-shadow);
}

/* 左栏列表 */
.agent-sidebar {
  position: sticky;
  top: 18px;
  padding-bottom: 12px;
}

.panel-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 54px;
  padding: 15px 16px 12px;
  border-bottom: 1px solid var(--app-border);
}

.panel-heading strong {
  display: block;
  color: var(--app-text);
  font-size: 15px;
}

.panel-heading span {
  color: var(--app-muted);
  font-size: 12px;
}

.new-agent-btn {
  margin: 12px 14px 0;
  width: calc(100% - 28px);
}

.agent-search {
  margin: 10px 14px 0;
  width: calc(100% - 28px);
}

.agent-list {
  display: grid;
  gap: 7px;
  align-content: start;
  max-height: calc(100vh - 320px);
  min-height: 280px;
  padding: 10px 10px 4px;
  overflow: auto;
}

.agent-item {
  width: 100%;
  padding: 11px 12px;
  border: 1px solid transparent;
  border-radius: var(--card-radius);
  background: #f7f8fc;
  text-align: left;
  cursor: pointer;
  transition: 160ms ease;
}

.agent-item:hover,
.agent-item.active {
  border-color: var(--app-primary);
  background: var(--app-surface);
  box-shadow: var(--app-shadow);
}

.agent-item.disabled {
  opacity: 0.6;
}

.agent-title {
  display: flex;
  align-items: center;
  gap: 6px;
}

.agent-title strong {
  flex: 1;
  overflow: hidden;
  color: var(--app-text);
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.priority-badge {
  flex-shrink: 0;
  padding: 0 6px;
  border-radius: 5px;
  color: #5656c5;
  background: #f0f1ff;
  font-size: 10px;
  font-weight: 800;
}

.default-badge {
  flex-shrink: 0;
  padding: 0 5px;
  border-radius: 5px;
  color: #0f7b6c;
  background: #e6faf5;
  font-size: 10px;
  font-weight: 700;
}

.agent-meta {
  display: block;
  margin-top: 5px;
  color: var(--app-muted);
  font-size: 12px;
}

.tag-row {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-top: 7px;
}

.tag-row span {
  padding: 1px 6px;
  border-radius: 999px;
  color: #4c5570;
  background: #f2f4f8;
  font-size: 10px;
}

/* 右栏编辑器 */
.editor-column {
  min-width: 0;
  overflow: hidden;
}

.editor-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 64px;
  padding: 16px 20px;
  border-bottom: 1px solid var(--app-border);
  background: #fbfcfe;
}

.editor-title {
  display: flex;
  align-items: baseline;
  gap: 10px;
  flex-wrap: wrap;
}

.editor-state {
  display: inline-flex;
  min-height: 22px;
  align-items: center;
  padding: 0 8px;
  border-radius: 6px;
  color: #31536f;
  background: #eaf2f8;
  font-size: 11px;
  font-weight: 800;
}

.editor-title strong {
  color: var(--app-text);
  font-size: 18px;
  overflow-wrap: anywhere;
}

.dirty-tag {
  padding: 2px 8px;
  border-radius: 6px;
  background: #fef3c7;
  color: #b45309;
  font-size: 11px;
  font-weight: 600;
}

.editor-toggles {
  display: flex;
  gap: 14px;
  flex-shrink: 0;
}

.toggle-chip {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--app-muted);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
}

.editor-tabs {
  padding: 8px 20px 20px;
}

.editor-tabs :deep(.el-tabs__header) {
  margin-bottom: 14px;
}

.tab-hint {
  margin: 0 0 14px;
  color: var(--app-muted);
  font-size: 13px;
}

.tab-body {
  display: grid;
  gap: 16px;
}

.field-grid {
  display: grid;
  gap: 16px;
}

.field-grid.two-columns {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.field-block {
  display: grid;
  gap: 7px;
}

.field-block > span {
  color: #4b5568;
  font-size: 12px;
  font-weight: 700;
}

.field-hint {
  color: var(--app-muted);
  font-size: 11px;
  font-weight: 400;
}

.tags-input {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: center;
}

.tag-input {
  width: 120px;
}

/* 校验 + 危险操作 */
.editor-footer-info {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-top: 16px;
  padding: 12px 16px;
  border: 1px solid var(--app-border);
  border-radius: var(--card-radius);
  background: #fafbfc;
  flex-wrap: wrap;
}

.issues-inline {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  font-size: 13px;
  color: #b45309;
}

.issues-inline.ok {
  color: #16a34a;
}

.issues-inline strong {
  font-weight: 600;
}

.issue-chip {
  padding: 2px 8px;
  border-radius: 6px;
  background: #fff7ed;
  font-size: 12px;
}

.issue-more {
  color: var(--app-muted);
  font-size: 12px;
}

/* sticky 保存条 */
.sticky-save-bar {
  position: sticky;
  bottom: 0;
  z-index: 9;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-top: 16px;
  padding: 12px 20px;
  border: 1px solid var(--app-border);
  border-radius: var(--card-radius);
  background: var(--app-surface);
  box-shadow: var(--shadow-sticky);
  transition: border-color 200ms ease;
}

.sticky-save-bar.dirty {
  border-color: #f59e0b;
  background: #fffbeb;
}

.save-bar-info {
  display: flex;
  align-items: baseline;
  gap: 12px;
  flex-wrap: wrap;
  min-width: 0;
}

.save-bar-info strong {
  font-size: 14px;
  color: var(--app-text);
}

.save-bar-info span {
  color: var(--app-muted);
  font-size: 12px;
}

.save-bar-actions {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
}

@media (max-width: 980px) {
  .content-grid {
    grid-template-columns: 1fr;
  }

  .agent-sidebar {
    position: static;
  }

  .field-grid.two-columns {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 720px) {
  .editor-header {
    flex-direction: column;
    align-items: flex-start;
  }
}

@media print {
  .sticky-save-bar {
    display: none;
  }
}
</style>
