import axios from 'axios'
import { ElMessage } from 'element-plus/es/components/message/index'
import { useAuthStore } from '@/stores/auth'
import router from '@/router'

function getErrorMessage(error: any): string {
  if (error.code === 'ECONNABORTED') return '请求超时，请稍后重试'
  const data = error.response?.data
  if (typeof data === 'string' && data.trim()) return data
  return data?.message || data?.error || data?.msg || error.message || '请求失败'
}

// expireAuth 清空 token 并跳转登录页，保留当前路径作为登录后的回跳目标。
// 避免清空 token 后 router-view 消失导致用户看到空壳；避免在登录页重复跳转。
function expireAuth() {
  const auth = useAuthStore()
  auth.clearToken()
  const current = router.currentRoute.value
  if (current.path !== '/login') {
    ElMessage.error('登录已失效，请重新输入 Token')
    router.replace({ path: '/login', query: { redirect: current.fullPath } })
  }
}

const request = axios.create({
  baseURL: '/api/v1',
  timeout: 30000,
})

request.interceptors.request.use(
  (config) => {
    const auth = useAuthStore()
    if (auth.token) {
      config.headers.Authorization = `Bearer ${auth.token}`
    }
    return config
  },
  (error) => {
    return Promise.reject(error)
  }
)

request.interceptors.response.use(
  (response) => {
    const body = response.data
    if (body && typeof body === 'object' && 'code' in body && 'data' in body) {
      if (body.code !== 0) {
        if (body.code === 401) {
          expireAuth()
          return Promise.reject(new Error(body.message))
        }
        if (body.code !== 404) {
          ElMessage.error(body.message || '请求失败')
        }
        return Promise.reject(new Error(body.message))
      }
      return body.data
    }
    return body
  },
  (error) => {
    if (error.response?.status === 401) {
      expireAuth()
      return Promise.reject(error)
    }
    const message = getErrorMessage(error)
    // __silent=true 时只 reject，不弹全局 toast（用于非关键请求，如版本探测）。
    const silent = error.config?.__silent === true
    if (error.response?.status !== 404 && !silent) {
      ElMessage.error(message)
    }
    return Promise.reject(error)
  }
)

export default request
