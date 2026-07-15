import { ref } from 'vue'
import { getProviderRepos } from '@/api/config'
import type { PlatformRepo } from '@/types/config'

export { splitRepo } from '@/utils/repo'

// 后端 GitProviderService.ListRepos 在 page=1 且 perPage>=100 时，会一次性聚合该平台
// 全部仓库并缓存 5 分钟。因此前端只需发起一次请求即可拿到完整列表——历史上各页面用
// while 循环翻页，反而触发后端真实分页，导致重复请求与重复数据。
const PER_PAGE = 100
const CACHE_TTL = 5 * 60 * 1000

interface CacheEntry {
  repos: PlatformRepo[]
  ts: number
}

// 模块级缓存与并发去重：同一平台的列表在 TTL 内跨组件/页面共享，只请求一次。
const repoCache = new Map<string, CacheEntry>()
const inflight = new Map<string, Promise<PlatformRepo[]>>()

// 加载某个平台的全部仓库（带缓存与并发去重）。
export function loadAllRepos(provider: string): Promise<PlatformRepo[]> {
  if (!provider) return Promise.resolve([])

  const cached = repoCache.get(provider)
  if (cached && Date.now() - cached.ts < CACHE_TTL) return Promise.resolve(cached.repos)

  const pending = inflight.get(provider)
  if (pending) return pending

  const task = getProviderRepos(provider, 1, PER_PAGE)
    .then((repos) => {
      const list = repos ?? []
      repoCache.set(provider, { repos: list, ts: Date.now() })
      inflight.delete(provider)
      return list
    })
    .catch((err) => {
      inflight.delete(provider)
      throw err
    })

  inflight.set(provider, task)
  return task
}

// 清除缓存（可用于平台 token 变更后强制刷新）。
export function clearRepoCache(provider?: string) {
  if (provider) {
    repoCache.delete(provider)
    inflight.delete(provider)
  } else {
    repoCache.clear()
    inflight.clear()
  }
}

// 仓库列表加载 hook：维护 repos / loading 状态，供需要直接持有列表的场景使用。
export function useRepoLoader() {
  const repos = ref<PlatformRepo[]>([])
  const loading = ref(false)

  async function fetchRepos(provider: string) {
    if (!provider) {
      repos.value = []
      return
    }
    loading.value = true
    try {
      repos.value = await loadAllRepos(provider)
    } catch {
      repos.value = []
      throw new Error('获取仓库列表失败')
    } finally {
      loading.value = false
    }
  }

  function reset() {
    repos.value = []
  }

  return { repos, loading, fetchRepos, reset }
}
