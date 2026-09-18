<template>
  <el-dialog
    v-model="visible"
    title="发起代码审查"
    width="880"
    class="review-create-dialog"
    @close="resetForm"
  >
    <div class="create-root" @keydown="onRootKeydown">
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

    <ReviewUrlInput
      v-if="inputMode === 'url'"
      :url-input="urlInput"
      :parsed-url="parsedUrl"
      :parsed-compare-url="parsedCompareUrl"
      :submit-disabled="!canSubmit || creating"
      @update:url-input="urlInput = $event"
      @url-input="onUrlInput"
      @url-paste="onUrlPaste"
      @apply-url="applyParsedUrl('pr')"
      @apply-compare-url="applyParsedCompareUrl('branch')"
      @submit="handleSubmit"
    >
      <submit-options
        v-model:engine="engine"
        v-model:mode="submitMode"
        v-model:advanced="showAdvanced"
        v-model:idempotency-key="createForm.idempotency_key"
        v-model:callback-mode="createForm.callback_mode"
        v-model:requirements="requirements"
        v-model:ephemeral-reviewers="ephemeralReviewers"
        v-model:task-kind="taskKind"
        :argus-blocked="argusModeBlocked"
        :argus-block-reason="argusBlockReason"
        @add-reviewer="ephemeralReviewers.push({ name: '', background: '' })"
        @remove-reviewer="(i: number) => ephemeralReviewers.splice(i, 1)"
      />
    </ReviewUrlInput>

    <ReviewPrSelector
      v-if="inputMode === 'pr'"
      :provider="createForm.provider"
      :platforms="availablePlatforms"
      :selected-repo="selectedRepo"
      :pull-requests="pullRequests"
      :filtered-pull-requests="filteredPullRequests"
      :prs-loading="prsLoading"
      :selected-pr-number="selectedPR"
      :pr-state="prState"
      :pr-keyword="prKeyword"
      @update:provider="createForm.provider = $event; onPlatformChange()"
      @select="onRepoSelect"
      @update:pr-state="prState = $event"
      @update:pr-keyword="prKeyword = $event"
      @reload-prs="reloadPullRequests"
      @select-pr="onPRChange"
    >
      <submit-options
        v-model:engine="engine"
        v-model:mode="submitMode"
        v-model:advanced="showAdvanced"
        v-model:idempotency-key="createForm.idempotency_key"
        v-model:callback-mode="createForm.callback_mode"
        v-model:requirements="requirements"
        v-model:ephemeral-reviewers="ephemeralReviewers"
        v-model:task-kind="taskKind"
        :argus-blocked="argusModeBlocked"
        :argus-block-reason="argusBlockReason"
        @add-reviewer="ephemeralReviewers.push({ name: '', background: '' })"
        @remove-reviewer="(i: number) => ephemeralReviewers.splice(i, 1)"
      />
    </ReviewPrSelector>

    <ReviewBranchSelector
      v-if="inputMode === 'branch'"
      :form="createForm"
      :platforms="availablePlatforms"
      :selected-repo="selectedRepo"
      :branches="branches"
      :branches-loading="branchesLoading"
      :branch-summary="branchSummary"
      @update:provider="createForm.provider = $event; onPlatformChange()"
      @select="onRepoSelect"
      @update:base-branch="createForm.base_branch = $event"
      @update:head-branch="createForm.head_branch = $event"
      @update:base-sha="createForm.base_sha = $event"
      @update:head-sha="createForm.head_sha = $event"
      @reload-branches="reloadBranches"
      @swap-branches="swapBranches"
    >
      <submit-options
        v-model:engine="engine"
        v-model:mode="submitMode"
        v-model:advanced="showAdvanced"
        v-model:idempotency-key="createForm.idempotency_key"
        v-model:callback-mode="createForm.callback_mode"
        v-model:requirements="requirements"
        v-model:ephemeral-reviewers="ephemeralReviewers"
        v-model:task-kind="taskKind"
        :argus-blocked="argusModeBlocked"
        :argus-block-reason="argusBlockReason"
        @add-reviewer="ephemeralReviewers.push({ name: '', background: '' })"
        @remove-reviewer="(i: number) => ephemeralReviewers.splice(i, 1)"
      />
    </ReviewBranchSelector>
    </div>

    <template #footer>
      <div class="dialog-footer">
        <div class="footer-left">
          <el-button :loading="previewLoading" :disabled="!canSubmit" @click="runPreview">
            <el-icon><View /></el-icon>
            预览变更
          </el-button>
          <span class="submit-hint">
            {{ submitHint }}<template v-if="canSubmit && !creating">（⌘/Ctrl + Enter 快捷提交）</template>
          </span>
        </div>
        <div>
          <el-button @click="visible = false">取消</el-button>
          <el-button type="primary" @click="handleSubmit" :loading="creating" :disabled="!canSubmit">
            {{ submitButtonLabel }}
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
        <div v-for="(path, idx) in previewFiles" :key="idx" class="preview-file">
          <div class="preview-file-head">
            <el-icon class="preview-file-icon"><Document /></el-icon>
            <span class="preview-file-path">{{ path }}</span>
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
  Document,
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
import { getGitPlatforms, getProviderBranches, getProviderPulls, getBranchSHA } from '@/api/config'
import { parseMRUrl, parseCompareUrl, getProviderLabel } from '@/utils/url-parser'
import { splitRepo } from '@/utils/repo'
import type { GitPlatform, PlatformRepo, PlatformBranch, PlatformPullRequest } from '@/types/config'
import type { EphemeralReviewer } from '@/types/review'
import type { ArgusTaskKind } from '@/types/argus'
import SubmitOptions from './SubmitOptions.vue'
import ReviewUrlInput from './review-create/ReviewUrlInput.vue'
import ReviewPrSelector from './review-create/ReviewPrSelector.vue'
import ReviewBranchSelector from './review-create/ReviewBranchSelector.vue'

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
// 审查引擎：'' = 跟随项目策略（不发送，后端解析）；'ocr' | 'argus' = 显式指定。
const engine = ref<'' | 'ocr' | 'argus'>('')
// argus 任务类型（仅 engine=argus 时随 payload 发送）
const taskKind = ref<ArgusTaskKind>('code_review')
const urlInput = ref('')
const parsedUrl = ref<{ provider: string; repo: string; number: number } | null>(null)
const parsedCompareUrl = ref<{ provider: string; repo: string; base_branch: string; head_branch: string } | null>(null)
const submitMode = ref<'async' | 'sync'>('async')
const showAdvanced = ref<string[]>([])

