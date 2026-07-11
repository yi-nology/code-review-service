<template>
  <el-dialog
    v-model="visible"
    class="global-search-dialog"
    align-center
    :show-close="false"
    :close-on-click-modal="true"
    width="min(620px, 92vw)"
    @opened="focusInput"
    @closed="onClosed"
  >
    <div class="search-input-wrap">
      <el-icon class="search-input-icon"><Search /></el-icon>
      <input
        ref="inputEl"
        v-model="query"
        class="search-input"
        type="text"
        placeholder="搜索页面、审查记录，或直接跳转…（↑↓ 选择，回车打开）"
        @keydown.up.prevent="moveSelection(-1)"
        @keydown.down.prevent="moveSelection(1)"
        @keydown.enter.prevent="openSelected"
      />
      <kbd class="search-input-esc">ESC</kbd>
    </div>

    <div class="search-results" ref="resultsEl">
      <template v-if="results.length">
        <button
          v-for="(item, index) in results"
          :key="item.key"
          type="button"
          class="search-result"
          :class="{ active: index === selection }"
          @mouseenter="selection = index"
          @click="open(item)"
        >
          <span class="result-icon" :class="item.kind">
            <el-icon v-if="item.icon"><component :is="item.icon" /></el-icon>
            <span v-else>{{ item.kind === 'review' ? '#' : '↪' }}</span>
          </span>
          <span class="result-main">
            <span class="result-label" v-html="highlight(item.label)" />
            <span class="result-sub">{{ item.sub }}</span>
          </span>
          <span class="result-kind">{{ kindLabel(item.kind) }}</span>
        </button>
      </template>
      <div v-else-if="!query.trim()" class="search-hint">
        输入关键词搜索：页面、审查记录、快捷操作。
      </div>
      <div v-else class="search-hint">
        {{ searching ? '搜索中…' : `没有匹配「${query.trim()}」的结果` }}
      </div>
    </div>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick, type Component } from 'vue'
import { useRouter } from 'vue-router'
import { Search } from '@element-plus/icons-vue'
import { ElDialog } from 'element-plus/es/components/dialog/index'
import { navGroups } from '@/constants/nav'
import { getReviews } from '@/api/review'
import type { Review } from '@/types/review'

interface SearchResult {
  key: string
  kind: 'page' | 'review' | 'action'
  label: string
  sub: string
  path?: string
  icon?: Component
}

const props = defineProps<{ modelValue: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: boolean] }>()

const router = useRouter()
const visible = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v),
})

const query = ref('')
const selection = ref(0)
const inputEl = ref<HTMLInputElement | null>(null)
const searching = ref(false)
let searchTimer: ReturnType<typeof setTimeout> | null = null
let searchSeq = 0

// 评审搜索结果（异步）。菜单与动作是同步的。
const reviewResults = ref<SearchResult[]>([])

// 输入变化时去拉评审匹配（防抖），仅复用现有 GET /reviews 的 repo 模糊匹配。
watch(query, (q) => {
  selection.value = 0
  const trimmed = q.trim()
  if (!trimmed) {
    reviewResults.value = []
    return
  }
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(async () => {
    const mySeq = ++searchSeq
    searching.value = true
    try {
      const res = await getReviews({ repo: trimmed, page: 1, page_size: 8 })
      if (mySeq !== searchSeq) return // 过期请求，丢弃
      reviewResults.value = (res.items || []).map((r: Review) => ({
        key: `review-${r.review_id}`,
        kind: 'review' as const,
        label: r.repo,
        sub: `${r.status} · ${new Date(r.updated_at).toLocaleDateString()}`,
        path: `/reviews/${r.review_id}`,
      }))
    } catch {
      if (mySeq === searchSeq) reviewResults.value = []
    } finally {
      if (mySeq === searchSeq) searching.value = false
    }
  }, 250)
})

// 菜单项 + 快捷动作（同步，本地数据）。
const staticResults = computed<SearchResult[]>(() => {
  const pages: SearchResult[] = navGroups.flatMap((g) =>
    g.items.map((item) => ({
      key: `page-${item.path}`,
      kind: 'page' as const,
      label: item.label,
      sub: g.label,
      path: item.path,
      icon: item.icon,
    })),
  )
  const actions: SearchResult[] = [
    { key: 'action-create', kind: 'action', label: '发起审查', sub: '快捷操作', path: '/reviews' },
    { key: 'action-running', kind: 'action', label: '查看运行中的审查', sub: '快捷操作', path: '/reviews?status=running' },
    { key: 'action-scan', kind: 'action', label: '扫描代码库', sub: '快捷操作', path: '/scan' },
  ]
  return [...pages, ...actions]
})

