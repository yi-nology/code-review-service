<template>
  <div class="rule-editor">
    <el-empty v-if="rules.length === 0" :description="emptyDescription">
      <div class="empty-actions">
        <el-button type="primary" @click="addRule">
          <el-icon><Plus /></el-icon>
          {{ addLabel }}
        </el-button>
        <el-button v-if="templates.length > 0" plain @click="applyTemplate(templates[0])">
          使用推荐模板
        </el-button>
      </div>
    </el-empty>

    <div v-else class="rule-list">
      <div
        v-for="(rule, idx) in rules"
        :key="idx"
        class="rule-item"
        :class="{ invalid: getRuleIssue(idx) }"
      >
        <div class="rule-index">{{ idx + 1 }}</div>
        <div class="rule-fields">
          <div class="field-row">
            <label>适用文件</label>
            <el-input
              :model-value="rule.path"
              placeholder="**/* 或 **/*.go"
              @update:model-value="updateRule(idx, 'path', String($event))"
            />
          </div>
          <div class="field-row">
            <label>审查要求</label>
            <el-input
              :model-value="rule.rule"
              type="textarea"
              :autosize="{ minRows: 2, maxRows: 5 }"
              placeholder="写清楚模型应该重点检查什么，以及什么情况需要指出问题"
              @update:model-value="updateRule(idx, 'rule', String($event))"
            />
          </div>
          <div v-if="getRuleIssue(idx)" class="rule-error">{{ getRuleIssue(idx) }}</div>
        </div>
        <div class="rule-actions">
          <el-button text :disabled="idx === 0" @click="moveRule(idx, -1)">
            <el-icon><ArrowUp /></el-icon>
          </el-button>
          <el-button text :disabled="idx === rules.length - 1" @click="moveRule(idx, 1)">
            <el-icon><ArrowDown /></el-icon>
          </el-button>
          <el-button text type="danger" @click="removeRule(idx)">
            <el-icon><Delete /></el-icon>
          </el-button>
        </div>
      </div>

      <div class="editor-footer">
        <el-button size="small" plain @click="addRule">
          <el-icon><Plus /></el-icon>
          {{ addLabel }}
        </el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { ArrowDown, ArrowUp, Delete, Plus } from '@element-plus/icons-vue'
import type { ReviewRule } from '@/types/config'

export type RuleTemplate = ReviewRule & { name: string }

const props = withDefaults(defineProps<{
  modelValue: ReviewRule[]
  issues?: string[]
  templates?: RuleTemplate[]
  emptyDescription?: string
  addLabel?: string
}>(), {
  issues: () => [],
  templates: () => [],
  emptyDescription: '暂无规则',
  addLabel: '添加规则',
})

const emit = defineEmits<{
  (event: 'update:modelValue', value: ReviewRule[]): void
  (event: 'apply-template', value: RuleTemplate): void
}>()

const rules = computed(() => props.modelValue || [])
const templates = computed(() => props.templates || [])

function cloneRules() {
  return rules.value.map((rule) => ({ ...rule }))
}

function addRule() {
  emit('update:modelValue', [...cloneRules(), { path: '**/*', rule: '' }])
}

function applyTemplate(template: RuleTemplate) {
  emit('update:modelValue', [...cloneRules(), { path: template.path, rule: template.rule }])
  emit('apply-template', template)
}

function updateRule(index: number, key: keyof ReviewRule, value: string) {
  const next = cloneRules()
  next[index] = { ...next[index], [key]: value }
  emit('update:modelValue', next)
}

function removeRule(index: number) {
  const next = cloneRules()
  next.splice(index, 1)
  emit('update:modelValue', next)
}

function moveRule(index: number, offset: -1 | 1) {
  const target = index + offset
  if (target < 0 || target >= rules.value.length) return
  const next = cloneRules()
  const [item] = next.splice(index, 1)
  next.splice(target, 0, item)
  emit('update:modelValue', next)
}

function getRuleIssue(index: number) {
  const prefix = `第 ${index + 1} 条`
  return props.issues.find((issue) => issue.startsWith(prefix)) || ''
}
</script>

<style scoped>
.rule-list {
  display: grid;
  gap: 12px;
}

.rule-item {
  display: grid;
  grid-template-columns: 34px minmax(0, 1fr) 42px;
  gap: 12px;
  padding: 14px;
  border: 1px solid #edf0f5;
  border-radius: 8px;
  background: #fff;
}

.rule-item.invalid {
  border-color: #ffd8dd;
  background: #fffafb;
}

.rule-index {
  display: grid;
  width: 30px;
  height: 30px;
  place-items: center;
  border-radius: 8px;
  color: #555fc9;
  background: #f0f1ff;
  font-size: 12px;
  font-weight: 800;
}

.rule-fields {
  min-width: 0;
}

.field-row {
  display: grid;
  grid-template-columns: 76px minmax(0, 1fr);
  gap: 10px;
  align-items: start;
  margin-bottom: 10px;
}

.field-row:last-child {
  margin-bottom: 0;
}

.field-row label {
  padding-top: 7px;
  color: #4b5568;
  font-size: 12px;
  font-weight: 700;
}

.rule-error {
  margin-top: 8px;
  color: var(--app-danger);
  font-size: 12px;
}

.rule-actions {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
}

.editor-footer {
  display: flex;
  justify-content: flex-start;
}

.empty-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 8px;
}

@media (max-width: 720px) {
  .rule-item {
    grid-template-columns: 1fr;
  }

  .rule-actions {
    flex-direction: row;
    justify-content: flex-end;
  }

  .field-row {
    grid-template-columns: 1fr;
    gap: 5px;
  }

  .field-row label {
    padding-top: 0;
  }
}
</style>
