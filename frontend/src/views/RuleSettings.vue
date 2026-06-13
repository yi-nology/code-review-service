<template>
  <AppLayout>
    <div class="rule-settings-page">
      <header class="page-header">
        <div class="eyebrow">REVIEW RULES</div>
        <h1>审查规则管理</h1>
        <p>配置全局审查规则和项目级规则，指导模型聚焦关键问题。项目规则优先级高于全局规则。</p>
      </header>

      <el-row :gutter="24">
        <el-col :xs="24" :xl="12">
          <el-card>
            <template #header>
              <div class="card-header">
                <span>全局规则</span>
              </div>
            </template>
            <div v-loading="globalLoading">
              <div v-for="(rule, idx) in globalRules" :key="idx" class="rule-item">
                <el-row :gutter="8" align="middle">
                  <el-col :span="8">
                    <el-input v-model="rule.path" placeholder="**/*.java" size="small" />
                  </el-col>
                  <el-col :span="14">
                    <el-input v-model="rule.rule" placeholder="审查规则描述" size="small" />
                  </el-col>
                  <el-col :span="2">
                    <el-button type="danger" size="small" circle @click="removeGlobalRule(idx)">
                      <el-icon><Delete /></el-icon>
                    </el-button>
                  </el-col>
                </el-row>
              </div>
              <el-button size="small" @click="addGlobalRule">+ 添加规则</el-button>
              <div style="margin-top: 16px">
                <el-button type="primary" @click="saveGlobalRules" :loading="globalSaving">保存全局规则</el-button>
              </div>
            </div>
          </el-card>
        </el-col>

        <el-col :xs="24" :xl="12">
          <el-card>
            <template #header>
              <div class="card-header">
                <span>项目级规则</span>
              </div>
            </template>
            <el-form :model="projectForm" label-width="80px" size="small">
              <el-form-item label="代码平台">
                <el-select v-model="projectForm.provider">
                  <el-option label="Gitea" value="gitea" />
                  <el-option label="GitLab" value="gitlab" />
                  <el-option label="GitHub" value="github" />
                </el-select>
              </el-form-item>
              <el-form-item label="仓库">
                <el-input v-model="projectForm.repo" placeholder="group/project" />
              </el-form-item>
              <el-form-item label="门禁级别">
                <el-select v-model="projectForm.fail_on">
                  <el-option label="严重问题" value="critical" />
                  <el-option label="高风险及以上" value="major" />
                  <el-option label="中风险及以上" value="minor" />
                  <el-option label="所有问题" value="info" />
                </el-select>
              </el-form-item>
              <el-form-item label="忽略路径">
                <el-input v-model="projectIgnorePathsText" type="textarea" :rows="3" placeholder="vendor/**\ngenerated/**" />
              </el-form-item>
              <el-form-item label="审查规则">
                <div v-for="(rule, idx) in projectForm.rules" :key="idx" class="rule-item">
                  <el-row :gutter="8" align="middle">
                    <el-col :span="8">
                      <el-input v-model="rule.path" placeholder="**/*.java" size="small" />
                    </el-col>
                    <el-col :span="14">
                      <el-input v-model="rule.rule" placeholder="审查规则描述" size="small" />
                    </el-col>
                    <el-col :span="2">
                      <el-button type="danger" size="small" circle @click="removeProjectRule(idx)">
                        <el-icon><Delete /></el-icon>
                      </el-button>
                    </el-col>
                  </el-row>
                </div>
                <el-button size="small" @click="addProjectRule">+ 添加规则</el-button>
              </el-form-item>
              <el-form-item>
                <el-button type="primary" @click="saveProjectPolicy" :loading="projectSaving">保存项目策略</el-button>
              </el-form-item>
            </el-form>
          </el-card>

          <el-card style="margin-top: 16px">
            <template #header>
              <div class="card-header">
                <span>已配置项目</span>
                <el-button size="small" @click="fetchPolicies">刷新</el-button>
              </div>
            </template>
            <div v-loading="policyLoading">
              <div v-if="policies.length === 0" class="empty-state">暂无配置的项目策略</div>
              <div v-else class="policy-list">
                <div
                  v-for="policy in policies"
                  :key="`${policy.provider}:${policy.repo}`"
                  class="policy-item"
                  @click="editPolicy(policy)"
                >
                  <div class="policy-header">
                    <el-tag size="small">{{ policy.provider }}</el-tag>
                    <span class="policy-repo">{{ policy.repo }}</span>
                  </div>
                  <div class="policy-meta">
                    门禁: {{ getFailOnLabel(policy.fail_on) }} |
                    规则: {{ policy.rules?.length || 0 }} 条 |
                    忽略: {{ policy.ignore_paths?.length || 0 }} 条
                  </div>
                  <el-button type="danger" size="small" class="delete-btn" @click.stop="deletePolicy(policy)">删除</el-button>
                </div>
              </div>
            </div>
          </el-card>
        </el-col>
      </el-row>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Delete } from '@element-plus/icons-vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { getGlobalRules, updateGlobalRules, getProjectPolicies, putProjectPolicy, deleteProjectPolicy } from '@/api/config'
