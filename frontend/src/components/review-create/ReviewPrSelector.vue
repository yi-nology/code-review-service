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
      <el-select
        :model-value="selectedRepo"
        filterable
        remote
        allow-create
        default-first-option
        reserve-keyword
        :remote-method="(query: string) => $emit('search-repos', query)"
        :loading="reposLoading"
        placeholder="搜索或输入 owner/repo"
        @change="$emit('update:selectedRepo', $event)"
        @focus="$emit('ensure-repos')"
        class="pr-config-repo"
      >
        <el-option v-for="r in repos" :key="r.full_name" :label="r.full_name" :value="r.full_name">
          <div class="repo-option">
            <span class="repo-option-name">{{ r.full_name }}</span>
            <span v-if="r.description" class="repo-option-desc">{{ r.description }}</span>
          </div>
        </el-option>
      </el-select>
      <el-button :icon="Refresh" :loading="prsLoading" :disabled="!selectedRepo" @click="$emit('reload-prs')" circle />
    </div>

    <div v-if="selectedRepo" class="pr-panel">
      <div class="pr-panel-header" v-if="pullRequests.length > 0 || prsLoading">
        <div class="pr-panel-title">
            <span>PR 列表</span>
            <el-tag size="small" type="info">{{ filteredPullRequests.length }} / {{ pullRequests.length }}</el-tag>
          </div>
        <div class="pr-panel-controls">
          <el-segmented :model-value="prState" :options="prStateOptions" size="small" @change="$emit('update:prState', $event); $emit('reload-prs')" />
          <el-input :model-value="prKeyword" :prefix-icon="Search" placeholder="搜索标题、编号、分支" clearable size="small" class="pr-panel-search" @update:model-value="$emit('update:prKeyword', $event)" />
        </div>
      </div>

      <div v-if="prsLoading" class="pr-list">
        <div v-for="i in 4" :key="i" class="pr-card-skeleton">
          <el-skeleton :rows="2" animated />
        </div>
      </div>

      <div v-else-if="filteredPullRequests.length > 0" class="pr-list">
          <button
            v-for="pr in filteredPullRequests"
            :key="pr.number"
            type="button"
            :class="['pr-card', { selected: selectedPrNumber === pr.number }]"
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
        </button>
      </div>

      <div v-else class="pr-panel-empty">
        <el-icon size="36" color="#c0c4cc"><Connection /></el-icon>
        <p>该仓库暂无{{ prState === 'opened' ? '打开中的' : '' }} PR/MR/CR</p>
        <small v-if="prState === 'opened' && pullRequests.length === 0">可以切换到「分支对比」模式直接比较两个分支</small>
        <small v-else-if="prKeyword">没有匹配「{{ prKeyword }}」的结果，试试清除搜索</small>
      </div>
    </div>

    <div v-else class="pr-empty-hint">
      <el-icon size="28"><Connection /></el-icon>
      <p>请先选择代码平台和仓库</p>
    </div>

    <slot />
  </div>
</template>

<script setup lang="ts">
import { Connection, Refresh, Right, Search } from '@element-plus/icons-vue'
import { getProviderLabel } from '@/utils/url-parser'
import type { GitPlatform, PlatformRepo, PlatformPullRequest } from '@/types/config'

defineProps<{
  provider: string
  platforms: GitPlatform[]
  selectedRepo: string
  repos: PlatformRepo[]
  reposLoading: boolean
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
  'search-repos': [query: string]
  'ensure-repos': []
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
  transition: background 0.15s;
}

.pr-card:hover {
  background: #f5f7fa;
}

.pr-card.selected {
  background: #ecf5ff;
  border-color: #b3d8ff;
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
</style>
