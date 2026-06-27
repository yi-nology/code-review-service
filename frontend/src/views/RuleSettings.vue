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
          <el-button plain @click="openNewRulePackage">
            新建规则包
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
                <strong>规则包市场</strong>
              </div>
            </template>
            <div v-if="rulePackages.length" class="package-list">
              <div
                v-for="pkg in rulePackages"
                :key="pkg.id || pkg.name"
                class="package-item"
              >
                <button type="button" class="package-main" @click="applyRulePackage(pkg)">
                  <span>{{ pkg.name }}</span>
                  <small>{{ pkg.description || (pkg.rules.length + ' 条规则') }}</small>
                  <em>{{ pkg.built_in ? '内置' : '自定义' }} · {{ pkg.enabled ? '启用' : '停用' }}</em>
                </button>
                <div class="package-actions">
                  <el-switch size="small" :model-value="pkg.enabled" @change="toggleRulePackage(pkg)" />
                  <el-button size="small" text @click="editRulePackage(pkg)">{{ pkg.built_in ? '复制' : '编辑' }}</el-button>
                  <el-button v-if="pkg.id" size="small" text @click="showRulePackageHistory(pkg)">历史</el-button>
                  <el-button v-if="!pkg.built_in" size="small" text type="danger" @click="removeRulePackage(pkg)">删除</el-button>
                </div>
              </div>
            </div>
            <el-divider v-if="rulePackages.length" />
            <div class="card-header compact">
              <strong>快速模板</strong>
            </div>
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

          <el-card>
            <template #header>
              <div class="card-header compact">
                <strong>规则验证</strong>
              </div>
            </template>
            <div style="display: flex; gap: 8px; margin-bottom: 8px;">
              <el-input
                v-model="checkPath"
                placeholder="src/auth/handler.go"
                size="small"
                @keyup.enter="handleRulesCheck"
              />
              <el-button size="small" type="primary" @click="handleRulesCheck" :loading="checking">检查</el-button>
            </div>
            <div v-if="checkResult !== null">
              <el-tag v-if="checkResult.applicable" type="success">匹配</el-tag>
              <el-tag v-else type="info">不匹配</el-tag>
              <div v-if="checkResult.rule" style="margin-top: 8px; font-size: 12px; color: #606266; white-space: pre-wrap;">{{ checkResult.rule }}</div>
              <div v-if="checkResult.message" style="margin-top: 4px; font-size: 12px; color: #909399;">{{ checkResult.message }}</div>
            </div>
          </el-card>
        </aside>
      </div>

      <el-dialog v-model="rulePackageDialog" :title="rulePackageDialogTitle" width="760px">
        <el-form label-position="top">
          <div class="dialog-grid">
            <el-form-item label="名称">
              <el-input v-model="rulePackageForm.name" placeholder="Java 后端规则包" />
            </el-form-item>
            <el-form-item label="启用">
              <el-switch v-model="rulePackageForm.enabled" />
            </el-form-item>
          </div>
          <el-form-item label="描述">
            <el-input v-model="rulePackageForm.description" type="textarea" :rows="2" placeholder="这个规则包适合什么项目、解决什么问题" />
          </el-form-item>
          <div class="dialog-grid two">
            <el-form-item label="语言">
              <el-input v-model="rulePackageForm.languagesText" placeholder="java, go, vue" />
            </el-form-item>
            <el-form-item label="标签">
              <el-input v-model="rulePackageForm.tagsText" placeholder="backend, security" />
            </el-form-item>
          </div>
          <div class="dialog-grid two">
            <el-form-item label="Include">
              <el-input v-model="rulePackageForm.includesText" type="textarea" :rows="2" placeholder="每行一个 glob，例如 **/*.go" />
            </el-form-item>
            <el-form-item label="Exclude">
              <el-input v-model="rulePackageForm.excludesText" type="textarea" :rows="2" placeholder="每行一个 glob，例如 **/generated/**" />
            </el-form-item>
          </div>
          <el-form-item label="项目绑定">
            <el-input v-model="rulePackageForm.projectRefsText" type="textarea" :rows="2" placeholder="每行一个 provider/repo 或 owner/repo；留空表示全部项目" />
          </el-form-item>
          <el-form-item label="规则列表">
            <RuleEditor v-model="rulePackageForm.rules" empty-description="这个规则包还没有规则" />
          </el-form-item>
        </el-form>
        <template #footer>
          <el-button @click="rulePackageDialog = false">取消</el-button>
          <el-button type="primary" :loading="rulePackageSaving" @click="saveRulePackageForm">保存规则包</el-button>
        </template>
      </el-dialog>

      <el-dialog v-model="rulePackageHistoryDialog" title="规则包版本历史" width="680px">
        <div class="history-title">
          <strong>{{ historyRulePackage?.name || '规则包' }}</strong>
          <span>保存规则包时会自动记录修改前的版本</span>
        </div>
        <div v-loading="rulePackageHistoryLoading" class="version-list">
          <div
            v-for="version in rulePackageVersions"
            :key="version.version"
            class="version-item"
          >
            <div>
              <strong>版本 {{ version.version }}</strong>
              <span>{{ formatVersionTime(version.created_at) }}</span>
              <small>{{ version.snapshot?.name || '-' }} · {{ version.snapshot?.rules?.length || 0 }} 条规则</small>
            </div>
            <el-button
              size="small"
              type="primary"
              plain
              :loading="rulePackageHistoryRestoring"
              @click="restoreRulePackage(version.version)"
            >
              恢复
            </el-button>
          </div>
          <el-empty v-if="!rulePackageVersions.length" description="暂无历史版本" :image-size="56" />
        </div>
      </el-dialog>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'
