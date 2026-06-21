<template>
  <el-dialog
    v-model="visible"
    title="发起代码审查"
    width="880"
    class="review-create-dialog"
    @close="resetForm"
  >
    <div class="mode-switch">
      <button
        v-for="mode in modeCards"
        :key="mode.name"
        type="button"
        :class="['mode-card', { active: inputMode === mode.name }]"
        @click="switchMode(mode.name)"
      >
        <el-icon><component :is="mode.icon" /></el-icon>
        <strong>{{ mode.title }}</strong>
        <span>{{ mode.desc }}</span>
      </button>
    </div>

    <section v-if="inputMode === 'url'" class="create-panel">
      <div class="panel-heading">
        <div>
          <h3>粘贴链接</h3>
          <p>支持 PR/MR/CR 链接和 compare 分支对比链接，粘贴后自动识别平台、仓库、编号或对比分支。</p>
        </div>
        <el-button :icon="DocumentCopy" @click="pasteFromClipboard">读取剪贴板</el-button>
      </div>

      <el-input
        v-model="urlInput"
        class="url-box"
        type="textarea"
        :rows="3"
        placeholder="例如：https://github.com/owner/repo/pull/42 或 https://github.com/owner/repo/compare/main...feature"
        clearable
        @input="onUrlInput"
        @paste="onUrlPaste"
      />

      <div v-if="parsedUrl" class="parse-card success">
        <div class="parse-main">
          <el-icon><Select /></el-icon>
          <div>
            <strong>已识别 {{ getProviderLabel(parsedUrl.provider) }} 审查请求</strong>
            <span>{{ parsedUrl.repo }} · #{{ parsedUrl.number }}</span>
          </div>
        </div>
        <el-button type="primary" plain @click="applyParsedUrl('pr')">去选择详情</el-button>
      </div>
      <div v-else-if="parsedCompareUrl" class="parse-card success">
        <div class="parse-main">
          <el-icon><Select /></el-icon>
          <div>
            <strong>已识别 {{ getProviderLabel(parsedCompareUrl.provider) }} 分支对比</strong>
            <span>{{ parsedCompareUrl.repo }} · {{ parsedCompareUrl.head_branch }} → {{ parsedCompareUrl.base_branch }}</span>
          </div>
        </div>
        <el-button type="primary" plain @click="applyParsedCompareUrl('branch')">去分支对比</el-button>
      </div>
      <div v-else-if="urlInput.trim()" class="parse-card danger">
        <div class="parse-main">
          <el-icon><Warning /></el-icon>
          <div>
            <strong>暂时无法解析这个链接</strong>
            <span>请确认链接包含仓库路径、PR/MR/CR 编号，或 compare 的 base...head 分支。</span>
          </div>
        </div>
      </div>

      <submit-options
        v-model:mode="submitMode"
        v-model:advanced="showAdvanced"
        v-model:idempotency-key="createForm.idempotency_key"
        v-model:callback-mode="createForm.callback_mode"
        v-model:requirements="requirements"
        v-model:ephemeral-reviewers="ephemeralReviewers"
        @add-reviewer="ephemeralReviewers.push({ name: '', background: '' })"
        @remove-reviewer="(i: number) => ephemeralReviewers.splice(i, 1)"
      />
    </section>

    <el-form
      v-if="inputMode === 'pr'"
      :model="createForm"
      label-width="96px"
      class="create-form create-panel"
    >
      <div class="panel-heading compact">
        <div>
          <h3>选择 PR/CR</h3>
          <p>先选平台和仓库，再从打开中的 PR/MR/CR 里挑一个。支持标题、编号、分支过滤。</p>
        </div>
        <el-button :icon="Refresh" :loading="prsLoading" :disabled="!selectedRepo" @click="reloadPullRequests">刷新列表</el-button>
      </div>

      <div class="form-grid">
        <el-form-item label="代码平台" required>
          <el-select v-model="createForm.provider" style="width: 100%" placeholder="选择平台" @change="onPlatformChange">
            <el-option v-for="p in availablePlatforms" :key="p.provider" :label="getProviderLabel(p.provider)" :value="p.provider" />
          </el-select>
        </el-form-item>
        <el-form-item label="仓库搜索" required>
          <el-select
            v-model="selectedRepo"
            style="width: 100%"
            filterable
            remote
            allow-create
            default-first-option
            reserve-keyword
            :remote-method="searchRepos"
            :loading="reposLoading"
            placeholder="输入 owner/repo 或仓库名"
            @change="onRepoChange"
            @focus="ensureReposLoaded"
          >
            <el-option v-for="r in filteredRepos" :key="r.full_name" :label="r.full_name" :value="r.full_name">
              <div class="repo-option">
                <span class="repo-option-name">{{ r.full_name }}</span>
                <span v-if="r.description" class="repo-option-desc">{{ r.description }}</span>
              </div>
            </el-option>
          </el-select>
        </el-form-item>
      </div>

      <div class="pr-toolbar">
        <el-radio-group v-model="prState" size="small" @change="reloadPullRequests">
          <el-radio-button label="opened">打开中</el-radio-button>
          <el-radio-button label="all">全部</el-radio-button>
        </el-radio-group>
        <el-input v-model="prKeyword" :prefix-icon="Search" placeholder="过滤标题、编号、分支" clearable />
      </div>

      <el-form-item label="PR/MR/CR" required>
        <el-select
          v-model="selectedPR"
          class="rich-select"
          style="width: 100%"
          filterable
          :loading="prsLoading"
          placeholder="选择 PR/MR/CR..."
          :disabled="!selectedRepo"
          @change="onPRChange"
        >
          <el-option v-for="pr in filteredPullRequests" :key="pr.number" :label="`#${pr.number} ${pr.title}`" :value="pr.number">
            <div class="pr-option">
              <span class="pr-option-title">#{{ pr.number }} {{ pr.title }}</span>
              <span class="pr-option-meta">
                {{ pr.source_branch || '-' }} → {{ pr.target_branch || '-' }}
                <template v-if="pr.author"> · {{ pr.author.name || pr.author.username }}</template>
              </span>
            </div>
          </el-option>
        </el-select>
      </el-form-item>

      <div v-if="selectedPRInfo" class="selected-pr-info">
        <div class="summary-line">
          <strong>#{{ selectedPRInfo.number }} {{ selectedPRInfo.title }}</strong>
          <el-tag size="small">{{ selectedPRInfo.state || 'opened' }}</el-tag>
        </div>
        <div class="branch-flow">
          <span>{{ selectedPRInfo.source_branch || 'source' }}</span>
          <el-icon><Right /></el-icon>
          <span>{{ selectedPRInfo.target_branch || 'target' }}</span>
        </div>
        <el-descriptions :column="2" border size="small">
          <el-descriptions-item label="Head SHA"><code>{{ shortSha(selectedPRInfo.head_sha) }}</code></el-descriptions-item>
          <el-descriptions-item label="Base SHA"><code>{{ shortSha(selectedPRInfo.base_sha) }}</code></el-descriptions-item>
        </el-descriptions>
      </div>
      <el-empty v-else-if="selectedRepo && !prsLoading && pullRequests.length === 0" description="这个仓库暂无可选 PR/MR/CR" />

      <submit-options
        v-model:mode="submitMode"
        v-model:advanced="showAdvanced"
        v-model:idempotency-key="createForm.idempotency_key"
        v-model:callback-mode="createForm.callback_mode"
        v-model:requirements="requirements"
        v-model:ephemeral-reviewers="ephemeralReviewers"
        @add-reviewer="ephemeralReviewers.push({ name: '', background: '' })"
        @remove-reviewer="(i: number) => ephemeralReviewers.splice(i, 1)"
      />
    </el-form>

    <el-form
      v-if="inputMode === 'branch'"
      :model="createForm"
      label-width="96px"
      class="create-form create-panel"
    >
      <div class="panel-heading compact">
        <div>
          <h3>分支对比</h3>
          <p>适合没有 PR 的场景。选择基准分支和目标分支，提交时会自动补齐最新 SHA。</p>
        </div>
        <el-button :icon="Refresh" :loading="branchesLoading" :disabled="!selectedRepo" @click="reloadBranches">刷新分支</el-button>
      </div>

      <div class="form-grid">
        <el-form-item label="代码平台" required>
          <el-select v-model="createForm.provider" style="width: 100%" placeholder="选择平台" @change="onPlatformChange">
            <el-option v-for="p in availablePlatforms" :key="p.provider" :label="getProviderLabel(p.provider)" :value="p.provider" />
          </el-select>
        </el-form-item>
        <el-form-item label="代码仓库" required>
          <el-select
            v-model="selectedRepo"
            style="width: 100%"
            filterable
            remote
            allow-create
            default-first-option
            reserve-keyword
            :remote-method="searchRepos"
            :loading="reposLoading"
            placeholder="输入 owner/repo 或仓库名"
            @change="onRepoChange"
            @focus="ensureReposLoaded"
          >
            <el-option v-for="r in filteredRepos" :key="r.full_name" :label="r.full_name" :value="r.full_name">
              <div class="repo-option">
                <span class="repo-option-name">{{ r.full_name }}</span>
                <span v-if="r.default_branch" class="repo-option-desc">默认分支：{{ r.default_branch }}</span>
              </div>
            </el-option>
          </el-select>
        </el-form-item>
      </div>

      <div class="branch-picker">
        <el-form-item label="基准分支" required>
          <el-select
            v-model="createForm.base_branch"
            style="width: 100%"
            filterable
            allow-create
            default-first-option
            :loading="branchesLoading"
            placeholder="例如 main/master"
            :disabled="!selectedRepo"
          >
            <el-option v-for="b in branches" :key="b.name" :label="b.name" :value="b.name" />
          </el-select>
        </el-form-item>
        <el-button class="swap-button" :icon="Switch" :disabled="!createForm.base_branch || !createForm.head_branch" @click="swapBranches" />
        <el-form-item label="目标分支" required>
          <el-select
            v-model="createForm.head_branch"
            style="width: 100%"
            filterable
            allow-create
            default-first-option
            :loading="branchesLoading"
            placeholder="选择要审查的分支"
            :disabled="!selectedRepo"
          >
            <el-option v-for="b in branches" :key="b.name" :label="b.name" :value="b.name" />
          </el-select>
        </el-form-item>
      </div>

      <div v-if="selectedRepo && !branchesLoading && branches.length === 0" class="manual-hint">
        分支列表暂不可用，可以直接输入基准分支和目标分支名称。
      </div>

      <div v-if="branchSummary" class="branch-summary">
        <div class="branch-flow">
          <span>{{ createForm.head_branch }}</span>
          <el-icon><Right /></el-icon>
          <span>{{ createForm.base_branch }}</span>
        </div>
        <p>{{ branchSummary }}</p>
      </div>

      <el-collapse class="sha-collapse">
        <el-collapse-item title="指定提交 SHA（可选）" name="sha">
          <div class="form-grid">
            <el-form-item label="基准 SHA">
              <el-input v-model="createForm.base_sha" placeholder="留空则使用基准分支最新提交" clearable />
            </el-form-item>
            <el-form-item label="目标 SHA">
              <el-input v-model="createForm.head_sha" placeholder="留空则使用目标分支最新提交" clearable />
            </el-form-item>
          </div>
        </el-collapse-item>
      </el-collapse>

      <submit-options
        v-model:mode="submitMode"
        v-model:advanced="showAdvanced"
        v-model:idempotency-key="createForm.idempotency_key"
        v-model:callback-mode="createForm.callback_mode"
        v-model:requirements="requirements"
        v-model:ephemeral-reviewers="ephemeralReviewers"
        @add-reviewer="ephemeralReviewers.push({ name: '', background: '' })"
        @remove-reviewer="(i: number) => ephemeralReviewers.splice(i, 1)"
      />
    </el-form>

    <template #footer>
      <div class="dialog-footer">
        <div class="footer-left">
          <el-button :loading="previewLoading" :disabled="!canSubmit" @click="runPreview">
            <el-icon><View /></el-icon>
            预览变更
          </el-button>
          <span class="submit-hint">{{ submitHint }}</span>
        </div>
        <div>
          <el-button @click="visible = false">取消</el-button>
          <el-button type="primary" @click="handleSubmit" :loading="creating" :disabled="!canSubmit">
            {{ submitMode === 'sync' ? '提交并等待' : '提交审查' }}
          </el-button>
        </div>
      </div>
    </template>

    <el-drawer v-model="showPreview" title="变更文件预览" direction="rtl" size="520px" append-to-body>
      <div v-loading="previewLoading">
      <el-alert v-if="!previewLoading && previewFiles.length === 0" type="info" :closable="false" title="暂无变更文件" description="可能尚未获取到差异，或所选范围无变更。" />
      <div v-else class="preview-summary">
        共 <strong>{{ previewFiles.length }}</strong> 个文件变更
      </div>
      <div class="preview-file-list">
        <div v-for="(file, idx) in previewFiles" :key="idx" class="preview-file">
          <div class="preview-file-head">
            <el-tag v-if="file.status" size="small" :type="previewStatusType(file.status)">{{ file.status }}</el-tag>
            <span class="preview-file-path">{{ file.path }}</span>
            <span v-if="file.language" class="preview-file-lang">{{ file.language }}</span>
          </div>
          <div v-if="file.additions != null || file.deletions != null" class="preview-file-diffstat">
            <span class="add">+{{ file.additions ?? 0 }}</span>
            <span class="del">-{{ file.deletions ?? 0 }}</span>
          </div>
        </div>
      </div>
      </div>
    </el-drawer>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, markRaw, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import {
  Collection,
  Connection,
  DocumentCopy,
  Link,
  Refresh,
  Right,
  Search,
  Select,
  Switch,
  View,
  Warning,
} from '@element-plus/icons-vue'
import { createReview, previewReview } from '@/api/review'
import { getGitPlatforms, getProviderRepos, getProviderBranches, getProviderPulls, getBranchSHA } from '@/api/config'
import { parseMRUrl, parseCompareUrl, getProviderLabel } from '@/utils/url-parser'
import type { GitPlatform, PlatformRepo, PlatformBranch, PlatformPullRequest } from '@/types/config'
import type { EphemeralReviewer, PreviewFile } from '@/types/review'
import SubmitOptions from './SubmitOptions.vue'

