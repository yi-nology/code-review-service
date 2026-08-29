<template>
  <el-select
    :model-value="modelValue"
    filterable
    remote
    reserve-keyword
    :allow-create="allowCreate"
    default-first-option
    :loading="loading"
    :placeholder="placeholder"
    :remote-method="onSearch"
    @change="onChange"
    @visible-change="onVisibleChange"
  >
    <template v-if="loading" #loading>
      <div class="repo-select-loading">
        <el-icon class="is-loading"><Loading /></el-icon>
        <span>加载仓库列表...</span>
      </div>
    </template>
    <el-option v-for="r in filteredRepos" :key="r.full_name" :label="r.full_name" :value="r.full_name">
      <div v-if="secondaryField !== 'none' && secondaryText(r)" class="repo-select-option">
        <span class="repo-select-option-name">{{ r.full_name }}</span>
        <span class="repo-select-option-desc">{{ secondaryText(r) }}</span>
      </div>
    </el-option>
  </el-select>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Loading } from '@element-plus/icons-vue'
import { useRepoLoader } from '@/composables/useRepoLoader'
import type { PlatformRepo } from '@/types/config'

const props = withDefaults(defineProps<{
  modelValue: string
  provider: string
  allowCreate?: boolean
  placeholder?: string
  // 是否在 provider 变化时立即加载；默认 false（首次展开下拉时才加载）。
  immediate?: boolean
  // 前端过滤后展示的最大条数，避免一次渲染过多 option。
  filterLimit?: number
  // option 副文本字段。
  secondaryField?: 'description' | 'default_branch' | 'none'
}>(), {
  allowCreate: false,
  placeholder: '搜索或输入 owner/repo',
  immediate: false,
  filterLimit: 50,
  secondaryField: 'description',
})

const emit = defineEmits<{
  'update:modelValue': [value: string]
  // 同时传出选中的完整仓库对象（allow-create 手动输入时为 undefined），便于父组件取 default_branch 等。
  change: [value: string, repo: PlatformRepo | undefined]
}>()

const { repos, loading, fetchRepos } = useRepoLoader()
const keyword = ref('')
const loaded = ref(false)

const filteredRepos = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  const list = repos.value
  if (!kw) return list.slice(0, props.filterLimit)
  return list
    .filter((r) =>
      r.full_name.toLowerCase().includes(kw)
      || r.name?.toLowerCase().includes(kw)
      || r.description?.toLowerCase().includes(kw),
    )
    .slice(0, props.filterLimit)
})

function secondaryText(r: PlatformRepo): string {
  if (props.secondaryField === 'default_branch') return r.default_branch ? `默认分支：${r.default_branch}` : ''
  if (props.secondaryField === 'description') return r.description || ''
  return ''
}

async function load() {
  if (!props.provider || loading.value) return
  try {
    await fetchRepos(props.provider)
    loaded.value = true
  } catch {
    // 加载失败保持空列表；allow-create 的场景下用户仍可手动输入。
    loaded.value = true
  }
}

function onSearch(query: string) {
  keyword.value = query
  if (!loaded.value && props.provider && !loading.value) load()
}

function onVisibleChange(visible: boolean) {
  if (visible && !loaded.value && props.provider && !loading.value) load()
}

function onChange(value: string) {
  const repo = repos.value.find((r) => r.full_name === value)
  emit('update:modelValue', value)
  emit('change', value, repo)
}

watch(
  () => props.provider,
  (next) => {
    loaded.value = false
    keyword.value = ''
    repos.value = []
    if (props.immediate && next) load()
  },
  { immediate: true },
)
</script>

<style scoped>
.repo-select-loading {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 0;
  color: #909399;
  font-size: 13px;
}

.repo-select-option {
  display: flex;
  flex-direction: column;
}

.repo-select-option-name {
  font-weight: 500;
}

.repo-select-option-desc {
  color: #909399;
  font-size: 12px;
  margin-top: 2px;
}
</style>
