<template>
  <AppLayout>
    <div class="agent-settings-page">
      <header class="page-header">
        <div>
          <div class="eyebrow">REVIEWER AGENTS</div>
          <h1>审查员</h1>
          <p>维护全局审查员能力库。项目未配置团队时，会使用所有启用的审查员；项目策略可覆盖启用范围和优先级。</p>
        </div>
        <div class="header-actions">
          <el-button plain @click="startCreate">
            <el-icon><Plus /></el-icon>
            新建审查员
          </el-button>
          <el-button type="primary" @click="handleSave" :loading="submitting" :disabled="validationIssues.length > 0">
            保存审查员
          </el-button>
        </div>
      </header>

      <section class="summary-grid">
        <div class="summary-card">
          <span>审查员总数</span>
          <strong>{{ agents.length }}</strong>
        </div>
        <div class="summary-card">
          <span>全局启用</span>
          <strong>{{ enabledCount }}</strong>
        </div>
        <div class="summary-card">
          <span>能力标签</span>
          <strong>{{ tagCount }}</strong>
        </div>
        <div class="summary-card warning" :class="{ active: validationIssues.length > 0 }">
          <span>待处理问题</span>
          <strong>{{ validationIssues.length }}</strong>
        </div>
      </section>

      <div class="content-grid">
        <aside class="agent-sidebar">
          <el-card class="panel-card">
            <template #header>
              <div class="card-header">
                <div>
                  <strong>审查员池</strong>
                  <span>按优先级从小到大执行</span>
                </div>
                <el-button text @click="fetchAgents" :loading="loading">
                  <el-icon><Refresh /></el-icon>
                </el-button>
              </div>
            </template>

            <el-input v-model="keyword" class="agent-search" clearable placeholder="搜索名称、标识或标签">
              <template #prefix><el-icon><Search /></el-icon></template>
            </el-input>

            <el-empty v-if="!loading && filteredAgents.length === 0" description="暂无审查员">
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
                  <span v-if="agent.is_default" class="default-badge">DEFAULT</span>
                </span>
                <span class="agent-meta">
                  {{ agent.name }} / {{ agent.enabled ? '启用' : '禁用' }}
                </span>
                <span v-if="agent.tags?.length" class="tag-row">
                  <span v-for="tag in agent.tags.slice(0, 3)" :key="tag">{{ tag }}</span>
                </span>
              </button>
            </div>
          </el-card>
        </aside>

        <main class="editor-column">
          <el-card class="panel-card">
            <template #header>
              <div class="card-header">
                <div>
                  <strong>{{ selectedAgent ? '编辑审查员' : '新建审查员' }}</strong>
                  <span>{{ dirty ? '有未保存修改' : '当前表单已同步' }}</span>
                </div>
                <el-switch
                  v-model="form.enabled"
                  inline-prompt
                  active-text="启用"
                  inactive-text="禁用"
                />
                <el-switch
                  v-model="form.is_default"
                  inline-prompt
                  active-text="默认"
                  inactive-text="非默认"
                />
              </div>
            </template>

            <div class="editor-layout">
              <section class="form-section">
                <div class="section-title">
                  <strong>1. 基本信息</strong>
                  <span>标识名创建后不可修改，用于项目配置和审查结果归档</span>
                </div>
                <div class="field-grid two-columns">
                  <label class="field-block">
                    <span>标识名</span>
                    <el-input v-model="form.name" placeholder="security" :disabled="!!selectedAgent" />
                  </label>
                  <label class="field-block">
                    <span>显示名称</span>
                    <el-input v-model="form.display_name" placeholder="安全审查员" />
                  </label>
                </div>
                <label class="field-block full">
                  <span>职责描述</span>
                  <el-input v-model="form.description" placeholder="说明这个审查员负责什么风险面" />
                </label>
              </section>

              <section class="form-section">
                <div class="section-title">
                  <strong>2. 执行策略</strong>
                  <span>全局优先级用于默认团队；项目策略里可单独覆盖</span>
                </div>
                <div class="field-grid two-columns">
                  <label class="field-block">
                    <span>全局优先级</span>
                    <el-input-number v-model="form.priority" :min="0" :max="100" />
                  </label>
                  <label class="field-block">
                    <span>能力标签</span>
                    <div class="tags-input">
                      <el-tag
                        v-for="tag in form.tags"
                        :key="tag"
                        closable
                        @close="removeTag(tag)"
                      >
                        {{ tag }}
                      </el-tag>
                      <el-input
                        v-if="tagInputVisible"
                        ref="tagInputRef"
                        v-model="tagInputValue"
                        size="small"
                        class="tag-input"
                        @keyup.enter="addTag"
                        @blur="addTag"
                      />
                      <el-button v-else size="small" @click="showTagInput">添加标签</el-button>
                    </div>
                  </label>
                </div>
              </section>

              <section class="form-section">
                <div class="section-title">
                  <strong>3. 审查侧重点</strong>
                  <span>这段内容会作为模型审查背景，决定审查员从哪个角度看代码</span>
                </div>
                <el-input
                  v-model="form.background"
                  type="textarea"
                  :rows="9"
                  placeholder="例如：重点关注鉴权绕过、越权访问、敏感信息泄露、输入校验和不安全配置。"
                />
              </section>
            </div>
          </el-card>
        </main>

        <aside class="inspector-column">
          <el-card class="panel-card">
            <template #header>
              <div class="card-header compact">
                <strong>保存前检查</strong>
              </div>
            </template>
            <div v-if="validationIssues.length === 0" class="check-ok">
              <el-icon><CircleCheckFilled /></el-icon>
              可以保存
            </div>
            <ul v-else class="issue-list">
              <li v-for="issue in validationIssues" :key="issue">{{ issue }}</li>
            </ul>
          </el-card>

          <el-card class="panel-card">
            <template #header>
              <div class="card-header compact">
                <strong>预设模板</strong>
              </div>
            </template>
            <div class="preset-list">
              <button
                v-for="preset in presets"
                :key="preset.name"
                type="button"
                class="preset-item"
                @click="applyPreset(preset)"
              >
                <strong>{{ preset.display_name }}</strong>
                <span>{{ preset.description }}</span>
              </button>
            </div>
          </el-card>

          <el-card class="panel-card">
            <template #header>
              <div class="card-header compact">
                <strong>生效说明</strong>
              </div>
            </template>
            <div class="help-content">
              <p>项目未配置审查员时，系统按仓库技术栈（Go/Java/Node 等）自动推断匹配的审查员。</p>
              <p>技术栈推断无匹配时，运行标记为「默认」的审查员；仍无则运行全部启用的审查员。</p>
              <p>项目策略配置审查员后，完全覆盖上述逻辑，只运行项目中启用的审查员。</p>
              <p>标签含 <code>always</code> 的审查员在任何项目中都会被自动选中。</p>
              <p>优先级数字越小越靠前；项目优先级会覆盖这里的全局优先级。</p>
            </div>
          </el-card>

          <el-card v-if="selectedAgent" class="panel-card danger-card">
            <template #header>
              <div class="card-header compact">
                <strong>危险操作</strong>
              </div>
            </template>
            <p>删除后，项目中引用该审查员的配置将无法继续创建对应审查角色。</p>
            <el-button type="danger" plain @click="handleDeleteSelected">删除当前审查员</el-button>
          </el-card>
        </aside>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref } from 'vue'