const props = defineProps<{ modelValue: boolean }>()
const emit = defineEmits<{
  (e: 'update:modelValue', val: boolean): void
  (e: 'created'): void
  (e: 'goto', id: string): void
}>()

type InputMode = 'url' | 'pr' | 'branch'

const visible = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v),
})

const modeCards = [
  { name: 'url' as const, title: '粘贴链接', desc: '最快：复制 PR/MR/CR 地址即可', icon: markRaw(Link) },
  { name: 'pr' as const, title: '选择 PR/CR', desc: '从平台拉取打开中的审查请求', icon: markRaw(Collection) },
  { name: 'branch' as const, title: '分支对比', desc: '没有 PR 时直接比较两个分支', icon: markRaw(Connection) },
]

const creating = ref(false)
const inputMode = ref<InputMode>('url')
const urlInput = ref('')
const parsedUrl = ref<{ provider: string; repo: string; number: number } | null>(null)
const parsedCompareUrl = ref<{ provider: string; repo: string; base_branch: string; head_branch: string } | null>(null)
const submitMode = ref<'async' | 'sync'>('async')
const showAdvanced = ref<string[]>([])

const availablePlatforms = ref<GitPlatform[]>([])
const repos = ref<PlatformRepo[]>([])
const repoKeyword = ref('')
const reposLoading = ref(false)
const selectedRepo = ref('')

