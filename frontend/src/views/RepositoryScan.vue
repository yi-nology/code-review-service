<template>
  <AppLayout>
    <div class="scan-page">
      <header class="page-header">
        <div>
          <div class="eyebrow">OCR SCAN</div>
          <h1>代码库扫描</h1>
          <p>对整个仓库或指定目录执行 Open Code Review 全量扫描</p>
        </div>
        <el-button type="primary" :loading="running" :disabled="!canRun" @click="runScan(false)">
          <el-icon><Search /></el-icon>
          开始扫描
        </el-button>
      </header>

      <div class="scan-grid">
        <section class="panel config-panel">
          <div class="panel-header">
            <h2>扫描参数</h2>
            <span>Full-file audit</span>
          </div>

          <el-form label-position="top" :model="form">
            <el-form-item label="代码平台">
              <el-select v-model="form.provider" placeholder="选择平台" @change="handleProviderChange">
                <el-option v-for="p in platforms" :key="p.provider" :label="p.provider" :value="p.provider" />
              </el-select>
            </el-form-item>

            <el-form-item label="仓库">
              <el-select
                v-model="form.repo"
                filterable
                remote
                reserve-keyword
                :loading="reposLoading"
                placeholder="选择或输入 owner/repo"
                @focus="loadRepos"
                @change="handleRepoChange"
              >
                <el-option v-for="repo in repos" :key="repo.full_name" :label="repo.full_name" :value="repo.full_name" />
              </el-select>
            </el-form-item>

            <el-form-item label="分支 / 提交">
              <div class="branch-row">
                <el-select
                  v-model="branch"
                  filterable
                  allow-create
                  default-first-option
                  :loading="branchesLoading"
                  placeholder="main / release / commit sha"
                  @focus="loadBranches"
                  @change="resolveHead"
                >
                  <el-option v-for="b in branches" :key="b.name" :label="b.name" :value="b.name" />
                </el-select>
                <el-button :loading="resolving" @click="resolveHead">解析</el-button>
              </div>
              <div class="form-tip">当前 head：<code>{{ form.head_sha || '未解析' }}</code></div>
            </el-form-item>

            <el-form-item label="扫描路径">
              <el-input
                v-model="pathsText"
                type="textarea"
                :rows="3"
                placeholder="每行一个目录或文件，留空扫描整个仓库&#10;internal&#10;frontend/src"
              />
            </el-form-item>

            <el-form-item label="排除路径">
              <el-input
                v-model="excludeText"
                type="textarea"
                :rows="3"
                placeholder="每行一个 gitignore 风格规则&#10;**/generated/**&#10;vendor/**"
              />
            </el-form-item>

            <el-form-item label="业务背景">
              <el-input v-model="form.background" type="textarea" :rows="3" placeholder="例如：迁移前质量体检，重点关注兼容性和安全风险" />
            </el-form-item>

            <div class="inline-fields">
              <el-form-item label="预算上限">
                <el-input-number v-model="form.max_tokens_budget" :min="0" :step="100000" />
              </el-form-item>
              <el-form-item label="分批策略">
                <el-select v-model="form.batch">
                  <el-option label="默认" value="" />
                  <el-option label="按语言" value="by-language" />
                  <el-option label="按目录" value="by-directory" />
                  <el-option label="不分批" value="none" />
                </el-select>
              </el-form-item>
            </div>

            <div class="switch-row">
              <el-checkbox v-model="form.no_plan">跳过计划</el-checkbox>
              <el-checkbox v-model="form.no_dedup">跳过去重</el-checkbox>
              <el-checkbox v-model="form.no_summary">跳过总结</el-checkbox>
            </div>

            <div class="action-row">
              <el-button :loading="previewing" :disabled="!canRun" @click="runScan(true)">
                <el-icon><Files /></el-icon>
                预览文件
              </el-button>
              <el-button type="primary" :loading="running" :disabled="!canRun" @click="runScan(false)">
                <el-icon><Search /></el-icon>
                执行扫描
              </el-button>
            </div>
          </el-form>
        </section>

        <section class="panel result-panel">
          <div class="panel-header">
            <h2>扫描结果</h2>
            <span>{{ result?.result?.ocr?.status || (previewFiles.length ? 'preview' : 'waiting') }}</span>
          </div>

          <el-empty v-if="!result && previewFiles.length === 0 && !running && !previewing" description="先选择仓库并执行预览或扫描" />

          <div v-if="previewFiles.length" class="preview-box">
            <div class="result-title">
              <strong>将扫描 {{ previewFiles.length }} 个文件</strong>
              <el-button text type="primary" @click="previewFiles = []">清空</el-button>
            </div>
            <div class="file-list">
              <code v-for="file in previewFiles" :key="file">{{ file }}</code>
            </div>
          </div>

          <div v-if="result?.result" class="scan-result">
            <el-alert
              v-if="result.review_id"
              class="saved-alert"
              type="success"
              :closable="false"
              title="扫描报告已保存到审查记录"
            >
              <template #default>
                <div class="saved-alert-body">
                  <code>{{ result.review_id }}</code>
                  <el-button size="small" type="success" plain @click="router.push(`/reviews/${result?.review_id}`)">查看详情</el-button>
                </div>
              </template>
            </el-alert>

            <div class="metric-strip">
              <div>
                <small>发现问题</small>
                <strong>{{ result.result.findings?.length || 0 }}</strong>
              </div>
              <div>
                <small>扫描文件</small>
                <strong>{{ result.result.ocr.files_reviewed || 0 }}</strong>
              </div>
              <div>
                <small>Tokens</small>
                <strong>{{ formatNumber(result.result.ocr.total_tokens) }}</strong>
              </div>
              <div>
                <small>工具调用</small>
                <strong>{{ result.result.ocr.tool_calls?.total || 0 }}</strong>
              </div>
            </div>

            <el-alert v-if="result.analysis?.overview" class="overview" type="info" :closable="false" :title="result.analysis.overview" />

            <div v-if="toolCallRows.length" class="tool-calls">
              <h3>工具调用分布</h3>
              <div v-for="row in toolCallRows" :key="row.name" class="tool-row">
                <span>{{ row.name }}</span>
                <strong>{{ row.count }}</strong>
              </div>
            </div>

            <div class="finding-list">
              <h3>问题列表</h3>
              <el-empty v-if="!result.result.findings?.length" description="未发现问题" :image-size="80" />
              <article v-for="(finding, index) in result.result.findings" :key="index" class="finding-item">
                <div class="finding-head">
                  <el-tag :type="severityType(finding.severity)" size="small">{{ finding.severity }}</el-tag>
                  <strong>{{ finding.title }}</strong>
                </div>
                <p>{{ finding.body }}</p>
                <code>{{ finding.path }}{{ finding.start_line ? `:${finding.start_line}` : '' }}</code>
              </article>
            </div>
          </div>
        </section>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus/es/components/message/index'
