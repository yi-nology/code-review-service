<template>
  <AppLayout>
    <div class="project-settings-page">
      <header class="page-header">
        <div class="eyebrow">PROJECT POLICY</div>
        <h1>项目审查策略</h1>
        <p>为不同仓库独立设置门禁等级和不参与代码审查的文件。</p>
      </header>

      <el-row :gutter="24">
        <el-col :xs="24" :xl="12">
          <el-card>
            <template #header>
              <div class="card-header">
                <span>仓库规则</span>
              </div>
            </template>
            <el-form :model="form" label-width="100px">
              <el-form-item label="代码平台">
                <el-select v-model="form.provider">
                  <el-option label="Gitea" value="gitea" />
                  <el-option label="GitLab" value="gitlab" />
                  <el-option label="GitHub" value="github" />
                </el-select>
              </el-form-item>
              <el-form-item label="仓库">
                <el-input v-model="form.repo" placeholder="group/project" />
              </el-form-item>
              <el-form-item label="门禁阻断级别">
                <el-select v-model="form.fail_on">
                  <el-option label="严重问题" value="critical" />
                  <el-option label="高风险及以上" value="major" />
                  <el-option label="中风险及以上" value="minor" />
                  <el-option label="所有问题" value="info" />
                </el-select>
              </el-form-item>
              <el-form-item label="忽略路径">
                <el-input
                  v-model="ignorePathsText"
                  type="textarea"
                  :rows="6"
                  placeholder="vendor/**&#10;generated/**&#10;**/*.pb.go"
                />
                <div class="form-tip">支持 glob；规则在模型调用前生效，排除文件不会消耗审查额度。</div>
              </el-form-item>
              <el-form-item label="审查规则">
                <div v-for="(rule, idx) in form.rules" :key="idx" class="rule-row">
                  <el-row :gutter="8" align="middle">
                    <el-col :span="8">
                      <el-input v-model="rule.path" placeholder="**/*.java" />
                    </el-col>
                    <el-col :span="14">
                      <el-input v-model="rule.rule" placeholder="审查规则描述" />
                    </el-col>
                    <el-col :span="2">
                      <el-button type="danger" size="small" circle @click="form.rules.splice(idx, 1)">×</el-button>
                    </el-col>
                  </el-row>
                </div>
                <el-button size="small" @click="form.rules.push({ path: '', rule: '' })">+ 添加规则</el-button>
              </el-form-item>
              <el-form-item>
                <el-button type="primary" @click="handleSave" :loading="saving">保存项目策略</el-button>
              </el-form-item>
            </el-form>
          </el-card>
        </el-col>

        <el-col :xs="24" :xl="12">
          <el-card>
            <template #header>
              <div class="card-header">
                <span>已配置项目</span>
                <el-button @click="fetchPolicies">刷新</el-button>
              </div>
            </template>
            <div v-loading="loading">
              <div v-if="policies.length === 0" class="empty-state">
                暂无配置的项目策略
              </div>
              <div v-else class="policy-list">
                <div
                  v-for="policy in policies"
                  :key="`${policy.provider}:${policy.repo}`"
                  class="policy-item"
                  @click="handleEdit(policy)"
                >
                  <div class="policy-header">
                    <el-tag size="small">{{ policy.provider }}</el-tag>
                    <span class="policy-repo">{{ policy.repo }}</span>
                  </div>
                  <div class="policy-meta">
                    门禁: {{ getFailOnLabel(policy.fail_on) }} |
                    忽略: {{ policy.ignore_paths?.length || 0 }} 条 |
                    规则: {{ policy.rules?.length || 0 }} 条
                  </div>
                  <el-button
                    type="danger"
                    size="small"
                    class="delete-btn"
                    @click.stop="handleDelete(policy)"
                  >
                    删除
                  </el-button>
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
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import AppLayout from '@/components/layout/AppLayout.vue'
import { getProjectPolicies, putProjectPolicy, deleteProjectPolicy } from '@/api/config'
import type { ProjectPolicy, ReviewRule } from '@/types/config'

const loading = ref(false)
const saving = ref(false)
const policies = ref<ProjectPolicy[]>([])

const form = reactive<ProjectPolicy & { rules: ReviewRule[] }>({
  provider: 'gitea',
  repo: '',
  fail_on: 'critical',
  ignore_paths: [],
  rules: [],
})

const ignorePathsText = ref('')

async function fetchPolicies() {
  loading.value = true
  try {
    const result = await getProjectPolicies()
    policies.value = result.items || []
  } finally {
    loading.value = false
  }
}

async function handleSave() {
  if (!form.repo) {
    ElMessage.warning('请输入仓库名称')
    return
  }
  saving.value = true
  try {
    form.ignore_paths = ignorePathsText.value
      .split('\n')
      .map((s) => s.trim())
      .filter(Boolean)
    await putProjectPolicy(form)
    ElMessage.success('策略已保存')
    fetchPolicies()
  } finally {
    saving.value = false
  }
}

function handleEdit(policy: ProjectPolicy) {
  form.provider = policy.provider
  form.repo = policy.repo
  form.fail_on = policy.fail_on
  form.ignore_paths = policy.ignore_paths || []
  form.rules = policy.rules ? policy.rules.map(r => ({ ...r })) : []
  ignorePathsText.value = (policy.ignore_paths || []).join('\n')
}

async function handleDelete(policy: ProjectPolicy) {
  await ElMessageBox.confirm('确定删除该策略？', '确认')
  try {
    await deleteProjectPolicy(policy.provider, policy.repo)
    ElMessage.success('已删除')
    fetchPolicies()
  } catch {
    // 用户取消
  }
}

function getFailOnLabel(failOn: string) {
  const map: Record<string, string> = {
    critical: '严重问题',
    major: '高风险及以上',
    minor: '中风险及以上',
    info: '所有问题',
  }
  return map[failOn] || failOn
}

onMounted(fetchPolicies)
</script>

<style scoped>
.project-settings-page {
  width: min(1320px, 100%);
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

.form-tip {
  font-size: 12px;
  color: #999;
  margin-top: 4px;
}

.rule-row {
  margin-bottom: 8px;
}

.empty-state {
  text-align: center;
  color: #999;
  padding: 40px 0;
}

.policy-item {
  padding: 12px;
  border: 1px solid #eee;
  border-radius: 8px;
  margin-bottom: 8px;
  cursor: pointer;
  position: relative;
}

.policy-item:hover {
  background: #f5f5f5;
}

.policy-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
}

.policy-repo {
  font-weight: bold;
}

.policy-meta {
  font-size: 12px;
  color: #999;
}

.delete-btn {
  position: absolute;
  right: 12px;
  top: 12px;
}

@media (max-width: 1199px) {
  .project-settings-page :deep(.el-col + .el-col) {
    margin-top: 16px;
  }
}

@media (max-width: 720px) {
  .project-settings-page {
    padding: 22px 14px 36px;
  }

  .page-header h1 {
    font-size: 24px;
  }
}
</style>