const pullRequests = ref<PlatformPullRequest[]>([])
const prsLoading = ref(false)
const selectedPR = ref<number | null>(null)
const selectedPRInfo = ref<PlatformPullRequest | null>(null)
const prKeyword = ref('')
const prState = ref('opened')

const branches = ref<PlatformBranch[]>([])
const branchesLoading = ref(false)

const requirements = ref('')
const ephemeralReviewers = ref<EphemeralReviewer[]>([])
const previewFiles = ref<PreviewFile[]>([])
const previewLoading = ref(false)
const showPreview = ref(false)

const createForm = reactive({
  provider: 'github',
  repo: '',
  base_sha: '',
  head_sha: '',
  base_branch: '',
  head_branch: '',
  idempotency_key: '',
  callback_mode: 'comment',
})

const filteredRepos = computed(() => {
  const keyword = repoKeyword.value.trim().toLowerCase()
  const list = repos.value
  if (!keyword) return list.slice(0, 50)
  return list.filter((repo) => {
    return repo.full_name.toLowerCase().includes(keyword)
      || repo.name?.toLowerCase().includes(keyword)
      || repo.description?.toLowerCase().includes(keyword)
  }).slice(0, 50)
})

const filteredPullRequests = computed(() => {
  const keyword = prKeyword.value.trim().toLowerCase()
  if (!keyword) return pullRequests.value
  return pullRequests.value.filter((pr) => {
    return String(pr.number).includes(keyword)
      || pr.title.toLowerCase().includes(keyword)
      || pr.source_branch?.toLowerCase().includes(keyword)
      || pr.target_branch?.toLowerCase().includes(keyword)
      || pr.author?.name?.toLowerCase().includes(keyword)
      || pr.author?.username?.toLowerCase().includes(keyword)
  })
})

