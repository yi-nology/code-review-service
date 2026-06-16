<template>
  <el-dialog
    v-model="visible"
    title="发起代码审查"
    width="720"
    @close="resetForm"
  >
    <el-tabs v-model="inputMode" class="create-tabs">
      <el-tab-pane label="粘贴链接" name="url" />
      <el-tab-pane label="选择 PR/CR" name="pr" />
      <el-tab-pane label="分支对比" name="branch" />
    </el-tabs>

    <div v-if="inputMode === 'url'" class="url-input-section">
      <el-form-item label="PR/MR 链接">
        <el-input
          v-model="urlInput"
          placeholder="https://github.com/owner/repo/pull/42"
          clearable
          @input="onUrlInput"
          @paste="onUrlPaste"
        />
      </el-form-item>
      <div v-if="parsedUrl" class="parse-result">
        <el-tag type="success" size="small">解析成功</el-tag>
        <span class="parse-detail">
          {{ getProviderLabel(parsedUrl.provider) }} / {{ parsedUrl.repo }} / #{{ parsedUrl.number }}
        </span>
        <el-button link type="primary" size="small" @click="applyParsedUrl">
          应用到表单
        </el-button>
      </div>
      <div v-else-if="urlInput && !parsedUrl" class="parse-result parse-error">
        <el-tag type="danger" size="small">无法解析</el-tag>
        <span class="parse-detail">请输入有效的 PR/MR 链接</span>
      </div>
    </div>

    <el-form
      v-if="inputMode === 'pr'"
      :model="createForm"
      label-width="100px"
      class="create-form"
    >
      <slot name="repo-fields" :on-platform-change="onPlatformChange" :on-repo-change="onRepoChange" :repos-loading="reposLoading">
        <el-form-item label="代码平台" required>
          <el-select v-model="createForm.provider" style="width: 100%" @change="onPlatformChange">
            <el-option v-for="p in availablePlatforms" :key="p.provider" :label="getProviderLabel(p.provider)" :value="p.provider" />
          </el-select>
        </el-form-item>
        <el-form-item label="代码仓库" required>
          <el-select v-model="selectedRepo" style="width: 100%" filterable remote :remote-method="searchRepos" :loading="reposLoading" placeholder="搜索仓库..." @change="onRepoChange">
            <el-option v-for="r in repos" :key="r.full_name" :label="r.full_name" :value="r.full_name">
              <div class="repo-option">
                <span class="repo-option-name">{{ r.full_name }}</span>
                <span v-if="r.description" class="repo-option-desc">{{ r.description }}</span>
              </div>
            </el-option>
          </el-select>
        </el-form-item>
      </slot>
      <el-form-item label="PR/MR" required>
        <el-select v-model="selectedPR" style="width: 100%" filterable :loading="prsLoading" placeholder="选择 PR/MR..." @change="onPRChange" :disabled="!selectedRepo">
          <el-option v-for="pr in pullRequests" :key="pr.number" :label="`#${pr.number} ${pr.title}`" :value="pr.number">
            <div class="pr-option">
              <span class="pr-option-title">#{{ pr.number }} {{ pr.title }}</span>
              <span class="pr-option-meta">
                {{ pr.source_branch }} → {{ pr.target_branch }}
                <template v-if="pr.author"> · {{ pr.author.name || pr.author.username }}</template>
              </span>
            </div>
          </el-option>
        </el-select>
      </el-form-item>
      <div v-if="selectedPRInfo" class="selected-pr-info">
        <el-descriptions :column="2" border size="small">
          <el-descriptions-item label="源分支">{{ selectedPRInfo.source_branch }}</el-descriptions-item>
          <el-descriptions-item label="目标分支">{{ selectedPRInfo.target_branch }}</el-descriptions-item>
          <el-descriptions-item label="Head SHA"><code>{{ selectedPRInfo.head_sha?.slice(0, 8) }}</code></el-descriptions-item>
          <el-descriptions-item label="Base SHA"><code>{{ selectedPRInfo.base_sha?.slice(0, 8) }}</code></el-descriptions-item>
        </el-descriptions>
      </div>
      <submit-options v-model:mode="submitMode" v-model:advanced="showAdvanced" v-model:idempotency-key="createForm.idempotency_key" v-model:callback-mode="createForm.callback_mode" />
    </el-form>

    <el-form
      v-if="inputMode === 'branch'"
      :model="createForm"
      label-width="100px"
      class="create-form"
    >
      <el-form-item label="代码平台" required>
        <el-select v-model="createForm.provider" style="width: 100%" @change="onPlatformChange">
          <el-option v-for="p in availablePlatforms" :key="p.provider" :label="getProviderLabel(p.provider)" :value="p.provider" />
        </el-select>
      </el-form-item>
      <el-form-item label="代码仓库" required>
        <el-select v-model="selectedRepo" style="width: 100%" filterable remote :remote-method="searchRepos" :loading="reposLoading" placeholder="搜索仓库..." @change="onRepoChange">
          <el-option v-for="r in repos" :key="r.full_name" :label="r.full_name" :value="r.full_name">
            <div class="repo-option">
              <span class="repo-option-name">{{ r.full_name }}</span>
              <span v-if="r.description" class="repo-option-desc">{{ r.description }}</span>
            </div>
          </el-option>
        </el-select>
      </el-form-item>
      <el-form-item label="基准分支" required>
        <el-select v-model="createForm.base_branch" style="width: 100%" filterable :loading="branchesLoading" placeholder="选择基准分支..." :disabled="!selectedRepo">
          <el-option v-for="b in branches" :key="b.name" :label="b.name" :value="b.name" />
        </el-select>
      </el-form-item>
      <el-form-item label="目标分支" required>
        <el-select v-model="createForm.head_branch" style="width: 100%" filterable :loading="branchesLoading" placeholder="选择目标分支..." :disabled="!selectedRepo">
          <el-option v-for="b in branches" :key="b.name" :label="b.name" :value="b.name" />
        </el-select>
      </el-form-item>
      <el-form-item label="基准 SHA">
        <el-input v-model="createForm.base_sha" placeholder="留空则使用分支最新提交 (可选)" />
      </el-form-item>
      <el-form-item label="目标 SHA">
        <el-input v-model="createForm.head_sha" placeholder="留空则使用分支最新提交 (可选)" />
      </el-form-item>
      <submit-options v-model:mode="submitMode" v-model:advanced="showAdvanced" v-model:idempotency-key="createForm.idempotency_key" v-model:callback-mode="createForm.callback_mode" />
    </el-form>

    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" @click="handleSubmit" :loading="creating" :disabled="!canSubmit">
        {{ submitMode === 'sync' ? '提交并等待' : '提交审查' }}
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch } from 'vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import { createReview } from '@/api/review'
import { getGitPlatforms, getProviderRepos, getProviderBranches, getProviderPulls, getBranchSHA } from '@/api/config'
import { parseMRUrl, getProviderLabel } from '@/utils/url-parser'
import type { GitPlatform, PlatformRepo, PlatformBranch, PlatformPullRequest } from '@/types/config'
import SubmitOptions from './SubmitOptions.vue'

