<template>
  <header class="page-header">
    <div class="page-header__main">
      <div v-if="eyebrow" class="eyebrow">{{ eyebrow }}</div>
      <h1>{{ title }}</h1>
      <p v-if="description" class="page-header__desc">{{ description }}</p>
    </div>
    <div v-if="$slots.actions" class="page-header__actions">
      <slot name="actions" />
    </div>
  </header>
</template>

<script setup lang="ts">
// 统一页头（卡片包裹型）。
// 替换原各页面 8 种命名：page-header / dashboard-hero / policy-hero /
// ms-header / page-hero / reports-hero / scan-hero / cd-hero。
defineProps<{
  eyebrow?: string
  title: string
  description?: string
}>()
</script>

<style scoped>
.page-header {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: var(--page-header-gap);
  align-items: start;
  margin-bottom: var(--page-gap);
  padding: var(--page-header-padding);
  border: 1px solid var(--app-border);
  border-radius: var(--page-header-radius);
  background: var(--app-surface);
  box-shadow: var(--app-shadow);
}

.page-header__main {
  min-width: 0;
}

.eyebrow {
  color: var(--eyebrow-color);
  font-size: var(--eyebrow-size);
  font-weight: var(--eyebrow-weight);
  letter-spacing: var(--eyebrow-spacing);
}

.page-header h1 {
  margin: 5px 0 8px;
  color: var(--app-text);
  font-size: var(--page-h1-size);
  line-height: var(--page-h1-line-height);
}

.page-header__desc {
  max-width: 760px;
  margin: 0;
  color: var(--app-muted);
  font-size: 14px;
  line-height: 1.7;
}

.page-header__actions {
  display: flex;
  flex: 0 0 auto;
  flex-wrap: wrap;
  align-items: flex-start;
  justify-content: flex-end;
  gap: 10px;
}

@media (max-width: 720px) {
  .page-header {
    grid-template-columns: minmax(0, 1fr);
  }

  .page-header__actions {
    justify-content: flex-start;
  }

  .page-header__desc {
    display: none;
  }
}
</style>
