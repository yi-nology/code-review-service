<template>
  <AppLayout>
    <div class="rule-settings-page">
      <header class="page-header">
        <div>
          <div class="eyebrow">REVIEW RULES</div>
          <h1>审查规则</h1>
          <p>全局规则会合并到每一次模型审查中；项目策略里的规则会与这里的规则一起生效。</p>
        </div>
        <div class="header-actions">
          <el-button plain @click="fetchGlobalRules" :loading="globalLoading">
            <el-icon><Refresh /></el-icon>
            刷新
          </el-button>
          <el-button type="primary" @click="saveGlobalRules" :loading="globalSaving" :disabled="hasBlockingIssues">
            保存规则
          </el-button>
        </div>
      </header>

      <section class="summary-grid">
        <div class="summary-card">
          <span>规则总数</span>
          <strong>{{ normalizedRules.length }}</strong>
        </div>
        <div class="summary-card">
          <span>覆盖路径</span>
          <strong>{{ coveredPathCount }}</strong>
        </div>
        <div class="summary-card warning" :class="{ active: validationIssues.length > 0 }">
          <span>待处理问题</span>
          <strong>{{ validationIssues.length }}</strong>
        </div>
      </section>

      <div class="content-grid">
        <el-card class="rules-card" v-loading="globalLoading">
          <template #header>
            <div class="card-header">
              <div>
                <strong>全局规则库</strong>
                <span>{{ dirty ? '有未保存修改' : '当前配置已同步' }}</span>
              </div>
            </div>
          </template>

          <RuleEditor
            v-model="globalRules"
            :issues="validationIssues"
            :templates="ruleTemplates"
            empty-description="暂无全局规则"
          />
        </el-card>

        <aside class="side-panel">
          <el-card>
            <template #header>
              <div class="card-header compact">
                <strong>规则模板</strong>
              </div>
            </template>
            <div class="template-list">
              <button
                v-for="template in ruleTemplates"
                :key="template.name"
                class="template-item"
                type="button"
                @click="applyTemplate(template)"
              >
                <span>{{ template.name }}</span>
                <small>{{ template.path }}</small>
              </button>
            </div>
          </el-card>

          <el-card>
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

          <el-card>
            <template #header>
              <div class="card-header compact">
                <strong>生效预览</strong>
              </div>
            </template>
            <pre class="preview-json">{{ previewJson }}</pre>
          </el-card>
        </aside>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import { CircleCheckFilled, Refresh } from '@element-plus/icons-vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import RuleEditor from '@/components/RuleEditor.vue'
import { getGlobalRules, updateGlobalRules } from '@/api/config'
import type { ReviewRule } from '@/types/config'

type RuleTemplate = ReviewRule & { name: string }

const ruleTemplates: RuleTemplate[] = [
  {
    name: '通用安全检查',
    path: '**/*',
    rule: '重点检查鉴权绕过、越权访问、敏感信息泄露、未校验输入、SQL/命令注入和不安全的默认配置。',
  },
  {
    name: 'Go 后端服务',
    path: '**/*.go',
    rule: '重点检查 context 传递、错误处理、并发安全、事务边界、资源释放、panic 风险和日志中的敏感信息。',
  },
  {
    name: '前端交互',
    path: '**/*.{vue,ts,tsx,js,jsx}',
    rule: '重点检查接口错误处理、加载/空状态、权限态、表单校验、响应式布局和用户操作后的反馈。',
  },
  {
    name: '数据库变更',
    path: '**/*.{sql,go}',
    rule: '重点检查迁移兼容性、索引缺失、N+1 查询、事务一致性、软删除条件和分页查询性能。',
  },
  {
    name: '测试质量',
    path: '**/*_test.go',
    rule: '重点检查断言是否覆盖关键分支、边界条件、失败路径、并发行为和外部依赖隔离。',
  },
]

const globalRules = ref<ReviewRule[]>([])
const originalJson = ref('[]')
const globalLoading = ref(false)
const globalSaving = ref(false)

