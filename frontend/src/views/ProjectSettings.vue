<template>
  <AppLayout>
    <PageShell>
      <PageHeader
        eyebrow="PROJECT POLICY"
        title="项目策略工作台"
        description="为每个仓库定义门禁、审查范围、引擎覆盖和专属规则，让高风险变更在合并前被清楚拦住。"
      >
        <template #actions>
          <StatStrip :items="policyStats" />
        </template>
      </PageHeader>

      <div class="content-grid">
        <!-- 左栏：策略库 -->
        <aside class="policy-sidebar panel-shell">
          <div class="panel-heading">
            <div>
              <strong>策略库</strong>
              <span>选择一个仓库编辑</span>
            </div>
            <el-button circle text aria-label="刷新策略列表" @click="fetchPolicies" :loading="loading">
              <el-icon><Refresh /></el-icon>
            </el-button>
          </div>

          <el-button type="primary" plain class="new-policy-btn" @click="startCreate">
            <el-icon><Plus /></el-icon>
            新建策略
          </el-button>

          <el-input
            v-if="policies.length"
            v-model="policyKeyword"
            class="policy-search"
            clearable
            placeholder="搜索平台或仓库"
          >
            <template #prefix><el-icon><Search /></el-icon></template>
          </el-input>

          <el-empty v-if="!loading && filteredPolicies.length === 0 && policies.length === 0" description="暂无项目策略">
            <template #description>
              <p>暂无项目策略</p>
              <p class="empty-hint">被回调审查过的项目会自动生成默认策略；也可手动创建。</p>
            </template>
            <el-button type="primary" @click="startCreate">
              <el-icon><Plus /></el-icon>
              创建策略
            </el-button>
          </el-empty>

          <div v-else v-loading="loading" class="policy-list">
            <button
              v-for="policy in filteredPolicies"
              :key="policyKey(policy)"
              class="policy-item"
              :class="{ active: !isWizard && policyKey(policy) === currentPolicyKey }"
              type="button"
              @click="handleEdit(policy)"
            >
              <span class="policy-main">
                <span class="provider-tag">{{ policy.provider }}</span>
                <strong>{{ policy.repo }}</strong>
              </span>
              <span class="policy-meta">
                <span>{{ getFailOnLabel(policy.fail_on) }}</span>
                <span>忽略 {{ policy.ignore_paths?.length || 0 }}</span>
                <span>规则 {{ policy.rules?.length || 0 }}</span>
              </span>
            </button>
          </div>
        </aside>

        <!-- 右栏：编辑器（新建=向导 / 编辑=标签页） -->
        <main class="editor-column panel-shell">
          <div class="editor-header">
            <div class="editor-title">
              <span class="editor-state">{{ isWizard ? '新建策略' : editingExisting ? '正在编辑' : '新建策略' }}</span>
              <strong>{{ form.repo || '选择或输入仓库' }}</strong>
              <small v-if="dirty" class="dirty-tag">有未保存修改</small>
            </div>
            <el-button
              v-if="!isWizard && editingExisting"
              type="success"
              plain
              size="small"
              @click="openAgentDialog"
            >
              <el-icon><User /></el-icon>
              配置审查员
            </el-button>
          </div>

          <!-- ===== 新建：分步向导 ===== -->
          <div v-if="isWizard" class="wizard">
            <el-steps :active="wizardStep" align-center class="wizard-steps">
              <el-step title="仓库定位" description="选平台和仓库" />
              <el-step title="质量门禁" description="阻断级别" />
              <el-step title="审查范围" description="忽略路径" />
              <el-step title="完成" description="确认保存" />
            </el-steps>

            <div class="wizard-body">
              <!-- Step 0: 仓库定位 -->
              <section v-if="wizardStep === 0" class="wizard-step">
                <div class="field-grid two-columns">
                  <label class="field-block">
                    <span>代码平台</span>
                    <el-select v-model="form.provider" @change="handleProviderChange" placeholder="请先配置代码平台">
                      <el-option v-for="p in platforms" :key="p.provider" :label="p.provider" :value="p.provider" />
                    </el-select>
                    <small v-if="platforms.length === 0">
                      暂无代码平台，请先到 <router-link to="/settings/platforms">Git 平台</router-link> 添加。
                    </small>
                  </label>
                  <label class="field-block">
                    <span>仓库</span>
                    <RepoSelect
                      v-model="form.repo"
                      :provider="form.provider"
                      :allow-create="true"
                      :immediate="true"
                      secondary-field="none"
                      placeholder="选择或输入 owner/repo"
                    />
                    <small>仓库列表加载失败时也可以手动输入。</small>
                  </label>
                </div>
              </section>

              <!-- Step 1: 质量门禁 -->
              <section v-else-if="wizardStep === 1" class="wizard-step">
                <p class="step-hint">达到所选严重级别时，审查结果会判定为未通过。推荐大多数仓库选「阻断高风险及以上」。</p>
                <div class="gate-options">
                  <button
                    v-for="option in failOnOptions"
                    :key="option.value"
                    class="gate-option"
                    :class="{ active: form.fail_on === option.value }"
                    type="button"
                    @click="form.fail_on = option.value"
                  >
                    <strong>{{ option.label }}</strong>
                    <span>{{ option.description }}</span>
                  </button>
                </div>
                <div class="section-title compact-title engine-title"><strong>审查引擎</strong></div>
                <p class="step-hint">本仓库默认使用的审查引擎；单次审查仍可在创建时临时切换。</p>
                <el-radio-group v-model="form.review_engine">
                  <el-radio value="">OCR 多智能体（默认）</el-radio>
                  <el-radio value="ocr">OCR（显式）</el-radio>
                  <el-radio
                    value="argus"
                    :disabled="!argusPlatformSupported"
                    :title="argusPlatformSupported ? '' : 'Argus 暂不支持 ' + form.provider + ' 平台（仅 gitea/github/gitlab）'"
                  >
                    Argus Bot
                  </el-radio>
                </el-radio-group>
                <small v-if="!argusPlatformSupported" class="engine-hint argus-unsupported-hint">
                  {{ form.provider }} 平台暂不支持 Argus（仅 gitea/github/gitlab），已禁用该选项。
                </small>
                <div class="section-title compact-title engine-title"><strong>门禁依赖模式</strong></div>
                <p class="step-hint">决定 PR 上门禁 check 的红绿：强=终态失败标红（可配分支保护必需检查）；弱=一律留痕，不阻塞「检查通过后合并」的自动合并。</p>
                <el-radio-group v-model="form.gate_mode">
                  <el-radio value="">跟随默认（OCR 强 / Argus 弱）</el-radio>
                  <el-radio value="strong">强依赖</el-radio>
                  <el-radio value="weak">弱依赖</el-radio>
                </el-radio-group>
              </section>

              <!-- Step 2: 审查范围 -->
              <section v-else-if="wizardStep === 2" class="wizard-step">
                <p class="step-hint">忽略生成文件、依赖目录和构建产物，减少模型调用噪音。可留空。</p>
                <el-input v-model="ignorePathsText" type="textarea" :rows="5" placeholder="vendor/**&#10;dist/**&#10;**/*.pb.go" />
                <div class="quick-actions">
                  <el-button size="small" @click="addIgnoreTemplate('vendor/**')">vendor/**</el-button>
                  <el-button size="small" @click="addIgnoreTemplate('dist/**')">dist/**</el-button>
                  <el-button size="small" @click="addIgnoreTemplate('**/*.pb.go')">*.pb.go</el-button>
                  <el-button size="small" @click="addIgnoreTemplate('node_modules/**')">node_modules</el-button>
                </div>
              </section>

              <!-- Step 3: 完成确认 -->
              <section v-else class="wizard-step">
                <el-result icon="success" title="策略信息已就绪" sub-title="确认以下信息，点击保存完成创建。如需高级配置（OCR 覆盖、专属规则），保存后在编辑页继续。">
                  <template #extra>
                    <div class="confirm-summary">
                      <div><span>平台 / 仓库</span><strong>{{ form.provider }} / {{ form.repo }}</strong></div>
                      <div><span>门禁</span><strong>{{ getFailOnLabel(form.fail_on) }}</strong></div>
                      <div><span>忽略路径</span><strong>{{ normalizedIgnorePaths.length }} 条</strong></div>
                    </div>
                  </template>
                </el-result>
              </section>
            </div>

            <!-- 向导底部导航 -->
            <div class="wizard-nav">
              <el-button v-if="wizardStep > 0" @click="wizardPrev">上一步</el-button>
              <div class="wizard-nav-spacer" />
              <el-button v-if="wizardStep < 3" type="primary" :disabled="!wizardStepValid" @click="wizardNext">下一步</el-button>
              <el-button v-else type="primary" :loading="saving" :disabled="hasBlockingIssues" @click="handleSave">
                <el-icon><CircleCheckFilled /></el-icon>保存策略
              </el-button>
            </div>
          </div>

          <!-- ===== 编辑：标签页 ===== -->
          <div v-else class="editor-tabs">
            <el-tabs v-model="activeTab">
              <el-tab-pane label="基本信息" name="basic">
                <section class="form-section">
                  <div class="field-grid two-columns">
                    <label class="field-block">
                      <span>代码平台</span>
                      <el-select v-model="form.provider" @change="handleProviderChange" :disabled="editingExisting">
                        <el-option v-for="p in platforms" :key="p.provider" :label="p.provider" :value="p.provider" />
                      </el-select>
                    </label>
                    <label class="field-block">
                      <span>仓库</span>
                      <RepoSelect
                        v-model="form.repo"
                        :provider="form.provider"
                        :allow-create="true"
                        :immediate="true"
                        secondary-field="none"
                        :disabled="editingExisting"
                      />
                    </label>
                  </div>
                </section>

                <section class="form-section">
                  <div class="section-title compact-title"><strong>质量门禁</strong></div>
                  <div class="gate-options">
                    <button v-for="option in failOnOptions" :key="option.value" class="gate-option" :class="{ active: form.fail_on === option.value }" type="button" @click="form.fail_on = option.value">
                      <strong>{{ option.label }}</strong>
                      <span>{{ option.description }}</span>
                    </button>
                  </div>
                  <div class="section-title compact-title engine-title"><strong>审查引擎</strong></div>
                  <el-radio-group v-model="form.review_engine">
                    <el-radio value="">OCR 多智能体（默认）</el-radio>
                    <el-radio value="ocr">OCR（显式）</el-radio>
                    <el-radio
                      value="argus"
                      :disabled="!argusPlatformSupported"
                      :title="argusPlatformSupported ? '' : 'Argus 暂不支持 ' + form.provider + ' 平台（仅 gitea/github/gitlab）'"
                    >
                      Argus Bot
                    </el-radio>
                  </el-radio-group>
                  <small class="engine-hint">选 Argus 后，本仓库的 webhook 与控制台创建的评审默认派发给外部 Argus Bot；单次创建时仍可临时切换。</small>
                  <small v-if="!argusPlatformSupported" class="engine-hint argus-unsupported-hint">
                    {{ form.provider }} 平台暂不支持 Argus（仅 gitea/github/gitlab），已禁用该选项；已保存的 argus 策略保存时会回落为空（OCR）。
                  </small>
                  <div class="section-title compact-title engine-title"><strong>门禁依赖模式</strong></div>
                  <el-radio-group v-model="form.gate_mode">
                    <el-radio value="">跟随默认（OCR 强 / Argus 弱）</el-radio>
                    <el-radio value="strong">强依赖</el-radio>
                    <el-radio value="weak">弱依赖</el-radio>
                  </el-radio-group>
                  <small class="engine-hint">强依赖：评审未过 / 执行失败在 PR 上标红，可配为必需检查；弱依赖：任何结果仅 success 留痕（描述带「【弱门禁】」），不阻塞自动合并。</small>
                </section>

                <section class="form-section">
                  <div class="section-title compact-title"><strong>审查范围（忽略路径）</strong></div>
                  <el-input v-model="ignorePathsText" type="textarea" :rows="4" placeholder="vendor/**&#10;dist/**" />
                  <div class="quick-actions">
                    <el-button size="small" @click="addIgnoreTemplate('vendor/**')">vendor/**</el-button>
                    <el-button size="small" @click="addIgnoreTemplate('dist/**')">dist/**</el-button>
                    <el-button size="small" @click="addIgnoreTemplate('node_modules/**')">node_modules</el-button>
                  </div>
                </section>
              </el-tab-pane>

              <el-tab-pane name="ocr">
                <template #label>
                  OCR 引擎覆盖
                  <el-badge v-if="ocrOverrideSummary !== '继承全局'" is-dot type="primary" />
                </template>
                <OCREngineOverride
                  :form="form"
                  :ocr-enabled-mode="ocrEnabledMode"
                  @update:ocr-enabled-mode="ocrEnabledMode = $event as any"
                  @update:ocr-default-severity="form.ocr_default_severity = $event"
                  @update:ocr-audience="form.ocr_audience = $event"
                  @update:ocr-language="form.ocr_language = $event"
                  @update:ocr-background="form.ocr_background = $event"
                  @update:ocr-background-file="form.ocr_background_file = $event"
                  @update:ocr-concurrency="form.ocr_concurrency = $event"
                  @update:ocr-max-git-procs="form.ocr_max_git_procs = $event"
                  @update:ocr-timeout-minutes="form.ocr_timeout_minutes = $event"
                  @update:ocr-max-tools="form.ocr_max_tools = $event"
                  @update:ocr-use-anthropic="form.ocr_use_anthropic = $event"
                  @update:ocr-protocol="form.ocr_protocol = $event"
                  @update:ocr-url="form.ocr_url = $event"
                  @update:ocr-model="form.ocr_model = $event"
                  @update:ocr-binary="form.ocr_binary = $event"
                  @update:ocr-tools-path="form.ocr_tools_path = $event"
                  @update:ocr-extra-body="form.ocr_extra_body = $event"
                  @update:ocr-telemetry-enabled="form.ocr_telemetry_enabled = $event"
                  @update:ocr-telemetry-exporter="form.ocr_telemetry_exporter = $event"
                  @update:ocr-telemetry-otlp-endpoint="form.ocr_telemetry_otlp_endpoint = $event"
                  @update:ocr-telemetry-content-logging="form.ocr_telemetry_content_logging = $event"
                />
              </el-tab-pane>

              <el-tab-pane label="项目专属规则" name="rules">
                <p class="tab-hint">这里只写该仓库特有要求；通用规则在「审查规则」页维护，可在下方引用。</p>
                <RuleEditor v-model="form.rules" :issues="projectRuleIssues" empty-description="当前没有项目专属规则" />

                <div class="rule-package-section">
                  <div class="section-title">
                    <strong>引用规则包</strong>
                    <span>勾选的规则集会与项目专属规则一起生效；规则集中维护，改了自动同步</span>
                  </div>
                  <RulePackagePicker v-model="form.rule_package_ids" />
                </div>
              </el-tab-pane>
            </el-tabs>

            <!-- 编辑模式：保存前检查 + 生效预览（折叠到标签页下方，不再是独立第三栏） -->
            <div class="editor-footer-info">
              <div v-if="validationIssues.length" class="issues-inline">
                <strong>需处理（{{ validationIssues.length }}）：</strong>
                <span v-for="(issue, i) in validationIssues.slice(0, 3)" :key="i" class="issue-chip">{{ issue }}</span>
                <span v-if="validationIssues.length > 3" class="issue-more">等 {{ validationIssues.length }} 项</span>
              </div>
              <div v-else class="issues-inline ok">
                <el-icon><CircleCheckFilled /></el-icon> 表单校验通过，可保存
              </div>
              <el-button v-if="editingExisting" type="danger" plain size="small" @click="handleDeleteSelected">
                <el-icon><Delete /></el-icon>删除策略
              </el-button>
            </div>
          </div>
        </main>
      </div>

      <!-- 底部 sticky 保存条（编辑模式） -->
      <div v-if="!isWizard" class="sticky-save-bar" :class="{ dirty }">
        <div class="save-bar-info">
          <strong>{{ form.provider }}/{{ form.repo || '未选择' }}</strong>
          <span>{{ getFailOnLabel(form.fail_on) }} · 忽略 {{ normalizedIgnorePaths.length }} · 规则 {{ normalizedRules.length }}</span>
        </div>
        <div class="save-bar-actions">
          <el-button v-if="dirty" plain size="small" @click="handleEdit(payloadPreview)">放弃修改</el-button>
          <el-button type="primary" :loading="saving" :disabled="hasBlockingIssues" @click="handleSave">
            <el-icon><CircleCheckFilled /></el-icon>保存策略
          </el-button>
        </div>
      </div>

      <el-dialog
        v-model="agentDialogVisible"
        :title="`${currentProject?.provider}/${currentProject?.repo} - 审查员配置`"
        width="760px"
      >
        <ProjectAgentConfig
          v-if="currentProject"
          :provider="currentProject.provider"
          :repo="currentProject.repo"
        />
      </el-dialog>
    </PageShell>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'
import {
  CircleCheckFilled,
  Delete,
  Plus,
  Refresh,
  Search,
  User,
  Files,
  Lock,
  Document,
} from '@element-plus/icons-vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import PageShell from '@/components/common/PageShell.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import StatStrip from '@/components/common/StatStrip.vue'
import ProjectAgentConfig from '@/components/ProjectAgentConfig.vue'
import RuleEditor from '@/components/RuleEditor.vue'
import RulePackagePicker from '@/components/RulePackagePicker.vue'
import OCREngineOverride from '@/components/project-settings/OCREngineOverride.vue'
import RepoSelect from '@/components/common/RepoSelect.vue'
import { normalizeRules, policyKey, uniqueLines, validatePolicyDraft } from '@/utils/policy-form'
import { getProjectPolicies, putProjectPolicy, deleteProjectPolicy, getGitPlatforms } from '@/api/config'
import type { ProjectPolicy, ReviewRule, GitPlatform } from '@/types/config'

type OCREnabledMode = 'inherit' | 'enabled' | 'disabled'
type ProjectPolicyForm = Omit<ProjectPolicy, 'ignore_paths' | 'rules' | 'ocr_enabled' | 'rule_package_ids'> & {
  ignore_paths: string[]
  rules: ReviewRule[]
  ocr_enabled: boolean | null
  ocr_default_severity: ProjectPolicy['ocr_default_severity']
  rule_package_ids: number[]
}

const failOnOptions = [
  { value: 'critical', label: '只阻断严重问题', description: '适合刚接入时降低噪音' },
  { value: 'major', label: '阻断高风险及以上', description: '推荐用于大多数业务仓库' },
  { value: 'minor', label: '阻断中风险及以上', description: '适合质量要求较高的核心仓库' },
  { value: 'info', label: '阻断所有问题', description: '适合强规范或发布前专项检查' },
]

const loading = ref(false)
const saving = ref(false)
const policies = ref<ProjectPolicy[]>([])
const agentDialogVisible = ref(false)
const currentProject = ref<{ provider: string; repo: string } | null>(null)
const platforms = ref<GitPlatform[]>([])
const policyKeyword = ref('')
const originalJson = ref('')

// 向导/编辑模式：新建策略走分步向导（降低 47 字段的信息过载）；编辑走标签页。
const isWizard = ref(true) // 新建时 true，编辑已有策略时 false
const wizardStep = ref(0) // 0 仓库定位 · 1 质量门禁 · 2 审查范围 · 3（完成可保存）
const activeTab = ref('basic') // 编辑模式标签：basic / ocr / rules

const form = reactive<ProjectPolicyForm>({
  provider: '',
  repo: '',
  fail_on: 'major',
  review_engine: '' as any,
  gate_mode: '' as any,
  ignore_paths: [] as string[],
  rules: [] as ReviewRule[],
  ocr_enabled: null,
  ocr_concurrency: 0,
  ocr_timeout_minutes: 0,
  ocr_max_tools: 0,
  ocr_default_severity: '',
  ocr_background: '',
  ocr_background_file: '',
  ocr_audience: '',
  ocr_provider: '',
  ocr_url: '',
  ocr_model: '',
  ocr_max_git_procs: 0,
  ocr_use_anthropic: null as any,
  ocr_protocol: '' as any,
  ocr_extra_body: '',
  ocr_language: '',
  ocr_binary: '',
  ocr_tools_path: '',
  ocr_telemetry_enabled: null as any,
  ocr_telemetry_exporter: '',
  ocr_telemetry_otlp_endpoint: '',
  ocr_telemetry_content_logging: null as any,
  rule_package_ids: [] as number[],
})

const ignorePathsText = ref('')

const normalizedIgnorePaths = computed(() => uniqueLines(ignorePathsText.value))

// argus 平台支持面（与后端 argusclient.SupportsPlatform 同源）：不支持时
// 前端禁用 argus 选项，后端 resolveEngine 亦会 fail-fast 兜底。
const ARGUS_PLATFORMS = ['gitea', 'github', 'gitlab']
const argusPlatformSupported = computed(() => ARGUS_PLATFORMS.includes(form.provider))

const normalizedRules = computed(() => normalizeRules(form.rules))

const payloadPreview = computed<ProjectPolicy>(() => ({
  provider: form.provider.trim(),
  repo: form.repo.trim(),
  fail_on: form.fail_on,
  review_engine: form.review_engine || '',
  gate_mode: form.gate_mode || '',
  ignore_paths: normalizedIgnorePaths.value,
  rules: normalizedRules.value,
  ocr_enabled: ocrEnabledValue.value,
  ocr_concurrency: form.ocr_concurrency || 0,
  ocr_timeout_minutes: form.ocr_timeout_minutes || 0,
  ocr_max_tools: form.ocr_max_tools || 0,
  ocr_default_severity: form.ocr_default_severity || '',
  ocr_background: form.ocr_background?.trim() || '',
  ocr_background_file: form.ocr_background_file?.trim() || '',
  ocr_audience: form.ocr_audience || '',
  ocr_provider: form.ocr_provider?.trim() || '',
  ocr_url: form.ocr_url?.trim() || '',
  ocr_model: form.ocr_model?.trim() || '',
  ocr_max_git_procs: form.ocr_max_git_procs || 0,
  ocr_use_anthropic: form.ocr_use_anthropic as any,
  ocr_protocol: form.ocr_protocol || '',
  ocr_extra_body: form.ocr_extra_body?.trim() || '',
  ocr_language: form.ocr_language || '',
  ocr_binary: form.ocr_binary?.trim() || '',
  ocr_tools_path: form.ocr_tools_path?.trim() || '',
  ocr_telemetry_enabled: form.ocr_telemetry_enabled as any,
  ocr_telemetry_exporter: form.ocr_telemetry_exporter || '',
  ocr_telemetry_otlp_endpoint: form.ocr_telemetry_otlp_endpoint?.trim() || '',
  ocr_telemetry_content_logging: form.ocr_telemetry_content_logging as any,
  rule_package_ids: [...form.rule_package_ids],
}))

const previewJson = computed(() => JSON.stringify(payloadPreview.value, null, 2))
const ocrEnabledValue = computed<boolean | null>(() => {
  if (ocrEnabledMode.value === 'enabled') return true
  if (ocrEnabledMode.value === 'disabled') return false
  return null
})
const ocrEnabledMode = computed<OCREnabledMode>({
  get() {
    if (form.ocr_enabled === true) return 'enabled'
    if (form.ocr_enabled === false) return 'disabled'
    return 'inherit'
  },
  set(value) {
    form.ocr_enabled = value === 'enabled' ? true : value === 'disabled' ? false : null
  },
})
const dirty = computed(() => previewJson.value !== originalJson.value)
const currentPolicyKey = computed(() => form.provider && form.repo ? `${form.provider}:${form.repo}` : '')
const editingExisting = computed(() => policies.value.some((policy) => policyKey(policy) === currentPolicyKey.value))
const strictPolicyCount = computed(() => policies.value.filter((policy) => ['major', 'minor', 'info'].includes(policy.fail_on)).length)
const totalRuleCount = computed(() => policies.value.reduce((sum, policy) => sum + (policy.rules?.length || 0), 0))

const policyStats = computed(() => [
  { icon: Files, label: '已配置', value: policies.value.length, tone: 'primary' as const },
  { icon: Lock, label: '严格门禁', value: strictPolicyCount.value, tone: 'danger' as const },
  { icon: Document, label: '项目规则', value: totalRuleCount.value, tone: 'info' as const },
])
const ocrOverrideSummary = computed(() => {
  const items: string[] = []
  if (ocrEnabledMode.value !== 'inherit') items.push(ocrEnabledMode.value === 'enabled' ? '启用' : '禁用')
  if (form.ocr_default_severity) items.push(`严重级别 ${form.ocr_default_severity}`)
  if (form.ocr_concurrency) items.push(`并发 ${form.ocr_concurrency}`)
  if (form.ocr_timeout_minutes) items.push(`超时 ${form.ocr_timeout_minutes}m`)
  if (form.ocr_max_tools) items.push(`工具 ${form.ocr_max_tools}`)
  if (form.ocr_background?.trim()) items.push('背景')
  return items.length > 0 ? `${items.length} 项` : '继承全局'
})

const filteredPolicies = computed(() => {
  const keyword = policyKeyword.value.trim().toLowerCase()
  if (!keyword) return policies.value
  return policies.value.filter((policy) => {
    return policy.provider.toLowerCase().includes(keyword) || policy.repo.toLowerCase().includes(keyword)
  })
})

const validationIssues = computed(() => validatePolicyDraft({
  provider: form.provider,
  repo: form.repo,
  fail_on: form.fail_on,
  ocr_default_severity: form.ocr_default_severity,
  failOnOptions,
  ignorePaths: normalizedIgnorePaths.value,
  rules: form.rules,
}))
const projectRuleIssues = computed(() => validationIssues.value.filter((issue) => issue.includes('项目规则')))
const hasBlockingIssues = computed(() => validationIssues.value.length > 0)


async function fetchPlatforms() {
  try {
    const result = await getGitPlatforms()
    platforms.value = result.items || []
    if (platforms.value.length > 0 && !form.provider) {
      form.provider = platforms.value[0].provider
      syncOriginal()
    }
  } catch (e) {
    console.error('Failed to load platforms:', e)
  }
}

function handleProviderChange() {
  form.repo = ''
  // 切到 argus 不支持的平台时回落引擎选择，避免保存后创建期被后端拒绝。
  if (!argusPlatformSupported.value && form.review_engine === 'argus') {
    form.review_engine = '' as any
  }
}

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
  if (hasBlockingIssues.value) {
    ElMessage.warning('请先处理保存前检查中的问题')
    return
  }
  saving.value = true
  try {
    const saved = await putProjectPolicy(payloadPreview.value)
    ElMessage.success('项目策略已保存')
    await fetchPolicies()
    handleEdit(saved)
  } finally {
    saving.value = false
  }
}

function handleEdit(policy: ProjectPolicy) {
  form.provider = policy.provider
  form.repo = policy.repo
  form.fail_on = policy.fail_on || 'major'
  form.review_engine = (policy.review_engine || '') as any
  form.gate_mode = (policy.gate_mode || '') as any
  form.ignore_paths = policy.ignore_paths || []
  form.rules = policy.rules ? policy.rules.map((rule) => ({ ...rule })) : []
  form.ocr_enabled = policy.ocr_enabled ?? null
  form.ocr_concurrency = policy.ocr_concurrency || 0
  form.ocr_timeout_minutes = policy.ocr_timeout_minutes || 0
  form.ocr_max_tools = policy.ocr_max_tools || 0
  form.ocr_default_severity = policy.ocr_default_severity || ''
  form.ocr_background = policy.ocr_background || ''
  form.ocr_background_file = policy.ocr_background_file || ''
  form.ocr_audience = policy.ocr_audience || ''
  form.ocr_provider = policy.ocr_provider || ''
  form.ocr_url = policy.ocr_url || ''
  form.ocr_model = policy.ocr_model || ''
  form.ocr_max_git_procs = policy.ocr_max_git_procs || 0
  form.ocr_use_anthropic = policy.ocr_use_anthropic ?? null as any
  form.ocr_protocol = policy.ocr_protocol || ''
  form.ocr_extra_body = policy.ocr_extra_body || ''
  form.ocr_language = policy.ocr_language || ''
  form.ocr_binary = policy.ocr_binary || ''
  form.ocr_tools_path = policy.ocr_tools_path || ''
  form.ocr_telemetry_enabled = policy.ocr_telemetry_enabled ?? null as any
  form.ocr_telemetry_exporter = policy.ocr_telemetry_exporter || ''
  form.ocr_telemetry_otlp_endpoint = policy.ocr_telemetry_otlp_endpoint || ''
  form.ocr_telemetry_content_logging = policy.ocr_telemetry_content_logging ?? null as any
  form.rule_package_ids = policy.rule_package_ids ? [...policy.rule_package_ids] : []
  ignorePathsText.value = (policy.ignore_paths || []).join('\n')
  syncOriginal()
  isWizard.value = false
  activeTab.value = 'basic'
}

function startCreate() {
  form.provider = platforms.value[0]?.provider || ''
  form.repo = ''
  form.fail_on = 'major'
  form.review_engine = '' as any
  form.gate_mode = '' as any
  form.ignore_paths = []
  form.rules = []
  form.ocr_enabled = null
  form.ocr_concurrency = 0
  form.ocr_timeout_minutes = 0
  form.ocr_max_tools = 0
  form.ocr_default_severity = ''
  form.ocr_background = ''
  form.ocr_background_file = ''
  form.ocr_audience = ''
  form.ocr_provider = ''
  form.ocr_url = ''
  form.ocr_model = ''
  form.ocr_max_git_procs = 0
  form.ocr_use_anthropic = null as any
  form.ocr_protocol = ''
  form.ocr_extra_body = ''
  form.ocr_language = ''
  form.ocr_binary = ''
  form.ocr_tools_path = ''
  form.ocr_telemetry_enabled = null as any
  form.ocr_telemetry_exporter = ''
  form.ocr_telemetry_otlp_endpoint = ''
  form.ocr_telemetry_content_logging = null as any
  form.rule_package_ids = []
  ignorePathsText.value = ''
  syncOriginal()
  isWizard.value = true
  wizardStep.value = 0
}

// 向导"下一步"校验：每步只检查该步必填项，逐步降低认知负担。
const wizardStepValid = computed(() => {
  if (wizardStep.value === 0) return Boolean(form.provider && form.repo.trim())
  if (wizardStep.value === 1) return failOnOptions.some((o) => o.value === form.fail_on)
  return true
})

function wizardNext() {
  if (wizardStep.value < 3 && wizardStepValid.value) wizardStep.value++
}
function wizardPrev() {
  if (wizardStep.value > 0) wizardStep.value--
}

function openAgentDialog() {
  currentProject.value = {
    provider: form.provider,
    repo: form.repo,
  }
  agentDialogVisible.value = true
}

async function handleDeleteSelected() {
  const policy = policies.value.find((item) => policyKey(item) === currentPolicyKey.value)
  if (policy) await handleDelete(policy)
}

async function handleDelete(policy: ProjectPolicy) {
  try {
    await ElMessageBox.confirm(`确定删除 ${policy.provider}/${policy.repo} 的项目策略？`, '确认删除')
    await deleteProjectPolicy(policy.provider, policy.repo)
    ElMessage.success('项目策略已删除')
    await fetchPolicies()
    if (policyKey(policy) === currentPolicyKey.value) startCreate()
  } catch {
    // 用户取消
  }
}

function addIgnoreTemplate(path: string) {
  const next = new Set(normalizedIgnorePaths.value)
  next.add(path)
  ignorePathsText.value = Array.from(next).join('\n')
}

function syncOriginal() {
  originalJson.value = previewJson.value
}

function getFailOnLabel(failOn: string) {
  const option = failOnOptions.find((item) => item.value === failOn)
  return option?.label || failOn
}

onMounted(async () => {
  await fetchPlatforms()
  await fetchPolicies()
  if (policies.value.length > 0) {
    handleEdit(policies.value[0])
  } else {
    syncOriginal()
  }
})
</script>

<style scoped>
.panel-heading span,
.section-title span,
.editor-header small,
.preview-list span {
  display: block;
  color: var(--app-muted);
  font-size: 12px;
}

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

.panel-heading,
.editor-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.panel-heading {
  min-height: 54px;
  padding: 15px 16px 12px;
  border-bottom: 1px solid var(--app-border);
}

.panel-heading strong,
.editor-header strong {
  display: block;
  color: var(--app-text);
  font-size: 15px;
}

.panel-heading.compact {
  min-height: auto;
  padding: 0;
  border-bottom: 0;
  margin-bottom: 13px;
}

.policy-sidebar {
  position: sticky;
  top: 18px;
  padding-bottom: 12px;
}

.policy-search {
  padding: 14px 14px 12px;
  margin: 0;
}

.policy-list {
  display: grid;
  gap: 7px;
  align-content: start;
  max-height: calc(100vh - 300px);
  min-height: 280px;
  padding: 0 10px 10px;
  overflow: auto;
}

.policy-item {
  width: 100%;
  padding: 11px 12px;
  border: 1px solid transparent;
  border-radius: var(--card-radius);
  background: #f7f8fc;
  text-align: left;
  cursor: pointer;
  transition: 160ms ease;
}

.policy-item:hover,
.policy-item.active {
  border-color: var(--app-primary);
  background: var(--app-surface);
  box-shadow: var(--app-shadow);
}

.policy-main {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.policy-main strong {
  flex: 1;
  overflow: hidden;
  color: var(--app-text);
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.provider-tag {
  display: inline-flex;
  align-items: center;
  flex: 0 0 auto;
  min-height: 22px;
  padding: 0 7px;
  border-radius: 6px;
  color: #5656c5;
  background: #f0f1ff;
  font-size: 11px;
  font-weight: 800;
}

.policy-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 8px;
  color: var(--app-muted);
  font-size: 12px;
}

.policy-meta span {
  display: inline-flex;
  min-height: 22px;
  align-items: center;
  padding: 0 7px;
  border-radius: 6px;
  background: #eef2f7;
}

.editor-column {
  min-width: 0;
  overflow: hidden;
}

.editor-header {
  min-height: 78px;
  padding: 18px 20px;
  border-bottom: 1px solid var(--app-border);
  background: #fbfcfe;
}

.editor-header strong {
  margin-top: 3px;
  overflow-wrap: anywhere;
  font-size: 19px;
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

.editor-header small {
  margin-top: 4px;
}

.editor-layout {
  display: grid;
  gap: 0;
}

.form-section {
  padding: 22px 20px 24px;
  border-bottom: 1px solid var(--app-border);
}

.form-section:last-child {
  border-bottom: none;
}

.section-title {
  display: grid;
  grid-template-columns: 42px minmax(0, 1fr);
  gap: 12px;
  align-items: start;
  margin-bottom: 16px;
}

.section-index {
  display: grid;
  width: 34px;
  height: 28px;
  place-items: center;
  border-radius: 8px;
  color: #4b54bf;
  background: #eef0ff;
  font-size: 12px;
  font-weight: 850;
}

.section-title strong {
  display: block;
  color: var(--app-text);
  font-size: 16px;
}

.section-title div > span {
  margin-top: 4px;
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

.field-block.full-width {
  margin-top: 14px;
}

.field-block > span {
  color: #445166;
  font-size: 12px;
  font-weight: 750;
}

.field-block small {
  color: var(--app-muted);
  font-size: 12px;
}

.gate-options {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.gate-option {
  min-height: 76px;
  padding: 13px 15px;
  border: 1px solid var(--app-border);
  border-radius: var(--card-radius);
  background: #f7f8fc;
  color: var(--app-text);
  text-align: left;
  cursor: pointer;
  transition: 160ms ease;
}

.gate-option.active {
  border-color: var(--app-primary);
  background: var(--app-surface);
  box-shadow: var(--app-shadow);
}

.gate-option strong,
.gate-option span {
  display: block;
}

.gate-option strong {
  font-size: 13px;
}

.gate-option span {
  margin-top: 6px;
  color: var(--app-muted);
  font-size: 12px;
  line-height: 1.5;
}

.quick-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 12px;
}

.inspector-column {
  min-width: 0;
}

.inspector-sticky {
  position: sticky;
  top: 18px;
  display: grid;
  gap: 12px;
}

.inspector-card,
.danger-card {
  padding: 16px;
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

.preview-list {
  display: grid;
  gap: 1px;
  overflow: hidden;
  margin-bottom: 0;
  border: 1px solid var(--app-border);
  border-radius: var(--card-radius);
  background: var(--app-border);
}

.preview-list > div {
  padding: 10px 12px;
  background: #fff;
}

.preview-list strong {
  display: block;
  margin-top: 3px;
  overflow-wrap: anywhere;
  color: var(--app-text);
  font-size: 13px;
}

.preview-json {
  display: none;
}

.advanced-collapse {
  margin-top: 16px;
  border: 1px solid var(--app-border);
  border-radius: var(--card-radius);
  overflow: hidden;
}

.advanced-collapse :deep(.el-collapse-item__header) {
  padding: 0 16px;
  border-bottom: 1px solid var(--app-border);
  color: var(--app-primary);
  font-size: 13px;
  font-weight: 600;
  background: #f7f8fc;
}

.advanced-collapse :deep(.el-collapse-item__content) {
  padding: 16px;
}

.advanced-collapse :deep(.el-collapse-item__wrap) {
  border-bottom: none;
}

.danger-card p {
  margin: 0 0 12px;
  color: var(--app-muted);
  font-size: 12px;
  line-height: 1.6;
}

/* ===== 新建策略按钮（侧栏顶部） ===== */
.new-policy-btn {
  margin: 12px 14px 0;
  width: calc(100% - 28px);
}