import type { ProjectPolicy, ReviewRule } from '@/types/config'

const globalRules = ref<ReviewRule[]>([])
const globalLoading = ref(false)
const globalSaving = ref(false)

const policies = ref<ProjectPolicy[]>([])
const policyLoading = ref(false)
const projectSaving = ref(false)

const projectForm = reactive<{
  provider: string
  repo: string
  fail_on: string
  rules: ReviewRule[]
}>({
  provider: 'gitea',
  repo: '',
  fail_on: 'critical',
  rules: [],
})

const projectIgnorePathsText = ref('')

async function fetchGlobalRules() {
  globalLoading.value = true
  try {
    const data = await getGlobalRules()
    globalRules.value = data?.rules || []
  } finally {
    globalLoading.value = false
  }
}

async function saveGlobalRules() {
  globalSaving.value = true
  try {
    await updateGlobalRules({ rules: globalRules.value.filter(r => r.path || r.rule) })
    ElMessage.success('全局规则已保存')
  } finally {
    globalSaving.value = false
  }
}

function addGlobalRule() {
  globalRules.value.push({ path: '', rule: '' })
}

function removeGlobalRule(idx: number) {
  globalRules.value.splice(idx, 1)
}

async function fetchPolicies() {
  policyLoading.value = true
  try {
    const result = await getProjectPolicies()
    policies.value = result.items || []
  } finally {
    policyLoading.value = false
  }
}

function addProjectRule() {
  projectForm.rules.push({ path: '', rule: '' })
}

function removeProjectRule(idx: number) {
  projectForm.rules.splice(idx, 1)
}

async function saveProjectPolicy() {
  if (!projectForm.repo) {
    ElMessage.warning('请输入仓库名称')
    return
  }
  projectSaving.value = true
  try {
    const ignorePaths = projectIgnorePathsText.value
      .split('\n')
      .map(s => s.trim())
      .filter(Boolean)
    await putProjectPolicy({
      provider: projectForm.provider,
      repo: projectForm.repo,
      fail_on: projectForm.fail_on,
      ignore_paths: ignorePaths,
      rules: projectForm.rules.filter(r => r.path || r.rule),
    })
    ElMessage.success('项目策略已保存')
    fetchPolicies()
  } finally {
    projectSaving.value = false
  }
}

function editPolicy(policy: ProjectPolicy) {
  projectForm.provider = policy.provider
  projectForm.repo = policy.repo
  projectForm.fail_on = policy.fail_on
  projectForm.rules = policy.rules ? [...policy.rules.map(r => ({ ...r }))] : []
  projectIgnorePathsText.value = (policy.ignore_paths || []).join('\n')
}

async function deletePolicy(policy: ProjectPolicy) {
  await ElMessageBox.confirm('确定删除该策略？', '确认')
  try {
    await deleteProjectPolicy(policy.provider, policy.repo)
    ElMessage.success('已删除')
    fetchPolicies()
  } catch { /* cancel */ }
}

function getFailOnLabel(failOn: string) {
  const map: Record<string, string> = { critical: '严重问题', major: '高风险及以上', minor: '中风险及以上', info: '所有问题' }
  return map[failOn] || failOn
}

onMounted(() => {
  fetchGlobalRules()
  fetchPolicies()
})
</script>

<style scoped>
.rule-settings-page { width: min(1320px, 100%); margin: 0 auto; padding: 34px 36px 48px; }
.page-header { margin-bottom: 24px; }
.page-header h1 { margin: 4px 0 8px; color: var(--app-text); font-size: 28px; }
.page-header p { margin: 0; color: var(--app-muted); font-size: 14px; }
.eyebrow { color: var(--app-primary); font-size: 10px; font-weight: 800; letter-spacing: 1.3px; }
.card-header { display: flex; justify-content: space-between; align-items: center; }
.rule-item { margin-bottom: 8px; }
.empty-state { text-align: center; color: #999; padding: 40px 0; }
.policy-item { padding: 12px; border: 1px solid #eee; border-radius: 8px; margin-bottom: 8px; cursor: pointer; position: relative; }
.policy-item:hover { background: #f5f5f5; }
.policy-header { display: flex; align-items: center; gap: 8px; margin-bottom: 4px; }
.policy-repo { font-weight: bold; }
.policy-meta { font-size: 12px; color: #999; }
.delete-btn { position: absolute; right: 12px; top: 12px; }
@media (max-width: 1199px) {
  .rule-settings-page :deep(.el-col + .el-col) { margin-top: 16px; }
}
@media (max-width: 720px) {
  .rule-settings-page { padding: 22px 14px 36px; }
  .page-header h1 { font-size: 24px; }
}
</style>
