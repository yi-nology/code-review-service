<template>
  <div class="create-panel">
    <div class="panel-heading compact">
      <div>
        <h3>选择 PR/CR</h3>
        <p>选择平台和仓库，然后点击要审查的 PR/MR/CR。</p>
      </div>
    </div>

    <div class="pr-config-row">
      <el-select :model-value="provider" placeholder="选择平台" @change="$emit('update:provider', $event)" class="pr-config-platform">
        <el-option v-for="p in platforms" :key="p.provider" :label="getProviderLabel(p.provider)" :value="p.provider" />
      </el-select>
      <RepoSelect
        :model-value="selectedRepo"
        :provider="provider"
        :allow-create="true"
        placeholder="搜索或输入 owner/repo"
        class="pr-config-repo"
        @update:model-value="$emit('update:selectedRepo', $event)"
        @change="(v: string, repo: PlatformRepo | undefined) => $emit('select', v, repo)"
      />
      <el-button :icon="Refresh" :loading="prsLoading" :disabled="!selectedRepo" @click="$emit('reload-prs')" circle />
    </div>

    <Transition name="fade-slide" mode="out-in">
      <div v-if="selectedRepo" key="pr-panel" class="pr-panel">
        <div class="pr-panel-header" v-if="pullRequests.length > 0 || prsLoading">
          <div class="pr-panel-title">
            <span>PR 列表</span>
            <Transition name="fade" mode="out-in">
              <el-tag v-if="!prsLoading" size="small" type="info" :key="filteredPullRequests.length">
                {{ filteredPullRequests.length }} / {{ pullRequests.length }}
              </el-tag>
            </Transition>
          </div>
          <div class="pr-panel-controls">
            <el-segmented :model-value="prState" :options="prStateOptions" size="small" @change="$emit('update:prState', $event); $emit('reload-prs')" />
            <el-input :model-value="prKeyword" :prefix-icon="Search" placeholder="搜索标题、编号、分支" clearable size="small" class="pr-panel-search" @update:model-value="$emit('update:prKeyword', $event)" />
          </div>
        </div>

        <Transition name="fade" mode="out-in">
          <div v-if="prsLoading" key="loading" class="pr-list">
            <div v-for="i in 4" :key="i" class="pr-card-skeleton" :style="{ animationDelay: `${i * 0.08}s` }">
              <el-skeleton :rows="2" animated />
            </div>
          </div>

          <div v-else-if="filteredPullRequests.length > 0" key="list" class="pr-list">
            <TransitionGroup name="pr-card" tag="div">
              <button
                v-for="(pr, idx) in filteredPullRequests"
                :key="pr.number"
                type="button"
                :class="['pr-card', { selected: selectedPrNumber === pr.number }]"
                :style="{ animationDelay: `${idx * 0.03}s` }"
                @click="$emit('select-pr', pr.number)"
              >
                <div class="pr-card-main">
                  <span class="pr-card-num">#{{ pr.number }}</span>
                  <strong class="pr-card-title">{{ pr.title }}</strong>
                  <el-tag v-if="pr.state && pr.state !== 'opened'" size="small" :type="pr.state === 'closed' ? 'info' : 'success'">{{ pr.state === 'closed' ? '已关闭' : '已合并' }}</el-tag>
                </div>
                <div class="pr-card-meta">
                  <span class="pr-card-branch">
                    <el-icon><Right /></el-icon>
                    {{ pr.source_branch || '?' }} → {{ pr.target_branch || '?' }}
                  </span>
                  <span v-if="pr.author" class="pr-card-author">
                    <img v-if="pr.author.avatar_url" :src="pr.author.avatar_url" class="pr-card-avatar" />
                    {{ pr.author.name || pr.author.username }}
                  </span>
                </div>
                <div v-if="selectedPrNumber === pr.number" class="pr-card-check">
                  <el-icon><Check /></el-icon>
                </div>
              </button>
            </TransitionGroup>
          </div>

          <div v-else key="empty" class="pr-panel-empty">
            <el-icon size="36" color="#c0c4cc"><Connection /></el-icon>
            <p>该仓库暂无{{ prState === 'opened' ? '打开中的' : '' }} PR/MR/CR</p>
            <small v-if="prState === 'opened' && pullRequests.length === 0">可以切换到「分支对比」模式直接比较两个分支</small>
            <small v-else-if="prKeyword">没有匹配「{{ prKeyword }}」的结果，试试清除搜索</small>
          </div>
        </Transition>
      </div>

      <div v-else key="empty-hint" class="pr-empty-hint">
        <el-icon size="28"><Connection /></el-icon>
        <p>请先选择代码平台和仓库</p>
      </div>
    </Transition>

    <slot />
  </div>