const branchSummary = computed(() => {
  if (!createForm.base_branch || !createForm.head_branch) return ''
  if (createForm.base_branch === createForm.head_branch) return '基准分支和目标分支相同，请选择不同分支。'
  return `将审查 ${createForm.head_branch} 相对 ${createForm.base_branch} 的变更。`
})

const canSubmit = computed(() => {
  if (!createForm.provider) return false
  if (inputMode.value === 'url') return !!parsedUrl.value || !!parsedCompareUrl.value
  if (inputMode.value === 'pr') return !!createForm.repo && selectedPR.value !== null
  if (inputMode.value === 'branch') {
    return !!createForm.repo
      && !!createForm.base_branch
      && !!createForm.head_branch
      && createForm.base_branch !== createForm.head_branch
  }
  return false
})

const submitHint = computed(() => {
  if (inputMode.value === 'url') {
    if (parsedUrl.value) return `${parsedUrl.value.repo} #${parsedUrl.value.number}`
    if (parsedCompareUrl.value) return `${parsedCompareUrl.value.head_branch} → ${parsedCompareUrl.value.base_branch}`
    return '粘贴链接后即可提交'
  }
  if (inputMode.value === 'pr') {
    return selectedPRInfo.value ? `将审查 #${selectedPRInfo.value.number}` : '请选择仓库和 PR/MR/CR'
  }
  if (createForm.base_branch && createForm.head_branch) {
    return createForm.base_branch === createForm.head_branch ? '请选择两个不同分支' : `${createForm.head_branch} → ${createForm.base_branch}`
  }
  return '请选择仓库和对比分支'
})

