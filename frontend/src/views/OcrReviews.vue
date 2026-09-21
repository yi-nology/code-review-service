<template>
  <ReviewListView
    eyebrow="OCR"
    title="OCR 审查"
    description="OCR 多智能体引擎的审查任务：手动发起、webhook 自动触发与重新审查。"
    pinned-engine="ocr"
    :show-saved-views="false"
  />
</template>

<script setup lang="ts">
// OCR 审查：锁定 OCR 引擎的审查视图；保存视图是跨引擎语义，故隐藏。
// 列表实现在 components/review-list/ReviewListView.vue。
import ReviewListView from '@/components/review-list/ReviewListView.vue'
</script>
