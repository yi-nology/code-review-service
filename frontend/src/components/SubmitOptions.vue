<template>
  <div class="submit-options">
    <div class="option-label">审查引擎</div>
    <div class="engine-cards">
      <button
        v-for="opt in engineOptions"
        :key="opt.value"
        type="button"
        class="engine-card"
        :class="{ active: engine === opt.value, disabled: opt.disabled }"
        :disabled="opt.disabled"
        :title="opt.disabled ? opt.disabledReason : undefined"
        @click="$emit('update:engine', opt.value as any)"
      >
        <el-icon class="engine-icon"><component :is="opt.icon" /></el-icon>
        <strong>{{ opt.title }}</strong>
        <span>{{ opt.disabled ? opt.disabledReason : opt.desc }}</span>
      </button>
    </div>

    <!-- 任务类型：仅 Argus（代码评审 / 需求复核 R1-R5 / 视觉检查） -->
    <div v-if="engine === 'argus'" class="option-label with-gap">任务类型</div>
    <el-radio-group
      v-if="engine === 'argus'"
      :model-value="taskKind"
      @update:model-value="$emit('update:taskKind', $event as any)"
    >
      <el-radio value="code_review">
        代码评审
        <span class="radio-hint">多专家插件评审 + 真值表裁决</span>
      </el-radio>
      <el-radio value="requirement_compliance">
        需求复核
        <span class="radio-hint">R1-R5 需求完成度判定（可在高级选项填验收清单）</span>
      </el-radio>
      <el-radio value="visual_check">
        视觉检查
        <span class="radio-hint">需 argus 侧启用 visual-check 插件</span>
      </el-radio>
    </el-radio-group>

    <!-- 提交方式：Argus 为异步回报引擎，无同步语义，整行隐藏而不是禁用 -->
    <div v-if="engine !== 'argus'" class="option-label with-gap">提交方式</div>
    <el-radio-group
      v-if="engine !== 'argus'"
      :model-value="mode"
      @update:model-value="$emit('update:mode', $event as any)"
    >
      <el-radio value="async">
        异步提交
        <span class="radio-hint">提交后立即返回，后台执行</span>
      </el-radio>
      <el-radio value="sync">
        同步等待
        <span class="radio-hint">等待审查完成后跳转详情</span>
      </el-radio>
    </el-radio-group>

    <el-collapse
      class="advanced-collapse"
      :model-value="advanced"
      @update:model-value="$emit('update:advanced', $event as string[])"
    >
      <el-collapse-item name="advanced">
        <template #title>
          <span class="advanced-title">高级选项</span>
          <el-tag
            v-if="requirements.trim() || ephemeralReviewers.length"
            size="small"
            type="success"
            class="adv-badge"
          >
            已配置补充
          </el-tag>
        </template>
        <template v-if="engine !== 'argus'">
          <el-form-item label="审查需求">
            <el-input
              :model-value="requirements"
              type="textarea"
              :rows="3"
              placeholder="逐行填写审查需求，例如：&#10;支持分页查询&#10;所有错误需返回统一格式"
              @update:model-value="$emit('update:requirements', $event)"
            />
            <span class="field-hint">每行一条，会作为本次审查的额外关注点</span>
          </el-form-item>
          <el-form-item label="临时审查员">
            <div class="ephemeral-list">
              <div
                v-for="(rev, idx) in ephemeralReviewers"
                :key="idx"
                class="ephemeral-item"
              >
                <el-input
                  :model-value="rev.name"
                  placeholder="名称（可选）"
                  class="ephemeral-name"
                  @update:model-value="updateReviewer(idx, 'name', $event)"
                />
                <el-input
                  :model-value="rev.background"
                  type="textarea"
                  :rows="2"
                  placeholder="审查关注背景，例如：重点关注并发安全"
                  class="ephemeral-bg"
                  @update:model-value="updateReviewer(idx, 'background', $event)"
                />
                <el-button
                  :icon="Delete"
                  circle
                  plain
                  @click="$emit('removeReviewer', idx)"
                />
              </div>
              <el-button
                :icon="Plus"
                plain
                size="small"
                @click="$emit('addReviewer')"
              >
                添加临时审查员
              </el-button>
            </div>
          </el-form-item>
        </template>
        <template v-else>
          <!-- Argus：需求清单经 artifacts 注入，需求复核任务将其作为验收标准 -->
          <el-form-item label="需求清单">
            <el-input
              :model-value="requirements"
              type="textarea"
              :rows="3"
              placeholder="逐行填写验收标准 / 需求项（可选），例如：&#10;支持分页查询&#10;所有错误需返回统一格式"
              @update:model-value="$emit('update:requirements', $event)"
            />
            <span class="field-hint">每行一条；需求复核任务据此逐项判定完成度，代码评审任务作为补充关注点</span>
          </el-form-item>
        </template>
        <el-form-item label="幂等键">
          <el-input
            :model-value="idempotencyKey"
            placeholder="pipeline:job:sha (可选)"
            @update:model-value="$emit('update:idempotencyKey', $event)"
          />
        </el-form-item>
        <!-- 回调模式：Argus 由 Argus 侧自行回写，该选项无意义，隐藏 -->
        <el-form-item v-if="engine !== 'argus'" label="回调模式">
          <el-select
            :model-value="callbackMode"
            style="width: 100%"
            @update:model-value="$emit('update:callbackMode', $event)"
          >
            <el-option label="默认 (comment)" value="comment" />
            <el-option label="无回调" value="none" />
          </el-select>
        </el-form-item>
      </el-collapse-item>
    </el-collapse>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { Aim, Cpu, View, Delete, Plus } from '@element-plus/icons-vue'