function switchMode(mode: InputMode) {
  inputMode.value = mode
  if (mode !== 'url' && createForm.provider) ensureReposLoaded()
  if (mode === 'pr' && selectedRepo.value) reloadPullRequests()
  if (mode === 'branch' && selectedRepo.value) reloadBranches()
}

function onUrlInput() {
  parsedUrl.value = parseMRUrl(urlInput.value)
  parsedCompareUrl.value = parsedUrl.value ? null : parseCompareUrl(urlInput.value)
  if (parsedUrl.value) syncParsedUrlToForm()
  if (parsedCompareUrl.value) syncParsedCompareUrlToForm()
}

function onUrlPaste() {
  requestAnimationFrame(() => {
    onUrlInput()
  })
}

async function pasteFromClipboard() {
  try {
    const text = await navigator.clipboard.readText()
    urlInput.value = text
    onUrlInput()
  } catch {
    ElMessage.warning('无法读取剪贴板，请手动粘贴链接')
  }
}

function syncParsedUrlToForm() {
  if (!parsedUrl.value) return
  createForm.provider = parsedUrl.value.provider
  createForm.repo = parsedUrl.value.repo
  selectedRepo.value = parsedUrl.value.repo
  selectedPR.value = parsedUrl.value.number
}

function syncParsedCompareUrlToForm() {
  if (!parsedCompareUrl.value) return
  createForm.provider = parsedCompareUrl.value.provider
  createForm.repo = parsedCompareUrl.value.repo
  createForm.base_branch = parsedCompareUrl.value.base_branch
  createForm.head_branch = parsedCompareUrl.value.head_branch
  selectedRepo.value = parsedCompareUrl.value.repo
  selectedPR.value = null
  selectedPRInfo.value = null
}

function applyParsedUrl(nextMode: InputMode = 'pr') {
  if (!parsedUrl.value) return
  syncParsedUrlToForm()
  inputMode.value = nextMode
  ensureReposLoaded()
  reloadPullRequests()
}

function applyParsedCompareUrl(nextMode: InputMode = 'branch') {
  if (!parsedCompareUrl.value) return
  syncParsedCompareUrlToForm()
  inputMode.value = nextMode
  ensureReposLoaded()
  reloadBranches()
}

async function loadPlatforms() {
  try {
    const res = await getGitPlatforms()
    availablePlatforms.value = (res.items || []).filter((platform) => platform.enabled)
    if (availablePlatforms.value.length > 0 && !availablePlatforms.value.some((p) => p.provider === createForm.provider)) {
      createForm.provider = availablePlatforms.value[0].provider
    }
  } catch {
    availablePlatforms.value = []
  }
}

async function onPlatformChange() {
  selectedRepo.value = ''
  selectedPR.value = null
  selectedPRInfo.value = null
  repoKeyword.value = ''
  repos.value = []
  pullRequests.value = []
  branches.value = []
  createForm.repo = ''
  createForm.base_sha = ''
  createForm.head_sha = ''
  createForm.base_branch = ''
  createForm.head_branch = ''
  await loadRepos()
}

async function ensureReposLoaded() {
  if (repos.value.length === 0 && !reposLoading.value) await loadRepos()
}

async function loadRepos() {
  if (!createForm.provider) return
  reposLoading.value = true
  try {
    repos.value = await getProviderRepos(createForm.provider, 1, 100)
    if (selectedRepo.value && !repos.value.some((repo) => repo.full_name === selectedRepo.value)) {
      repos.value.unshift({
        id: 0,
        full_name: selectedRepo.value,
        name: selectedRepo.value.split('/').pop() || selectedRepo.value,
        owner: selectedRepo.value.split('/')[0] || '',
        description: '',
        default_branch: '',
        private: false,
      })
    }
  } catch {
    repos.value = selectedRepo.value
      ? [{
          id: 0,
          full_name: selectedRepo.value,
          name: selectedRepo.value.split('/').pop() || selectedRepo.value,
          owner: selectedRepo.value.split('/')[0] || '',
          description: '',
          default_branch: '',
          private: false,
        }]
      : []
    ElMessage.error('获取仓库列表失败')
  } finally {
    reposLoading.value = false
  }
}

function searchRepos(query: string) {
  repoKeyword.value = query
  ensureReposLoaded()
}

async function onRepoChange(repoFullName: string) {
  if (!repoFullName) return
  createForm.repo = repoFullName
  selectedRepo.value = repoFullName
  selectedPR.value = null
  selectedPRInfo.value = null
  createForm.base_sha = ''
  createForm.head_sha = ''

  if (inputMode.value === 'pr') {
    await reloadPullRequests()
  } else if (inputMode.value === 'branch') {
    await reloadBranches()
  }
}

