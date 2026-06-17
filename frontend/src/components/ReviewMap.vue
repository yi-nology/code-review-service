<template>
  <div class="review-map">
    <div v-if="!map" class="empty">
      <el-empty description="暂无审查地图" />
    </div>
    <div v-else class="map-shell">
      <section class="map-hero">
        <div class="hero-copy">
          <div class="eyebrow">
            <el-icon><Aim /></el-icon>
            <span>代码审查地图</span>
          </div>
          <h3>{{ map.title }}</h3>
          <p>{{ map.description || map.summary }}</p>
        </div>
        <div class="hero-score">
          <div class="score-ring" :style="{ '--progress': `${overallProgress * 3.6}deg` }">
            <span>{{ overallProgress }}</span>
            <small>%</small>
          </div>
          <div class="score-meta">
            <strong>{{ reviewedCount }}/{{ totalFiles }}</strong>
            <span>文件已审查</span>
          </div>
        </div>
      </section>

      <section class="signal-grid">
        <div class="signal-tile">
          <el-icon><Files /></el-icon>
          <strong>{{ map.file_count }}</strong>
          <span>变更文件</span>
        </div>
        <div class="signal-tile">
          <el-icon><DataAnalysis /></el-icon>
          <strong>{{ map.section_count }}</strong>
          <span>审查区域</span>
        </div>
        <div class="signal-tile">
          <el-icon><Connection /></el-icon>
          <strong>{{ dependencyCount }}</strong>
          <span>依赖通道</span>
        </div>
        <div class="signal-tile hot">
          <el-icon><Warning /></el-icon>
          <strong>{{ findingCount }}</strong>
          <span>重点发现</span>
        </div>
      </section>

      <section v-if="map.summary" class="map-summary">
        <div class="summary-pulse"></div>
        <p>{{ map.summary }}</p>
      </section>

      <section v-if="hasDependencies" class="dependency-stage">
        <div class="stage-header">
          <div>
            <div class="eyebrow">
              <el-icon><Lightning /></el-icon>
              <span>依赖关系</span>
            </div>
            <h4>依赖图</h4>
          </div>
          <div class="type-filter">
            <button
              v-for="type in dependencyFilters"
              :key="type"
              :class="{ active: activeDependencyType === type }"
              type="button"
              @click="activeDependencyType = type"
            >
              {{ type === 'all' ? '全部' : type }}
            </button>
          </div>
        </div>

        <div class="graph-command">
          <div class="graph-panel">
            <svg
              class="dependency-svg"
              viewBox="0 0 920 520"
              role="img"
              aria-label="依赖图"
              @mouseleave="activeNodeId = null"
            >
              <defs>
                <pattern id="review-map-grid" width="46" height="46" patternUnits="userSpaceOnUse">
                  <path d="M 46 0 L 0 0 0 46" fill="none" stroke="rgba(255,255,255,.08)" stroke-width="1" />
                </pattern>
                <filter id="node-glow" x="-40%" y="-40%" width="180%" height="180%">
                  <feGaussianBlur stdDeviation="5" result="blur" />
                  <feMerge>
                    <feMergeNode in="blur" />
                    <feMergeNode in="SourceGraphic" />
                  </feMerge>
                </filter>
                <marker id="edge-arrow" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
                  <path d="M 0 0 L 10 5 L 0 10 z" fill="currentColor" />
                </marker>
              </defs>

              <rect width="920" height="520" rx="18" fill="#111318" />
              <rect width="920" height="520" rx="18" fill="url(#review-map-grid)" />
              <path
                v-for="ring in radarRings"
                :key="ring.r"
                class="radar-ring"
                :d="`M ${460 - ring.r} 260 a ${ring.r} ${ring.r} 0 1 0 ${ring.r * 2} 0 a ${ring.r} ${ring.r} 0 1 0 -${ring.r * 2} 0`"
              />

              <g class="edges">
                <path
                  v-for="edge in dependencyEdges"
                  :key="edge.id"
                  :class="['edge-line', edge.typeClass, { muted: isEdgeMuted(edge) }]"
                  :d="edge.path"
                  marker-end="url(#edge-arrow)"
                />
              </g>

              <g
                v-for="node in dependencyNodes"
                :key="node.id"
                :class="['graph-node', { active: activeNodeId === node.id, dimmed: activeNodeId && activeNodeId !== node.id && !node.connectedToActive }]"
                :transform="`translate(${node.x} ${node.y})`"
                @mouseenter="activeNodeId = node.id"
                @click="emit('node-click', node.id)"
              >
                <circle class="node-halo" :r="node.radius + 12" />
                <circle class="node-core" :r="node.radius" :style="{ '--node-color': node.color }" />
                <text class="node-label" y="-28">{{ node.label }}</text>
                <text class="node-subtitle" y="40">{{ node.weight }} 连接</text>
              </g>
            </svg>
          </div>

          <aside class="graph-sidebar">
            <div class="sidebar-block">
              <div class="sidebar-title">
                <el-icon><Compass /></el-icon>
                <span>核心枢纽</span>
              </div>
              <div class="impact-list">
                <button
                  v-for="node in topImpactNodes"
                  :key="node.id"
                  type="button"
                  class="impact-row"
                  @mouseenter="activeNodeId = node.id"
                  @mouseleave="activeNodeId = null"
                  @click="emit('node-click', node.id)"
                >
                  <span class="impact-dot" :style="{ background: node.color }"></span>
                  <span class="impact-name">{{ node.label }}</span>
                  <strong>{{ node.weight }}</strong>
                </button>
              </div>
            </div>

            <div class="sidebar-block">
              <div class="sidebar-title">
                <el-icon><TrendCharts /></el-icon>
                <span>依赖强度</span>
              </div>
              <div class="strength-stack">
                <div v-for="item in strengthStats" :key="item.name" class="strength-row">
                  <span>{{ item.name }}</span>
                  <div class="strength-bar">
                    <i :style="{ width: `${item.percent}%` }"></i>
                  </div>
                  <strong>{{ item.count }}</strong>
                </div>
              </div>
            </div>
          </aside>
        </div>
      </section>

      <section class="sections">
        <div
          v-for="section in sortedSections"
          :key="section.id"
          :id="`section-${section.id}`"
          :class="['section', `priority-${section.priority || 'normal'}`]"
        >
          <div class="section-header" @click="toggleSection(section.id)">
            <div class="section-title">
              <el-icon>
                <ArrowRight v-if="!expandedSections.includes(section.id)" />
                <ArrowDown v-else />
              </el-icon>
              <div>
                <h5>{{ section.title }}</h5>
                <p>{{ section.description }}</p>
              </div>
            </div>
            <div class="section-meta">
              <span>{{ getSectionReviewedCount(section) }}/{{ section.files.length }} 文件</span>
              <span v-if="section.findings.length > 0">{{ section.findings.length }} 发现</span>
              <el-progress
                :percentage="getSectionProgress(section)"
                :stroke-width="5"
                :show-text="false"
                style="width: 110px"
              />
              <el-button size="small" @click.stop="toggleAllFiles(section)">
                {{ areAllFilesReviewed(section) ? '取消全选' : '全选' }}
              </el-button>
            </div>
          </div>
          <div v-if="expandedSections.includes(section.id)" class="section-content">
            <div class="files">
              <div
                v-for="file in section.files"
                :key="file.path"
                :class="['file', { reviewed: file.reviewed }]"
              >
                <el-checkbox
                  v-model="file.reviewed"
                  @change="(val: boolean) => onFileReviewedChange(section.id, file.path, val)"
                />
                <div class="file-icon">
                  <el-icon><Document /></el-icon>
                </div>
                <div class="file-info">
                  <span class="file-path">{{ file.path }}</span>
                  <span class="file-language">{{ file.language || 'text' }}</span>
                </div>
                <div class="file-stats">
                  <span v-if="file.lines_added > 0" class="added">+{{ file.lines_added }}</span>
                  <span v-if="file.lines_removed > 0" class="removed">-{{ file.lines_removed }}</span>
                </div>
              </div>
            </div>
            <div v-if="section.findings.length > 0" class="findings">
              <h6>发现</h6>
              <ul>
                <li v-for="finding in section.findings" :key="finding">{{ finding }}</li>
              </ul>
            </div>
          </div>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import {
  Aim,
  ArrowDown,
  ArrowRight,
  Compass,
  Connection,
  DataAnalysis,
  Document,
  Files,
  Lightning,
  TrendCharts,
  Warning,
} from '@element-plus/icons-vue'

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
  review_id?: string
  session_id: string
  title: string
  description: string
  sections: Section[]
  dependencies: Dependency[]
  summary: string
  file_count: number
  section_count: number
  created_at: string
  mermaid?: string
}

