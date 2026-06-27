<template>
  <el-form
    :model="form"
    label-width="96px"
    class="create-form create-panel"
  >
    <div class="panel-heading compact">
      <div>
        <h3>分支对比</h3>
        <p>适合没有 PR 的场景。选择基准分支和目标分支，提交时会自动补齐最新 SHA。</p>
      </div>
      <el-button :icon="Refresh" :loading="branchesLoading" :disabled="!selectedRepo" @click="$emit('reload-branches')">刷新分支</el-button>
    </div>

    <div class="form-grid">
      <el-form-item label="代码平台" required>
        <el-select :model-value="form.provider" style="width: 100%" placeholder="选择平台" @change="$emit('update:provider', $event)">
          <el-option v-for="p in platforms" :key="p.provider" :label="getProviderLabel(p.provider)" :value="p.provider" />
        </el-select>
      </el-form-item>
      <el-form-item label="代码仓库" required>
        <el-select
          :model-value="selectedRepo"
          style="width: 100%"
          filterable
          remote
          allow-create
          default-first-option
          reserve-keyword
          :remote-method="(query: string) => $emit('search-repos', query)"
          :loading="reposLoading"
          placeholder="输入 owner/repo 或仓库名"
          @change="$emit('update:selectedRepo', $event)"
          @focus="$emit('ensure-repos')"
        >
          <el-option v-for="r in repos" :key="r.full_name" :label="r.full_name" :value="r.full_name">
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
          :model-value="form.base_branch"
          style="width: 100%"
          filterable
          allow-create
          default-first-option
          :loading="branchesLoading"
          placeholder="例如 main/master"
          :disabled="!selectedRepo"
          @change="$emit('update:baseBranch', $event)"
        >
          <el-option v-for="b in branches" :key="b.name" :label="b.name" :value="b.name" />
        </el-select>
      </el-form-item>
      <el-button class="swap-button" :icon="Switch" :disabled="!form.base_branch || !form.head_branch" @click="$emit('swap-branches')" />
      <el-form-item label="目标分支" required>
        <el-select
          :model-value="form.head_branch"
          style="width: 100%"
          filterable
          allow-create
          default-first-option
          :loading="branchesLoading"
          placeholder="选择要审查的分支"
          :disabled="!selectedRepo"
          @change="$emit('update:headBranch', $event)"
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
        <span>{{ form.head_branch }}</span>
        <el-icon><Right /></el-icon>
        <span>{{ form.base_branch }}</span>
      </div>
      <p>{{ branchSummary }}</p>
    </div>

    <el-collapse class="sha-collapse">
      <el-collapse-item title="指定提交 SHA（可选）" name="sha">
        <div class="form-grid">
          <el-form-item label="基准 SHA">
            <el-input :model-value="form.base_sha" placeholder="留空则使用基准分支最新提交" clearable @update:model-value="$emit('update:baseSha', $event)" />
          </el-form-item>
          <el-form-item label="目标 SHA">
            <el-input :model-value="form.head_sha" placeholder="留空则使用目标分支最新提交" clearable @update:model-value="$emit('update:headSha', $event)" />
          </el-form-item>
        </div>
      </el-collapse-item>
    </el-collapse>

    <slot />
  </el-form>
</template>

<script setup lang="ts">
import { Refresh, Right, Switch } from '@element-plus/icons-vue'
import { getProviderLabel } from '@/utils/url-parser'
import type { GitPlatform, PlatformRepo, PlatformBranch } from '@/types/config'

defineProps<{
  form: {
    provider: string
    base_branch: string
    head_branch: string
    base_sha: string
    head_sha: string
  }
  platforms: GitPlatform[]
  selectedRepo: string
  repos: PlatformRepo[]
  reposLoading: boolean
  branches: PlatformBranch[]
  branchesLoading: boolean
  branchSummary: string
}>()

defineEmits<{
  'update:provider': [value: string]
  'update:selectedRepo': [value: string]
  'update:baseBranch': [value: string]
  'update:headBranch': [value: string]
  'update:baseSha': [value: string]
  'update:headSha': [value: string]
  'search-repos': [query: string]
  'ensure-repos': []
  'reload-branches': []
  'swap-branches': []
}>()
</script>

<style scoped>
.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0 20px;
}

.branch-picker {
  display: grid;
  grid-template-columns: 1fr auto 1fr;
  align-items: start;
  gap: 12px;
}

.swap-button {
  margin-top: 30px;
}

.repo-option {
  display: flex;
  flex-direction: column;
}

.repo-option-name {
  font-weight: 500;
}

.repo-option-desc {
  color: #909399;
  font-size: 12px;
  margin-top: 2px;
}

.manual-hint {
  padding: 10px 14px;
  margin: 12px 0;
  background: #fdf6ec;
  border: 1px solid #f5dab1;
  border-radius: 6px;
  color: #b88230;
  font-size: 13px;
}

.branch-summary {
  padding: 12px 14px;
  margin: 12px 0;
  background: #f0f9eb;
  border: 1px solid #d8efcd;
  border-radius: 6px;
}

.branch-flow {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
  font-weight: 600;
  font-size: 14px;
}

.branch-summary p {
  margin: 0;
  color: #606266;
  font-size: 13px;
}

.sha-collapse {
  margin-top: 12px;
}
</style>
