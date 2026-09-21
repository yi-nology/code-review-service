<template>
  <ReviewListView
    eyebrow="REVIEWS"
    title="审查记录"
    description="跟踪审查队列、质量门禁和风险项处理状态。"
  />
</template>

<script setup lang="ts">
// 审查记录：全部引擎（OCR + Argus）的评审记录，支持引擎筛选与保存视图。
// 列表实现在 components/review-list/ReviewListView.vue。
import ReviewListView from '@/components/review-list/ReviewListView.vue'
</script>
