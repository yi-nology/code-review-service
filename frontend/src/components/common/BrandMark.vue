<script setup lang="ts">
import { useId } from 'vue'

withDefaults(defineProps<{
  size?: number
}>(), {
  size: 40,
})

// 同一页面可能同时渲染多个 BrandMark（例如登录页的桌面/移动两处），
// 渐变 id 必须按实例唯一，否则后一个实例会复用前一个的定义。
const gradId = `brand-mark-bg-${useId()}`
</script>

<template>
  <span class="brand-mark" :style="{ width: `${size}px`, height: `${size}px` }">
    <svg viewBox="0 0 40 40" fill="none" xmlns="http://www.w3.org/2000/svg">
      <defs>
        <linearGradient
          :id="gradId"
          x1="0"
          y1="0"
          x2="40"
          y2="40"
          gradientUnits="userSpaceOnUse"
        >
          <stop stop-color="#60a5fa" />
          <stop offset="1" stop-color="#2563eb" />
        </linearGradient>
      </defs>
      <rect width="40" height="40" rx="11" :fill="`url(#${gradId})`" />
      <!-- 放大镜镜身：审查 / 检查代码 -->
      <circle
        cx="17.8"
        cy="17.2"
        r="9.8"
        stroke="#ffffff"
        stroke-width="3"
        fill="#ffffff"
        fill-opacity="0.14"
      />
      <!-- 镜内对勾：审查通过 / 质量门禁 -->
      <path
        d="M13.6 17.4 16.4 20.2 21.6 14.4"
        stroke="#ffffff"
        stroke-width="3"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
      <!-- 手柄 -->
      <path d="M24.9 24.3 31.8 31.2" stroke="#ffffff" stroke-width="3.6" stroke-linecap="round" />
    </svg>
  </span>
</template>

<style scoped>
.brand-mark {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  overflow: hidden;
  flex: 0 0 auto;
}
.brand-mark svg {
  width: 100%;
  height: 100%;
  display: block;
}
</style>
