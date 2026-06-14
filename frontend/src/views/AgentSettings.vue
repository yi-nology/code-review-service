<template>
  <AppLayout>
    <div class="agent-settings-page">
      <header class="page-header">
        <div>
          <div class="eyebrow">AGENT SETTINGS</div>
          <h1>代理设置</h1>
          <p>配置审查代理团队，每个代理拥有独立的审查侧重点，审查时并行运行。</p>
        </div>
      </header>

      <el-row :gutter="20">
        <el-col :span="16">
          <el-card>
            <template #header>
              <div class="card-header">
                <h3>代理团队</h3>
                <el-button type="primary" @click="openDialog()">
                  <el-icon><Plus /></el-icon>
                  添加代理
                </el-button>
              </div>
            </template>

            <div v-if="loading" class="loading">
              <el-skeleton :rows="5" animated />
            </div>

            <div v-else-if="agents.length === 0" class="empty">
              <el-empty description="暂无代理配置，点击上方按钮添加">
                <el-button type="primary" @click="openPresetDialog">从预设创建</el-button>
              </el-empty>
            </div>

            <div v-else class="agent-list">
              <div v-for="agent in agents" :key="agent.ID" class="agent-item" :class="{ disabled: !agent.enabled }">
                <div class="agent-info">
                  <div class="agent-header">
                    <h4>{{ agent.display_name }}</h4>
                    <el-tag v-if="!agent.enabled" type="info" size="small">已禁用</el-tag>
                    <el-tag v-else type="success" size="small">启用中</el-tag>
                  </div>
                  <p v-if="agent.description" class="agent-desc">{{ agent.description }}</p>
                  <div class="agent-background">
                    <el-text class="background-text" truncated :title="agent.background">
                      {{ agent.background }}
                    </el-text>
                  </div>
                  <div v-if="agent.tags && agent.tags.length" class="agent-tags">
                    <el-tag v-for="tag in agent.tags" :key="tag" size="small" type="info" effect="plain">
                      {{ tag }}
                    </el-tag>
                  </div>
                </div>
                <div class="agent-actions">
                  <el-switch v-model="agent.enabled" @change="toggleAgent(agent)" />
                  <el-button size="small" @click="openDialog(agent)">编辑</el-button>
                  <el-popconfirm title="确定删除此代理？" @confirm="handleDelete(agent)">
                    <template #reference>
                      <el-button size="small" type="danger">删除</el-button>
                    </template>
                  </el-popconfirm>
                </div>
              </div>
            </div>
          </el-card>
        </el-col>

        <el-col :span="8">
          <el-card>
            <template #header>
              <h3>预设模板</h3>
            </template>
            <div class="preset-list">
              <div v-for="preset in presets" :key="preset.name" class="preset-item">
                <h4>{{ preset.display_name }}</h4>
                <p>{{ preset.description }}</p>
                <el-button size="small" type="primary" plain @click="createFromPreset(preset)">一键创建</el-button>
              </div>
            </div>
          </el-card>

          <el-card class="mt-20">
            <template #header>
              <h3>说明</h3>
            </template>
            <div class="help-content">
              <h4>什么是代理？</h4>
              <p>每个代理代表一个独立的审查视角。审查时，系统会为每个启用的代理分别运行一次 OCR，各自使用不同的审查侧重点（<code>--background</code> 参数）。</p>
              <h4>如何使用？</h4>
              <p>创建代理后，系统在审查时会自动并行运行所有启用的代理，最终聚合各方发现。</p>
              <h4>审查侧重点</h4>
              <p>填写的内容会作为 OCR 的 <code>--background</code> 参数传入，引导 AI 从特定角度审查代码。例如："重点关注前端组件的可维护性和 CSS 规范"。</p>
            </div>
          </el-card>
        </el-col>
      </el-row>

      <el-dialog
        v-model="dialogVisible"
        :title="editingAgent ? '编辑代理' : '添加代理'"
        width="600px"
        :close-on-click-modal="false"
      >
        <el-form :model="form" label-width="100px" :rules="rules" ref="formRef">
          <el-form-item label="标识名" prop="name">
            <el-input v-model="form.name" placeholder="如 frontend、security" :disabled="!!editingAgent" />
            <div class="form-tip">唯一标识，创建后不可修改</div>
          </el-form-item>
          <el-form-item label="显示名称" prop="display_name">
            <el-input v-model="form.display_name" placeholder="如 前端审查员、安全审查员" />
          </el-form-item>
          <el-form-item label="描述">
            <el-input v-model="form.description" placeholder="简要描述此代理的职责" />
          </el-form-item>
          <el-form-item label="审查侧重点" prop="background">
            <el-input
              v-model="form.background"
              type="textarea"
              :rows="6"
              placeholder="例如：重点关注前端代码质量，包括 Vue 组件设计、CSS 样式规范、响应式设计"
            />
            <div class="form-tip">此内容将作为 OCR 的 --background 参数，引导 AI 从特定角度审查代码</div>
          </el-form-item>
          <el-form-item label="标签">
            <div class="tags-input">
              <el-tag
                v-for="tag in form.tags"
                :key="tag"
                closable
                @close="removeTag(tag)"
                class="tag-item"
              >{{ tag }}</el-tag>
              <el-input
                v-if="tagInputVisible"
                ref="tagInputRef"
                v-model="tagInputValue"
                size="small"
                style="width: 100px"
                @keyup.enter="addTag"
                @blur="addTag"
              />
              <el-button v-else size="small" @click="showTagInput">+ 标签</el-button>
            </div>
          </el-form-item>
          <el-form-item label="启用">
            <el-switch v-model="form.enabled" />
          </el-form-item>
        </el-form>
        <template #footer>
          <el-button @click="dialogVisible = false">取消</el-button>
          <el-button type="primary" @click="handleSubmit" :loading="submitting">
            {{ editingAgent ? '保存' : '创建' }}
          </el-button>
        </template>
      </el-dialog>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, nextTick } from 'vue'