import { CircleCheckFilled, Plus, Refresh, Search } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'
import AppLayout from '@/components/layout/AppLayout.vue'
import { listReviewerAgents, createReviewerAgent, updateReviewerAgent, deleteReviewerAgent } from '@/api/config'
import type { ReviewerAgent } from '@/types/config'

type AgentPreset = {
  name: string
  display_name: string
  description: string
  background: string
  tags: string[]
  priority: number
}

const presets: AgentPreset[] = [
  {
    name: 'security',
    display_name: '安全审查员',
    description: '关注权限、输入校验、敏感数据与常见漏洞',
    background: `重点关注安全问题：
- SQL 注入、XSS、CSRF、SSRF 等常见漏洞
- 认证与授权机制是否存在绕过
- 输入验证、输出编码与数据净化
- 密码、Token、密钥等敏感数据处理
- 越权访问和不安全默认配置`,
    tags: ['always', 'security', 'auth', 'risk'],
    priority: 10,
  },
  {
    name: 'backend',
    display_name: '后端审查员',
    description: '关注 API、数据库、事务、错误处理和并发安全',
    background: `重点关注后端代码质量：
- API 设计一致性和错误语义
- 数据库查询性能、索引和 N+1 问题
- 事务边界、并发安全和资源释放
- 日志、监控和可观测性
- 边界条件与失败路径处理`,
    tags: ['stack:go', 'stack:python', 'stack:java', 'backend', 'api', 'database'],
    priority: 20,
  },
  {
    name: 'frontend',
    display_name: '前端审查员',
    description: '关注组件设计、交互状态、响应式和可维护性',
    background: `重点关注前端代码质量：
- Vue/React 组件职责划分和可维护性
- 表单校验、加载态、空状态和错误反馈
- 响应式布局、可访问性和用户体验
- 状态管理、接口调用和缓存策略
- 前端性能和代码分割`,
    tags: ['stack:node', 'stack:frontend', 'frontend', 'ux', 'vue'],
    priority: 30,
  },
  {
    name: 'performance',
    display_name: '性能审查员',
    description: '关注性能瓶颈、缓存、内存和扩展性',
    background: `重点关注性能问题：
- 算法复杂度和热点路径
- 内存使用、泄漏风险和资源释放
- 缓存策略、批处理和异步执行
- 数据库索引与慢查询风险
- 高并发场景下的扩展性`,
    tags: ['always', 'performance', 'scalability'],
    priority: 40,
  },
]

