<template>
  <div class="page-shell">
    <slot />
  </div>
</template>

<script setup lang="ts">
// 统一页面根容器：负责 padding + 全宽布局（不限制 max-width）。
// 所有非 Login 页面用 <PageShell> 替换各自手写的 <div class="xxx-page">。
// 间距 token 来自 style.css 的 :root，修改一处即全局生效。
</script>

<style scoped>
.page-shell {
  padding: var(--page-padding-top) var(--page-padding-x) var(--page-padding-bottom);
}

@media (max-width: 720px) {
  .page-shell {
    padding: 20px 16px 32px;
  }
}
</style>
