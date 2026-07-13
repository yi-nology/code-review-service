<template>
  <AppLayout>
    <PageShell>
      <PageHeader
        eyebrow="REVIEW RULES"
        title="审查规则"
        description="这里是规则集的统一维护入口。标为「全局」的规则集始终合并到每次审查；其余规则集可被审查员（「审查员」页）或项目（「项目策略」页）引用。项目策略里的内联规则也会叠加生效。"
      >
        <template #actions>
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
        </template>
      </PageHeader>

      <StatStrip :items="ruleStats" />

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
                  <span>
                    {{ pkg.name }}
                    <el-tag v-if="(pkg.scope || 'project') === 'global'" size="small" type="warning" effect="plain" class="scope-tag">全局</el-tag>
                  </span>
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

            <!-- 推荐规则包（前端预制模板，可一键应用或创建为自定义包） -->
            <div class="card-header compact">
              <strong>推荐规则包</strong>
              <small>按语言/场景预置，一键应用或保存</small>
            </div>
            <div class="preset-package-list">
              <div
                v-for="tpl in rulePackageTemplates"
                :key="tpl.name"
                class="preset-package-item"
              >
                <div class="preset-package-main">
                  <div class="preset-package-head">
                    <span class="preset-package-name">{{ tpl.name }}</span>
                    <el-tag
                      v-for="lang in tpl.languages.slice(0, 3)"
                      :key="lang"
                      size="small"
                      effect="plain"
                      class="lang-tag"
                    >{{ lang }}</el-tag>
                  </div>
                  <small class="preset-package-desc">{{ tpl.description }}</small>
                  <div class="preset-package-meta">
                    <em>{{ tpl.rules.length }} 条规则</em>
                    <span v-if="tpl.includes.length" class="incl-text">{{ tpl.includes.join(' · ') }}</span>
                  </div>
                </div>
                <div class="preset-package-actions">
                  <el-button size="small" type="primary" plain @click="applyPresetPackage(tpl)">应用</el-button>
                  <el-button size="small" text @click="createPackageFromTemplate(tpl)">保存为包</el-button>
                </div>
              </div>
            </div>
            <el-divider />
            <div class="card-header compact">
              <strong>语言规则模板</strong>
              <small>一键加入对应语言的审查规则</small>
            </div>
            <div class="template-groups">
              <div v-for="group in ruleTemplateGroups" :key="group.category" class="template-group">
                <button
                  type="button"
                  class="group-header"
                  :class="{ expanded: expandedGroups.has(group.category) }"
                  @click="toggleGroup(group.category)"
                >
                  <el-icon class="group-caret"><ArrowRight /></el-icon>
                  <span class="group-label">{{ group.label }}</span>
                  <em class="group-count">{{ group.templates.length }}</em>
                </button>
                <div v-show="expandedGroups.has(group.category)" class="group-templates">
                  <button
                    v-for="template in group.templates"
                    :key="template.name"
                    class="template-item"
                    type="button"
                    @click="applyTemplate(template)"
                  >
                    <span class="template-add"><el-icon><Plus /></el-icon></span>
                    <span class="template-name">{{ template.name }}</span>
                    <small class="template-path">{{ template.path }}</small>
                  </button>
                </div>
              </div>
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
          <el-form-item label="作用范围">
            <el-radio-group v-model="rulePackageForm.scope">
              <el-radio value="project">可引用（默认，需被审查员/项目引用或 project_refs 匹配）</el-radio>
              <el-radio value="global">全局（始终合并到每次审查）</el-radio>
            </el-radio-group>
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
    </PageShell>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'
import { CircleCheckFilled, Plus, Refresh, ArrowRight, Files, FolderOpened, WarningFilled } from '@element-plus/icons-vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import PageShell from '@/components/common/PageShell.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import StatStrip from '@/components/common/StatStrip.vue'
import RuleEditor from '@/components/RuleEditor.vue'
import { ruleTemplateGroups, ruleTemplates, rulePackageTemplates } from '@/constants/rule-templates'
import type { RuleTemplate, RulePackageTemplate } from '@/constants/rule-templates'
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

// 语言模板分组折叠状态：默认展开「通用」组，其余点击展开。
const expandedGroups = ref<Set<string>>(new Set(['common']))
function toggleGroup(category: string) {
  const next = new Set(expandedGroups.value)
  if (next.has(category)) next.delete(category)
  else next.add(category)
  expandedGroups.value = next
}

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

const ruleStats = computed(() => [
  { icon: Files, label: '规则总数', value: normalizedRules.value.length, tone: 'primary' as const },
  { icon: FolderOpened, label: '覆盖路径', value: coveredPathCount.value, tone: 'info' as const },
  { icon: WarningFilled, label: '待处理问题', value: validationIssues.value.length, tone: validationIssues.value.length > 0 ? ('danger' as const) : ('success' as const) },
])

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
  scope: 'project' as 'global' | 'project',
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

