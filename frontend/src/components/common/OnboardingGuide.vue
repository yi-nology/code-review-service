<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { CircleCheck, Right } from '@element-plus/icons-vue'

const router = useRouter()
const props = defineProps<{
  /**
   * 每个步骤的真实完成状态（由父组件查询实际配置得出）。
   * 优先于 completedSteps；未传时回退到 completedSteps 的顺序推断。
   */
  stepStatus?: { git?: boolean; llm?: boolean; ocr?: boolean; reviewed?: boolean }
  /** 旧版顺序完成数（向后兼容），stepStatus 未传时使用。 */
  completedSteps?: number
}>()

const dismissed = ref(localStorage.getItem('onboarding-dismissed') === 'true')

function stepDone(key: keyof NonNullable<typeof props.stepStatus>, ordinal: number): boolean {
  if (props.stepStatus) return Boolean(props.stepStatus[key])
  return (props.completedSteps ?? 0) >= ordinal
}

const steps = computed(() => [
  {
    label: '配置 Git 平台',
    desc: '添加 GitHub/GitLab/Gitea 等 Token，让评审能读取代码',
    done: stepDone('git', 1),
    action: () => router.push('/settings/platforms'),
  },
  {
    label: '配置 LLM 模型',
    desc: '选择服务商和模型，这是评审的核心引擎',
    done: stepDone('llm', 2),
    action: () => router.push('/settings/engine'),
  },
  {
    label: '配置执行参数',
    desc: '设置并发、超时、工具与审查背景',
    done: stepDone('ocr', 3),
    action: () => router.push('/settings/engine'),
  },
  {
    label: '触发第一次评审',
    desc: '创建一个评审或开启 webhook 自动评审',
    done: stepDone('reviewed', 4),
    action: () => router.push('/reviews'),
  },
])

const allDone = computed(() => steps.value.every((s) => s.done))

function dismiss() {
  dismissed.value = true
  localStorage.setItem('onboarding-dismissed', 'true')
}
</script>

<template>
  <el-card v-if="!dismissed && !allDone" class="onboarding-card" shadow="never">
    <template #header>
      <div class="onboarding-header">
        <span class="onboarding-title">🚀 快速上手</span>
        <span class="onboarding-progress">{{ steps.filter(s => s.done).length }} / {{ steps.length }}</span>
        <button class="onboarding-dismiss" @click="dismiss">不再提示</button>
      </div>
    </template>
    <div class="onboarding-steps">
      <div
        v-for="(step, i) in steps"
        :key="i"
        class="onboarding-step"
        :class="{ done: step.done }"
        @click="step.action()"
      >
        <el-icon class="step-icon" :class="{ done: step.done }">
          <CircleCheck v-if="step.done" />
          <span v-else class="step-num">{{ i + 1 }}</span>
        </el-icon>
        <div class="step-content">
          <div class="step-label">{{ step.label }}</div>
          <div class="step-desc">{{ step.desc }}</div>
        </div>
        <el-icon v-if="!step.done" class="step-arrow"><Right /></el-icon>
      </div>
    </div>
  </el-card>
</template>

<style scoped>
.onboarding-card {
  border: 1px solid #e0e7ff;
  background: linear-gradient(135deg, #f0f5ff 0%, #ffffff 100%);
}
.onboarding-header {
  display: flex;
  align-items: center;
  gap: 12px;
}
.onboarding-title {
  font-weight: 600;
  font-size: 15px;
  color: #1f2937;
}
.onboarding-progress {
  font-size: 13px;
  color: #4f46e5;
  font-weight: 600;
}
.onboarding-dismiss {
  margin-left: auto;
  border: none;
  background: none;
  color: #9ca3af;
  font-size: 12px;
  cursor: pointer;
}
.onboarding-dismiss:hover {
  color: #6b7280;
}
.onboarding-steps {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 12px;
}
.onboarding-step {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 14px;
  border: 1px solid #e5e7eb;
  border-radius: 10px;
  background: #fff;
  cursor: pointer;
  transition: all 0.15s;
}
.onboarding-step:hover:not(.done) {
  border-color: #c7d2fe;
  box-shadow: 0 1px 4px rgba(79, 70, 229, 0.08);
}
.onboarding-step.done {
  opacity: 0.7;
  background: #f9fafb;
}
.step-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  flex-shrink: 0;
  border-radius: 50%;
  font-size: 16px;
  color: #d1d5db;
  background: #f3f4f6;
}
.step-icon.done {
  color: #22c55e;
  background: #dcfce7;
}
.step-num {
  font-size: 13px;
  font-weight: 600;
}
.step-content {
  flex: 1;
  min-width: 0;
}
.step-label {
  font-size: 13px;
  font-weight: 600;
  color: #1f2937;
}
.step-desc {
  font-size: 11px;
  color: #6b7280;
  margin-top: 2px;
}
.step-arrow {
  color: #c7d2fe;
  font-size: 14px;
}
</style>