const availablePlatforms = ref<GitPlatform[]>([])
const selectedRepo = ref('')
const selectedRepoInfo = ref<PlatformRepo | null>(null)

const pullRequests = ref<PlatformPullRequest[]>([])
const prsLoading = ref(false)
const selectedPR = ref<number | null>(null)
const selectedPRInfo = ref<PlatformPullRequest | null>(null)
const prKeyword = ref('')
const prState = ref('opened')
const prStateOptions = [
  { label: '打开中', value: 'opened' },
  { label: '全部', value: 'all' },
]

const branches = ref<PlatformBranch[]>([])
const branchesLoading = ref(false)

const requirements = ref('')
const ephemeralReviewers = ref<EphemeralReviewer[]>([])
const previewFiles = ref<string[]>([])
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
  if (argusUnsupported.value) return false
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

// argus 平台支持面（与后端 argusclient.SupportsPlatform 同源）
const ARGUS_PLATFORMS = ['gitea', 'github', 'gitlab']
const argusPlatformUnsupported = computed(
  () => !!createForm.provider && !ARGUS_PLATFORMS.includes(createForm.provider),
)

// 当前模式是否带 PR 号 + 平台是否在 argus 支持面内。
// 注意与「当前已选引擎」解耦——回落到其他引擎后卡片仍要保持禁用。
const argusModeBlocked = computed(() => {
  if (argusPlatformUnsupported.value) return true
  if (inputMode.value === 'branch') return true
  return inputMode.value === 'url' && !parsedUrl.value && !!parsedCompareUrl.value
})

// Argus 引擎按 PR 维度审查：选中 Argus 且模式不支持时，禁止提交并提示。
const argusUnsupported = computed(() => engine.value === 'argus' && argusModeBlocked.value)