import { Files, Search } from '@element-plus/icons-vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { getBranchSHA, getGitPlatforms, getProviderBranches, getProviderRepos } from '@/api/config'
import { scanRepository } from '@/api/review'
import type { GitPlatform, PlatformBranch, PlatformRepo } from '@/types/config'
import type { ScanRepositoryResult } from '@/types/review'

const platforms = ref<GitPlatform[]>([])
const router = useRouter()
const repos = ref<PlatformRepo[]>([])
const branches = ref<PlatformBranch[]>([])
const reposLoading = ref(false)
const branchesLoading = ref(false)
const resolving = ref(false)
const previewing = ref(false)
const running = ref(false)
const previewFiles = ref<string[]>([])
const result = ref<ScanRepositoryResult | null>(null)
const branch = ref('')
const pathsText = ref('')
const excludeText = ref('')

const form = reactive({
  provider: '',
  repo: '',
  head_sha: '',
  background: '',
  max_tokens_budget: 0,
  batch: '' as '' | 'none' | 'by-language' | 'by-directory',
  no_plan: false,
  no_dedup: false,
  no_summary: false,
})

const canRun = computed(() => Boolean(form.provider && form.repo && form.head_sha))
const toolCallRows = computed(() => {
  const byTool = result.value?.result?.ocr?.tool_calls?.by_tool || {}
  return Object.entries(byTool)
    .map(([name, count]) => ({ name, count }))
    .sort((a, b) => b.count - a.count)
})

function lines(value: string) {
  return value.split('\n').map((line) => line.trim()).filter(Boolean)
}

function splitRepo(repo: string) {
  const parts = repo.split('/').filter(Boolean)
  return { owner: parts.slice(0, -1).join('/'), name: parts[parts.length - 1] || '' }
}

async function loadPlatforms() {
  const data = await getGitPlatforms()
  platforms.value = data.items || []
  if (!form.provider && platforms.value.length) form.provider = platforms.value[0].provider
}

async function loadRepos() {
  if (!form.provider || reposLoading.value) return
  reposLoading.value = true
  try {
    repos.value = await getProviderRepos(form.provider, 1, 100)
  } catch (e: any) {
    ElMessage.error(e.message || '获取仓库失败')
  } finally {
    reposLoading.value = false
  }
}

async function loadBranches() {
  if (!form.provider || !form.repo || branchesLoading.value) return
  const { owner, name } = splitRepo(form.repo)
  if (!owner || !name) return
  branchesLoading.value = true
  try {
    branches.value = await getProviderBranches(form.provider, owner, name)
  } catch (e: any) {
    ElMessage.error(e.message || '获取分支失败')
  } finally {
    branchesLoading.value = false
  }
}

function handleProviderChange() {
  form.repo = ''
  form.head_sha = ''
  branch.value = ''
  repos.value = []
  branches.value = []
}

function handleRepoChange() {
  form.head_sha = ''
  branches.value = []
  const repo = repos.value.find((item) => item.full_name === form.repo)
  branch.value = repo?.default_branch || ''
  if (branch.value) resolveHead()
}

