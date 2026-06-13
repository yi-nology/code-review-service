<script setup lang="ts">
import { getScoreColor } from '@/utils/colors'

const props = withDefaults(defineProps<{
  score: number
  label?: string
}>(), {
  label: '平均分',
})

const color = () => getScoreColor(props.score)
const circumference = 327
const offset = () => circumference - (circumference * props.score / 100)
</script>

<template>
  <div class="score-ring" :style="{ '--score-color': color() }">
    <svg viewBox="0 0 120 120">
      <circle cx="60" cy="60" r="52" fill="none" stroke="#edf0f4" stroke-width="10" />
      <circle
        cx="60" cy="60" r="52" fill="none" stroke="var(--score-color)" stroke-width="10"
        stroke-linecap="round"
        :stroke-dasharray="circumference"
        :stroke-dashoffset="offset()"
        transform="rotate(-90 60 60)"
      />
    </svg>
    <div class="score-ring-text">
      <strong>{{ score }}</strong>
      <small>{{ label }}</small>
    </div>
  </div>
</template>

<style scoped>
.score-ring {
  position: relative;
  width: 120px;
  height: 120px;
  flex-shrink: 0;
}
.score-ring svg {
  width: 100%;
  height: 100%;
}
.score-ring-text {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
}
.score-ring-text strong {
  font-size: 28px;
  color: var(--score-color);
}
.score-ring-text small {
  font-size: 12px;
  color: #7d8698;
}
</style>
