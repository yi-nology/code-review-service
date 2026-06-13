<script setup lang="ts">
defineProps<{
  label: string
  value: string | number
  valueClass?: string
}>()
</script>

<template>
  <div class="metric-card">
    <span>{{ label }}</span>
    <strong :class="valueClass">{{ value }}</strong>
  </div>
</template>

<style scoped>
.metric-card {
  background: #fff;
  border: 1px solid #edf0f4;
  border-radius: 8px;
  padding: 12px 16px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.metric-card span {
  font-size: 12px;
  color: #7d8698;
}
.metric-card strong {
  font-size: 22px;
  color: #1f2933;
}
</style>
