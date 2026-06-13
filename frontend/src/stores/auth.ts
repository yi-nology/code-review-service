import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(sessionStorage.getItem('token') || '')

  function setToken(newToken: string) {
    token.value = newToken
    sessionStorage.setItem('token', newToken)
  }

  function clearToken() {
    token.value = ''
    sessionStorage.removeItem('token')
  }

  return { token, setToken, clearToken }
})
