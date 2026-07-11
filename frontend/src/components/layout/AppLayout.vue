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
        <!-- 三主域导航：评审 / 洞察 / 配置，数据驱动渲染 -->
        <template v-for="group in navGroupsData" :key="group.key">
          <p class="nav-label" :class="{ 'settings-label': group.key !== 'review' }">{{ group.label }}</p>
          <router-link
            v-for="item in group.items"
            :key="item.path"
            :to="item.path"
            class="nav-item"
          >
            <el-icon><component :is="item.icon" /></el-icon>
            <span>{{ item.label }}</span>
          </router-link>
        </template>
      </nav>

      <div class="sidebar-footer">
        <span class="health-dot" :class="healthStatus" />
        <div class="service-status">
          <strong>Review Service</strong>
          <span>AI 代码评审控制台</span>
        </div>
      </div>
    </aside>

    <div class="content-shell">
      <header class="app-topbar">
        <button class="topbar-menu-btn" aria-label="打开菜单" @click="sidebarOpen = true">
          <el-icon><Menu /></el-icon>
        </button>
        <!-- 面包屑 + 当前页标题：让用户始终知道自己在哪 -->
        <nav v-if="crumb.group || crumb.title" class="topbar-breadcrumb" aria-label="页面位置">
          <span v-if="crumb.group" class="crumb-group">{{ crumb.group }}</span>
          <el-icon v-if="crumb.group && crumb.title" class="crumb-sep"><ArrowRight /></el-icon>
          <span class="crumb-title">{{ crumb.title }}</span>
        </nav>
        <div class="topbar-spacer" />
        <!-- 全局搜索入口：点击或 Cmd/Ctrl+K 唤起 -->
        <button class="topbar-search-btn" aria-label="全局搜索" @click="searchOpen = true">
          <el-icon><Search /></el-icon>
          <kbd class="search-kbd">⌘K</kbd>
        </button>
        <span class="health-dot topbar-health-dot" :class="healthStatus" />
        <span class="topbar-health-text">{{ healthLabel }}</span>
        <NotificationCenter />
        <button class="logout-button topbar-logout" aria-label="退出登录" title="退出登录" @click="logout">
          <el-icon><SwitchButton /></el-icon>
        </button>
      </header>
      <main class="main-content">
        <slot />
      </main>
    </div>

    <!-- 全局搜索（Cmd/Ctrl+K 唤起） -->
    <GlobalSearch v-model="searchOpen" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { ArrowRight, Close, Finished, Menu, Search, SwitchButton } from '@element-plus/icons-vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { navGroups, resolveBreadcrumb } from '@/constants/nav'
import NotificationCenter from './NotificationCenter.vue'
import GlobalSearch from './GlobalSearch.vue'

const sidebarOpen = ref(false)
const searchOpen = ref(false)
const auth = useAuthStore()
const route = useRoute()
const router = useRouter()

// 菜单数据来自共享常量，GlobalSearch 与面包屑共用同一数据源。
const navGroupsData = navGroups

// 当前页面包屑：分组 → 页面标题。
const crumb = computed(() => resolveBreadcrumb(route.path))

const healthStatus = ref<'healthy' | 'down' | 'unknown'>('unknown')
const healthLabel = computed(() => healthStatus.value === 'healthy' ? '服务运行中' : healthStatus.value === 'down' ? '服务异常' : '检测中…')
let healthTimer: ReturnType<typeof setInterval> | null = null

async function checkHealth() {
  try {
    const resp = await window.fetch('/ping', { method: 'GET' })
    healthStatus.value = resp.ok ? 'healthy' : 'down'
  } catch {
    healthStatus.value = 'down'
  }
}

function logout() {
  auth.clearToken()
  // 退出后跳登录页，避免 router-view 因 token 清空而消失、用户看到空壳。
  router.replace('/login')
}

// Cmd/Ctrl+K 唤起全局搜索；输入框聚焦时不拦截，避免与文本快捷键冲突。
function onGlobalKeydown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
    e.preventDefault()
    searchOpen.value = true
  }
}

onMounted(() => {
  checkHealth()
  healthTimer = setInterval(checkHealth, 30000)
  window.addEventListener('keydown', onGlobalKeydown)
})

onBeforeUnmount(() => {
  if (healthTimer) {
    clearInterval(healthTimer)
    healthTimer = null
  }
  window.removeEventListener('keydown', onGlobalKeydown)
})
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
  border-right: 1px solid #20283a;
  color: #d8dee9;
  background: #111827;
}

