<template>
  <AppLayout>
    <div class="detail-page">
      <div class="back-link" @click="router.push('/contributors')">
        <el-icon><ArrowLeft /></el-icon>
        <span>返回排行榜</span>
      </div>

      <div v-loading="loading" class="detail-content">
        <template v-if="detail">
          <!-- Profile Header -->
          <div class="profile-header">
            <div class="profile-left">
              <img v-if="detail.author_avatar" :src="detail.author_avatar" class="profile-avatar" :alt="detail.author" />
              <span v-else class="profile-avatar-fallback">{{ getInitials(detail.author_name || detail.author) }}</span>
              <div>
                <h1>{{ detail.author_name || detail.author }}</h1>
                <p>@{{ detail.author }}</p>
                <span class="last-active">最后活跃: {{ formatDateFull(detail.last_review_at) }}</span>
              </div>
            </div>
            <div class="profile-score">
              <ScoreRing :score="detail.avg_score" />
            </div>
          </div>

          <!-- Key Metrics Row -->
          <el-row :gutter="16" class="metrics-row">
            <el-col :xs="12" :sm="8" :md="4">
              <StatCard label="PR 审查数" :value="detail.review_count" />
            </el-col>
            <el-col :xs="12" :sm="8" :md="4">
              <StatCard label="通过率" :value="`${detail.pass_rate.toFixed(0)}%`" :value-class="detail.pass_rate >= 80 ? 'text-green' : detail.pass_rate >= 50 ? 'text-orange' : 'text-red'" />
            </el-col>
            <el-col :xs="12" :sm="8" :md="4">
              <StatCard label="中位分" :value="detail.median_score" />
            </el-col>
            <el-col :xs="12" :sm="8" :md="4">
              <StatCard label="最高分" :value="detail.max_score" value-class="text-green" />
            </el-col>
            <el-col :xs="12" :sm="8" :md="4">
              <StatCard label="最低分" :value="detail.min_score" value-class="text-red" />
            </el-col>
            <el-col :xs="12" :sm="8" :md="4">
              <StatCard label="平均问题数" :value="detail.avg_findings.toFixed(1)" />
            </el-col>
          </el-row>

          <!-- Charts Row -->
          <el-row :gutter="16">
            <el-col :xs="24" :md="14">
              <el-card shadow="never" class="chart-card">
                <h3>评分趋势</h3>
                <p class="chart-desc">按周聚合的平均质量评分</p>
                <TrendChart :data="trend" />
              </el-card>
            </el-col>

            <el-col :xs="24" :md="10">
              <el-card shadow="never" class="chart-card">
                <h3>问题类别分布</h3>
                <p class="chart-desc">按类别统计的代码问题数量</p>
                <div v-if="categories.length > 0" class="category-list">
                  <div v-for="cat in categories" :key="cat.category" class="category-bar">
                    <div class="cat-header">
                      <span>{{ cat.category }}</span>
                      <strong>{{ cat.count }}</strong>
                    </div>
                    <el-progress
                      :percentage="catMaxCount > 0 ? (cat.count / catMaxCount * 100) : 0"
                      :show-text="false"
                      :stroke-width="8"
                      :color="getCategoryColor(cat.category)"
                    />
                  </div>
                </div>
                <div v-else class="empty-chart">
                  <p>暂无问题分类数据</p>
                </div>
              </el-card>
            </el-col>
          </el-row>

          <!-- Severity Distribution & Grade Distribution -->
          <el-row :gutter="16" style="margin-top: 16px">
            <el-col :xs="24" :md="12">
              <el-card shadow="never" class="chart-card">
                <h3>严重程度分布</h3>
                <p class="chart-desc">所有审查中发现的问题按严重程度统计</p>
                <div class="severity-grid">
                  <div class="severity-item critical">
                    <span class="sev-dot" />
                    <div>
                      <strong>{{ detail.critical_count }}</strong>
                      <small>严重</small>
                    </div>
                  </div>
                  <div class="severity-item major">
                    <span class="sev-dot" />
                    <div>
                      <strong>{{ detail.major_count }}</strong>
                      <small>高风险</small>
                    </div>
                  </div>
                  <div class="severity-item minor">
                    <span class="sev-dot" />
                    <div>
                      <strong>{{ detail.minor_count }}</strong>
                      <small>中风险</small>
                    </div>
                  </div>
                  <div class="severity-item info">
                    <span class="sev-dot" />
                    <div>
                      <strong>{{ detail.info_count }}</strong>
                      <small>提示</small>
                    </div>
                  </div>
                </div>
              </el-card>
            </el-col>

            <el-col :xs="24" :md="12">
              <el-card shadow="never" class="chart-card">
                <h3>质量等级分布</h3>
                <p class="chart-desc">所有 PR 审查的质量等级占比</p>
                <div class="grade-list">
                  <div v-for="g in gradeList" :key="g.label" class="grade-item">
                    <span class="grade-badge" :class="g.class">{{ g.label }}</span>
                    <el-progress :percentage="detail.review_count > 0 ? (g.count / detail.review_count * 100) : 0" :stroke-width="10" :color="g.color" />
                    <strong>{{ g.count }}</strong>
                  </div>
                </div>
              </el-card>
            </el-col>
          </el-row>
        </template>

        <div v-else-if="!loading" class="not-found">
          <p>未找到该开发者的质量数据</p>
          <el-button type="primary" @click="router.push('/contributors')">返回排行榜</el-button>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft } from '@element-plus/icons-vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import ScoreRing from '@/components/common/ScoreRing.vue'