import { Plus } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import type { FormInstance, FormRules } from 'element-plus'
import AppLayout from '@/components/layout/AppLayout.vue'
import { listReviewerAgents, createReviewerAgent, updateReviewerAgent, deleteReviewerAgent } from '@/api/config'
import type { ReviewerAgent } from '@/types/config'

const loading = ref(true)
const submitting = ref(false)
const agents = ref<ReviewerAgent[]>([])
const dialogVisible = ref(false)
const editingAgent = ref<ReviewerAgent | null>(null)
const formRef = ref<FormInstance>()
const tagInputVisible = ref(false)
const tagInputValue = ref('')
const tagInputRef = ref()

const form = reactive({
  name: '',
  display_name: '',
  description: '',
  background: '',
  enabled: true,
  tags: [] as string[],
})

const rules: FormRules = {
  name: [{ required: true, message: '请输入标识名', trigger: 'blur' }],
  display_name: [{ required: true, message: '请输入显示名称', trigger: 'blur' }],
  background: [{ required: true, message: '请输入审查侧重点', trigger: 'blur' }],
}

const presets = [
  {
    name: 'frontend',
    display_name: '前端审查员',
    description: '关注 Vue/React 组件设计、CSS 样式、响应式设计',
    background: `重点关注前端代码质量：
- Vue/React 组件设计与可维护性
- CSS/样式规范与响应式设计
- 用户交互体验与无障碍访问
- 前端性能优化（懒加载、代码分割）
- 状态管理的合理性`,
    tags: ['frontend', 'vue', 'css'],
  },
  {
    name: 'backend',
    display_name: '后端审查员',
    description: '关注 API 设计、数据库查询、错误处理',
    background: `重点关注后端代码质量：
- API 接口设计规范与一致性
- 数据库查询性能与 N+1 问题
- 错误处理与边界条件覆盖
- 日志记录与可观测性
- 并发安全与资源泄漏`,
    tags: ['backend', 'api', 'database'],
  },
  {
    name: 'security',
    display_name: '安全审查员',
    description: '关注安全漏洞、权限控制、数据保护',
    background: `重点关注安全问题：
- SQL 注入、XSS、CSRF 等常见漏洞
- 认证与授权机制的正确性
- 输入验证与数据净化
- 敏感数据处理（密码、Token、密钥）
- 权限控制与越权访问风险`,
    tags: ['security', 'auth', 'vulnerability'],
  },
  {
    name: 'performance',
    display_name: '性能审查员',
    description: '关注性能瓶颈、资源使用、可扩展性',
    background: `重点关注性能问题：
- 算法复杂度与性能瓶颈
- 内存使用与泄漏风险
- 数据库索引与查询优化
- 缓存策略的合理性
- 异步处理与并发性能`,
    tags: ['performance', 'optimization'],
  },
]

async function fetchAgents() {
  loading.value = true
  try {
    agents.value = await listReviewerAgents()
  } catch (e) {
    console.error('Failed to load agents:', e)
  } finally {
    loading.value = false
  }
}

function openDialog(agent?: ReviewerAgent) {
  editingAgent.value = agent || null
  if (agent) {
    form.name = agent.name
    form.display_name = agent.display_name
    form.description = agent.description || ''
    form.background = agent.background
    form.enabled = agent.enabled
    form.tags = agent.tags ? [...agent.tags] : []
  } else {
    form.name = ''
    form.display_name = ''
    form.description = ''
    form.background = ''
    form.enabled = true
    form.tags = []
  }
  dialogVisible.value = true
}

function openPresetDialog() {
  dialogVisible.value = false
}