const props = defineProps<{ modelValue: boolean }>()
const emit = defineEmits<{
  (e: 'update:modelValue', val: boolean): void
  (e: 'created'): void
  (e: 'goto', id: string): void
}>()

const visible = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v),
})

const creating = ref(false)
const inputMode = ref<'url' | 'pr' | 'branch'>('url')
const urlInput = ref('')
const parsedUrl = ref<{ provider: string; repo: string; number: number } | null>(null)
const submitMode = ref<'async' | 'sync'>('async')
const showAdvanced = ref<string[]>([])

const availablePlatforms = ref<GitPlatform[]>([])
const repos = ref<PlatformRepo[]>([])
const reposLoading = ref(false)
const selectedRepo = ref('')

const pullRequests = ref<PlatformPullRequest[]>([])
const prsLoading = ref(false)
const selectedPR = ref<number | null>(null)
const selectedPRInfo = ref<PlatformPullRequest | null>(null)

const branches = ref<PlatformBranch[]>([])
const branchesLoading = ref(false)

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

const canSubmit = computed(() => {
  if (!createForm.provider) return false
  if (inputMode.value === 'url') return !!createForm.repo
  if (inputMode.value === 'pr') return !!createForm.repo && selectedPR.value !== null
  if (inputMode.value === 'branch') return !!createForm.repo && !!createForm.base_branch && !!createForm.head_branch
  return false
})

function onUrlInput() {
  parsedUrl.value = parseMRUrl(urlInput.value)
}

function onUrlPaste() {
  requestAnimationFrame(() => {
    parsedUrl.value = parseMRUrl(urlInput.value)
    if (parsedUrl.value) applyParsedUrl()
  })
}

function applyParsedUrl() {
  if (!parsedUrl.value) return
  createForm.provider = parsedUrl.value.provider
  createForm.repo = parsedUrl.value.repo
  inputMode.value = 'pr'
}

async function loadPlatforms() {
  try {
    const res = await getGitPlatforms()
    availablePlatforms.value = (res.items || []).filter(p => p.enabled)
  } catch {
    availablePlatforms.value = []
  }
}