import TrendChart from '@/components/common/TrendChart.vue'
import StatCard from '@/components/common/StatCard.vue'
import { getContributor, getContributorTrend, getContributorCategories } from '@/api/stats'
import type { ContributorDetail, TrendPoint, CategoryStat } from '@/types/stats'
import { getInitials, formatDateFull } from '@/utils/format'
import { getScoreColor } from '@/utils/colors'

const route = useRoute()
const router = useRouter()
const loading = ref(true)
const detail = ref<ContributorDetail | null>(null)
const trend = ref<TrendPoint[]>([])
const categories = ref<CategoryStat[]>([])

const catMaxCount = computed(() => categories.value.length > 0 ? Math.max(...categories.value.map(c => c.count)) : 0)

const gradeList = computed(() => {
  if (!detail.value) return []
  const dist = detail.value.grade_distribution || {}
  return [
    { label: '优秀', count: dist['优秀'] || 0, class: 'g-excellent', color: '#159570' },
    { label: '良好', count: dist['良好'] || 0, class: 'g-good', color: '#3984cf' },
    { label: '需改进', count: dist['需改进'] || 0, class: 'g-improve', color: '#d98616' },
    { label: '高风险', count: dist['高风险'] || 0, class: 'g-risk', color: '#d84b5b' },
  ]
})

async function fetchDetail() {
  loading.value = true
  const author = decodeURIComponent(route.params.author as string)
  try {
    const [d, t, c] = await Promise.all([
      getContributor(author),
      getContributorTrend(author),
      getContributorCategories(author),
    ])
    detail.value = d
    trend.value = t
    categories.value = c
  } finally {
    loading.value = false
  }
}

function getCategoryColor(cat: string) {
  const map: Record<string, string> = {
    '安全性': '#d84b5b',
    '性能': '#d98616',
    '测试': '#3984cf',
    '正确性': '#9260d4',
    '可维护性': '#5b5bd6',
  }
  return map[cat] || '#7d8698'
}

onMounted(fetchDetail)
</script>

<style scoped>
.detail-page {
  width: min(1440px, 100%);
  margin: 0 auto;
  padding: 34px 36px 48px;
}

.back-link {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 20px;
  color: var(--app-muted);
  font-size: 13px;
  cursor: pointer;
  transition: color 160ms ease;
}

.back-link:hover {
  color: var(--app-primary);
}

.profile-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 28px;
  border-radius: 16px;
  background: linear-gradient(135deg, #f8f9ff, #f0f1fc);
  margin-bottom: 20px;
}

.profile-left {
  display: flex;
  align-items: center;
  gap: 18px;
}

.profile-avatar {
  width: 72px;
  height: 72px;
  border-radius: 50%;
  border: 3px solid #fff;
  box-shadow: 0 4px 14px rgba(0, 0, 0, 0.08);
  object-fit: cover;
}