import { CircleCheckFilled, Refresh } from '@element-plus/icons-vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import RuleEditor from '@/components/RuleEditor.vue'
import {
  deleteRulePackage,
  getGlobalRules,
  listRulePackageVersions,
  listRulePackages,
  restoreRulePackageVersion,
  rulesCheck,
  saveRulePackage,
  updateGlobalRules,
} from '@/api/config'
import type { OCRAssetVersion, OCRRulePackage, ReviewRule, RulesCheckResult } from '@/types/config'

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
const rulePackages = ref<OCRRulePackage[]>([])
const originalJson = ref('[]')
const globalLoading = ref(false)
const globalSaving = ref(false)
const rulePackageDialog = ref(false)
const rulePackageSaving = ref(false)
const editingRulePackageID = ref<number | undefined>()
const checkPath = ref('')
const checkResult = ref<RulesCheckResult | null>(null)
const checking = ref(false)
const rulePackageHistoryDialog = ref(false)
const rulePackageHistoryLoading = ref(false)
const rulePackageHistoryRestoring = ref(false)
const historyRulePackage = ref<OCRRulePackage | null>(null)
const rulePackageVersions = ref<OCRAssetVersion<OCRRulePackage>[]>([])

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
const rulePackageDialogTitle = computed(() => editingRulePackageID.value ? '编辑规则包' : '新建规则包')
const rulePackageForm = ref({
  name: '',
  description: '',
  enabled: true,
  languagesText: '',
  tagsText: '',
  includesText: '',
  excludesText: '',
  projectRefsText: '',
  rules: [] as ReviewRule[],
})

function normalizePath(path: string) {
  const value = path.trim()
  return value || '**/*'
}