async function resolveHead() {
  if (!form.provider || !form.repo || !branch.value) return
  const looksLikeSHA = /^[0-9a-f]{7,64}$/i.test(branch.value.trim())
  if (looksLikeSHA) {
    form.head_sha = branch.value.trim()
    return
  }
  const { owner, name } = splitRepo(form.repo)
  if (!owner || !name) return
  resolving.value = true
  try {
    form.head_sha = await getBranchSHA(form.provider, owner, name, branch.value)
    if (!form.head_sha) ElMessage.warning('没有解析到分支 SHA')
  } catch (e: any) {
    ElMessage.error(e.message || '解析分支失败')
  } finally {
    resolving.value = false
  }
}

async function runScan(preview: boolean) {
  if (!canRun.value) return
  if (preview) previewing.value = true
  else running.value = true
  try {
    const data = await scanRepository({
      ...form,
      paths: lines(pathsText.value),
      exclude: lines(excludeText.value),
      preview,
    })
    if (preview) {
      previewFiles.value = data.preview_files || []
      ElMessage.success(`预览完成：${previewFiles.value.length} 个文件`)
    } else {
      previewFiles.value = []
      result.value = data
      ElMessage.success('扫描完成')
    }
  } catch (e: any) {
    ElMessage.error(e.message || '扫描失败')
  } finally {
    previewing.value = false
    running.value = false
  }
}

function formatNumber(value?: number) {
  return value ? value.toLocaleString() : '-'
}

function severityType(severity?: string) {
  if (severity === 'critical') return 'danger'
  if (severity === 'major') return 'warning'
  if (severity === 'minor') return 'info'
  return 'success'
}

onMounted(loadPlatforms)
</script>

<style scoped>
.scan-page {
  width: min(1320px, 100%);
  margin: 0 auto;
  padding: 34px 36px 48px;
}

.page-header,
.panel-header,
.result-title,
.finding-head,
.tool-row,
.action-row,
.branch-row,
.switch-row {
  display: flex;
  align-items: center;
}

.page-header {
  justify-content: space-between;
  gap: 18px;
  margin-bottom: 24px;
}

.page-header h1,
.panel-header h2,
.tool-calls h3,
.finding-list h3 {
  margin: 0;
  color: var(--app-text);
}

.page-header h1 {
  font-size: 28px;
}

.page-header p,
.panel-header span,
.form-tip {
  color: var(--app-muted);
}

.eyebrow {
  color: var(--app-primary);
  font-size: 12px;
  font-weight: 800;
  letter-spacing: 0;
}

.scan-grid {
  display: grid;
  grid-template-columns: minmax(360px, 430px) minmax(0, 1fr);
  gap: 18px;
}

.panel {
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: var(--app-surface);
}

.config-panel,
.result-panel {
  padding: 18px;
}

.panel-header {
  justify-content: space-between;
  margin-bottom: 18px;
}

.panel-header h2 {
  font-size: 18px;
}

.branch-row {
  gap: 8px;
}

.branch-row .el-select {
  flex: 1;
}

.form-tip {
  margin-top: 6px;
  font-size: 12px;
}

.inline-fields {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.switch-row {
  flex-wrap: wrap;
  gap: 14px;
  margin-bottom: 18px;
}

.action-row {
  justify-content: flex-end;
  gap: 10px;
}

.preview-box,
.scan-result {
  display: grid;
  gap: 16px;
}

.result-title {
  justify-content: space-between;
}

.file-list {
  display: grid;
  max-height: 360px;
  overflow: auto;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: #f8fafc;
}

.file-list code {
  padding: 9px 12px;
  border-bottom: 1px solid var(--app-border);
  color: #334155;
}

.file-list code:last-child {
  border-bottom: 0;
}

.metric-strip {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
}

.metric-strip > div {
  min-height: 78px;
  padding: 12px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: #f7fbff;
}

.metric-strip small {
  display: block;
  margin-bottom: 8px;
  color: var(--app-muted);
}

.metric-strip strong {
  font-size: 24px;
  color: #0f172a;
}

.overview {
  --el-alert-padding: 12px 14px;
}

.saved-alert {
  --el-alert-padding: 12px 14px;
}

.saved-alert-body {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-top: 6px;
}

.tool-calls,
.finding-list {
  display: grid;
  gap: 10px;
}

.tool-calls h3,
.finding-list h3 {
  font-size: 15px;
}

.tool-row {
  justify-content: space-between;
  padding: 10px 12px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: #fff;
}

.finding-item {
  padding: 14px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: #fff;
}

.finding-head {
  gap: 8px;
  margin-bottom: 8px;
}

.finding-item p {
  margin: 0 0 10px;
  color: #475569;
  white-space: pre-wrap;
}

.finding-item code {
  color: #2563eb;
}

@media (max-width: 980px) {
  .scan-page {
    padding: 22px 16px 32px;
  }

  .scan-grid,
  .inline-fields,
  .metric-strip {
    grid-template-columns: 1fr;
  }

  .page-header {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