</template>

<script setup lang="ts">
import { Check, Connection, Refresh, Right, Search } from '@element-plus/icons-vue'
import { getProviderLabel } from '@/utils/url-parser'
import RepoSelect from '@/components/common/RepoSelect.vue'
import type { GitPlatform, PlatformRepo, PlatformPullRequest } from '@/types/config'

defineProps<{
  provider: string
  platforms: GitPlatform[]
  selectedRepo: string
  pullRequests: PlatformPullRequest[]
  filteredPullRequests: PlatformPullRequest[]
  prsLoading: boolean
  selectedPrNumber: number | null
  prState: string
  prKeyword: string
}>()

defineEmits<{
  'update:provider': [value: string]
  'update:selectedRepo': [value: string]
  'update:prState': [value: string]
  'update:prKeyword': [value: string]
  select: [value: string, repo: PlatformRepo | undefined]
  'reload-prs': []
  'select-pr': [number: number]
}>()

const prStateOptions = [
  { label: '打开中', value: 'opened' },
  { label: '全部', value: 'all' },
]
</script>

<style scoped>
.pr-config-row {
  display: flex;
  gap: 10px;
  margin-bottom: 14px;
}

.pr-config-platform {
  width: 140px;
}

.pr-config-repo {
  flex: 1;
}

.repo-loading {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 0;
  color: #909399;
  font-size: 13px;
}

.pr-panel {
  border: 1px solid #e4e7ed;
  border-radius: 8px;
  overflow: hidden;
}

.pr-panel-header {
  padding: 12px 14px;
  background: #fafbfc;
  border-bottom: 1px solid #e4e7ed;
}

.pr-panel-title {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
  font-weight: 600;
  font-size: 14px;
}

.pr-panel-controls {
  display: flex;
  gap: 10px;
  align-items: center;
}

.pr-panel-search {
  flex: 1;
}

.pr-list {
  max-height: 320px;
  overflow-y: auto;
}

.pr-card-skeleton {
  padding: 14px;
  border-bottom: 1px solid #f0f2f5;
  animation: skeleton-fade-in 0.3s ease both;
}

@keyframes skeleton-fade-in {
  from {
    opacity: 0;
    transform: translateY(4px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.pr-card {
  display: block;
  width: 100%;
  padding: 14px;
  background: #fff;
  border: none;
  border-bottom: 1px solid #f0f2f5;
  cursor: pointer;
  text-align: left;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  position: relative;
  animation: card-slide-in 0.25s ease both;
}

@keyframes card-slide-in {
  from {
    opacity: 0;
    transform: translateY(6px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.pr-card:hover {
  background: #f5f7fa;
  padding-left: 18px;
}

.pr-card.selected {
  background: #ecf5ff;
  border-color: #b3d8ff;
}

.pr-card.selected:hover {
  background: #d9ecff;
}

.pr-card-main {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
}

.pr-card-num {
  color: #909399;
  font-size: 13px;
  font-family: monospace;
  min-width: 48px;
}

.pr-card-title {
  flex: 1;
  font-size: 14px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pr-card-meta {
  display: flex;
  align-items: center;
  gap: 14px;
  font-size: 12px;
  color: #909399;
}

.pr-card-branch {
  display: flex;
  align-items: center;
  gap: 4px;
  font-family: monospace;
}

.pr-card-author {
  display: flex;
  align-items: center;
  gap: 6px;
}

.pr-card-avatar {
  width: 18px;
  height: 18px;
  border-radius: 50%;
}

.pr-card-check {
  position: absolute;
  right: 14px;
  top: 50%;
  transform: translateY(-50%);
  color: #409eff;
  font-size: 18px;
  animation: check-pop 0.2s cubic-bezier(0.34, 1.56, 0.64, 1);
}

@keyframes check-pop {
  from {
    opacity: 0;
    transform: translateY(-50%) scale(0.5);
  }
  to {
    opacity: 1;
    transform: translateY(-50%) scale(1);
  }
}

.pr-panel-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 32px 16px;
  text-align: center;
}

.pr-panel-empty p {
  margin: 10px 0 4px;
  color: #606266;
  font-size: 14px;
}

.pr-panel-empty small {
  color: #909399;
  font-size: 12px;
}

.pr-empty-hint {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 40px 16px;
  text-align: center;
}

.pr-empty-hint p {
  margin: 10px 0 0;
  color: #909399;
  font-size: 14px;
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

.pr-card-enter-active {
  transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}

.pr-card-leave-active {
  transition: all 0.15s ease;
}

.pr-card-enter-from {
  opacity: 0;
  transform: translateX(-10px);
}

.pr-card-leave-to {
  opacity: 0;
  transform: translateX(10px);
}
</style>
