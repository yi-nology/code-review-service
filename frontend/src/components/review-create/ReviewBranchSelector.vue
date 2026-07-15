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
        <RepoSelect
          :model-value="selectedRepo"
          :provider="form.provider"
          :allow-create="true"
          secondary-field="default_branch"
          placeholder="输入 owner/repo 或仓库名"
          style="width: 100%"
          @update:model-value="$emit('update:selectedRepo', $event)"
          @change="(v: string, repo: PlatformRepo | undefined) => $emit('select', v, repo)"
        />
      </el-form-item>
    </div>

    <Transition name="fade-slide" mode="out-in">
      <div v-if="selectedRepo" key="branch-section">
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
              <template #loading>
                <div class="branch-loading">
                  <el-icon class="is-loading"><Loading /></el-icon>
                  <span>加载分支...</span>
                </div>
              </template>
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
              <template #loading>
                <div class="branch-loading">
                  <el-icon class="is-loading"><Loading /></el-icon>
                  <span>加载分支...</span>
                </div>
              </template>
              <el-option v-for="b in branches" :key="b.name" :label="b.name" :value="b.name" />
            </el-select>
          </el-form-item>
        </div>

        <Transition name="fade" mode="out-in">
          <div v-if="branchesLoading" key="loading" class="branch-skeleton">
            <el-skeleton :rows="1" animated />
          </div>
          <div v-else-if="branches.length === 0" key="hint" class="manual-hint">
            分支列表暂不可用，可以直接输入基准分支和目标分支名称。
          </div>
        </Transition>

        <Transition name="fade-slide">
          <div v-if="branchSummary" class="branch-summary">
            <div class="branch-flow">
              <span>{{ form.head_branch }}</span>
              <el-icon><Right /></el-icon>
              <span>{{ form.base_branch }}</span>
            </div>
            <p>{{ branchSummary }}</p>
          </div>
        </Transition>

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
      </div>

      <div v-else key="empty" class="branch-empty-hint">
        <el-icon size="28"><Connection /></el-icon>
        <p>请先选择代码仓库</p>
      </div>
    </Transition>

    <slot />
  </el-form>
</template>

<script setup lang="ts">
import { Connection, Loading, Refresh, Right, Switch } from '@element-plus/icons-vue'
import { getProviderLabel } from '@/utils/url-parser'
import RepoSelect from '@/components/common/RepoSelect.vue'
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
  select: [value: string, repo: PlatformRepo | undefined]
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
  transition: transform 0.2s ease;
}

.swap-button:hover:not(:disabled) {
  transform: rotate(180deg);
}

.branch-loading {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 0;
  color: #909399;
  font-size: 13px;
}

.branch-skeleton {
  padding: 12px 0;
  animation: skeleton-fade-in 0.3s ease;
}

@keyframes skeleton-fade-in {
  from { opacity: 0; }
  to { opacity: 1; }
}

.manual-hint {
  padding: 10px 14px;
  margin: 12px 0;
  background: #fdf6ec;
  border: 1px solid #f5dab1;
  border-radius: 6px;
  color: #b88230;
  font-size: 13px;
  animation: hint-appear 0.3s ease;
}

@keyframes hint-appear {
  from {
    opacity: 0;
    transform: translateY(-4px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
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

.branch-empty-hint {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 32px 16px;
  text-align: center;
}

.branch-empty-hint p {
  margin: 10px 0 0;
  color: #909399;
  font-size: 14px;
}

.sha-collapse {
  margin-top: 12px;
}

/* Transitions */
.fade-slide-enter-active,
.fade-slide-leave-active {
  transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}

.fade-slide-enter-from {
  opacity: 0;
  transform: translateY(8px);
}

.fade-slide-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