interface GraphNode {
  id: string
  label: string
  x: number
  y: number
  radius: number
  weight: number
  color: string
  connectedToActive: boolean
}

interface GraphEdge {
  id: string
  from: string
  to: string
  path: string
  type: string
  typeClass: string
  strength: string
}

const props = defineProps<{
  map: ReviewMap | null
}>()

const emit = defineEmits<{
  (e: 'file-reviewed', sectionId: string, filePath: string, reviewed: boolean): void
  (e: 'section-reviewed', sectionId: string, reviewed: boolean): void
  (e: 'node-click', nodeId: string): void
}>()

const expandedSections = ref<string[]>([])
const activeDependencyType = ref('all')
const activeNodeId = ref<string | null>(null)
const radarRings = [{ r: 80 }, { r: 150 }, { r: 220 }]

const totalFiles = computed(() => props.map?.sections.reduce((sum, section) => sum + section.files.length, 0) || 0)

const reviewedCount = computed(() => {
  if (!props.map) return 0
  return props.map.sections.reduce((sum, section) => sum + section.files.filter((file) => file.reviewed).length, 0)
})

const findingCount = computed(() => props.map?.sections.reduce((sum, section) => sum + section.findings.length, 0) || 0)
const dependencyCount = computed(() => props.map?.dependencies?.length || 0)
const hasDependencies = computed(() => dependencyCount.value > 0)

