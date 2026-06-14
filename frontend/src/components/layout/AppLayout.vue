<template>
  <div class="app-layout">
    <div v-if="sidebarOpen" class="sidebar-backdrop" @click="sidebarOpen = false" />

    <aside class="sidebar" :class="{ open: sidebarOpen }">
      <div class="brand">
        <span class="brand-mark"><el-icon><Finished /></el-icon></span>
        <div class="brand-copy">
          <strong>Review Hub</strong>
          <span>代码质量控制台</span>
        </div>
        <button class="close-sidebar" aria-label="关闭菜单" @click="sidebarOpen = false">
          <el-icon><Close /></el-icon>
        </button>
      </div>

      <nav class="navigation" @click="sidebarOpen = false">
        <p class="nav-label">工作台</p>
        <router-link to="/reviews" class="nav-item">
          <el-icon><DocumentChecked /></el-icon>
          <span>审查记录</span>
        </router-link>
        <router-link to="/contributors" class="nav-item">
          <el-icon><DataAnalysis /></el-icon>
          <span>开发者质量</span>
        </router-link>

        <p class="nav-label settings-label">配置中心</p>
        <router-link to="/settings/ocr" class="nav-item">
          <el-icon><Cpu /></el-icon>
          <span>模型配置</span>
        </router-link>
        <router-link to="/settings/projects" class="nav-item">
          <el-icon><FolderChecked /></el-icon>
          <span>项目策略</span>
        </router-link>
        <router-link to="/settings/rules" class="nav-item">
          <el-icon><List /></el-icon>
          <span>审查规则</span>
        </router-link>
        <router-link to="/settings/platforms" class="nav-item">
          <el-icon><Connection /></el-icon>
          <span>Git 平台</span>
        </router-link>
        <router-link to="/settings/agents" class="nav-item">
          <el-icon><User /></el-icon>
          <span>代理设置</span>
        </router-link>
      </nav>

      <div class="sidebar-footer">
        <span class="health-dot" />
        <div class="service-status">
          <strong>服务运行中</strong>
          <span>Review Service Console</span>
        </div>
        <button class="logout-button" aria-label="退出登录" title="退出登录" @click="logout">
          <el-icon><SwitchButton /></el-icon>
        </button>
      </div>
    </aside>

    <div class="content-shell">
      <header class="mobile-header">
        <button aria-label="打开菜单" @click="sidebarOpen = true">
          <el-icon><Menu /></el-icon>
        </button>
        <strong>Review Hub</strong>
      </header>
      <main class="main-content">
        <slot />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import {
  Close,
  Connection,
  Cpu,
  DataAnalysis,
  DocumentChecked,
  Finished,
  FolderChecked,
  List,
  Menu,
  SwitchButton,
  User,
} from '@element-plus/icons-vue'
import { useAuthStore } from '@/stores/auth'

const sidebarOpen = ref(false)
const auth = useAuthStore()

function logout() {
  auth.clearToken()
}
</script>

<style scoped>
.app-layout {
  display: flex;
  min-height: 100vh;
}

.sidebar {
  position: fixed;
  inset: 0 auto 0 0;
  z-index: 30;
  display: flex;
  width: 252px;
  flex-direction: column;
  color: #dfe4f2;
  background:
    radial-gradient(circle at 20% 0%, rgba(105, 105, 223, 0.22), transparent 28%),
    #171b2e;
}

.brand {
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 84px;
  padding: 0 22px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
}

.brand-mark {
  display: grid;
  width: 40px;
  height: 40px;
  flex: 0 0 40px;
  place-items: center;
  border-radius: 12px;
  color: #fff;
  background: linear-gradient(135deg, #7777eb, #5252c7);
  box-shadow: 0 8px 20px rgba(69, 69, 181, 0.35);
  font-size: 21px;
}

.brand-copy {
  min-width: 0;
}

.brand-copy strong,
.brand-copy span {
  display: block;
}

.brand-copy strong {
  color: #fff;
  font-size: 16px;
  letter-spacing: 0.2px;
}

.brand-copy span {
  margin-top: 3px;
  color: #8991a8;
  font-size: 12px;
}

.close-sidebar {
  display: none;
}

.navigation {
  flex: 1;
  padding: 22px 14px;
}

.nav-label {
  margin: 0 10px 8px;
  color: #707990;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 1.2px;
}

.settings-label {
  margin-top: 26px;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 44px;
  margin-bottom: 4px;
  padding: 0 12px;
  border-radius: 10px;
  color: #aeb6ca;
  text-decoration: none;
  transition: 160ms ease;
}

.nav-item .el-icon {
  font-size: 18px;
}

.nav-item:hover {
  color: #fff;
  background: rgba(255, 255, 255, 0.06);
}

.nav-item.router-link-active {
  color: #fff;
  background: linear-gradient(90deg, rgba(91, 91, 214, 0.34), rgba(91, 91, 214, 0.16));
  box-shadow: inset 3px 0 #8181ee;
}

.sidebar-footer {
  display: flex;
  align-items: center;
  gap: 11px;
  margin: 14px;
  padding: 14px;
  border: 1px solid rgba(255, 255, 255, 0.07);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.035);
}

.health-dot {
  width: 9px;
  height: 9px;
  border-radius: 50%;
  background: #45d6a2;
  box-shadow: 0 0 0 4px rgba(69, 214, 162, 0.12);
}

.sidebar-footer strong,
.sidebar-footer span {
  display: block;
}

.service-status {
  min-width: 0;
  flex: 1;
}

.logout-button {
  display: grid;
  width: 30px;
  height: 30px;
  flex: 0 0 30px;
  place-items: center;
  border: 0;
  border-radius: 8px;
  color: #7e879d;
  background: transparent;
  cursor: pointer;
}

.logout-button:hover {
  color: #fff;
  background: rgba(255, 255, 255, 0.08);
}

.sidebar-footer strong {
  color: #dce2ef;
  font-size: 12px;
}

.sidebar-footer span {
  margin-top: 2px;
  color: #747e94;
  font-size: 10px;
}

.content-shell {
  width: calc(100% - 252px);
  min-width: 0;
  margin-left: 252px;
}

.main-content {
  min-height: 100vh;
  background: #f5f7fb;
}

.mobile-header,
.sidebar-backdrop {
  display: none;
}

@media (max-width: 880px) {
  .sidebar {
    transform: translateX(-100%);
    transition: transform 180ms ease;
  }

  .sidebar.open {
    transform: translateX(0);
  }

  .sidebar-backdrop {
    position: fixed;
    inset: 0;
    z-index: 20;
    display: block;
    background: rgba(17, 22, 38, 0.46);
    backdrop-filter: blur(2px);
  }

  .close-sidebar {
    display: grid;
    width: 32px;
    height: 32px;
    margin-left: auto;
    place-items: center;
    border: 0;
    border-radius: 8px;
    color: #aeb6ca;
    background: transparent;
    cursor: pointer;
  }

  .content-shell {
    width: 100%;
    margin-left: 0;
  }

  .mobile-header {
    position: sticky;
    top: 0;
    z-index: 10;
    display: flex;
    align-items: center;
    gap: 12px;
    height: 58px;
    padding: 0 18px;
    border-bottom: 1px solid var(--app-border);
    background: rgba(255, 255, 255, 0.9);
    backdrop-filter: blur(12px);
  }

  .mobile-header button {
    display: grid;
    width: 34px;
    height: 34px;
    place-items: center;
    border: 1px solid var(--app-border);
    border-radius: 9px;
    color: var(--app-text);
    background: #fff;
  }
}
</style>
