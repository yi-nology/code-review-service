<template>
  <AppLayout>
    <PageShell>
      <PageHeader
        eyebrow="API DOCUMENTATION"
        title="接口文档"
        description="基于 OpenAPI 规范的交互式 API 文档"
      >
        <template #actions>
          <el-button @click="reload">
            <el-icon><Refresh /></el-icon>
            刷新
          </el-button>
          <el-button tag="a" :href="swaggerUrl" target="_blank">
            <el-icon><FullScreen /></el-icon>
            新窗口打开
          </el-button>
        </template>
      </PageHeader>

      <div class="swagger-frame-wrapper">
        <iframe
          ref="frameRef"
          :src="swaggerUrl"
          class="swagger-frame"
          title="Swagger UI"
        />
      </div>
    </PageShell>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import PageShell from '@/components/common/PageShell.vue'
import PageHeader from '@/components/common/PageHeader.vue'
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
.swagger-frame-wrapper {
  overflow: hidden;
  border: 1px solid var(--app-border);
  border-radius: var(--card-radius-lg);
  background: var(--app-surface);
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