const overallProgress = computed(() => {
  if (!totalFiles.value) return 0
  return Math.round((reviewedCount.value / totalFiles.value) * 100)
})

const dependencyFilters = computed(() => {
  const types = new Set((props.map?.dependencies || []).map((dep) => dep.type || 'related'))
  return ['all', ...Array.from(types)]
})

const visibleDependencies = computed(() => {
  const dependencies = props.map?.dependencies || []
  if (activeDependencyType.value === 'all') return dependencies
  return dependencies.filter((dep) => (dep.type || 'related') === activeDependencyType.value)
})

const sortedSections = computed(() => {
  const order: Record<string, number> = { critical: 0, high: 1, medium: 2, low: 3 }
  return [...(props.map?.sections || [])].sort((a, b) => (order[a.priority] ?? 2) - (order[b.priority] ?? 2))
})

const nodeStats = computed(() => {
  const stats = new Map<string, { id: string; label: string; weight: number; incoming: number; outgoing: number }>()
  for (const dep of visibleDependencies.value) {
    const from = normalizeNode(dep.from)
    const to = normalizeNode(dep.to)
    if (!stats.has(from.id)) stats.set(from.id, { ...from, weight: 0, incoming: 0, outgoing: 0 })
    if (!stats.has(to.id)) stats.set(to.id, { ...to, weight: 0, incoming: 0, outgoing: 0 })
    stats.get(from.id)!.weight += strengthScore(dep.strength)
    stats.get(from.id)!.outgoing += 1
    stats.get(to.id)!.weight += strengthScore(dep.strength)
    stats.get(to.id)!.incoming += 1
  }
  return stats
})

const activeConnections = computed(() => {
  const ids = new Set<string>()
  if (!activeNodeId.value) return ids
  ids.add(activeNodeId.value)
  for (const dep of visibleDependencies.value) {
    const from = normalizeNode(dep.from).id
    const to = normalizeNode(dep.to).id
    if (from === activeNodeId.value) ids.add(to)
    if (to === activeNodeId.value) ids.add(from)
  }
  return ids
})

const dependencyNodes = computed<GraphNode[]>(() => {
  const nodes = Array.from(nodeStats.value.values()).sort((a, b) => b.weight - a.weight).slice(0, 18)
  const total = Math.max(nodes.length, 1)
  const maxWeight = Math.max(...nodes.map((node) => node.weight), 1)

  return nodes.map((node, index) => {
    const angle = (Math.PI * 2 * index) / total - Math.PI / 2
    const ring = 120 + (index % 3) * 55 + Math.min(30, node.weight * 2)
    const connectedToActive = !activeNodeId.value || activeConnections.value.has(node.id)
    return {
      id: node.id,
      label: node.label,
      x: 460 + Math.cos(angle) * ring,
      y: 260 + Math.sin(angle) * ring,
      radius: 13 + Math.round((node.weight / maxWeight) * 15),
      weight: node.incoming + node.outgoing,
      color: nodeColor(index),
      connectedToActive,
    }
  })
})

