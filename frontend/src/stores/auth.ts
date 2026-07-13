import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

// localStorage 持久化跨浏览器会话；sessionStorage 仅本次会话（关闭标签即登出）。
const TOKEN_KEY = 'token'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem(TOKEN_KEY) || sessionStorage.getItem(TOKEN_KEY) || '')

  const isAuthenticated = computed(() => !!token.value)

  // setToken 写入 token。persist=true（默认）写入 localStorage 跨会话保留；
  // persist=false 写入 sessionStorage，关闭浏览器即登出，适合共享机器场景。
  function setToken(newToken: string, persist = true) {
    token.value = newToken
    // 写入目标存储前，先清掉另一份，避免两处同时存在导致清退后仍残留。
    localStorage.removeItem(TOKEN_KEY)
    sessionStorage.removeItem(TOKEN_KEY)
    if (persist) {
      localStorage.setItem(TOKEN_KEY, newToken)
    } else {
      sessionStorage.setItem(TOKEN_KEY, newToken)
    }
  }

  function clearToken() {
    token.value = ''
    localStorage.removeItem(TOKEN_KEY)
    sessionStorage.removeItem(TOKEN_KEY)
  }

  function checkAuth(): boolean {
    return isAuthenticated.value
  }

  return { token, isAuthenticated, setToken, clearToken, checkAuth }
})
