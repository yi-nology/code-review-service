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
        <router-link to="/dashboard" class="nav-item">
          <el-icon><HomeFilled /></el-icon>
          <span>运营总览</span>
        </router-link>
        <router-link to="/reviews" class="nav-item">
          <el-icon><DocumentChecked /></el-icon>
          <span>审查记录</span>
        </router-link>
        <router-link to="/scan" class="nav-item">
          <el-icon><Search /></el-icon>
          <span>代码库扫描</span>
        </router-link>
        <router-link to="/contributors" class="nav-item">
          <el-icon><DataAnalysis /></el-icon>
          <span>开发者质量</span>
        </router-link>
        <router-link to="/projects/reports" class="nav-item">
          <el-icon><Notebook /></el-icon>
          <span>项目报告</span>
        </router-link>
        <router-link to="/api-docs" class="nav-item">
          <el-icon><Document /></el-icon>
          <span>接口文档</span>
        </router-link>

        <p class="nav-label settings-label">配置中心</p>

        <div class="nav-group">
          <button class="nav-group-header" @click="toggleGroup('system')">
            <span>系统设置</span>
            <el-icon class="nav-group-arrow" :class="{ expanded: expandedGroups.system }"><ArrowRight /></el-icon>
          </button>
          <div v-show="expandedGroups.system" class="nav-group-items">
            <router-link to="/settings/model" class="nav-item">
              <el-icon><Cpu /></el-icon>
              <span>模型配置</span>
            </router-link>
            <router-link to="/settings/ocr" class="nav-item">
              <el-icon><Tools /></el-icon>
              <span>OCR 设置</span>
            </router-link>
            <router-link to="/settings/platforms" class="nav-item">
              <el-icon><Connection /></el-icon>
              <span>Git 平台</span>
            </router-link>
            <router-link to="/settings/agents" class="nav-item">
              <el-icon><User /></el-icon>
              <span>审查代理</span>
            </router-link>
          </div>
        </div>

        <div class="nav-group">
          <button class="nav-group-header" @click="toggleGroup('rules')">
            <span>审查规则</span>
            <el-icon class="nav-group-arrow" :class="{ expanded: expandedGroups.rules }"><ArrowRight /></el-icon>
          </button>
          <div v-show="expandedGroups.rules" class="nav-group-items">
            <router-link to="/settings/rules" class="nav-item">
              <el-icon><List /></el-icon>
              <span>全局规则</span>
            </router-link>
          </div>
        </div>

        <div class="nav-group">
          <button class="nav-group-header" @click="toggleGroup('project')">
            <span>项目管理</span>
            <el-icon class="nav-group-arrow" :class="{ expanded: expandedGroups.project }"><ArrowRight /></el-icon>
          </button>
          <div v-show="expandedGroups.project" class="nav-group-items">
            <router-link to="/settings/projects" class="nav-item">
              <el-icon><FolderChecked /></el-icon>
              <span>项目策略</span>
            </router-link>
          </div>
        </div>
      </nav>

      <div class="sidebar-footer">
        <span class="health-dot" :class="healthStatus" />
        <div class="service-status">
          <strong>{{ healthLabel }}</strong>
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
import { ref, reactive, computed, onMounted, onBeforeUnmount } from 'vue'
import {
  ArrowRight,
  Close,
  Connection,
  Cpu,
  DataAnalysis,
  Document,
  DocumentChecked,
  Finished,
  FolderChecked,
  HomeFilled,
  List,
  Menu,
  Notebook,
  Search,
  SwitchButton,
  Tools,
  User,
} from '@element-plus/icons-vue'
import { useAuthStore } from '@/stores/auth'

const sidebarOpen = ref(false)
const auth = useAuthStore()

const expandedGroups = reactive({
  system: true,
  rules: true,
  project: true,
})

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

function toggleGroup(group: 'system' | 'rules' | 'project') {
  expandedGroups[group] = !expandedGroups[group]
}

function logout() {
  auth.clearToken()
}

onMounted(() => {
  checkHealth()
  healthTimer = setInterval(checkHealth, 30000)
})

onBeforeUnmount(() => {
  if (healthTimer) {
    clearInterval(healthTimer)
    healthTimer = null
  }
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

.nav-group {
  margin-bottom: 4px;
}

.nav-group-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  min-height: 38px;
  padding: 0 12px;
  border: none;
  border-radius: 6px;
  color: #8d98aa;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.5px;
  background: transparent;
  cursor: pointer;
  transition: 160ms ease;
}

.nav-group-header:hover {
  color: #dfe4f2;
  background: #182235;
}

.nav-group-arrow {
  font-size: 12px;
  transition: transform 200ms ease;
}

.nav-group-arrow.expanded {
  transform: rotate(90deg);
}

.nav-group-items {
  padding-left: 4px;
}

.nav-group-items .nav-item {
  min-height: 40px;
  font-size: 13px;
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