async function handleSubmit() {
  if (!formRef.value) return
  await formRef.value.validate()
  submitting.value = true
  try {
    if (editingAgent.value) {
      await updateReviewerAgent(editingAgent.value.ID, {
        display_name: form.display_name,
        description: form.description,
        background: form.background,
        enabled: form.enabled,
        tags: form.tags,
      })
      ElMessage.success('代理已更新')
    } else {
      await createReviewerAgent({
        name: form.name,
        display_name: form.display_name,
        description: form.description,
        background: form.background,
        enabled: form.enabled,
        tags: form.tags,
      })
      ElMessage.success('代理已创建')
    }
    dialogVisible.value = false
    await fetchAgents()
  } catch (e: any) {
    ElMessage.error(e?.message || '操作失败')
  } finally {
    submitting.value = false
  }
}

async function toggleAgent(agent: ReviewerAgent) {
  try {
    await updateReviewerAgent(agent.ID, { enabled: agent.enabled })
    ElMessage.success(agent.enabled ? '已启用' : '已禁用')
  } catch (e: any) {
    agent.enabled = !agent.enabled
    ElMessage.error(e?.message || '操作失败')
  }
}

async function handleDelete(agent: ReviewerAgent) {
  try {
    await deleteReviewerAgent(agent.ID)
    ElMessage.success('已删除')
    await fetchAgents()
  } catch (e: any) {
    ElMessage.error(e?.message || '删除失败')
  }
}

async function createFromPreset(preset: typeof presets[0]) {
  submitting.value = true
  try {
    await createReviewerAgent({
      name: preset.name,
      display_name: preset.display_name,
      description: preset.description,
      background: preset.background,
      enabled: true,
      tags: preset.tags,
    })
    ElMessage.success(`已创建 ${preset.display_name}`)
    await fetchAgents()
  } catch (e: any) {
    ElMessage.error(e?.message || '创建失败')
  } finally {
    submitting.value = false
  }
}

function removeTag(tag: string) {
  form.tags = form.tags.filter(t => t !== tag)
}

function showTagInput() {
  tagInputVisible.value = true
  nextTick(() => tagInputRef.value?.focus())
}

function addTag() {
  if (tagInputValue.value && !form.tags.includes(tagInputValue.value)) {
    form.tags.push(tagInputValue.value)
  }
  tagInputVisible.value = false
  tagInputValue.value = ''
}

onMounted(fetchAgents)
</script>

<style scoped>
.agent-settings-page {
  width: min(1200px, 100%);
  margin: 0 auto;
  padding: 34px 36px 48px;
}

.page-header {
  margin-bottom: 24px;
}

.page-header h1 {
  margin: 4px 0 8px;
  color: var(--app-text);
  font-size: 28px;
}

.page-header p {
  margin: 0;
  color: var(--app-muted);
  font-size: 14px;
}

.eyebrow {
  color: var(--app-primary);
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 1.3px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-header h3 {
  margin: 0;
}

.mt-20 {
  margin-top: 20px;
}

.loading {
  padding: 20px;
}

.empty {
  padding: 40px;
}

.agent-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.agent-item {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  padding: 16px;
  background: #f5f7fa;
  border-radius: 8px;
  border: 1px solid transparent;
  transition: all 0.2s;
}

.agent-item:hover {
  border-color: #dcdfe6;
}

.agent-item.disabled {
  opacity: 0.6;
}

.agent-info {
  flex: 1;
  min-width: 0;
}

.agent-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
}

.agent-header h4 {
  margin: 0;
  font-size: 15px;
}

.agent-desc {
  margin: 0 0 8px;
  color: #666;
  font-size: 13px;
}

.agent-background {
  margin-bottom: 8px;
}

.background-text {
  font-size: 12px;
  color: #909399;
  max-width: 500px;
}

.agent-tags {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
}

.agent-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
  margin-left: 16px;
}

.preset-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.preset-item {
  padding: 14px;
  background: #f5f7fa;
  border-radius: 8px;
}

.preset-item h4 {
  margin: 0 0 6px;
  font-size: 14px;
}

.preset-item p {
  margin: 0 0 10px;
  color: #666;
  font-size: 12px;
}

.help-content h4 {
  margin: 16px 0 8px;
  font-size: 14px;
}

.help-content h4:first-child {
  margin-top: 0;
}

.help-content p {
  margin: 0;
  color: #666;
  font-size: 13px;
  line-height: 1.6;
}

.help-content code {
  background: #f0f0f0;
  padding: 1px 4px;
  border-radius: 3px;
  font-size: 12px;
}

.form-tip {
  font-size: 12px;
  color: #999;
  margin-top: 4px;
}

.tags-input {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: center;
}

.tag-item {
  margin: 0;
}
</style>