// 合并 + 过滤。空查询时展示静态项（页面+动作），方便浏览发现。
const results = computed<SearchResult[]>(() => {
  const q = query.value.trim().toLowerCase()
  const filt = (list: SearchResult[]) =>
    q ? list.filter((r) => r.label.toLowerCase().includes(q) || r.sub.toLowerCase().includes(q)) : list
  const reviews = q ? reviewResults.value : []
  // 评审匹配优先，其次是页面/动作。
  return [...reviews, ...filt(staticResults.value)]
})

watch(results, () => {
  selection.value = 0
})

function moveSelection(delta: number) {
  const n = results.value.length
  if (!n) return
  selection.value = (selection.value + delta + n) % n
  scrollIntoView()
}

function openSelected() {
  const item = results.value[selection.value]
  if (item) open(item)
}

function open(item: SearchResult) {
  if (item.path) {
    router.push(item.path)
  }
  visible.value = false
}

function focusInput() {
  nextTick(() => inputEl.value?.focus())
}

function onClosed() {
  // 关闭后清空，下次打开是干净状态。
  query.value = ''
  reviewResults.value = []
  selection.value = 0
}

function scrollIntoView() {
  nextTick(() => {
    const el = resultsEl.value?.querySelector<HTMLElement>('.search-result.active')
    el?.scrollIntoView({ block: 'nearest' })
  })
}

const resultsEl = ref<HTMLElement | null>(null)

function kindLabel(kind: SearchResult['kind']) {
  return kind === 'page' ? '页面' : kind === 'review' ? '审查' : '操作'
}

// 高亮匹配片段（转义后插入 <mark>，避免 XSS）。
function highlight(text: string) {
  const q = query.value.trim()
  if (!q) return escapeHtml(text)
  const idx = text.toLowerCase().indexOf(q.toLowerCase())
  if (idx < 0) return escapeHtml(text)
  return escapeHtml(text.slice(0, idx)) + '<mark>' + escapeHtml(text.slice(idx, idx + q.length)) + '</mark>' + escapeHtml(text.slice(idx + q.length))
}

function escapeHtml(s: string) {
  return s.replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c] as string))
}
</script>

<style scoped>
.search-input-wrap {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 4px 16px 14px;
  border-bottom: 1px solid var(--app-border);
}

.search-input-icon {
  color: #929aab;
  font-size: 18px;
}

.search-input {
  flex: 1;
  height: 38px;
  border: 0;
  outline: 0;
  color: var(--app-text);
  background: transparent;
  font-size: 15px;
}

.search-input-esc {
  padding: 2px 7px;
  border: 1px solid var(--app-border);
  border-radius: 5px;
  color: #929aab;
  background: #f6f7f9;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 11px;
}

.search-results {
  max-height: 60vh;
  overflow-y: auto;
  padding: 8px;
}

.search-result {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
  padding: 10px 12px;
  border: 0;
  border-radius: 8px;
  color: var(--app-text);
  background: transparent;
  text-align: left;
  cursor: pointer;
  transition: background 120ms ease;
}

.search-result.active {
  background: #eff6ff;
}

.result-icon {
  display: grid;
  width: 30px;
  height: 30px;
  flex: 0 0 30px;
  place-items: center;
  border-radius: 7px;
  font-size: 14px;
  font-weight: 700;
  background: #f2f4f7;
  color: #6b7280;
}

.result-icon.page {
  color: #2563eb;
  background: #eff6ff;
}

.result-icon.review {
  color: #0891b2;
  background: #ecfeff;
}

.result-icon.action {
  color: #d97706;
  background: #fffbeb;
}

.result-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.result-label {
  font-size: 14px;
  font-weight: 600;
}

.result-label :deep(mark) {
  color: #1d4ed8;
  background: transparent;
}

.result-sub {
  color: #929aab;
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.result-kind {
  flex: 0 0 auto;
  padding: 1px 8px;
  border-radius: 5px;
  color: #929aab;
  background: #f2f4f7;
  font-size: 11px;
}

.search-hint {
  padding: 28px 16px;
  color: #929aab;
  text-align: center;
  font-size: 13px;
}
</style>

<style>
/* 对话框无内边距（自定义布局），居中圆角 */
.global-search-dialog .el-dialog__header {
  display: none;
}

.global-search-dialog .el-dialog__body {
  padding: 16px 0 8px;
}

.global-search-dialog .el-dialog {
  border-radius: 12px;
  overflow: hidden;
}
</style>
