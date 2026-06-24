<template>
  <div class="step-indicator">
    <div
      v-for="(step, index) in steps"
      :key="index"
      class="step-item"
      :class="{
        active: index === currentStep,
        completed: index < currentStep,
      }"
    >
      <div class="step-dot">
        <el-icon v-if="index < currentStep"><Check /></el-icon>
        <span v-else>{{ index + 1 }}</span>
      </div>
      <span class="step-label">{{ step.label }}</span>
      <div v-if="index < steps.length - 1" class="step-line" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { Check } from '@element-plus/icons-vue'

interface Step {
  label: string
  description?: string
}

defineProps<{
  steps: Step[]
  currentStep: number
}>()
</script>

<style scoped>
.step-indicator {
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding: 24px 0 32px;
}

.step-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  position: relative;
  flex: 1;
  max-width: 160px;
}

.step-dot {
  width: 40px;
  height: 40px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 14px;
  font-weight: 600;
  background: var(--app-border);
  color: var(--app-muted);
  transition: all 0.3s ease;
  position: relative;
  z-index: 1;
}

.step-item.active .step-dot {
  background: var(--app-primary);
  color: white;
  box-shadow: 0 0 0 4px rgba(91, 91, 214, 0.2);
}

.step-item.completed .step-dot {
  background: var(--app-success);
  color: white;
}

.step-label {
  margin-top: 10px;
  font-size: 12px;
  font-weight: 500;
  color: var(--app-muted);
  text-align: center;
  transition: color 0.3s ease;
}

.step-item.active .step-label {
  color: var(--app-primary);
  font-weight: 600;
}

.step-item.completed .step-label {
  color: var(--app-success);
}

.step-line {
  position: absolute;
  top: 20px;
  left: calc(50% + 24px);
  width: calc(100% - 48px);
  height: 2px;
  background: var(--app-border);
  transition: background 0.3s ease;
}

.step-item.completed .step-line {
  background: var(--app-success);
}

@media (max-width: 640px) {
  .step-indicator {
    padding: 16px 0 24px;
  }

  .step-dot {
    width: 32px;
    height: 32px;
    font-size: 12px;
  }

  .step-label {
    font-size: 10px;
    margin-top: 6px;
  }

  .step-line {
    top: 16px;
    left: calc(50% + 20px);
    width: calc(100% - 40px);
  }
}
</style>
