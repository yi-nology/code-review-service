<template>
  <main class="login-page">
    <section class="login-intro">
      <div class="login-brand">
        <span><el-icon><Finished /></el-icon></span>
        Review Hub
      </div>
      <div class="intro-copy">
        <div class="eyebrow">CODE QUALITY CONSOLE</div>
        <h1>让每一次合并<br>都有清晰依据</h1>
        <p>集中管理代码审查、质量门禁与项目策略，把风险留在合并之前。</p>
      </div>
      <div class="intro-points">
        <span><el-icon><CircleCheckFilled /></el-icon>统一审查记录</span>
        <span><el-icon><CircleCheckFilled /></el-icon>可追溯质量门禁</span>
        <span><el-icon><CircleCheckFilled /></el-icon>项目级规则配置</span>
      </div>
    </section>

    <section class="login-panel">
      <form class="login-card" @submit.prevent="handleLogin">
        <div class="mobile-brand">
          <span><el-icon><Finished /></el-icon></span>
          Review Hub
        </div>
        <h2>进入控制台</h2>
        <p>请输入服务端配置的访问 Token。</p>

        <label for="api-token">API Token</label>
        <el-input
          id="api-token"
          v-model="token"
          type="password"
          size="large"
          show-password
          autofocus
          placeholder="请输入 REVIEW_TOKEN"
        >
          <template #prefix><el-icon><Key /></el-icon></template>
        </el-input>
        <div v-if="error" class="login-error">{{ error }}</div>

        <el-button type="primary" size="large" native-type="submit" :loading="logging">
          进入控制台
          <el-icon><ArrowRight /></el-icon>
        </el-button>

        <label class="remember-row">
          <el-checkbox v-model="remember">保持登录</el-checkbox>
          <small>{{ remember ? '关闭浏览器后仍保持登录' : '仅本次浏览器会话保持登录' }}</small>
        </label>
        <small class="token-hint">Token 仅保存在本地浏览器中，不会上传服务器。</small>
      </form>
    </section>
  </main>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowRight, CircleCheckFilled, Finished, Key } from '@element-plus/icons-vue'
import { useAuthStore } from '@/stores/auth'
import axios from 'axios'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()
const token = ref('')
const error = ref('')
const logging = ref(false)
// 默认保持登录（localStorage）；取消勾选则仅本次会话（sessionStorage），关闭浏览器即登出。
const remember = ref(true)

async function handleLogin() {
  const value = token.value.trim()
  if (!value) {
    error.value = '请输入 API Token'
    return
  }
  error.value = ''
  logging.value = true
  try {
    const resp = await axios.get('/api/v1/reviews?page=1&page_size=1', {
      headers: { Authorization: `Bearer ${value}` },
      timeout: 10000,
    })
    if (resp.data?.code !== 0) {
      error.value = resp.data?.message || 'Token 无效，请检查后重试'
      return
    }
    auth.setToken(value, remember.value)
    // 登录后回到用户原本想去的页面（深链保留）；无目标则进总览。
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/dashboard'
    await router.replace(redirect)
  } catch (err: any) {
    if (err.response?.status === 401) {
      error.value = 'Token 无效，请检查后重试'
    } else if (err.code === 'ECONNABORTED') {
      error.value = '连接服务超时，请确认后端已启动'
    } else {
      error.value = '连接服务失败，请检查服务是否正常运行'
    }
  } finally {
    logging.value = false
  }
}
</script>

<style scoped>
.login-page {
  display: grid;
  min-height: 100vh;
  grid-template-columns: minmax(420px, 0.9fr) minmax(420px, 1.1fr);
  background: #f6f7f9;
}

.login-intro {
  display: flex;
  min-height: 100vh;
  flex-direction: column;
  padding: 42px clamp(44px, 6vw, 88px);
  color: #fff;
  background: #111827;
}

.login-brand,
.mobile-brand {
  display: flex;
  align-items: center;
  gap: 11px;
  font-size: 17px;
  font-weight: 750;
}

.login-brand > span,
.mobile-brand > span {
  display: grid;
  width: 38px;
  height: 38px;
  place-items: center;
  border-radius: 8px;
  color: #fff;
  background: #2563eb;
}

.intro-copy {
  max-width: 560px;
  margin: auto 0;
}

.eyebrow {
  margin-bottom: 14px;
  color: #93c5fd;
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0;
}

.intro-copy h1 {
  margin: 0;
  font-size: clamp(34px, 4vw, 48px);
  line-height: 1.18;
  letter-spacing: 0;
}

.intro-copy p {
  max-width: 480px;
  margin: 20px 0 0;
  color: #aeb7c8;
  font-size: 16px;
  line-height: 1.8;
}

.intro-points {
  display: flex;
  flex-wrap: wrap;
  gap: 22px;
  color: #aeb7ca;
  font-size: 12px;
}

.intro-points span {
  display: flex;
  align-items: center;
  gap: 6px;
}

.intro-points .el-icon {
  color: #55d4aa;
}

.login-panel {
  display: grid;
  min-height: 100vh;
  place-items: center;
  padding: 40px;
  background: #f6f7f9;
}

.login-card {
  width: min(400px, 100%);
  padding: 32px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: #fff;
  box-shadow: var(--app-shadow);
}

.mobile-brand {
  display: none;
  color: var(--app-text);
}

.login-card h2 {
  margin: 0 0 8px;
  color: var(--app-text);
  font-size: 26px;
  letter-spacing: 0;
}

.login-card > p {
  margin: 0 0 32px;
  color: var(--app-muted);
  font-size: 14px;
}

.login-card label {
  display: block;
  margin-bottom: 8px;
  color: #3e4758;
  font-size: 13px;
  font-weight: 700;
}

.login-card .el-button {
  width: 100%;
  height: 46px;
  margin-top: 18px;
  border-radius: 8px;
}

.remember-row {
  display: flex;
  align-items: center;
  gap: 10px;
  margin: 18px 0 0;
  font-weight: 400;
}

.remember-row small {
  color: #9aa2b1;
  font-size: 12px;
  font-weight: 400;
}

.token-hint {
  display: block;
  margin-top: 12px;
  color: #9aa2b1;
  text-align: center;
}

.login-error {
  margin-top: 8px;
  color: var(--app-danger);
  font-size: 12px;
}

@media (max-width: 860px) {
  .login-page {
    display: block;
    background: #f5f7fb;
  }

  .login-intro {
    display: none;
  }

  .login-panel {
    min-height: 100vh;
    padding: 28px;
  }

  .login-card {
    padding: 28px 24px;
    border: 1px solid var(--app-border);
    border-radius: 8px;
    background: #fff;
    box-shadow: var(--app-shadow);
  }

  .mobile-brand {
    display: flex;
    margin-bottom: 34px;
  }
}
</style>
