<script setup lang="ts">
/**
 * ReviewModeHero — 审查详情页模式主视觉条。
 *
 * 以模式主题色渐变横幅呈现三种审查模式的身份差异：
 * - 左侧模式图标块（OCR=多智能体 / Argus=百眼之眼 / Scan=全库扫描）
 * - 中部：模式徽章 + 仓库标题 + 模式说明 + 上下文 chips（平台/PR/commit/任务 ID）
 * - 右侧：actions 插槽（Debug 开关等）
 */
import { computed } from 'vue'
import { Cpu, View, Search, Link } from '@element-plus/icons-vue'
import { REVIEW_MODE_META, type ReviewMode } from '@/constants/reviewMode'
import { getProviderLabel } from '@/utils/url-parser'

const props = defineProps<{
  mode: ReviewMode
  repo: string
  provider: string
  reviewId: string
  pullRequestId?: string
  mergeRequestIid?: string
  headSha?: string
  baseSha?: string
  author?: string
  authorName?: string
  authorAvatar?: string
  /** argus 模式：外部任务 ID chip */
  argusTaskId?: string
}>()

const meta = computed(() => REVIEW_MODE_META[props.mode])

const modeIcon = computed(() => {
  if (props.mode === 'argus') return View
  if (props.mode === 'scan') return Search
  return Cpu
})

const chips = computed(() => {
  const items: string[] = [getProviderLabel(props.provider)]
  if (props.pullRequestId) items.push(`PR #${props.pullRequestId}`)
  else if (props.mergeRequestIid) items.push(`MR !${props.mergeRequestIid}`)
  if (props.mode === 'scan' && props.headSha) items.push(`HEAD ${props.headSha.slice(0, 7)}`)
  else if (!props.pullRequestId && !props.mergeRequestIid && props.headSha) {
    items.push(`${(props.baseSha || '').slice(0, 7)}..${props.headSha.slice(0, 7)}`)
  }
  if (props.mode === 'argus' && props.argusTaskId) items.push(`task ${props.argusTaskId.slice(0, 14)}`)
  items.push(props.reviewId)
  return items
})
</script>

<template>
  <header
    class="mode-hero"
    :style="{ '--mode-color': meta.color, '--mode-soft': meta.soft, '--mode-border': meta.border }"
  >
    <div class="hero-main">
      <div class="mode-icon-block">
        <el-icon :size="26">
          <component :is="modeIcon" />
        </el-icon>
      </div>

      <div class="hero-text">
        <div class="title-row">
          <span class="mode-badge">{{ meta.label }}</span>
          <h1 class="repo-title">{{ repo || '审查详情' }}</h1>
        </div>
        <p class="mode-tagline">{{ meta.tagline }}</p>
        <div class="chip-row">
          <span
            v-if="authorAvatar || authorName"
            class="chip chip-author"
          >
            <img v-if="authorAvatar" :src="authorAvatar" alt="" class="author-avatar" referrerpolicy="no-referrer">
            <span>{{ authorName || author }}</span>
          </span>
          <span v-for="chip in chips" :key="chip" class="chip">
            <el-icon v-if="chip === reviewId" :size="11"><Link /></el-icon>
            <span>{{ chip }}</span>
          </span>
        </div>
      </div>
    </div>

    <div v-if="$slots.actions" class="hero-actions">
      <slot name="actions" />
    </div>
  </header>
</template>

<style scoped>
.mode-hero {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 16px;
  align-items: flex-start;
  margin-bottom: var(--page-gap);
  padding: 20px 24px;
  border: 1px solid var(--mode-border);
  border-radius: var(--page-header-radius);
  background:
    radial-gradient(ellipse 480px 140px at 8% 0%, color-mix(in srgb, var(--mode-color) 12%, transparent), transparent),
    linear-gradient(135deg, var(--mode-soft) 0%, #ffffff 62%);
  box-shadow: var(--app-shadow);
}

.hero-main {
  display: flex;
  gap: 16px;
  min-width: 0;
}

.mode-icon-block {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 52px;
  height: 52px;
  border-radius: 14px;
  color: #fff;
  background: linear-gradient(135deg, var(--mode-color), color-mix(in srgb, var(--mode-color) 72%, #0f172a));
  box-shadow: 0 6px 16px color-mix(in srgb, var(--mode-color) 32%, transparent);
}

.hero-text {
  min-width: 0;
}

.title-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
}

.mode-badge {
  flex-shrink: 0;
  padding: 3px 10px;
  border-radius: 999px;
  border: 1px solid color-mix(in srgb, var(--mode-color) 36%, transparent);
  background: #fff;
  color: var(--mode-color);
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.2px;
}

.repo-title {
  margin: 0;
  color: var(--app-text);
  font-size: 22px;
  line-height: 1.25;
  overflow-wrap: anywhere;
}

.mode-tagline {
  margin: 6px 0 10px;
  color: color-mix(in srgb, var(--mode-color) 78%, #334155);
  font-size: 13px;
  line-height: 1.6;
}

.chip-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
}

.chip {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  max-width: 260px;
  padding: 2px 9px;
  border-radius: 999px;
  border: 1px solid var(--app-border);
  background: color-mix(in srgb, #ffffff 72%, transparent);
  color: #64748b;
  font-size: 11.5px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.chip-author {
  border-color: color-mix(in srgb, var(--mode-color) 30%, transparent);
  color: #475569;
}

.author-avatar {
  width: 14px;
  height: 14px;
  border-radius: 50%;
  object-fit: cover;
}

.hero-actions {
  display: flex;
  flex: 0 0 auto;
  flex-wrap: wrap;
  align-items: flex-start;
  justify-content: flex-end;
  gap: 10px;
}

@media (max-width: 720px) {
  .mode-hero {
    grid-template-columns: minmax(0, 1fr);
  }

  .hero-actions {
    justify-content: flex-start;
  }
}
</style>
