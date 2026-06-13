import axios from 'axios'
import { ElMessage } from 'element-plus'

const request = axios.create({
  baseURL: '/api/v1',
  timeout: 30000,
})

request.interceptors.request.use(
  (config) => {
    const token = sessionStorage.getItem('token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
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
        ElMessage.error(body.message || '请求失败')
        return Promise.reject(new Error(body.message))
      }
      return body.data
    }
    return body
  },
  (error) => {
    if (error.response?.status === 401) {
      sessionStorage.removeItem('token')
      if (window.location.pathname !== '/') {
        ElMessage.error('登录已失效，请重新输入 Token')
        window.location.href = '/'
      }
      return Promise.reject(error)
    }
    const message = error.response?.data?.error || error.message || '请求失败'
    if (error.response?.status !== 404) {
      ElMessage.error(message)
    }
    return Promise.reject(error)
  }
)

export default request