const loading = ref(true)
const submitting = ref(false)
const agents = ref<ReviewerAgent[]>([])
const selectedAgent = ref<ReviewerAgent | null>(null)
const keyword = ref('')
const originalJson = ref('')
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
})

const enabledCount = computed(() => agents.value.filter((agent) => agent.enabled).length)
const tagCount = computed(() => new Set(agents.value.flatMap((agent) => agent.tags || [])).size)
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
  syncOriginal()
}

async function handleSave() {
  if (validationIssues.value.length > 0) {
    ElMessage.warning('请先处理保存前检查中的问题')
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
.agent-settings-page {
  width: min(1480px, 100%);
  margin: 0 auto;
  padding: 34px 36px 48px;
}

.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 24px;
  margin-bottom: 22px;
}

.page-header h1 {
  margin: 4px 0 8px;
  color: var(--app-text);
  font-size: 30px;
  letter-spacing: -0.6px;
}

.page-header p {
  max-width: 760px;
  margin: 0;
  color: var(--app-muted);
  font-size: 14px;
  line-height: 1.7;
}

.eyebrow {
  color: var(--app-primary);
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 1.4px;
}

.header-actions {
  display: flex;
  flex: 0 0 auto;
  gap: 10px;
}

.summary-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
  margin-bottom: 18px;
}

.summary-card {
  min-height: 92px;
  padding: 18px 20px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: var(--app-surface);
}

.summary-card span,
.card-header span,
.section-title span {
  display: block;
  color: var(--app-muted);
  font-size: 12px;
}

.summary-card strong {
  display: block;
  margin-top: 8px;
  color: var(--app-text);
  font-size: 28px;
  line-height: 1;
}

.summary-card.warning.active strong {
  color: var(--app-danger);
}

.content-grid {
  display: grid;
  grid-template-columns: 330px minmax(0, 1fr) 340px;
  gap: 18px;
  align-items: start;
}