async function fetchGlobalRules() {
  globalLoading.value = true
  try {
    const [data, packages] = await Promise.all([getGlobalRules(), listRulePackages()])
    globalRules.value = (data?.rules || []).map((rule) => ({
      path: normalizePath(rule.path || ''),
      rule: rule.rule || '',
    }))
    rulePackages.value = packages
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

function applyRulePackage(pkg: OCRRulePackage) {
  if (!pkg.enabled) {
    ElMessage.warning('这个规则包当前未启用')
    return
  }
  const existing = new Set(normalizedRules.value.map((rule) => `${rule.path}::${rule.rule}`))
  const next = (pkg.rules || [])
    .map((rule) => ({ path: normalizePath(rule.path || ''), rule: rule.rule || '' }))
    .filter((rule) => {
      const key = `${rule.path}::${rule.rule}`
      if (existing.has(key)) return false
      existing.add(key)
      return true
    })
  if (!next.length) {
    ElMessage.info('规则包中的规则已经全部存在')
    return
  }
  globalRules.value.push(...next)
  ElMessage.success(`已加入 ${next.length} 条规则：${pkg.name}`)
}

function openNewRulePackage() {
  editingRulePackageID.value = undefined
  rulePackageForm.value = {
    name: '',
    description: '',
    enabled: true,
    languagesText: '',
    tagsText: '',
    includesText: '',
    excludesText: '',
    projectRefsText: '',
    rules: [{ path: '**/*', rule: '' }],
  }
  rulePackageDialog.value = true
}

function editRulePackage(pkg: OCRRulePackage) {
  editingRulePackageID.value = pkg.built_in ? undefined : pkg.id
  rulePackageForm.value = {
    name: pkg.built_in ? `${pkg.name} 副本` : pkg.name,
    description: pkg.description || '',
    enabled: pkg.enabled,
    languagesText: (pkg.languages || []).join(', '),
    tagsText: (pkg.tags || []).join(', '),
    includesText: (pkg.includes || []).join('\n'),
    excludesText: (pkg.excludes || []).join('\n'),
    projectRefsText: (pkg.project_refs || []).join('\n'),
    rules: (pkg.rules || []).map((rule) => ({ ...rule })),
  }
  rulePackageDialog.value = true
}

async function toggleRulePackage(pkg: OCRRulePackage) {
  try {
    await saveRulePackage({ ...pkg, enabled: !pkg.enabled })
    await fetchGlobalRules()
  } catch (e: any) {
    ElMessage.error(e.message || '更新规则包失败')
  }
}

async function removeRulePackage(pkg: OCRRulePackage) {
  if (!pkg.id || pkg.built_in) return
  try {
    await ElMessageBox.confirm(`确定删除规则包「${pkg.name}」？`, '删除规则包', {
      type: 'warning',
      confirmButtonText: '删除',
      cancelButtonText: '取消',
    })
  } catch {
    return
  }
  try {
    await deleteRulePackage(pkg.id)
    await fetchGlobalRules()
    ElMessage.success('规则包已删除')
  } catch (e: any) {
    ElMessage.error(e.message || '删除规则包失败')
  }
}

async function showRulePackageHistory(pkg: OCRRulePackage) {
  if (!pkg.id) return
  historyRulePackage.value = pkg
  rulePackageHistoryDialog.value = true
  rulePackageHistoryLoading.value = true
  try {
    rulePackageVersions.value = await listRulePackageVersions(pkg.id)
  } catch (e: any) {
    ElMessage.error(e.message || '获取规则包历史失败')
  } finally {
    rulePackageHistoryLoading.value = false
  }
}

async function restoreRulePackage(version: number) {
  if (!historyRulePackage.value?.id) return
  try {
    await ElMessageBox.confirm(`确定将「${historyRulePackage.value.name}」恢复到版本 ${version}？`, '恢复规则包', {
      type: 'warning',
      confirmButtonText: '恢复',
      cancelButtonText: '取消',
    })
  } catch {
    return
  }
  rulePackageHistoryRestoring.value = true
  try {
    const restored = await restoreRulePackageVersion(historyRulePackage.value.id, version)
    historyRulePackage.value = restored
    await fetchGlobalRules()
    rulePackageVersions.value = await listRulePackageVersions(restored.id!)
    ElMessage.success(`已恢复到版本 ${version}`)
  } catch (e: any) {
    ElMessage.error(e.message || '恢复规则包失败')
  } finally {
    rulePackageHistoryRestoring.value = false
  }
}

async function saveRulePackageForm() {
  const form = rulePackageForm.value
  const rules = form.rules
    .map((rule) => ({ path: normalizePath(rule.path || ''), rule: (rule.rule || '').trim() }))
    .filter((rule) => rule.path || rule.rule)
  if (!form.name.trim()) {
    ElMessage.warning('请填写规则包名称')
    return
  }
  if (!rules.length || rules.some((rule) => !rule.path || !rule.rule)) {
    ElMessage.warning('规则包至少需要一条完整规则')
    return
  }
  rulePackageSaving.value = true
  try {
    await saveRulePackage({
      id: editingRulePackageID.value,
      name: form.name.trim(),
      description: form.description.trim(),
      enabled: form.enabled,
      languages: splitInline(form.languagesText),
      tags: splitInline(form.tagsText),
      includes: splitLines(form.includesText),
      excludes: splitLines(form.excludesText),
      project_refs: splitLines(form.projectRefsText),
      rules,
    })
    rulePackageDialog.value = false
    await fetchGlobalRules()
    ElMessage.success('规则包已保存')
  } catch (e: any) {
    ElMessage.error(e.message || '保存规则包失败')
  } finally {
    rulePackageSaving.value = false
  }
}

function splitInline(value: string) {
  return value.split(/[,，\n]/).map((item) => item.trim()).filter(Boolean)
}

function splitLines(value: string) {
  return value.split('\n').map((item) => item.trim()).filter(Boolean)
}

function formatVersionTime(value: string) {
  if (!value) return '-'
  return new Date(value).toLocaleString('zh-CN')
}

async function handleRulesCheck() {
  if (!checkPath.value.trim()) return
  checking.value = true
  try {
    checkResult.value = await rulesCheck(checkPath.value.trim())
  } catch (e: any) {
    checkResult.value = { applicable: false, message: e.message || '检查失败' }
  } finally {
    checking.value = false
  }
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

.package-list {
  display: grid;
  gap: 8px;
  margin-bottom: 10px;
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

.package-item {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 8px;
  align-items: center;
  padding: 10px 10px 10px 12px;
  border: 1px solid #e6ebf2;
  border-radius: 8px;
  background: #f8fafc;
  color: var(--app-text);
}

.package-main {
  min-width: 0;
  border: 0;
  background: transparent;
  color: inherit;
  text-align: left;
  cursor: pointer;
}

.package-item:hover {
  border-color: #9db1ff;
  background: #fff;
}

.package-item span,
.package-item small,
.package-item em {
  display: block;
}

.package-item span {
  font-size: 13px;
  font-weight: 800;
}

.package-item small {
  margin-top: 4px;
  color: var(--app-muted);
  line-height: 1.5;
}

.package-item em {
  margin-top: 6px;
  color: var(--app-primary);
  font-size: 11px;
  font-style: normal;
}

.package-actions {
  display: grid;
  justify-items: end;
  gap: 2px;
}

.dialog-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 120px;
  gap: 12px;
}

.dialog-grid.two {
  grid-template-columns: repeat(2, minmax(0, 1fr));
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

.history-title {
  margin-bottom: 14px;
}

.history-title strong,
.history-title span {
  display: block;
}

.history-title strong {
  color: var(--app-text);
  font-size: 15px;
}

.history-title span {
  margin-top: 4px;
  color: var(--app-muted);
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
  color: var(--app-text);
  font-size: 13px;
}

.version-item span {
  margin-top: 4px;
  color: var(--app-muted);
  font-size: 12px;
}

.version-item small {
  margin-top: 4px;
  color: var(--app-primary);
  font-size: 11px;
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

  .package-item,
  .dialog-grid,
  .dialog-grid.two {
    grid-template-columns: 1fr;
  }

  .package-actions {
    display: flex;
    justify-content: flex-start;
  }

}
</style>
