import axios from 'axios'
import { ElMessage } from 'element-plus/es/components/message/index'
import { useAuthStore } from '@/stores/auth'

function getErrorMessage(error: any): string {
  if (error.code === 'ECONNABORTED') return '请求超时，请稍后重试'
  const data = error.response?.data
  if (typeof data === 'string' && data.trim()) return data
  return data?.message || data?.error || data?.msg || error.message || '请求失败'
}

function expireAuth() {
  const auth = useAuthStore()
  auth.clearToken()
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
          if (window.location.pathname !== '/') {
            ElMessage.error('登录已失效，请重新输入 Token')
          }
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
      if (window.location.pathname !== '/') {
        ElMessage.error('登录已失效，请重新输入 Token')
      }
      return Promise.reject(error)
    }
    const message = getErrorMessage(error)
    if (error.response?.status !== 404) {
      ElMessage.error(message)
    }
    return Promise.reject(error)
  }
)

export default request
