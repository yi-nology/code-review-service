<template>
  <el-row :gutter="16" class="stat-strip">
    <el-col
      v-for="(item, index) in items"
      :key="index"
      :xs="12"
      :sm="12"
      :md="spans[index] ?? 24"
    >
      <el-card shadow="never" class="stat-card" :class="item.tone || 'primary'">
        <div class="stat-icon">
          <el-icon><component :is="item.icon" /></el-icon>
        </div>
        <div class="stat-body">
          <span class="stat-label">{{ item.label }}</span>
          <strong
            class="stat-value"
            :class="valueSizeClass(item.value)"
            :title="item.title ?? String(item.value)"
          >{{ item.value }}</strong>
          <small v-if="item.sublabel" class="stat-sublabel">{{ item.sublabel }}</small>
        </div>
      </el-card>
    </el-col>
  </el-row>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { Component } from 'vue'

// 统一统计条（独立卡片型）。
// 替换原各页面 7 种命名：metric-strip / summary-strip / summary-grid /
// metric-grid / stats-row / health-band / status-strip。
// 颜色由 tone 映射，收敛原本散落各页面的硬编码色值。
export interface StatItem {
  icon: Component
  label: string
  value: string | number
  /** 悬浮提示：数值被缩写或换行时，完整值仍可查。 */
  title?: string
  sublabel?: string
  tone?: 'primary' | 'success' | 'warning' | 'danger' | 'info'
}

const props = defineProps<{
  items: StatItem[]
}>()

// 24 栅格按项数均分。
//
// 不能用 Math.floor(24/count) 给所有项同一个 span：项数不能整除 24 时会
// 留下整列空白（5 项 → 每项 4 栅格 = 只用掉 20/24，右侧空一列），
// 卡片被压窄后数值还会被 ellipsis 截断（如「776.…」）。
// 正确做法是逐项分配 base/base+1，总和恰好 24。
/**
 * 数值过长时降一档字号。
 *
 * 卡片宽度随项数变化（6 项时每张最窄），26px 的「776.2k」在窄卡里会被
 * ellipsis 截成「776.…」——数字被截断等于没有数字。
 */
function valueSizeClass(value: string | number): string {
  const len = String(value).length
  if (len > 9) return 'stat-value--xs'
  if (len > 5) return 'stat-value--sm'
  return ''
}

const spans = computed(() => {
  const count = props.items.length
  if (!count) return []
  const base = Math.max(1, Math.floor(24 / count))
  const remainder = 24 - base * count
  // 余数补给**最后**几项：末尾常常是数值最长的卡片（如「累计 Tokens」），
  // 放在最前面会让它变成整行最窄的一张。
  return props.items.map((_, i) => base + (i >= count - remainder ? 1 : 0))
})
</script>

<style scoped>
.stat-strip {
  margin-bottom: var(--page-gap);
}

.stat-strip :deep(.el-col) {
  margin-bottom: 16px;
}

.stat-card {
  height: var(--stat-card-height);
  overflow: hidden;
  border-radius: var(--stat-card-radius);
}

.stat-card :deep(.el-card__body) {
  display: flex;
  align-items: center;
  gap: 15px;
  height: 100%;
  padding: 20px;
}

.stat-icon {
  display: grid;
  width: var(--stat-icon-size);
  height: var(--stat-icon-size);
  flex: 0 0 var(--stat-icon-size);
  place-items: center;
  border-radius: var(--stat-icon-radius);
  font-size: 20px;
}

/* tone → 颜色映射：替换原散落页面的 #5b5bd6/#159570/#2878c7/#d98616 等硬编码色 */
.stat-card.primary .stat-icon { color: var(--app-primary); background: #eff6ff; }
.stat-card.success .stat-icon { color: var(--app-success); background: #e6f7f0; }
.stat-card.warning .stat-icon { color: var(--app-warning); background: #fff6e8; }
.stat-card.danger  .stat-icon { color: var(--app-danger);  background: #fef2f2; }
.stat-card.info    .stat-icon { color: #0891b2;            background: #ecfeff; }

.stat-body {
  min-width: 0;
  flex: 1;
}

.stat-label {
  display: block;
  margin-bottom: 2px;
  color: var(--app-muted);
  font-size: var(--stat-label-size);
  font-weight: 600;
}

.stat-value {
  display: block;
  overflow: hidden;
  color: var(--app-text);
  font-size: var(--stat-value-size);
  line-height: 1.15;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.stat-value--sm {
  font-size: 20px;
}

.stat-value--xs {
  font-size: 17px;
}

.stat-sublabel {
  display: block;
  margin-top: 4px;
  color: #9aa2b1;
  font-size: var(--stat-sublabel-size);
}

@media (max-width: 720px) {
  .stat-card {
    height: 96px;
  }

  .stat-card :deep(.el-card__body) {
    padding: 14px;
  }

  .stat-sublabel {
    display: none;
  }
}
</style>