.panel-card {
  border-radius: 8px;
  box-shadow: var(--app-shadow);
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.card-header strong {
  display: block;
  color: var(--app-text);
  font-size: 15px;
}

.card-header.compact {
  min-height: 28px;
}

.agent-search {
  margin-bottom: 12px;
}

.agent-list {
  display: grid;
  gap: 8px;
  max-height: 680px;
  overflow: auto;
}

.agent-item,
.preset-item {
  width: 100%;
  padding: 13px 14px;
  border: 1px solid #edf0f5;
  border-radius: 8px;
  background: #fff;
  text-align: left;
  cursor: pointer;
  transition: 160ms ease;
}

.agent-item:hover,
.agent-item.active,
.preset-item:hover {
  border-color: #cfd3ff;
  background: #fbfbff;
}

.agent-item.disabled {
  opacity: 0.62;
}

.agent-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.agent-title strong {
  overflow: hidden;
  color: var(--app-text);
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.priority-badge {
  display: inline-flex;
  min-height: 22px;
  align-items: center;
  padding: 0 7px;
  border-radius: 6px;
  color: #5656c5;
  background: #f0f1ff;
  font-size: 11px;
  font-weight: 800;
}

.default-badge {
  display: inline-flex;
  min-height: 18px;
  align-items: center;
  padding: 0 6px;
  border-radius: 6px;
  color: #0f7b6c;
  background: #e6faf5;
  font-size: 10px;
  font-weight: 700;
}

.agent-meta {
  display: block;
  margin-top: 6px;
  color: var(--app-muted);
  font-size: 12px;
}

.tag-row,
.tags-input {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.tag-row {
  margin-top: 8px;
}

.tag-row span {
  padding: 2px 7px;
  border-radius: 999px;
  color: #4c5570;
  background: #f2f4f8;
  font-size: 11px;
}

.editor-layout {
  display: grid;
  gap: 24px;
}

.form-section {
  padding-bottom: 22px;
  border-bottom: 1px solid var(--app-border);
}

.form-section:last-child {
  padding-bottom: 0;
  border-bottom: none;
}

.section-title {
  margin-bottom: 14px;
}

.section-title strong {
  display: block;
  color: var(--app-text);
  font-size: 15px;
}

.section-title span {
  margin-top: 3px;
}

.field-grid {
  display: grid;
  gap: 14px;
}

.field-grid.two-columns {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.field-block {
  display: grid;
  gap: 8px;
}

.field-block.full {
  margin-top: 14px;
}

.field-block > span {
  color: #4b5568;
  font-size: 12px;
  font-weight: 700;
}

.tag-input {
  width: 120px;
}

.check-ok {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--app-success);
  font-size: 13px;
  font-weight: 700;
}

.issue-list {
  margin: 0;
  padding-left: 18px;
  color: var(--app-danger);
  font-size: 12px;
  line-height: 1.8;
}

.preset-list {
  display: grid;
  gap: 8px;
}

.preset-item strong,
.preset-item span {
  display: block;
}

.preset-item strong {
  color: var(--app-text);
  font-size: 13px;
}

.preset-item span {
  margin-top: 5px;
  color: var(--app-muted);
  font-size: 12px;
  line-height: 1.5;
}

.help-content {
  display: grid;
  gap: 10px;
}

.help-content p,
.danger-card p {
  margin: 0;
  color: var(--app-muted);
  font-size: 12px;
  line-height: 1.7;
}

.danger-card p {
  margin-bottom: 12px;
}

@media (max-width: 1280px) {
  .content-grid {
    grid-template-columns: 310px minmax(0, 1fr);
  }

  .inspector-column {
    grid-column: 1 / -1;
  }
}

@media (max-width: 900px) {
  .page-header {
    display: block;
  }

  .header-actions {
    margin-top: 16px;
  }

  .summary-grid,
  .content-grid,
  .field-grid.two-columns {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 720px) {
  .agent-settings-page {
    padding: 22px 14px 36px;
  }

  .page-header h1 {
    font-size: 24px;
  }
}
</style>