async function reloadPullRequests() {
  const repoParts = splitRepo(createForm.repo || selectedRepo.value)
  if (!repoParts) return
  prsLoading.value = true
  selectedPRInfo.value = null
  try {
    pullRequests.value = await getProviderPulls(createForm.provider, repoParts.owner, repoParts.repo, prState.value)
    if (selectedPR.value) onPRChange(selectedPR.value)
  } catch {
    pullRequests.value = []
    ElMessage.error('获取 PR/MR/CR 列表失败')
  } finally {
    prsLoading.value = false
  }
}

async function reloadBranches() {
  const repoParts = splitRepo(createForm.repo || selectedRepo.value)
  if (!repoParts) return
  branchesLoading.value = true
  createForm.base_sha = ''
  createForm.head_sha = ''
  try {
    branches.value = await getProviderBranches(createForm.provider, repoParts.owner, repoParts.repo)
    applyDefaultBranches()
  } catch {
    branches.value = []
    ElMessage.error('获取分支列表失败')
  } finally {
    branchesLoading.value = false
  }
}

function applyDefaultBranches() {
  const repo = repos.value.find((item) => item.full_name === selectedRepo.value)
  const branchNames = branches.value.map((branch) => branch.name)
  const defaultBranch = repo?.default_branch || branchNames.find((name) => ['main', 'master', 'develop'].includes(name)) || branchNames[0] || ''
  if (!createForm.base_branch) createForm.base_branch = defaultBranch
  if (!createForm.head_branch) {
    createForm.head_branch = branchNames.find((name) => name !== createForm.base_branch) || ''
  }
}

function onPRChange(prNumber: number) {
  const pr = pullRequests.value.find((item) => item.number === prNumber)
  if (pr) {
    selectedPRInfo.value = pr
    createForm.base_sha = pr.base_sha || ''
    createForm.head_sha = pr.head_sha || ''
  }
}

function buildPreviewPayload(): { provider: string; repo: string; base_sha?: string; head_sha?: string; pull_request_id?: string; merge_request_iid?: string } | null {
  if (!createForm.provider || !createForm.repo) return null
  const payload: any = { provider: createForm.provider, repo: createForm.repo }
  const reviewNumber = inputMode.value === 'url' ? parsedUrl.value?.number : selectedPR.value
  if ((inputMode.value === 'url' || inputMode.value === 'pr') && reviewNumber) {
    const num = String(reviewNumber)
    if (createForm.provider === 'gitlab') payload.merge_request_iid = num
    else payload.pull_request_id = num
  }
  if (createForm.base_sha) payload.base_sha = createForm.base_sha
  if (createForm.head_sha) payload.head_sha = createForm.head_sha
  return payload
}

async function runPreview() {
  const payload = buildPreviewPayload()
  if (!payload) {
    ElMessage.warning('请先选择平台和仓库')
    return
  }
  previewLoading.value = true
  showPreview.value = true
  try {
    const repoParts = splitRepo(createForm.repo)
    const isBranchReview = inputMode.value === 'branch' || (inputMode.value === 'url' && !!parsedCompareUrl.value)
    if (isBranchReview && repoParts) {
      try {
        if (!createForm.base_sha && createForm.base_branch) {
          createForm.base_sha = await getBranchSHA(createForm.provider, repoParts.owner, repoParts.repo, createForm.base_branch)
        }
        if (!createForm.head_sha && createForm.head_branch) {
          createForm.head_sha = await getBranchSHA(createForm.provider, repoParts.owner, repoParts.repo, createForm.head_branch)
        }
        payload.base_sha = createForm.base_sha
        payload.head_sha = createForm.head_sha
      } catch {
        ElMessage.error('获取分支 SHA 失败')
        previewLoading.value = false
        return
      }
    }
    previewFiles.value = await previewReview(payload)
  } catch {
    previewFiles.value = []
  } finally {
    previewLoading.value = false
  }
}

function swapBranches() {
  const base = createForm.base_branch
  createForm.base_branch = createForm.head_branch
  createForm.head_branch = base
  const baseSha = createForm.base_sha
  createForm.base_sha = createForm.head_sha
  createForm.head_sha = baseSha
}

function resetForm() {
  inputMode.value = 'url'
  urlInput.value = ''
  parsedUrl.value = null
  parsedCompareUrl.value = null
  submitMode.value = 'async'
  showAdvanced.value = []
  createForm.provider = availablePlatforms.value[0]?.provider || 'github'
  createForm.repo = ''
  createForm.base_sha = ''
  createForm.head_sha = ''
  createForm.base_branch = ''
  createForm.head_branch = ''
  createForm.idempotency_key = ''
  createForm.callback_mode = 'comment'
  selectedRepo.value = ''
  repoKeyword.value = ''
  prKeyword.value = ''
  prState.value = 'opened'
  selectedPR.value = null
  selectedPRInfo.value = null
  repos.value = []
  pullRequests.value = []
  branches.value = []
  requirements.value = ''
  ephemeralReviewers.value = []
  previewFiles.value = []
  showPreview.value = false
}