const dependencyEdges = computed<GraphEdge[]>(() => {
  const nodeMap = new Map(dependencyNodes.value.map((node) => [node.id, node]))
  const seen = new Set<string>()
  const edges: GraphEdge[] = []

  for (const dep of visibleDependencies.value) {
    const from = normalizeNode(dep.from).id
    const to = normalizeNode(dep.to).id
    if (from === to || !nodeMap.has(from) || !nodeMap.has(to)) continue

    const id = `${from}-${to}-${dep.type || 'related'}`
    if (seen.has(id)) continue
    seen.add(id)

    const source = nodeMap.get(from)!
    const target = nodeMap.get(to)!
    const mx = (source.x + target.x) / 2
    const my = (source.y + target.y) / 2
    const curve = ((source.x - target.x) * 0.08) + ((source.y - target.y) * 0.04)
    edges.push({
      id,
      from,
      to,
      path: `M ${source.x} ${source.y} Q ${mx + curve} ${my - curve} ${target.x} ${target.y}`,
      type: dep.type || 'related',
      typeClass: `type-${sanitizeClass(dep.type || 'related')}`,
      strength: dep.strength || 'normal',
    })
  }

  return edges
})

const topImpactNodes = computed(() => dependencyNodes.value.slice().sort((a, b) => b.weight - a.weight).slice(0, 5))

const strengthStats = computed(() => {
  const buckets = new Map<string, number>()
  for (const dep of visibleDependencies.value) {
    const key = dep.strength || 'normal'
    buckets.set(key, (buckets.get(key) || 0) + 1)
  }
  const max = Math.max(...Array.from(buckets.values()), 1)
  return Array.from(buckets.entries()).map(([name, count]) => ({
    name,
    count,
    percent: Math.max(12, Math.round((count / max) * 100)),
  }))
})

watch(() => props.map?.id, () => {
  expandedSections.value = props.map?.sections?.slice(0, 2).map((section) => section.id) || []
  activeDependencyType.value = 'all'
  activeNodeId.value = null
}, { immediate: true })

function getSectionReviewedCount(section: Section): number {
  return section.files.filter((file) => file.reviewed).length
}

function getSectionProgress(section: Section): number {
  if (section.files.length === 0) return 0
  return Math.round((getSectionReviewedCount(section) / section.files.length) * 100)
}

function areAllFilesReviewed(section: Section): boolean {
  return section.files.every((file) => file.reviewed)
}

function toggleAllFiles(section: Section) {
  const reviewed = !areAllFilesReviewed(section)
  emit('section-reviewed', section.id, reviewed)
}

function onFileReviewedChange(sectionId: string, filePath: string, reviewed: boolean) {
  emit('file-reviewed', sectionId, filePath, reviewed)
}

function toggleSection(sectionId: string) {
  const index = expandedSections.value.indexOf(sectionId)
  if (index === -1) {
    expandedSections.value.push(sectionId)
  } else {
    expandedSections.value.splice(index, 1)
  }
}

function expandSection(sectionId: string) {
  if (!expandedSections.value.includes(sectionId)) {
    expandedSections.value.push(sectionId)
  }
}

function normalizeNode(path: string) {
  return {
    id: sanitizeId(path),
    label: compactLabel(path),
  }
}

function getDirectory(path: string): string {
  const parts = path.split('/')
  if (parts.length > 1) return parts.slice(0, -1).join('/')
  return '.'
}

function compactLabel(path: string): string {
  const parts = path.split('/').filter(Boolean)
  if (parts.length <= 2) return path
  return `${parts[0]}/.../${parts[parts.length - 1]}`
}

function sanitizeId(s: string): string {
  return s.replace(/[^a-zA-Z0-9]/g, '_')
}

function sanitizeClass(s: string): string {
  return sanitizeId(s).toLowerCase()
}

function strengthScore(strength: string): number {
  const normalized = (strength || '').toLowerCase()
  if (normalized.includes('strong') || normalized.includes('high') || normalized.includes('critical')) return 5
  if (normalized.includes('medium') || normalized.includes('normal')) return 3
  if (normalized.includes('weak') || normalized.includes('low')) return 1
  return 2
}

function nodeColor(index: number): string {
  const palette = ['#55d6c2', '#ffcf5a', '#ff6b8a', '#8fd14f', '#70a7ff', '#f28cff']
  return palette[index % palette.length]
}