/* ===== 向导 ===== */
.wizard {
  display: flex;
  flex-direction: column;
  min-height: 420px;
}

.wizard-steps {
  padding: 20px 24px 0;
}

.wizard-body {
  flex: 1;
  padding: 24px;
}

.wizard-step {
  max-width: 640px;
}

.step-hint {
  margin: 0 0 16px;
  color: var(--app-muted);
  font-size: 13px;
  line-height: 1.7;
}

.wizard-nav {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 14px 24px;
  border-top: 1px solid var(--app-border);
  background: #fafbfc;
  border-radius: 0 0 14px 14px;
}

.wizard-nav-spacer {
  flex: 1;
}

.confirm-summary {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 12px;
  text-align: left;
}

.confirm-summary > div {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  padding: 8px 14px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: #fff;
  font-size: 13px;
}

.confirm-summary span {
  color: var(--app-muted);
}

.confirm-summary strong {
  color: var(--app-text);
}

/* ===== 编辑模式标签页 ===== */
.editor-tabs {
  padding: 0 20px 16px;
}

.editor-tabs :deep(.el-tabs__nav-wrap) {
  padding: 0 4px;
}

.compact-title {
  margin: 0 0 12px;
  font-size: 14px;
  color: var(--app-text);
}

.engine-title {
  margin-top: 20px;
}
.engine-hint {
  display: block;
  margin-top: 8px;
  color: var(--app-muted);
  font-size: 12px;
}
.engine-hint.argus-unsupported-hint {
  color: var(--el-color-warning);
}

