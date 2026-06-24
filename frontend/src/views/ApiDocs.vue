<template>
  <AppLayout>
    <div class="api-docs-page">
      <header class="page-header">
        <div>
          <div class="eyebrow">API DOCUMENTATION</div>
          <h1>接口文档</h1>
          <p>基于 OpenAPI 规范的交互式 API 文档</p>
        </div>
        <div class="header-actions">
          <el-button @click="reload">
            <el-icon><Refresh /></el-icon>
            刷新
          </el-button>
          <el-button tag="a" :href="swaggerUrl" target="_blank">
            <el-icon><FullScreen /></el-icon>
            新窗口打开
          </el-button>
        </div>
      </header>

      <div class="swagger-frame-wrapper">
        <iframe
          ref="frameRef"
          :src="swaggerUrl"
          class="swagger-frame"
          title="Swagger UI"
        />
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { FullScreen, Refresh } from '@element-plus/icons-vue'

const swaggerUrl = '/swagger/index.html'
const frameRef = ref<HTMLIFrameElement | null>(null)

function reload() {
  if (frameRef.value) {
    frameRef.value.src = swaggerUrl
  }
}
</script>

<style scoped>
.api-docs-page {
  padding: 32px clamp(20px, 4vw, 48px);
}

.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 24px;
}

.eyebrow {
  margin-bottom: 8px;
  color: var(--app-muted);
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 1.5px;
}

.page-header h1 {
  margin: 0;
  font-size: 26px;
  letter-spacing: -0.5px;
}

.page-header p {
  margin: 6px 0 0;
  color: var(--app-muted);
  font-size: 14px;
}

.header-actions {
  display: flex;
  flex-shrink: 0;
  gap: 10px;
}

.swagger-frame-wrapper {
  overflow: hidden;
  border: 1px solid var(--app-border);
  border-radius: 14px;
  background: #fff;
  box-shadow: var(--app-shadow);
}

.swagger-frame {
  display: block;
  width: 100%;
  height: calc(100vh - 220px);
  min-height: 520px;
  border: 0;
}
</style>
