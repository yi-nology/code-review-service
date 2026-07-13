// 扩展 axios 配置：__silent=true 时错误只 reject，不弹全局 toast。
// 用于非关键请求（如版本探测），失败不应打扰用户。
import 'axios'

declare module 'axios' {
  interface AxiosRequestConfig {
    __silent?: boolean
  }
}