async function onPlatformChange() {
  selectedRepo.value = ''
  selectedPR.value = null
  selectedPRInfo.value = null
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

async function loadRepos() {
  if (!createForm.provider) return
  reposLoading.value = true
  try {
    repos.value = await getProviderRepos(createForm.provider)
  } catch {
    repos.value = []
    ElMessage.error('获取仓库列表失败')
  } finally {
    reposLoading.value = false
  }
}

function searchRepos(_query: string) {}

async function onRepoChange(repoFullName: string) {
  if (!repoFullName) return
  createForm.repo = repoFullName
  const parts = repoFullName.split('/')
  if (parts.length < 2) return
  const owner = parts[0]
  const repo = parts.slice(1).join('/')

  if (inputMode.value === 'pr') {
    prsLoading.value = true
    selectedPR.value = null
    selectedPRInfo.value = null
    try {
      pullRequests.value = await getProviderPulls(createForm.provider, owner, repo)
    } catch {
      pullRequests.value = []
      ElMessage.error('获取 PR/MR 列表失败')
    } finally {
      prsLoading.value = false
    }
  } else if (inputMode.value === 'branch') {
    branchesLoading.value = true
    createForm.base_branch = ''
    createForm.head_branch = ''
    try {
      branches.value = await getProviderBranches(createForm.provider, owner, repo)
    } catch {
      branches.value = []
      ElMessage.error('获取分支列表失败')
    } finally {
      branchesLoading.value = false
    }
  }
}

function onPRChange(prNumber: number) {
  const pr = pullRequests.value.find(p => p.number === prNumber)
  if (pr) {
    selectedPRInfo.value = pr
    createForm.base_sha = pr.base_sha || ''
    createForm.head_sha = pr.head_sha || ''
  }
}

function resetForm() {
  inputMode.value = 'url'
  urlInput.value = ''
  parsedUrl.value = null
  submitMode.value = 'async'
  showAdvanced.value = []
  createForm.provider = 'github'
  createForm.repo = ''
  createForm.base_sha = ''
  createForm.head_sha = ''
  createForm.base_branch = ''
  createForm.head_branch = ''
  createForm.idempotency_key = ''
  createForm.callback_mode = 'comment'
  selectedRepo.value = ''
  selectedPR.value = null
  selectedPRInfo.value = null
  repos.value = []
  pullRequests.value = []
  branches.value = []
}

async function handleSubmit() {
  if (!canSubmit.value) return
  creating.value = true
  try {
    const payload: Record<string, any> = {
      provider: createForm.provider,
      repo: createForm.repo,
      callback_mode: createForm.callback_mode,
      wait: submitMode.value === 'sync',
    }

    if (inputMode.value === 'pr' && selectedPR.value) {
      const num = String(selectedPR.value)
      if (createForm.provider === 'gitlab') {
        payload.merge_request_iid = num
      } else {
        payload.pull_request_id = num
      }
    }

    if (inputMode.value === 'branch') {
      const parts = createForm.repo.split('/')
      if (parts.length >= 2) {
        const owner = parts[0]
        const repo = parts.slice(1).join('/')
        try {
          if (!createForm.base_sha && createForm.base_branch) {
            createForm.base_sha = await getBranchSHA(createForm.provider, owner, repo, createForm.base_branch)
          }
          if (!createForm.head_sha && createForm.head_branch) {
            createForm.head_sha = await getBranchSHA(createForm.provider, owner, repo, createForm.head_branch)
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

watch(visible, (v) => {
  if (v) {
    resetForm()
    loadPlatforms()
  }
})
</script>

<style scoped>
.create-tabs {
  margin-bottom: 16px;
}
.url-input-section {
  padding: 0 0 16px;
}
.parse-result {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px;
  background: #f0f9eb;
  border-radius: 6px;
  margin-top: 8px;
}
.parse-result.parse-error {
  background: #fef0f0;
}
.parse-detail {
  flex: 1;
  font-size: 14px;
  color: #606266;
  font-family: monospace;
}
.create-form {
  padding-top: 8px;
}
.repo-option {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.repo-option-name {
  font-size: 14px;
}
.repo-option-desc {
  font-size: 12px;
  color: #909399;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.pr-option {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.pr-option-title {
  font-size: 14px;
}
.pr-option-meta {
  font-size: 12px;
  color: #909399;
}
.selected-pr-info {
  margin: 16px 0;
  padding: 12px;
  background: #f5f7fa;
  border-radius: 6px;
}
.selected-pr-info code {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
  background: #e8eaed;
  padding: 2px 6px;
  border-radius: 4px;
}
.radio-hint {
  font-size: 12px;
  color: #909399;
  margin-left: 4px;
}
:deep(.el-tabs__header) {
  margin-bottom: 0;
}
:deep(.el-collapse-item__header) {
  font-size: 14px;
  color: #409eff;
}
:deep(.el-collapse-item__content) {
  padding-bottom: 0;
}
</style>