import type { EphemeralReviewer } from '@/types/review'

const props = defineProps<{
  /** '' = 跟随项目策略（不发送 engine）；'ocr' | 'argus' = 显式指定 */
  engine: '' | 'ocr' | 'argus'
  mode: 'async' | 'sync'
  advanced: string[]
  idempotencyKey: string
  callbackMode: string
  requirements: string
  ephemeralReviewers: EphemeralReviewer[]
  /** engine=argus 时的任务类型 */
  taskKind?: string
  /** 当前模式下 Argus 不可用（分支对比无 PR 号/平台不支持）时禁用 Argus 卡片 */
  argusBlocked?: boolean
  argusBlockReason?: string
}>()

const emit = defineEmits<{
  (e: 'update:engine', v: '' | 'ocr' | 'argus'): void
  (e: 'update:mode', v: 'async' | 'sync'): void
  (e: 'update:advanced', v: string[]): void
  (e: 'update:idempotencyKey', v: string): void
  (e: 'update:callbackMode', v: string): void
  (e: 'update:requirements', v: string): void
  (e: 'update:ephemeralReviewers', v: EphemeralReviewer[]): void
  (e: 'update:taskKind', v: string): void
  (e: 'addReviewer'): void
  (e: 'removeReviewer', idx: number): void
}>()

const engineOptions = computed(() => [
  {
    value: '' as const,
    title: '跟随项目策略',
    desc: '不显式指定，由该仓库的项目策略决定',
    icon: Aim,
    disabled: false,
    disabledReason: '',
  },
  {
    value: 'ocr' as const,
    title: 'OCR 多智能体',
    desc: '本次显式走本服务多 agent 工作流',
    icon: Cpu,
    disabled: false,
    disabledReason: '',
  },
  {
    value: 'argus' as const,
    title: 'Argus Bot',
    desc: '派发外部 Argus 审查，完成后自动回报',
    icon: View,
    disabled: props.argusBlocked === true,
    disabledReason: props.argusBlockReason || '当前模式不支持 Argus',
  },
])

function updateReviewer(idx: number, field: 'name' | 'background', value: string) {
  const next = props.ephemeralReviewers.map((r, i) => (i === idx ? { ...r, [field]: value } : r))
  emit('update:ephemeralReviewers', next)
}
</script>

<style scoped>
.option-label {
  color: #606266;
  font-size: 13px;
  font-weight: 600;
  margin-bottom: 8px;
}
.option-label.with-gap {
  margin-top: 16px;
}

/* 引擎卡片：与弹窗顶部模式卡同一视觉语言 */
.engine-cards {
  display: grid;
  gap: 10px;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  margin-bottom: 4px;
}

.engine-card {
  align-items: flex-start;
  background: #f7f8fc;
  border: 1px solid #e4e7ed;
  border-radius: 8px;
  color: #303133;
  cursor: pointer;
  display: grid;
  gap: 4px;
  grid-template-columns: auto 1fr;
  padding: 12px;
  text-align: left;
  transition: all 0.18s ease;
}

.engine-card .engine-icon {
  color: var(--app-primary, #5b5bd6);
  font-size: 18px;
  grid-row: span 2;
  margin-top: 1px;
}

.engine-card strong {
  font-size: 13.5px;
}

.engine-card span {
  color: #707782;
  font-size: 12px;
  line-height: 1.4;
}

.engine-card.active {
  background: #eef0ff;
  border-color: var(--app-primary, #5b5bd6);
  box-shadow: 0 8px 22px rgba(91, 91, 214, 0.14);
}

.engine-card.disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

.radio-hint {
  font-size: 12px;
  color: #909399;
  margin-left: 4px;
}
.advanced-title {
  font-size: 14px;
  color: var(--app-primary, #5b5bd6);
}
.adv-badge {
  margin-left: 8px;
}
.field-hint {
  display: block;
  margin-top: 4px;
  color: #909399;
  font-size: 11px;
}
.ephemeral-list {
  width: 100%;
}
.ephemeral-item {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 8px;
  margin-bottom: 8px;
}
.ephemeral-name {
  margin-bottom: 4px;
}
.advanced-collapse {
  margin-top: 14px;
}
:deep(.el-collapse-item__header) {
  font-size: 14px;
}
:deep(.el-collapse-item__content) {
  padding-bottom: 0;
}

@media (max-width: 760px) {
  .engine-cards {
    grid-template-columns: 1fr;
  }
}
</style>