async function handleSubmit() {
  if (!canSubmit.value) return
  creating.value = true
  try {
    if (inputMode.value === 'url') syncParsedUrlToForm()
    if (inputMode.value === 'url') syncParsedCompareUrlToForm()

    const payload: Record<string, any> = {
      provider: createForm.provider,
      repo: createForm.repo,
      callback_mode: createForm.callback_mode,
      wait: submitMode.value === 'sync',
    }

    const reviewNumber = inputMode.value === 'url' ? parsedUrl.value?.number : selectedPR.value
    if ((inputMode.value === 'url' || inputMode.value === 'pr') && reviewNumber) {
      const num = String(reviewNumber)
      if (createForm.provider === 'gitlab') {
        payload.merge_request_iid = num
      } else {
        payload.pull_request_id = num
      }
    }

    const isBranchReview = inputMode.value === 'branch' || (inputMode.value === 'url' && !!parsedCompareUrl.value)
    if (isBranchReview) {
      const repoParts = splitRepo(createForm.repo)
      if (repoParts) {
        try {
          if (!createForm.base_sha && createForm.base_branch) {
            createForm.base_sha = await getBranchSHA(createForm.provider, repoParts.owner, repoParts.repo, createForm.base_branch)
          }
          if (!createForm.head_sha && createForm.head_branch) {
            createForm.head_sha = await getBranchSHA(createForm.provider, repoParts.owner, repoParts.repo, createForm.head_branch)
          }
        } catch {
          ElMessage.error('获取分支 SHA 失败')
          creating.value = false
          return
        }
      }
    }

    if (createForm.base_sha) payload.base_sha = createForm.base_sha
    if (createForm.head_sha) payload.head_sha = createForm.head_sha
    if (createForm.idempotency_key) payload.idempotency_key = createForm.idempotency_key

    const reqLines = requirements.value.split('\n').map((l) => l.trim()).filter(Boolean)
    if (reqLines.length > 0) payload.requirements = reqLines
    if (ephemeralReviewers.value.some((r) => r.background.trim())) {
      payload.ephemeral_reviewers = ephemeralReviewers.value
        .filter((r) => r.background.trim())
        .map((r) => ({ name: r.name?.trim() || '', background: r.background.trim() }))
    }

    const result = await createReview(payload as any)

    if (submitMode.value === 'sync' && result?.review_id) {
      ElMessage.success('审查完成，跳转详情...')
      emit('goto', result.review_id)
    } else {
      ElMessage.success('审查任务已创建')
    }
    visible.value = false
    emit('created')
  } catch {
    ElMessage.error('创建审查任务失败')
  } finally {
    creating.value = false
  }
}

function splitRepo(fullName: string): { owner: string; repo: string } | null {
  const parts = fullName.split('/').filter(Boolean)
  if (parts.length < 2) return null
  return { owner: parts[0], repo: parts.slice(1).join('/') }
}

function shortSha(sha?: string): string {
  return sha ? sha.slice(0, 8) : '-'
}

function previewStatusType(status: string): string {
  const map: Record<string, string> = {
    added: 'success',
    modified: 'warning',
    removed: 'danger',
    renamed: 'info',
  }
  return map[status] || 'info'
}

watch(visible, async (v) => {
  if (v) {
    resetForm()
    await loadPlatforms()
  }
})
</script>

<style scoped>
.mode-switch {
  display: grid;
  gap: 10px;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  margin-bottom: 18px;
}

.mode-card {
  align-items: flex-start;
  background: #f7f8fc;
  border: 1px solid #e4e7ed;
  border-radius: 8px;
  color: #303133;
  cursor: pointer;
  display: grid;
  gap: 4px;
  grid-template-columns: auto 1fr;
  min-height: 76px;
  padding: 14px;
  text-align: left;
  transition: all 0.18s ease;
}

.mode-card .el-icon {
  color: var(--app-primary);
  font-size: 20px;
  grid-row: span 2;
  margin-top: 1px;
}

.mode-card strong {
  font-size: 14px;
}

.mode-card span {
  color: #707782;
  font-size: 12px;
  line-height: 1.4;
}

.mode-card.active {
  background: #eef0ff;
  border-color: var(--app-primary);
  box-shadow: 0 8px 22px rgba(91, 91, 214, 0.14);
}

.create-panel {
  background: #fff;
  border: 1px solid #e9ecf2;
  border-radius: 8px;
  padding: 18px;
}

.panel-heading {
  align-items: flex-start;
  display: flex;
  gap: 16px;
  justify-content: space-between;
  margin-bottom: 14px;
}

.panel-heading.compact {
  margin-bottom: 18px;
}

.panel-heading h3 {
  color: #20242a;
  font-size: 18px;
  line-height: 1.3;
  margin: 0 0 5px;
}

.panel-heading p {
  color: #707782;
  font-size: 13px;
  line-height: 1.6;
  margin: 0;
}

