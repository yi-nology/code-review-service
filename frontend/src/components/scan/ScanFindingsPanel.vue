<script setup lang="ts">
import { ref, computed } from 'vue'
import type { Finding } from '@/types/review'

const props = defineProps<{
  findings: Finding[]
}>()

const severityFilter = ref<string[]>([])
const groupByPath = ref(false)
const expandedGroups = ref<string[]>([])
const findingsPage = ref(1)
const findingsPageSize = 20

const filteredFindings = computed(() => {
  if (!severityFilter.value.length) return props.findings
  return props.findings.filter((f) => severityFilter.value.includes(f.severity))
})

const pagedFindings = computed(() => {
  const start = (findingsPage.value - 1) * findingsPageSize
  return filteredFindings.value.slice(start, start + findingsPageSize)
})

const groupedFindings = computed(() => {
  const groups: Record<string, Finding[]> = {}
  for (const f of filteredFindings.value) {
    const key = f.path || '(unknown)'
    if (!groups[key]) groups[key] = []
    groups[key].push(f)
  }
  return Object.entries(groups)
    .map(([path, items]) => ({ path, items }))
    .sort((a, b) => b.items.length - a.items.length)
})

function severityType(severity?: string) {
  if (severity === 'critical') return 'danger'
  if (severity === 'major') return 'warning'
  if (severity === 'minor') return 'info'
  return 'success'
}
</script>

<template>
  <div class="findings-section">
    <div class="section-head">
      <h2>问题列表</h2>
      <div class="section-actions">
        <el-select
          v-model="severityFilter"
          multiple
          collapse-tags
          collapse-tags-tooltip
          placeholder="严重级别"
          size="small"
          style="width: 200px"
          clearable
        >
          <el-option label="Critical" value="critical" />
          <el-option label="Major" value="major" />
          <el-option label="Minor" value="minor" />
          <el-option label="Info" value="info" />
        </el-select>
        <el-select v-model="groupByPath" size="small" style="width: 120px">
          <el-option label="不分组" :value="false" />
          <el-option label="按文件" :value="true" />
        </el-select>
      </div>
    </div>

    <el-empty v-if="!filteredFindings.length" description="未发现问题" :image-size="80" />

    <template v-else-if="groupByPath">
      <el-collapse v-model="expandedGroups" class="finding-groups">
        <el-collapse-item v-for="group in groupedFindings" :key="group.path" :name="group.path">
          <template #title>
            <code class="group-path">{{ group.path }}</code>
            <el-badge :value="group.items.length" class="group-badge" />
          </template>
          <div class="finding-cards">
            <article v-for="(finding, idx) in group.items" :key="idx" class="finding-card">
              <div class="finding-head">
                <el-tag :type="severityType(finding.severity)" size="small">{{ finding.severity }}</el-tag>
                <strong>{{ finding.title }}</strong>
                <span v-if="finding.start_line" class="finding-line">L{{ finding.start_line }}</span>
              </div>
              <p class="finding-body">{{ finding.body }}</p>
              <div v-if="finding.existing_code || finding.suggestion_code" class="finding-code">
                <div v-if="finding.existing_code" class="code-block">
                  <small>当前代码</small>
                  <pre>{{ finding.existing_code }}</pre>
                </div>
                <div v-if="finding.suggestion_code" class="code-block">
                  <small>建议代码</small>
                  <pre>{{ finding.suggestion_code }}</pre>
                </div>
              </div>
            </article>
          </div>
        </el-collapse-item>
      </el-collapse>
    </template>

    <el-table v-else :data="pagedFindings" stripe>
      <el-table-column type="expand">
        <template #default="{ row }">
          <p class="finding-body">{{ row.body }}</p>
          <div v-if="row.existing_code || row.suggestion_code" class="finding-code">
            <div v-if="row.existing_code" class="code-block">
              <small>当前代码</small>
              <pre>{{ row.existing_code }}</pre>
            </div>
            <div v-if="row.suggestion_code" class="code-block">
              <small>建议代码</small>
              <pre>{{ row.suggestion_code }}</pre>
            </div>
          </div>
        </template>
      </el-table-column>
      <el-table-column width="100" label="级别">
        <template #default="{ row }">
          <el-tag :type="severityType(row.severity)" size="small">{{ row.severity }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="标题" min-width="200" show-overflow-tooltip>
        <template #default="{ row }"><strong>{{ row.title }}</strong></template>
      </el-table-column>
      <el-table-column label="文件" min-width="200" show-overflow-tooltip>
        <template #default="{ row }">
          <code class="finding-path">{{ row.path }}{{ row.start_line ? `:${row.start_line}` : '' }}</code>
        </template>
      </el-table-column>
    </el-table>

    <div v-if="filteredFindings.length > findingsPageSize" class="pagination-wrap">
      <el-pagination
        v-model:current-page="findingsPage"
        :page-size="findingsPageSize"
        :total="filteredFindings.length"
        layout="prev, pager, next"
        small
      />
    </div>
  </div>
</template>

<style scoped>
.findings-section {
  margin-top: 20px;
}
.section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}
.section-head h2 {
  font-size: 16px;
  font-weight: 600;
  color: #1f2937;
  margin: 0;
}
.section-actions {
  display: flex;
  gap: 8px;
}
.finding-groups {
  border: 1px solid var(--app-border);
  border-radius: 8px;
}
.group-path {
  font-size: 13px;
  color: #4f46e5;
  margin-right: 8px;
}
.group-badge {
  margin-left: 4px;
}
.finding-cards {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 4px 0;
}
.finding-card {
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  padding: 14px;
  background: #fff;
}
.finding-head {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
}
.finding-head strong {
  font-size: 14px;
  color: #1f2937;
}
.finding-line {
  font-size: 12px;
  color: #9ca3af;
  margin-left: auto;
}
.finding-body {
  font-size: 13px;
  color: #6b7280;
  line-height: 1.6;
  margin: 0 0 8px;
}
.finding-code {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}
.code-block {
  flex: 1;
  min-width: 200px;
}
.code-block small {
  display: block;
  font-size: 11px;
  color: #9ca3af;
  margin-bottom: 4px;
}
.code-block pre {
  margin: 0;
  padding: 10px;
  background: #f5f5f5;
  border-radius: 6px;
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-all;
}
.finding-path {
  font-size: 12px;
  color: #6b7280;
}
.pagination-wrap {
  margin-top: 16px;
  display: flex;
  justify-content: center;
}
</style>
