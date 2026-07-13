<template>
  <div class="rule-package-picker">
    <el-empty v-if="!loading && availablePackages.length === 0" :description="emptyDescription" :image-size="48" />

    <div v-loading="loading" class="picker-list">
      <label
        v-for="pkg in availablePackages"
        :key="pkg.id"
        class="picker-item"
        :class="{ checked: isSelected(pkg) }"
      >
        <el-checkbox
          :model-value="isSelected(pkg)"
          @change="(val: boolean) => toggle(pkg, val)"
        />
        <span class="pkg-main">
          <span class="pkg-head">
            <strong>{{ pkg.name }}</strong>
            <el-tag
              v-for="lang in (pkg.languages || []).slice(0, 3)"
              :key="lang"
              size="small"
              effect="plain"
              class="lang-tag"
            >{{ lang }}</el-tag>
            <el-tag v-if="pkg.built_in" size="small" type="info" effect="plain">内置</el-tag>
          </span>
          <small v-if="pkg.description" class="pkg-desc">{{ pkg.description }}</small>
          <span class="pkg-meta">
            <em>{{ (pkg.rules || []).length }} 条规则</em>
            <span v-if="!pkg.enabled" class="pkg-off">未启用</span>
          </span>
        </span>
      </label>
    </div>

    <p v-if="availablePackages.length > 0" class="picker-hint">
      {{ hint }}
    </p>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { listRulePackages } from '@/api/config'
import type { OCRRulePackage } from '@/types/config'

const props = withDefaults(defineProps<{
  modelValue: number[]
  /** 只展示哪些 scope 的包；默认 project（可引用），global 的始终合并、无需引用。 */
  scopes?: Array<'global' | 'project'>
  emptyDescription?: string
  hint?: string
}>(), {
  scopes: () => ['project'] as Array<'global' | 'project'>,
  emptyDescription: '暂无可引用的规则包',
  hint: '引用的规则包会与全局规则集一起合并到本次审查；规则包内容修改后自动生效，无需同步副本。',
})

const emit = defineEmits<{
  (event: 'update:modelValue', value: number[]): void
}>()

const loading = ref(true)
const packages = ref<OCRRulePackage[]>([])

const availablePackages = computed(() => {
  const scopeSet = new Set(props.scopes)
  return packages.value.filter((pkg) => {
    // scope 空值视为 project（与后端一致）。
    const scope = pkg.scope || 'project'
    return scopeSet.has(scope as 'global' | 'project')
  })
})

function isSelected(pkg: OCRRulePackage) {
  return pkg.id != null && props.modelValue.includes(pkg.id)
}

function toggle(pkg: OCRRulePackage, checked: boolean) {
  if (pkg.id == null) return
  const next = new Set(props.modelValue)
  if (checked) next.add(pkg.id)
  else next.delete(pkg.id)
  emit('update:modelValue', [...next])
}

async function fetchPackages() {
  loading.value = true
  try {
    packages.value = await listRulePackages()
  } catch (e) {
    console.error('Failed to load rule packages:', e)
  } finally {
    loading.value = false
  }
}

onMounted(fetchPackages)
</script>

<style scoped>
.picker-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.picker-item {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 12px;
  border: 1px solid #e6ebf2;
  border-radius: 10px;
  background: #fff;
  cursor: pointer;
  transition: 160ms ease;
}

.picker-item:hover {
  border-color: #9db1ff;
  background: #fbfbff;
}

.picker-item.checked {
  border-color: var(--app-primary);
  background: #f5f7ff;
}

.pkg-main {
  flex: 1;
  min-width: 0;
}

.pkg-head {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.pkg-head strong {
  font-size: 13px;
  color: var(--app-text);
}

.lang-tag {
  flex-shrink: 0;
}

.pkg-desc {
  display: block;
  margin-top: 5px;
  color: var(--app-muted);
  font-size: 12px;
  line-height: 1.5;
}

.pkg-meta {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 6px;
}

.pkg-meta em {
  color: var(--app-primary);
  font-size: 11px;
  font-style: normal;
}

.pkg-off {
  color: var(--app-danger);
  font-size: 11px;
}

.picker-hint {
  margin: 10px 0 0;
  color: var(--app-muted);
  font-size: 12px;
  line-height: 1.6;
}
</style>