.url-box {
  margin-bottom: 12px;
}

.parse-card {
  align-items: center;
  border-radius: 8px;
  display: flex;
  gap: 12px;
  justify-content: space-between;
  margin: 12px 0 16px;
  padding: 13px 14px;
}

.parse-card.success {
  background: #f0f9eb;
  border: 1px solid #d8efcd;
}

.parse-card.danger {
  background: #fef0f0;
  border: 1px solid #f6d5d5;
}

.parse-main {
  align-items: center;
  display: flex;
  gap: 10px;
  min-width: 0;
}

.parse-main .el-icon {
  flex: 0 0 auto;
  font-size: 20px;
}

.parse-main strong,
.parse-main span {
  display: block;
}

.parse-main strong {
  color: #20242a;
  font-size: 14px;
}

.parse-main span {
  color: #707782;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  margin-top: 2px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.create-form {
  padding-top: 18px;
}

.form-grid {
  display: grid;
  gap: 14px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.repo-option,
.pr-option {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.repo-option-name,
.pr-option-title {
  color: #303133;
  font-size: 14px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.repo-option-desc,
.pr-option-meta {
  color: #909399;
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pr-toolbar {
  align-items: center;
  display: grid;
  gap: 12px;
  grid-template-columns: auto minmax(0, 1fr);
  margin-bottom: 14px;
}

.rich-select :deep(.el-select-dropdown__item) {
  height: auto;
  line-height: 1.4;
  padding-bottom: 8px;
  padding-top: 8px;
}

.selected-pr-info,
.branch-summary {
  background: #f7f8fc;
  border: 1px solid #e8ebf2;
  border-radius: 8px;
  margin: 10px 0 16px;
  padding: 14px;
}

.summary-line {
  align-items: center;
  display: flex;
  gap: 10px;
  justify-content: space-between;
  margin-bottom: 12px;
}

.summary-line strong {
  color: #20242a;
  font-size: 14px;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.branch-flow {
  align-items: center;
  color: #303133;
  display: flex;
  flex-wrap: wrap;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 13px;
  gap: 8px;
  margin-bottom: 12px;
}

.branch-flow span {
  background: #fff;
  border: 1px solid #dfe3eb;
  border-radius: 999px;
  padding: 4px 10px;
}

.selected-pr-info code {
  background: #e8eaed;
  border-radius: 4px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  padding: 2px 6px;
}

.branch-picker {
  align-items: flex-start;
  display: grid;
  gap: 10px;
  grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr);
}

.swap-button {
  margin-top: 2px;
}

.branch-summary p {
  color: #707782;
  font-size: 13px;
  margin: 0;
}

.manual-hint {
  background: #fff7e8;
  border: 1px solid #f2d7a2;
  border-radius: 8px;
  color: #8a5a10;
  font-size: 13px;
  line-height: 1.5;
  margin: -4px 0 14px;
  padding: 10px 12px;
}

.sha-collapse {
  margin-bottom: 12px;
}

.dialog-footer {
  align-items: center;
  display: flex;
  gap: 16px;
  justify-content: space-between;
  width: 100%;
}

.footer-left {
  align-items: center;
  display: flex;
  gap: 12px;
  min-width: 0;
}

.submit-hint {
  color: #707782;
  font-size: 13px;
  min-width: 0;
  overflow: hidden;
  text-align: left;
  text-overflow: ellipsis;
  white-space: nowrap;
}

:deep(.el-dialog__body) {
  padding-top: 10px;
}

:deep(.el-collapse-item__header) {
  color: #5b5bd6;
  font-size: 14px;
}

:deep(.el-collapse-item__content) {
  padding-bottom: 0;
}

@media (max-width: 760px) {
  .mode-switch,
  .form-grid,
  .branch-picker,
  .pr-toolbar {
    grid-template-columns: 1fr;
  }

  .panel-heading,
  .dialog-footer,
  .footer-left {
    align-items: stretch;
    flex-direction: column;
  }

  .parse-card {
    align-items: stretch;
    flex-direction: column;
  }

  .swap-button {
    justify-self: start;
    margin: -8px 0 8px 96px;
  }
}

.preview-summary {
  margin-bottom: 12px;
  color: #606266;
  font-size: 13px;
}

.preview-file {
  padding: 12px 0;
  border-bottom: 1px solid #f0f0f0;
}

.preview-file:last-child {
  border-bottom: none;
}

.preview-file-head {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
}

.preview-file-path {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
  color: #303133;
  overflow-wrap: anywhere;
}

.preview-file-lang {
  margin-left: auto;
  color: #909399;
  font-size: 11px;
}

.preview-file-diffstat {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 11px;
}

.preview-file-diffstat .add {
  color: #67c23a;
  margin-right: 8px;
}

.preview-file-diffstat .del {
  color: #f56c6c;
}
</style>
