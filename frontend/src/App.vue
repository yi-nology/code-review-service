<template>
  <el-config-provider :locale="zhCn">
    <router-view v-if="auth.token" />

    <main v-else class="login-page">
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
          <small>Token 仅保存在本地浏览器中，不会上传服务器。</small>
        </form>
      </section>
    </main>
  </el-config-provider>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { ArrowRight, CircleCheckFilled, Finished, Key } from '@element-plus/icons-vue'
import { useAuthStore } from '@/stores/auth'
import axios from 'axios'
import router from '@/router'
import zhCn from 'element-plus/dist/locale/zh-cn.mjs'

const auth = useAuthStore()
const token = ref('')
const error = ref('')
const logging = ref(false)

async function handleLogin() {
  const value = token.value.trim()
  if (!value) {
    error.value = '请输入 API Token'
    return
  }
  error.value = ''
  logging.value = true
  try {
    await axios.get('/api/v1/reviews?page=1&page_size=1', {
      headers: { Authorization: `Bearer ${value}` },
      timeout: 10000,
    })
    auth.setToken(value)
    await router.replace('/reviews')
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
  grid-template-columns: minmax(420px, 1.05fr) minmax(420px, 0.95fr);
  background: #fff;
}

.login-intro {
  display: flex;
  min-height: 100vh;
  flex-direction: column;
  padding: 42px clamp(48px, 7vw, 110px);
  color: #fff;
  background:
    radial-gradient(circle at 78% 22%, rgba(124, 124, 239, 0.3), transparent 25%),
    radial-gradient(circle at 15% 85%, rgba(76, 190, 164, 0.12), transparent 28%),
    #171b2e;
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
  border-radius: 11px;
  color: #fff;
  background: linear-gradient(135deg, #7777eb, #5252c7);
}

.intro-copy {
  max-width: 560px;
  margin: auto 0;
}

.eyebrow {
  margin-bottom: 18px;
  color: #9393f3;
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 1.7px;
}

.intro-copy h1 {
  margin: 0;
  font-size: clamp(42px, 5vw, 68px);
  line-height: 1.12;
  letter-spacing: -2.5px;
}

.intro-copy p {
  max-width: 480px;
  margin: 24px 0 0;
  color: #9ea7bd;
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
  background:
    radial-gradient(circle at 100% 0%, rgba(91, 91, 214, 0.08), transparent 28%),
    #fff;
}

.login-card {
  width: min(400px, 100%);
}

.mobile-brand {
  display: none;
  color: var(--app-text);
}

.login-card h2 {
  margin: 0 0 8px;
  color: var(--app-text);
  font-size: 30px;
  letter-spacing: -0.7px;
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
  border-radius: 10px;
}

.login-card small {
  display: block;
  margin-top: 16px;
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
    border-radius: 18px;
    background: #fff;
    box-shadow: var(--app-shadow);
  }

  .mobile-brand {
    display: flex;
    margin-bottom: 34px;
  }
}
</style>
