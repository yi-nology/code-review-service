<script setup lang="ts">
withDefaults(defineProps<{
  size?: number
}>(), {
  size: 40,
})
</script>

<template>
  <span class="brand-mark" :style="{ width: `${size}px`, height: `${size}px` }">
    <svg viewBox="0 0 40 40" fill="none" xmlns="http://www.w3.org/2000/svg">
      <defs>
        <linearGradient id="brand-mark-bg" x1="0" y1="0" x2="0" y2="40" gradientUnits="userSpaceOnUse">
          <stop stop-color="#3b82f6" />
          <stop offset="1" stop-color="#1d4ed8" />
        </linearGradient>
      </defs>
      <rect width="40" height="40" rx="9" fill="url(#brand-mark-bg)" />
      <!-- 盾牌轮廓 -->
      <path
        d="M20 8.5L13 11v6.2c0 4.35 2.95 8.42 7 9.78 4.05-1.36 7-5.43 7-9.78V11l-7-2.5z"
        fill="none" stroke="#ffffff" stroke-width="1.6" stroke-linejoin="round" opacity="0.95"
      />
      <!-- 内嵌对勾（象征审查通过 / 质量把关） -->
      <path
        d="M15.5 20.2L18.6 23.5L24.8 16.5"
        fill="none" stroke="#ffffff" stroke-width="2.1"
        stroke-linecap="round" stroke-linejoin="round"
      />
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