.tab-hint {
  margin: 0 0 14px;
  color: var(--app-muted);
  font-size: 13px;
}

.empty-hint {
  margin: 4px 0 0;
  color: var(--app-muted);
  font-size: 12px;
}

.rule-package-section {
  margin-top: 22px;
  padding-top: 18px;
  border-top: 1px solid var(--app-border);
}

.rule-package-section .section-title {
  margin-bottom: 12px;
}

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

/* ===== 编辑器标题 ===== */
.editor-title {
  display: flex;
  align-items: baseline;
  gap: 10px;
  flex-wrap: wrap;
}

.editor-state {
  color: var(--app-muted);
  font-size: 12px;
  font-weight: 600;
}

.dirty-tag {
  padding: 2px 8px;
  border-radius: 6px;
  background: #fef3c7;
  color: #b45309;
  font-size: 11px;
  font-weight: 600;
}

/* ===== 底部 sticky 保存条 ===== */
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

  .policy-sidebar {
    position: static;
  }

  .policy-list {
    max-height: 280px;
  }

  .field-grid.two-columns,
  .gate-options {
    grid-template-columns: 1fr;
  }

  .sticky-save-bar {
    flex-direction: column;
    align-items: stretch;
  }
}

@media (max-width: 720px) {
  .wizard-body,
  .wizard-nav,
  .editor-tabs {
    padding-left: 14px;
    padding-right: 14px;
  }
}
</style>