function isEdgeMuted(edge: GraphEdge): boolean {
  if (!activeNodeId.value) return false
  return edge.from !== activeNodeId.value && edge.to !== activeNodeId.value
}

defineExpose({
  expandSection,
})
</script>

<style scoped>
.review-map {
  background: #101114;
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 8px;
  color: #f5f7fb;
  overflow: hidden;
  position: relative;
}

.review-map::before {
  content: "";
  position: absolute;
  inset: 0;
  background:
    linear-gradient(135deg, rgba(85, 214, 194, 0.16), transparent 30%),
    linear-gradient(315deg, rgba(255, 207, 90, 0.12), transparent 34%),
    repeating-linear-gradient(90deg, rgba(255, 255, 255, 0.03) 0, rgba(255, 255, 255, 0.03) 1px, transparent 1px, transparent 80px);
  pointer-events: none;
}

.map-shell,
.empty {
  position: relative;
  z-index: 1;
}

.empty {
  padding: 40px;
}

.map-shell {
  padding: 18px;
}

.map-hero {
  align-items: center;
  display: flex;
  gap: 24px;
  justify-content: space-between;
  min-height: 140px;
  padding: 18px 18px 22px;
}

.hero-copy {
  min-width: 0;
}

.eyebrow {
  align-items: center;
  color: #55d6c2;
  display: inline-flex;
  font-size: 12px;
  font-weight: 700;
  gap: 6px;
  letter-spacing: 0;
  margin-bottom: 8px;
}

.map-hero h3 {
  color: #fff;
  font-size: 24px;
  line-height: 1.18;
  margin: 0 0 10px;
}

.map-hero p {
  color: rgba(245, 247, 251, 0.7);
  font-size: 14px;
  line-height: 1.6;
  margin: 0;
  max-width: 760px;
}

.hero-score {
  align-items: center;
  display: flex;
  flex: 0 0 auto;
  gap: 14px;
}

