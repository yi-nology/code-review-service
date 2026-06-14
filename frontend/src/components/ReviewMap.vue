<template>
  <div class="review-map">
    <h3>代码审查地图</h3>
    <div v-if="!map" class="empty">
      暂无审查地图
    </div>
    <div v-else>
      <div class="map-header">
        <h4>{{ map.title }}</h4>
        <p>{{ map.description }}</p>
        <div class="map-stats">
          <span>{{ map.file_count }} 个文件</span>
          <span>{{ map.section_count }} 个部分</span>
        </div>
      </div>
      <div class="map-summary">
        <p>{{ map.summary }}</p>
      </div>
      <div class="sections">
        <div
          v-for="section in map.sections"
          :key="section.id"
          class="section"
        >
          <div class="section-header" @click="toggleSection(section.id)">
            <div class="section-title">
              <el-icon>
                <ArrowRight v-if="!expandedSections.includes(section.id)" />
                <ArrowDown v-else />
              </el-icon>
              <h5>{{ section.title }}</h5>
            </div>
            <div class="section-meta">
              <span>{{ section.files.length }} 个文件</span>
              <span>{{ section.findings.length }} 个发现</span>
              <el-progress
                :percentage="section.progress"
                :stroke-width="4"
                :show-text="false"
                style="width: 100px"
              />
            </div>
          </div>
          <div v-if="expandedSections.includes(section.id)" class="section-content">
            <div class="files">
              <div
                v-for="file in section.files"
                :key="file.path"
                :class="['file', { reviewed: file.reviewed }]"
              >
                <div class="file-info">
                  <span class="file-path">{{ file.path }}</span>
                  <span class="file-language">{{ file.language }}</span>
                </div>
                <div class="file-stats">
                  <span v-if="file.lines_added > 0" class="added">+{{ file.lines_added }}</span>
                  <span v-if="file.lines_removed > 0" class="removed">-{{ file.lines_removed }}</span>
                </div>
              </div>
            </div>
            <div v-if="section.findings.length > 0" class="findings">
              <h6>发现:</h6>
              <ul>
                <li v-for="finding in section.findings" :key="finding">{{ finding }}</li>
              </ul>
            </div>
          </div>
        </div>
      </div>
      <div v-if="map.dependencies && map.dependencies.length > 0" class="dependencies">
        <h4>依赖关系</h4>
        <div class="dependency-list">
          <div
            v-for="dep in map.dependencies"
            :key="`${dep.from}-${dep.to}`"
            class="dependency"
          >
            <span class="from">{{ dep.from }}</span>
            <span class="arrow">→</span>
            <span class="to">{{ dep.to }}</span>
            <span :class="['type', dep.type]">{{ dep.type }}</span>
          </div>
        </div>
      </div>
      <div v-if="mermaidDiagram" class="diagram">
        <h4>依赖图</h4>
        <div class="mermaid" v-html="mermaidDiagram"></div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { ArrowRight, ArrowDown } from '@element-plus/icons-vue'

interface File {
  path: string
  language: string
  lines_added: number
  lines_removed: number
  findings: string[]
  reviewed: boolean
}

interface Section {
  id: string
  title: string
  description: string
  files: File[]
  findings: string[]
  progress: number
  priority: string
}

interface Dependency {
  from: string
  to: string
  type: string
  strength: string
}

interface ReviewMap {
  id: string
  session_id: string
  title: string
  description: string
  sections: Section[]
  dependencies: Dependency[]
  summary: string
  file_count: number
  section_count: number
  created_at: string
}

const props = defineProps<{
  map: ReviewMap | null
}>()

const expandedSections = ref<string[]>([])

const mermaidDiagram = computed(() => {
  if (!props.map || !props.map.dependencies || props.map.dependencies.length === 0) {
    return null
  }

  let diagram = 'graph TD\n'
  const nodes = new Set<string>()

  for (const dep of props.map.dependencies) {
    const fromNode = sanitizeId(getDirectory(dep.from))
    const toNode = sanitizeId(getDirectory(dep.to))

    if (!nodes.has(fromNode)) {
      diagram += `    ${fromNode}[${getDirectory(dep.from)}]\n`
      nodes.add(fromNode)
    }

    if (!nodes.has(toNode)) {
      diagram += `    ${toNode}[${getDirectory(dep.to)}]\n`
      nodes.add(toNode)
    }

    diagram += `    ${fromNode} --> ${toNode}\n`
  }

  return diagram
})