const normalizedRules = computed(() => {
  return globalRules.value
    .map((rule) => ({
      path: normalizePath(rule.path),
      rule: rule.rule.trim(),
    }))
    .filter((rule) => rule.path || rule.rule)
})

const previewJson = computed(() => JSON.stringify({ rules: normalizedRules.value }, null, 2))
const dirty = computed(() => JSON.stringify(normalizedRules.value) !== originalJson.value)
const coveredPathCount = computed(() => new Set(normalizedRules.value.map((rule) => rule.path)).size)

const validationIssues = computed(() => {
  const issues: string[] = []
  const seen = new Set<string>()

  globalRules.value.forEach((rule, index) => {
    const path = normalizePath(rule.path)
    const text = rule.rule.trim()
    if (!path && !text) return
    if (!path) issues.push(`第 ${index + 1} 条缺少适用文件`)
    if (!text) issues.push(`第 ${index + 1} 条缺少审查要求`)
    if (text.length > 800) issues.push(`第 ${index + 1} 条审查要求过长`)

    const key = `${path}::${text}`
    if (path && text) {
      if (seen.has(key)) issues.push(`第 ${index + 1} 条与已有规则重复`)
      seen.add(key)
    }
  })

  return issues
})

const hasBlockingIssues = computed(() => validationIssues.value.length > 0)

function normalizePath(path: string) {
  const value = path.trim()
  return value || '**/*'
}

async function fetchGlobalRules() {
  globalLoading.value = true
  try {
    const data = await getGlobalRules()
    globalRules.value = (data?.rules || []).map((rule) => ({
      path: normalizePath(rule.path || ''),
      rule: rule.rule || '',
    }))
    originalJson.value = JSON.stringify(normalizedRules.value)
  } finally {
    globalLoading.value = false
  }
}

async function saveGlobalRules() {
  if (hasBlockingIssues.value) {
    ElMessage.warning('请先处理规则检查中的问题')
    return
  }
  globalSaving.value = true
  try {
    await updateGlobalRules({ rules: normalizedRules.value })
    originalJson.value = JSON.stringify(normalizedRules.value)
    ElMessage.success('审查规则已保存')
  } finally {
    globalSaving.value = false
  }
}

function applyTemplate(template: RuleTemplate) {
  globalRules.value.push({ path: template.path, rule: template.rule })
}

onMounted(fetchGlobalRules)
</script>

<style scoped>
.rule-settings-page {
  width: min(1320px, 100%);
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
  max-width: 680px;
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
  grid-template-columns: repeat(3, minmax(0, 1fr));
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
.card-header span {
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
  grid-template-columns: minmax(0, 1fr) 360px;
  gap: 18px;
  align-items: start;
}

.rules-card,
.side-panel .el-card {
  border-radius: 8px;
  box-shadow: var(--app-shadow);
}

.rules-card :deep(.el-card__body) {
  padding: 18px;
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

.side-panel {
  display: grid;
  gap: 14px;
}

.template-list {
  display: grid;
  gap: 8px;
}

.template-item {
  width: 100%;
  padding: 11px 12px;
  border: 1px solid #edf0f5;
  border-radius: 8px;
  background: #fff;
  color: var(--app-text);
  text-align: left;
  cursor: pointer;
  transition: 160ms ease;
}

.template-item:hover {
  border-color: #cfd3ff;
  background: #fbfbff;
}

.template-item span,
.template-item small {
  display: block;
}

.template-item span {
  font-size: 13px;
  font-weight: 700;
}

.template-item small {
  margin-top: 3px;
  color: var(--app-muted);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 11px;
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

.preview-json {
  max-height: 280px;
  margin: 0;
  overflow: auto;
  color: #2d3748;
  font-size: 12px;
  line-height: 1.65;
  white-space: pre-wrap;
  word-break: break-word;
}

@media (max-width: 1040px) {
  .content-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 720px) {
  .rule-settings-page {
    padding: 22px 14px 36px;
  }

  .page-header {
    display: block;
  }

  .page-header h1 {
    font-size: 24px;
  }

  .header-actions {
    margin-top: 16px;
  }

  .summary-grid {
    grid-template-columns: 1fr;
  }

}
</style>