.brand {
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 74px;
  padding: 0 18px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
}

.brand-mark {
  display: grid;
  width: 40px;
  height: 40px;
  flex: 0 0 40px;
  place-items: center;
  border-radius: 8px;
  color: #fff;
  background: #2563eb;
  box-shadow: none;
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
  color: #8d98aa;
  font-size: 12px;
}

.close-sidebar {
  display: none;
}

.navigation {
  flex: 1;
  padding: 18px 12px;
}

.nav-label {
  margin: 0 10px 8px;
  color: #778399;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0;
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
  border-radius: 7px;
  color: #a9b4c5;
  text-decoration: none;
  transition: 160ms ease;
}

.nav-item .el-icon {
  font-size: 18px;
}

.nav-item:hover {
  color: #fff;
  background: #182235;
}

.nav-item.router-link-active {
  color: #fff;
  background: #1d4ed8;
  box-shadow: none;
}

.sidebar-footer {
  display: flex;
  align-items: center;
  gap: 11px;
  margin: 14px;
  padding: 14px;
  border: 1px solid rgba(255, 255, 255, 0.07);
  border-radius: 8px;
  background: #162033;
}

.health-dot {
  width: 9px;
  height: 9px;
  flex: 0 0 9px;
  border-radius: 50%;
  background: #45d6a2;
  box-shadow: 0 0 0 4px rgba(69, 214, 162, 0.12);
}

.health-dot.down {
  background: #f56c6c;
  box-shadow: 0 0 0 4px rgba(245, 108, 108, 0.16);
}

.health-dot.unknown {
  background: #909399;
  box-shadow: 0 0 0 4px rgba(144, 147, 153, 0.14);
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
  border-radius: 6px;
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
  background: #f6f7f9;
}

/* 顶部栏：常驻，容纳健康状态、通知中心、退出登录 */
.app-topbar {
  position: sticky;
  top: 0;
  z-index: 10;
  display: flex;
  align-items: center;
  gap: 12px;
  height: 56px;
  padding: 0 24px;
  border-bottom: 1px solid #e5e7eb;
  background: rgba(255, 255, 255, 0.92);
  backdrop-filter: blur(12px);
}

.topbar-menu-btn {
  display: none;
  align-items: center;
  justify-content: center;
  width: 34px;
  height: 34px;
  border: 1px solid #e5e7eb;
  border-radius: 9px;
  color: #374151;
  background: #fff;
  cursor: pointer;
}

.topbar-health-dot {
  width: 8px;
  height: 8px;
  flex: 0 0 8px;
}

.topbar-health-text {
  font-size: 12px;
  color: #6b7280;
}

.topbar-spacer {
  flex: 1;
}

/* 面包屑：分组 > 页面标题，让用户始终知道自己在哪里 */
.topbar-breadcrumb {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.crumb-group {
  color: #929aab;
  font-size: 13px;
  white-space: nowrap;
}

.crumb-sep {
  color: #c4cad6;
  font-size: 12px;
}

.crumb-title {
  color: var(--app-text);
  font-size: 14px;
  font-weight: 700;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* 全局搜索按钮：放大镜 + 快捷键提示 */
.topbar-search-btn {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 34px;
  padding: 0 12px;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  color: #6b7280;
  background: #fff;
  cursor: pointer;
  transition: 160ms ease;
}

.topbar-search-btn:hover {
  border-color: #93c5fd;
  color: #1d4ed8;
}

.search-kbd {
  padding: 1px 6px;
  border: 1px solid #e5e7eb;
  border-radius: 5px;
  color: #929aab;
  background: #f6f7f9;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 11px;
  line-height: 1.5;
}

.topbar-logout {
  display: grid;
  width: 34px;
  height: 34px;
  flex: 0 0 34px;
  place-items: center;
  border: none;
  border-radius: 8px;
  color: #6b7280;
  background: transparent;
  cursor: pointer;
}

.topbar-logout:hover {
  color: #1f2937;
  background: #f3f4f6;
}

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

  .topbar-menu-btn {
    display: flex;
  }

  .topbar-health-text {
    display: none;
  }

  /* 移动端隐藏快捷键提示与分组前缀，只留页面标题，节省顶栏空间 */
  .search-kbd {
    display: none;
  }

  .crumb-group,
  .crumb-sep {
    display: none;
  }
}
</style>