.score-ring {
  --progress: 0deg;
  align-items: center;
  background: conic-gradient(#55d6c2 var(--progress), rgba(255, 255, 255, 0.1) 0);
  border-radius: 50%;
  display: flex;
  height: 86px;
  justify-content: center;
  position: relative;
  width: 86px;
}

.score-ring::after {
  background: #101114;
  border-radius: 50%;
  content: "";
  inset: 8px;
  position: absolute;
}

.score-ring span,
.score-ring small {
  position: relative;
  z-index: 1;
}

.score-ring span {
  font-size: 24px;
  font-weight: 800;
}

.score-ring small {
  color: rgba(245, 247, 251, 0.72);
  font-size: 12px;
  margin-left: 1px;
}

.score-meta {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.score-meta strong {
  font-size: 18px;
}

.score-meta span {
  color: rgba(245, 247, 251, 0.62);
  font-size: 12px;
}

.signal-grid {
  display: grid;
  gap: 10px;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  margin-bottom: 14px;
}

.signal-tile {
  align-items: center;
  background: rgba(255, 255, 255, 0.07);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 8px;
  display: grid;
  gap: 3px 10px;
  grid-template-columns: auto 1fr;
  padding: 13px;
}

.signal-tile .el-icon {
  color: #55d6c2;
  font-size: 20px;
  grid-row: span 2;
}

.signal-tile strong {
  color: #fff;
  font-size: 19px;
  line-height: 1;
}

.signal-tile span {
  color: rgba(245, 247, 251, 0.62);
  font-size: 12px;
}

.signal-tile.hot .el-icon {
  color: #ffcf5a;
}

.map-summary {
  align-items: flex-start;
  background: rgba(85, 214, 194, 0.08);
  border: 1px solid rgba(85, 214, 194, 0.2);
  border-radius: 8px;
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
  padding: 13px 14px;
}

.summary-pulse {
  animation: pulse 1.8s ease-in-out infinite;
  background: #55d6c2;
  border-radius: 50%;
  flex: 0 0 auto;
  height: 9px;
  margin-top: 6px;
  width: 9px;
}

.map-summary p {
  color: rgba(245, 247, 251, 0.78);
  font-size: 13px;
  line-height: 1.6;
  margin: 0;
}

.dependency-stage {
  background: rgba(255, 255, 255, 0.055);
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 8px;
  margin-bottom: 16px;
  padding: 14px;
}

.stage-header {
  align-items: flex-start;
  display: flex;
  gap: 16px;
  justify-content: space-between;
  margin-bottom: 12px;
}

.stage-header h4 {
  color: #fff;
  font-size: 18px;
  margin: 0;
}

.type-filter {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  justify-content: flex-end;
}

.type-filter button {
  background: rgba(255, 255, 255, 0.08);
  border: 1px solid rgba(255, 255, 255, 0.14);
  border-radius: 999px;
  color: rgba(245, 247, 251, 0.72);
  cursor: pointer;
  font-size: 12px;
  height: 30px;
  padding: 0 12px;
  transition: all 0.2s ease;
}

.type-filter button.active,
.type-filter button:hover {
  background: #55d6c2;
  border-color: #55d6c2;
  color: #101114;
}

.graph-command {
  display: grid;
  gap: 12px;
  grid-template-columns: minmax(0, 1fr) 250px;
}

.graph-panel {
  min-width: 0;
}

.dependency-svg {
  border-radius: 8px;
  display: block;
  height: auto;
  max-height: 560px;
  width: 100%;
}

.radar-ring {
  fill: none;
  opacity: 0.42;
  stroke: rgba(85, 214, 194, 0.18);
  stroke-dasharray: 8 12;
}

.edge-line {
  color: #55d6c2;
  fill: none;
  opacity: 0.82;
  stroke: currentColor;
  stroke-linecap: round;
  stroke-width: 2.1;
  transition: opacity 0.2s ease, stroke-width 0.2s ease;
}

.edge-line.type-import,
.edge-line.type-directory {
  color: #55d6c2;
}

.edge-line.type-related,
.edge-line.type-same_directory {
  color: #ffcf5a;
}

.edge-line.type-calls,
.edge-line.type-call {
  color: #ff6b8a;
}

.edge-line.muted {
  opacity: 0.13;
  stroke-width: 1.2;
}

.graph-node {
  cursor: pointer;
  transition: opacity 0.2s ease, transform 0.2s ease;
}

.graph-node.active {
  filter: url(#node-glow);
}

.graph-node.dimmed {
  opacity: 0.28;
}

.node-halo {
  animation: breathe 2.6s ease-in-out infinite;
  fill: rgba(255, 255, 255, 0.08);
  stroke: rgba(255, 255, 255, 0.16);
}

.node-core {
  fill: var(--node-color);
  stroke: rgba(255, 255, 255, 0.9);
  stroke-width: 2;
}

.node-label {
  fill: #fff;
  font-size: 12px;
  font-weight: 700;
  paint-order: stroke;
  stroke: rgba(17, 19, 24, 0.94);
  stroke-width: 5px;
  text-anchor: middle;
}

.node-subtitle {
  fill: rgba(245, 247, 251, 0.62);
  font-size: 11px;
  text-anchor: middle;
}

.graph-sidebar {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.sidebar-block {
  background: rgba(17, 19, 24, 0.74);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 8px;
  padding: 12px;
}

.sidebar-title {
  align-items: center;
  color: #fff;
  display: flex;
  font-size: 13px;
  font-weight: 700;
  gap: 7px;
  margin-bottom: 10px;
}

.sidebar-title .el-icon {
  color: #ffcf5a;
}

.impact-list,
.strength-stack {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.impact-row {
  align-items: center;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid transparent;
  border-radius: 8px;
  color: rgba(245, 247, 251, 0.82);
  cursor: pointer;
  display: grid;
  font-size: 12px;
  gap: 8px;
  grid-template-columns: auto minmax(0, 1fr) auto;
  min-height: 34px;
  padding: 8px;
  text-align: left;
}

.impact-row:hover {
  border-color: rgba(85, 214, 194, 0.44);
}

.impact-dot {
  border-radius: 50%;
  height: 9px;
  width: 9px;
}

.impact-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.impact-row strong {
  color: #fff;
}

.strength-row {
  align-items: center;
  color: rgba(245, 247, 251, 0.72);
  display: grid;
  font-size: 12px;
  gap: 8px;
  grid-template-columns: 58px minmax(0, 1fr) 22px;
}

.strength-bar {
  background: rgba(255, 255, 255, 0.1);
  border-radius: 99px;
  height: 7px;
  overflow: hidden;
}

.strength-bar i {
  background: linear-gradient(90deg, #55d6c2, #ffcf5a);
  border-radius: inherit;
  display: block;
  height: 100%;
}

.sections {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.section {
  background: rgba(255, 255, 255, 0.065);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-left: 4px solid #55d6c2;
  border-radius: 8px;
  overflow: hidden;
}

.section.priority-high,
.section.priority-critical {
  border-left-color: #ff6b8a;
}

.section.priority-medium {
  border-left-color: #ffcf5a;
}

.section-header {
  align-items: center;
  cursor: pointer;
  display: flex;
  gap: 16px;
  justify-content: space-between;
  padding: 13px 14px;
  transition: background 0.2s ease;
}

.section-header:hover {
  background: rgba(255, 255, 255, 0.06);
}

.section-title {
  align-items: center;
  display: flex;
  gap: 10px;
  min-width: 0;
}

.section-title .el-icon {
  color: #55d6c2;
  flex: 0 0 auto;
}

.section-title h5 {
  color: #fff;
  font-size: 14px;
  line-height: 1.35;
  margin: 0;
}

.section-title p {
  color: rgba(245, 247, 251, 0.56);
  font-size: 12px;
  line-height: 1.4;
  margin: 3px 0 0;
}

.section-meta {
  align-items: center;
  color: rgba(245, 247, 251, 0.62);
  display: flex;
  flex: 0 0 auto;
  font-size: 12px;
  gap: 12px;
}

.section-content {
  border-top: 1px solid rgba(255, 255, 255, 0.09);
  padding: 13px 14px 15px;
}

.files {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 14px;
}

.file {
  align-items: center;
  background: rgba(255, 255, 255, 0.08);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-left: 3px solid rgba(255, 255, 255, 0.2);
  border-radius: 8px;
  display: flex;
  gap: 11px;
  min-height: 42px;
  padding: 8px 10px;
  transition: border-color 0.2s ease, transform 0.2s ease;
}

.file:hover {
  border-color: rgba(85, 214, 194, 0.32);
  transform: translateY(-1px);
}

.file.reviewed {
  border-left-color: #8fd14f;
}

.file-icon {
  color: rgba(245, 247, 251, 0.54);
  font-size: 16px;
}

.file-info {
  align-items: center;
  display: flex;
  flex: 1;
  gap: 8px;
  min-width: 0;
}

.file-path {
  color: #fff;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.file-language {
  background: rgba(85, 214, 194, 0.14);
  border: 1px solid rgba(85, 214, 194, 0.24);
  border-radius: 999px;
  color: #55d6c2;
  flex: 0 0 auto;
  font-size: 11px;
  padding: 2px 7px;
}

.file-stats {
  display: flex;
  flex: 0 0 auto;
  font-size: 12px;
  gap: 7px;
}

.added {
  color: #8fd14f;
}

.removed {
  color: #ff6b8a;
}

.findings {
  background: rgba(255, 207, 90, 0.08);
  border: 1px solid rgba(255, 207, 90, 0.2);
  border-radius: 8px;
  padding: 11px 12px;
}

.findings h6 {
  color: #ffcf5a;
  font-size: 13px;
  margin: 0 0 8px;
}

.findings ul {
  margin: 0;
  padding-left: 18px;
}

.findings li {
  color: rgba(245, 247, 251, 0.75);
  font-size: 12px;
  line-height: 1.5;
  margin-bottom: 4px;
}

@keyframes pulse {
  0%,
  100% {
    box-shadow: 0 0 0 0 rgba(85, 214, 194, 0.45);
  }
  50% {
    box-shadow: 0 0 0 8px rgba(85, 214, 194, 0);
  }
}

@keyframes breathe {
  0%,
  100% {
    opacity: 0.55;
  }
  50% {
    opacity: 0.95;
  }
}

@media (max-width: 1100px) {
  .graph-command {
    grid-template-columns: 1fr;
  }

  .graph-sidebar {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 760px) {
  .map-shell {
    padding: 12px;
  }

  .map-hero,
  .stage-header,
  .section-header {
    align-items: stretch;
    flex-direction: column;
  }

  .hero-score {
    justify-content: space-between;
  }

  .signal-grid,
  .graph-sidebar {
    grid-template-columns: 1fr;
  }

  .section-meta {
    align-items: flex-start;
    flex-wrap: wrap;
  }

  .file-info {
    align-items: flex-start;
    flex-direction: column;
  }

  .file-path {
    white-space: normal;
    word-break: break-all;
  }
}
</style>