function toggleSection(sectionId: string) {
  const index = expandedSections.value.indexOf(sectionId)
  if (index === -1) {
    expandedSections.value.push(sectionId)
  } else {
    expandedSections.value.splice(index, 1)
  }
}

function sanitizeId(s: string): string {
  return s.replace(/[^a-zA-Z0-9]/g, '_')
}

function getDirectory(path: string): string {
  const parts = path.split('/')
  if (parts.length > 1) {
    return parts.slice(0, -1).join('/')
  }
  return '.'
}
</script>

<style scoped>
.review-map {
  padding: 20px;
  background: #fff;
  border-radius: 8px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.1);
}

.review-map h3 {
  margin: 0 0 20px;
  font-size: 18px;
  color: #303133;
}

.empty {
  padding: 40px;
  text-align: center;
  color: #909399;
  font-size: 14px;
}

.map-header {
  margin-bottom: 20px;
}

.map-header h4 {
  margin: 0 0 8px;
  font-size: 16px;
  color: #303133;
}

.map-header p {
  margin: 0 0 8px;
  font-size: 14px;
  color: #606266;
}

.map-stats {
  display: flex;
  gap: 16px;
  font-size: 13px;
  color: #909399;
}

.map-summary {
  padding: 12px;
  background: #f5f7fa;
  border-radius: 8px;
  margin-bottom: 20px;
}

.map-summary p {
  margin: 0;
  font-size: 14px;
  color: #606266;
}

.sections {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-bottom: 20px;
}

.section {
  background: #f5f7fa;
  border-radius: 8px;
  overflow: hidden;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  cursor: pointer;
  transition: background 0.2s;
}

.section-header:hover {
  background: #ecf5ff;
}

.section-title {
  display: flex;
  align-items: center;
  gap: 8px;
}

.section-title h5 {
  margin: 0;
  font-size: 14px;
  color: #303133;
}

.section-meta {
  display: flex;
  align-items: center;
  gap: 16px;
  font-size: 13px;
  color: #909399;
}

.section-content {
  padding: 16px;
  border-top: 1px solid #e4e7ed;
}

.files {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 16px;
}

.file {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 12px;
  background: #fff;
  border-radius: 4px;
  border-left: 3px solid #dcdfe6;
}

.file.reviewed {
  border-left-color: #67c23a;
}

.file-info {
  display: flex;
  align-items: center;
  gap: 8px;
}

.file-path {
  font-size: 13px;
  color: #303133;
  font-family: monospace;
}

.file-language {
  padding: 2px 6px;
  background: #e4e7ed;
  border-radius: 4px;
  font-size: 11px;
  color: #606266;
}

.file-stats {
  display: flex;
  gap: 8px;
  font-size: 12px;
}

.added {
  color: #67c23a;
}

.removed {
  color: #f56c6c;
}

.findings {
  margin-top: 12px;
}

.findings h6 {
  margin: 0 0 8px;
  font-size: 13px;
  color: #303133;
}

.findings ul {
  margin: 0;
  padding-left: 20px;
}

.findings li {
  font-size: 13px;
  color: #606266;
  margin-bottom: 4px;
}

.dependencies {
  margin-bottom: 20px;
}

.dependencies h4 {
  margin: 0 0 12px;
  font-size: 16px;
  color: #303133;
}

.dependency-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.dependency {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  background: #f5f7fa;
  border-radius: 4px;
  font-size: 13px;
}

.from, .to {
  font-family: monospace;
  color: #303133;
}

.arrow {
  color: #909399;
}

.type {
  padding: 2px 6px;
  background: #e4e7ed;
  border-radius: 4px;
  font-size: 11px;
  color: #606266;
}

.diagram {
  margin-top: 20px;
}

.diagram h4 {
  margin: 0 0 12px;
  font-size: 16px;
  color: #303133;
}

.mermaid {
  padding: 16px;
  background: #f5f7fa;
  border-radius: 8px;
  overflow-x: auto;
}
</style>