// 推荐规则包：把预制模板的规则去重后加入全局规则库（不校验 enabled，模板本身就是推荐）。
function applyPresetPackage(tpl: RulePackageTemplate) {
  const existing = new Set(normalizedRules.value.map((rule) => `${rule.path}::${rule.rule}`))
  const next = tpl.rules
    .map((rule) => ({ path: normalizePath(rule.path || ''), rule: rule.rule || '' }))
    .filter((rule) => {
      const key = `${rule.path}::${rule.rule}`
      if (existing.has(key)) return false
      existing.add(key)
      return true
    })
  if (!next.length) {
    ElMessage.info('该规则包的规则已经全部存在')
    return
  }
  globalRules.value.push(...next)
  ElMessage.success(`已加入 ${next.length} 条规则：${tpl.name}`)
}

// 推荐规则包：用模板数据填充"新建规则包"对话框，用户可调整后保存为自定义包。
function createPackageFromTemplate(tpl: RulePackageTemplate) {
  editingRulePackageID.value = undefined
  rulePackageForm.value = {
    name: tpl.name,
    description: tpl.description,
    enabled: true,
    scope: 'project',
    languagesText: tpl.languages.join(', '),
    tagsText: tpl.tags.join(', '),
    includesText: tpl.includes.join('\n'),
    excludesText: tpl.excludes.join('\n'),
    projectRefsText: '',
    rules: tpl.rules.map((rule) => ({ ...rule })),
  }
  rulePackageDialog.value = true
}

function openNewRulePackage() {
  editingRulePackageID.value = undefined
  rulePackageForm.value = {
    name: '',
    description: '',
    enabled: true,
    scope: 'project',
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
    scope: pkg.scope || 'project',
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
      scope: form.scope,
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
.card-header span {
  display: block;
  color: var(--app-muted);
  font-size: 12px;
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

/* 语言模板分组 */
.template-groups {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.template-group {
  border-radius: 8px;
  overflow: hidden;
}

.group-header {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 8px 10px;
  border: 0;
  border-radius: 8px;
  background: #f5f7fa;
  color: var(--app-text);
  text-align: left;
  cursor: pointer;
  transition: background 120ms ease;
}

.group-header:hover {
  background: #eef2f7;
}

.group-caret {
  color: var(--app-muted);
  font-size: 12px;
  transition: transform 160ms ease;
}

.group-header.expanded .group-caret {
  transform: rotate(90deg);
}

.group-label {
  flex: 1;
  font-size: 13px;
  font-weight: 700;
}

.group-count {
  padding: 1px 7px;
  border-radius: 8px;
  background: #e4e7ed;
  color: var(--app-muted);
  font-size: 11px;
  font-style: normal;
}

.group-templates {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 6px 4px 8px 10px;
}

.template-item {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 9px 11px;
  border: 1px solid #edf0f5;
  border-radius: 8px;
  background: #fff;
  color: var(--app-text);
  text-align: left;
  cursor: pointer;
  transition: 160ms ease;
}

.template-item:hover {
  border-color: #c7d2fe;
  background: #f5f7ff;
}

.template-add {
  display: grid;
  width: 22px;
  height: 22px;
  flex-shrink: 0;
  place-items: center;
  border-radius: 6px;
  background: #eff6ff;
  color: #2563eb;
  font-size: 12px;
}

.template-name {
  flex: 1;
  min-width: 0;
  font-size: 13px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.template-path {
  flex-shrink: 0;
  color: var(--app-muted);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 10px;
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
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
  font-size: 13px;
  font-weight: 800;
}

.scope-tag {
  flex-shrink: 0;
  font-weight: 400;
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

/* 推荐规则包（前端预制模板） */
.preset-package-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.preset-package-item {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 10px;
  align-items: center;
  padding: 12px;
  border: 1px solid #e6ebf2;
  border-radius: var(--card-radius);
  background: linear-gradient(135deg, #fafffefb 0%, #fff 100%);
}

.preset-package-item:hover {
  border-color: #9db1ff;
}

.preset-package-main {
  min-width: 0;
}

.preset-package-head {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.preset-package-name {
  font-size: 13px;
  font-weight: 800;
  color: var(--app-text);
}

.lang-tag {
  flex-shrink: 0;
}

.preset-package-desc {
  display: block;
  margin-top: 5px;
  color: var(--app-muted);
  font-size: 12px;
  line-height: 1.5;
}

.preset-package-meta {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 6px;
  flex-wrap: wrap;
}

.preset-package-meta em {
  color: var(--app-primary);
  font-size: 11px;
  font-style: normal;
}

.incl-text {
  color: var(--app-muted);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 10px;
}

.preset-package-actions {
  display: flex;
  flex-direction: column;
  gap: 4px;
  flex-shrink: 0;
}

.dialog-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 120px;
  gap: 12px;
}

.dialog-grid.two {
  grid-template-columns: repeat(2, minmax(0, 1fr));
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