// Argus 卡片禁用提示：平台不支持优先于「没有 PR 号」。
const argusBlockReason = computed(() =>
  argusPlatformUnsupported.value
    ? `Argus 暂不支持 ${createForm.provider} 平台（仅 gitea/github/gitlab）`
    : '分支对比没有 PR 号，暂不支持 Argus',
)

const submitButtonLabel = computed(() => {
  if (engine.value === 'argus') return '派发 Argus 审查'
  return submitMode.value === 'sync' ? '提交并等待' : '提交审查'
})

// ⌘/Ctrl + Enter 快捷提交
function onRootKeydown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key === 'Enter' && canSubmit.value && !creating.value) {
    e.preventDefault()
    handleSubmit()
  }
}

const submitHint = computed(() => {
  if (argusUnsupported.value) return 'Argus 引擎仅支持 PR/MR 审查，请选择 PR 链接或 PR 选择模式'
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
  previewFiles.value = []
  showPreview.value = false
  if (mode === 'pr' && selectedRepo.value) reloadPullRequests()
  if (mode === 'branch' && selectedRepo.value) reloadBranches()
}

function onUrlInput() {
  // 把已配置平台的 base_url 传给解析器：自定义域名（如自建 Gitea）按平台归属判定 provider，
  // 而不是被 github 通配路径误吃。
  const endpoints = availablePlatforms.value.map((p) => ({ provider: p.provider, baseUrl: p.base_url }))
  parsedUrl.value = parseMRUrl(urlInput.value, endpoints)
  parsedCompareUrl.value = parsedUrl.value ? null : parseCompareUrl(urlInput.value, endpoints)
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
  reloadPullRequests()
}

function applyParsedCompareUrl(nextMode: InputMode = 'branch') {
  if (!parsedCompareUrl.value) return
  syncParsedCompareUrlToForm()
  inputMode.value = nextMode
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

function onPlatformChange() {
  selectedRepo.value = ''
  selectedRepoInfo.value = null
  selectedPR.value = null
  selectedPRInfo.value = null
  pullRequests.value = []
  branches.value = []
  createForm.repo = ''
  createForm.base_sha = ''
  createForm.head_sha = ''
  createForm.base_branch = ''
  createForm.head_branch = ''
}

async function onRepoSelect(repoFullName: string, repo?: PlatformRepo) {
  if (!repoFullName) return
  createForm.repo = repoFullName
  selectedRepo.value = repoFullName
  selectedRepoInfo.value = repo ?? null
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
  const branchNames = branches.value.map((branch) => branch.name)
  const defaultBranch = selectedRepoInfo.value?.default_branch || branchNames.find((name) => ['main', 'master', 'develop'].includes(name)) || branchNames[0] || ''
  if (!createForm.base_branch) createForm.base_branch = defaultBranch
  if (!createForm.head_branch) {
    createForm.head_branch = branchNames.find((name) => name !== createForm.base_branch) || ''
  }
}

function onPRChange(prNumber: number) {
  selectedPR.value = prNumber
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
  // 引擎保留上次选择（localStorage），不做重置
  engine.value = (localStorage.getItem(ENGINE_PREF_KEY) as '' | 'ocr' | 'argus') || ''
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
  selectedRepoInfo.value = null
  prKeyword.value = ''
  prState.value = 'opened'
  selectedPR.value = null
  selectedPRInfo.value = null
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
      // '' = 跟随项目策略（后端按 review_engine 解析）；argus 无同步等待语义。
      wait: engine.value === 'argus' ? false : submitMode.value === 'sync',
    }
    if (engine.value === 'argus' || engine.value === 'ocr') payload.engine = engine.value
    if (engine.value === 'argus') payload.task_kind = taskKind.value

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

    if (submitMode.value === 'sync' && engine.value !== 'argus' && result?.review_id) {
      ElMessage.success('审查完成，跳转详情...')
      emit('goto', result.review_id)
    } else if (engine.value === 'argus') {
      ElMessage.success('已派发 Argus 审查，完成后在审查记录与 Argus 审查页查看结果')
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

function shortSha(sha?: string): string {
  return sha ? sha.slice(0, 8) : '-'
}

// 记住上次使用的引擎，下次打开弹窗不再重新选择。
const ENGINE_PREF_KEY = 'review-create-engine'

watch(engine, (v) => {
  localStorage.setItem(ENGINE_PREF_KEY, v)
  // argus 为异步回报引擎，切回 argus 时撤销同步等待选择。
  if (v === 'argus') submitMode.value = 'async'
})

// 当前模式不支持 Argus（切到分支对比 / 粘贴了 compare 链接）时自动回落到跟随项目策略。
watch(argusModeBlocked, (blocked) => {
  if (blocked && engine.value === 'argus') {
    engine.value = ''
    ElMessage.info('分支对比模式暂不支持 Argus，已切换为跟随项目策略')
  }
})

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

.pr-config-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 14px;
}

.pr-config-platform {
  width: 130px;
  flex-shrink: 0;
}

.pr-config-repo {
  flex: 1;
  min-width: 0;
}

.pr-empty-hint {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  min-height: 180px;
  color: #9aa2b1;
}

.pr-empty-hint p {
  margin: 0;
  font-size: 13px;
}

.pr-panel {
  border: 1px solid var(--app-border, #e4e7ed);
  border-radius: 12px;
  overflow: hidden;
}

.pr-panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 10px 12px;
  background: #f7f8fc;
  border-bottom: 1px solid var(--app-border, #e4e7ed);
  flex-wrap: wrap;
}

.pr-panel-title {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--app-text, #303133);
  font-size: 13px;
  font-weight: 600;
}

.pr-panel-controls {
  display: flex;
  align-items: center;
  gap: 8px;
}

.pr-panel-search {
  width: 180px;
}

.pr-list {
  max-height: 340px;
  overflow-y: auto;
  padding: 8px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.pr-card {
  display: block;
  width: 100%;
  padding: 10px 12px;
  border: 1.5px solid transparent;
  border-radius: 10px;
  background: #f7f8fc;
  text-align: left;
  cursor: pointer;
  transition: 140ms ease;
}

.pr-card:hover {
  border-color: #c5c8f0;
  background: #fff;
  box-shadow: 0 4px 12px rgba(91, 91, 214, 0.07);
}

.pr-card.selected {
  border-color: var(--app-primary, #5b5bd6);
  background: #eef0ff;
  box-shadow: 0 4px 14px rgba(91, 91, 214, 0.14);
}

.pr-card-main {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.pr-card-num {
  flex: 0 0 auto;
  padding: 1px 6px;
  border-radius: 5px;
  color: var(--app-primary, #5b5bd6);
  background: rgba(91, 91, 214, 0.1);
  font-size: 11px;
  font-weight: 800;
}

.pr-card-title {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  color: #303133;
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pr-card-meta {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 5px;
  padding-left: 2px;
}

.pr-card-branch {
  display: flex;
  align-items: center;
  gap: 4px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 11px;
  color: #909399;
}

.pr-card-branch .el-icon {
  font-size: 10px;
}

.pr-card-author {
  display: flex;
  align-items: center;
  gap: 5px;
  font-size: 11px;
  color: #9aa2b1;
}

.pr-card-avatar {
  width: 16px;
  height: 16px;
  border-radius: 50%;
  flex-shrink: 0;
}

.pr-card-skeleton {
  padding: 12px;
  border-radius: 10px;
  background: #f7f8fc;
}

.pr-panel-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 6px;
  min-height: 160px;
  padding: 32px 16px;
  text-align: center;
}

.pr-panel-empty p {
  margin: 0;
  color: #606266;
  font-size: 14px;
}

.pr-panel-empty small {
  color: #9aa2b1;
  font-size: 12px;
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
  .pr-panel-controls {
    grid-template-columns: 1fr;
  }

  .pr-config-row {
    flex-direction: column;
    align-items: stretch;
  }

  .pr-config-platform {
    width: 100%;
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
}

.preview-file-icon {
  color: #909399;
  font-size: 14px;
  flex-shrink: 0;
}

.preview-file-path {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
  color: #303133;
  overflow-wrap: anywhere;
}
</style>