.profile-avatar-fallback {
  display: grid;
  width: 72px;
  height: 72px;
  place-items: center;
  border-radius: 50%;
  color: #fff;
  background: linear-gradient(135deg, #7777eb, #5252c7);
  font-size: 24px;
  font-weight: 700;
}

.profile-left h1 {
  margin: 0;
  color: var(--app-text);
  font-size: 26px;
  letter-spacing: -0.5px;
}

.profile-left p {
  margin: 4px 0 0;
  color: var(--app-muted);
  font-size: 14px;
}

.last-active {
  display: inline-block;
  margin-top: 6px;
  color: #9aa2b1;
  font-size: 12px;
}

.profile-score {
  flex-shrink: 0;
}

.score-ring {
  position: relative;
  width: 120px;
  height: 120px;
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
  color: var(--score-color, var(--app-text));
  font-size: 32px;
  line-height: 1;
}

.score-ring-text small {
  color: var(--app-muted);
  font-size: 12px;
}

.metrics-row {
  margin-bottom: 16px;
}

.metrics-row .el-col {
  margin-bottom: 12px;
}

.metric-card {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 18px;
  border-radius: 12px;
  background: #fff;
  box-shadow: var(--app-shadow);
}

.metric-card span {
  color: var(--app-muted);
  font-size: 12px;
  font-weight: 600;
}

.metric-card strong {
  color: var(--app-text);
  font-size: 24px;
}

.text-green { color: #159570; }
.text-orange { color: #d98616; }
.text-red { color: #d84b5b; }

.chart-card {
  margin-bottom: 0;
  box-shadow: var(--app-shadow);
}

.chart-card h3 {
  margin: 0 0 4px;
  color: var(--app-text);
  font-size: 16px;
}

.chart-desc {
  margin: 0 0 18px;
  color: #929aab;
  font-size: 12px;
}

.trend-chart {
  width: 100%;
}

.trend-svg {
  width: 100%;
  height: 240px;
}

.trend-labels {
  display: flex;
  justify-content: space-between;
  padding: 8px 40px 0 20px;
}

.trend-label {
  color: #9aa2b1;
  font-size: 10px;
}

.empty-chart {
  padding: 40px 16px;
  text-align: center;
  color: #aab1be;
  font-size: 13px;
}

.category-list {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.category-bar {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.cat-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.cat-header span {
  color: var(--app-text);
  font-size: 13px;
  font-weight: 600;
}

.cat-header strong {
  color: var(--app-muted);
  font-size: 14px;
}

.severity-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px;
}

.severity-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 14px;
  border-radius: 10px;
  background: #f8f9fb;
}

.sev-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
}

.severity-item.critical .sev-dot { background: #d84b5b; }
.severity-item.major .sev-dot { background: #d98616; }
.severity-item.minor .sev-dot { background: #3984cf; }
.severity-item.info .sev-dot { background: #7d8698; }

.severity-item strong {
  display: block;
  color: var(--app-text);
  font-size: 20px;
}

.severity-item small {
  display: block;
  color: var(--app-muted);
  font-size: 11px;
}

.grade-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.grade-item {
  display: flex;
  align-items: center;
  gap: 12px;
}

.grade-badge {
  display: inline-grid;
  width: 56px;
  height: 28px;
  flex-shrink: 0;
  place-items: center;
  border-radius: 7px;
  font-size: 12px;
  font-weight: 700;
}

.grade-badge.g-excellent { color: #159570; background: #e6f7f0; }
.grade-badge.g-good { color: #3984cf; background: #eaf4ff; }
.grade-badge.g-improve { color: #d98616; background: #fff6e8; }
.grade-badge.g-risk { color: #d84b5b; background: #fff0f2; }

.grade-item .el-progress {
  flex: 1;
}

.grade-item strong {
  min-width: 28px;
  color: var(--app-text);
  font-size: 14px;
  text-align: right;
}

.not-found {
  padding: 60px 16px;
  text-align: center;
}

.not-found p {
  margin-bottom: 16px;
  color: #aab1be;
  font-size: 15px;
}

@media (max-width: 720px) {
  .detail-page {
    padding: 22px 14px 36px;
  }
  .profile-header {
    flex-direction: column;
    gap: 20px;
    padding: 20px;
    text-align: center;
  }
  .profile-left {
    flex-direction: column;
  }
}
</style>
