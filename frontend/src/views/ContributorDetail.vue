<template>
  <AppLayout>
    <PageShell>
      <div class="cd-page" v-loading="loading">
      <!-- 返回 -->
      <div class="cd-back" @click="router.push('/contributors')">
        <el-icon><ArrowLeft /></el-icon>
        <span>返回排行榜</span>
      </div>

      <template v-if="detail">
        <!-- Profile Hero：头像 + 名字 + 评分环 + 核心指标条 -->
        <div class="cd-hero">
          <div class="cd-hero-left">
            <img v-if="detail.author_avatar" :src="detail.author_avatar" class="cd-avatar" :alt="detail.author" />
            <span v-else class="cd-avatar-fallback">{{ getInitials(detail.author_name || detail.author) }}</span>
            <div class="cd-hero-info">
              <h1>{{ detail.author_name || detail.author }}</h1>
              <p>@{{ detail.author }}</p>
              <span class="cd-last-active">最后活跃 · {{ formatDateFull(detail.last_review_at) }}</span>
            </div>
          </div>
          <ScoreRing :score="detail.avg_score" />
        </div>

        <!-- 指标条：6 项核心数据 -->
        <div class="cd-stats">
          <div class="cd-stat">
            <small>PR 审查</small>
            <strong>{{ detail.review_count }}</strong>
          </div>
          <div class="cd-stat">
            <small>通过率</small>
            <strong :class="passClass">{{ detail.pass_rate.toFixed(0) }}<i>%</i></strong>
          </div>
          <div class="cd-stat">
            <small>中位分</small>
            <strong>{{ detail.median_score }}</strong>
          </div>
          <div class="cd-stat">
            <small>最高分</small>
            <strong class="text-green">{{ detail.max_score }}</strong>
          </div>
          <div class="cd-stat">
            <small>最低分</small>
            <strong class="text-red">{{ detail.min_score }}</strong>
          </div>
          <div class="cd-stat">
            <small>平均问题</small>
            <strong>{{ detail.avg_findings.toFixed(1) }}</strong>
          </div>
        </div>

        <!-- 主体双栏：左=趋势+类别  右=严重度+等级 -->
        <div class="cd-grid">
          <!-- 左栏 -->
          <div class="cd-col">
            <!-- 评分趋势 -->
            <div class="cd-card">
              <div class="cd-card-head">
                <h3>评分趋势</h3>
                <span>按周聚合</span>
              </div>
              <TrendChart :data="trend" :height="220" />
            </div>

            <!-- 问题类别 -->
            <div class="cd-card">
              <div class="cd-card-head">
                <h3>问题类别分布</h3>
                <span>{{ categories.length }} 个类别</span>
              </div>
              <div v-if="categories.length" class="cd-cat-list">
                <div v-for="cat in categories" :key="cat.category" class="cd-cat">
                  <div class="cd-cat-head">
                    <span>{{ cat.category }}</span>
                    <strong>{{ cat.count }}</strong>
                  </div>
                  <div class="cd-cat-bar">
                    <div class="cd-cat-fill" :style="{ width: `${catMax ? cat.count / catMax * 100 : 0}%`, background: getCategoryColor(cat.category) }" />
                  </div>
                </div>
              </div>
              <div v-else class="cd-empty">暂无问题分类数据</div>
            </div>
          </div>

          <!-- 右栏 -->
          <div class="cd-col">
            <!-- 严重程度 -->
            <div class="cd-card">
              <div class="cd-card-head">
                <h3>严重程度分布</h3>
                <span>{{ totalFindings }} 个问题</span>
              </div>
              <div class="cd-sev-grid">
                <div class="cd-sev critical">
                  <span class="cd-sev-dot" />
                  <strong>{{ detail.critical_count }}</strong>
                  <small>严重</small>
                </div>
                <div class="cd-sev major">
                  <span class="cd-sev-dot" />
                  <strong>{{ detail.major_count }}</strong>
                  <small>高风险</small>
                </div>
                <div class="cd-sev minor">
                  <span class="cd-sev-dot" />
                  <strong>{{ detail.minor_count }}</strong>
                  <small>中风险</small>
                </div>
                <div class="cd-sev info">
                  <span class="cd-sev-dot" />
                  <strong>{{ detail.info_count }}</strong>
                  <small>提示</small>
                </div>
              </div>
            </div>

            <!-- 等级分布 -->
            <div class="cd-card">
              <div class="cd-card-head">
                <h3>质量等级分布</h3>
                <span>{{ detail.review_count }} 次审查</span>
              </div>
              <div class="cd-grade-list">
                <div v-for="g in gradeList" :key="g.label" class="cd-grade">
                  <span class="cd-grade-badge" :class="g.class">{{ g.label }}</span>
                  <div class="cd-grade-bar">
                    <div class="cd-grade-fill" :style="{ width: `${detail.review_count ? g.count / detail.review_count * 100 : 0}%`, background: g.color }" />
                  </div>
                  <strong>{{ g.count }}</strong>
                </div>
              </div>
            </div>
          </div>
        </div>
      </template>

      <div v-else-if="!loading" class="cd-not-found">
        <p>未找到该开发者的质量数据</p>
        <el-button type="primary" @click="router.push('/contributors')">返回排行榜</el-button>
      </div>
      </div>
    </PageShell>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft } from '@element-plus/icons-vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import PageShell from '@/components/common/PageShell.vue'
import ScoreRing from '@/components/common/ScoreRing.vue'
import TrendChart from '@/components/common/TrendChart.vue'
import { getContributor, getContributorTrend, getContributorCategories } from '@/api/stats'
import type { ContributorDetail, TrendPoint, CategoryStat } from '@/types/stats'
import { getInitials, formatDateFull } from '@/utils/format'

const route = useRoute()
const router = useRouter()
const loading = ref(true)
const detail = ref<ContributorDetail | null>(null)
const trend = ref<TrendPoint[]>([])
const categories = ref<CategoryStat[]>([])

const catMax = computed(() => categories.value.length ? Math.max(...categories.value.map(c => c.count)) : 0)
const totalFindings = computed(() => {
  if (!detail.value) return 0
  return detail.value.critical_count + detail.value.major_count + detail.value.minor_count + detail.value.info_count
})
const passClass = computed(() => {
  if (!detail.value) return ''
  return detail.value.pass_rate >= 80 ? 'text-green' : detail.value.pass_rate >= 50 ? 'text-orange' : 'text-red'
})

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
    '安全性': '#d84b5b', '性能': '#d98616', '测试': '#3984cf',
    '正确性': '#9260d4', '可维护性': '#5b5bd6',
  }
  return map[cat] || '#7d8698'
}

onMounted(fetchDetail)
</script>

<style scoped>
/* 返回 */
.cd-back {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 16px;
  color: var(--app-muted);
  font-size: 13px;
  cursor: pointer;
  transition: color 0.15s;
}
.cd-back:hover { color: var(--app-primary); }

/* Hero */
.cd-hero {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  padding: 24px 28px;
  border-radius: 16px;
  background: linear-gradient(135deg, #f8f9ff, #f0f1fc);
  margin-bottom: 16px;
}
.cd-hero-left { display: flex; align-items: center; gap: 16px; }
.cd-avatar {
  width: 64px; height: 64px;
  border-radius: 50%; border: 3px solid #fff;
  box-shadow: 0 4px 14px rgba(0,0,0,0.08); object-fit: cover;
}
.cd-avatar-fallback {
  display: grid; width: 64px; height: 64px; place-items: center;
  border-radius: 50%; color: #fff;
  background: linear-gradient(135deg, #7777eb, #5252c7);
  font-size: 22px; font-weight: 700;
}
.cd-hero-info h1 { margin: 0; font-size: 24px; color: var(--app-text); letter-spacing: -0.4px; }
.cd-hero-info p { margin: 3px 0 0; color: var(--app-muted); font-size: 14px; }
.cd-last-active { display: inline-block; margin-top: 5px; color: #9aa2b1; font-size: 12px; }

/* 指标条 */
.cd-stats {
  display: grid;
  grid-template-columns: repeat(6, 1fr);
  gap: 12px;
  margin-bottom: 16px;
}
.cd-stat {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 14px 16px;
  border: 1px solid #edf0f4;
  border-radius: var(--card-radius);
  background: #fff;
}
.cd-stat small { font-size: 11px; color: #7d8698; font-weight: 600; }
.cd-stat strong { font-size: 22px; color: #1f2933; }
.cd-stat strong i { font-size: 14px; font-style: normal; }
.text-green { color: #159570; }
.text-orange { color: #d98616; }
.text-red { color: #d84b5b; }

/* 主体双栏 */
.cd-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
  align-items: start;
}
.cd-col { display: flex; flex-direction: column; gap: 16px; }

/* 卡片 */
.cd-card {
  border: 1px solid var(--app-border);
  border-radius: var(--card-radius);
  background: var(--app-surface);
  padding: 18px 20px;
  box-shadow: var(--app-shadow);
}
.cd-card-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  margin-bottom: 14px;
}
.cd-card-head h3 { margin: 0; font-size: 15px; color: var(--app-text); }
.cd-card-head span { font-size: 12px; color: #9ca3af; }

/* 类别条形图 */
.cd-cat-list { display: flex; flex-direction: column; gap: 12px; }
.cd-cat-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 5px; }
.cd-cat-head span { font-size: 13px; font-weight: 600; color: var(--app-text); }
.cd-cat-head strong { font-size: 13px; color: var(--app-muted); }
.cd-cat-bar { height: 8px; background: #f0f1f3; border-radius: 4px; overflow: hidden; }
.cd-cat-fill { height: 100%; border-radius: 4px; transition: width 0.3s; }

/* 严重度网格 */
.cd-sev-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 10px;
}
.cd-sev {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 14px;
  border-radius: var(--card-radius);
  background: #f8f9fb;
}
.cd-sev-dot { width: 10px; height: 10px; border-radius: 50%; flex-shrink: 0; }
.cd-sev.critical .cd-sev-dot { background: #d84b5b; }
.cd-sev.major .cd-sev-dot { background: #d98616; }
.cd-sev.minor .cd-sev-dot { background: #3984cf; }
.cd-sev.info .cd-sev-dot { background: #7d8698; }
.cd-sev strong { font-size: 20px; color: var(--app-text); }
.cd-sev small { font-size: 11px; color: var(--app-muted); }

/* 等级条形图 */
.cd-grade-list { display: flex; flex-direction: column; gap: 11px; }
.cd-grade { display: flex; align-items: center; gap: 12px; }
.cd-grade-badge {
  display: inline-grid;
  width: 52px; height: 26px;
  flex-shrink: 0; place-items: center;
  border-radius: 7px; font-size: 12px; font-weight: 700;
}
.cd-grade-badge.g-excellent { color: #159570; background: #e6f7f0; }
.cd-grade-badge.g-good { color: #3984cf; background: #eaf4ff; }
.cd-grade-badge.g-improve { color: #d98616; background: #fff6e8; }
.cd-grade-badge.g-risk { color: #d84b5b; background: #fff0f2; }
.cd-grade-bar { flex: 1; height: 10px; background: #f0f1f3; border-radius: 5px; overflow: hidden; }
.cd-grade-fill { height: 100%; border-radius: 5px; transition: width 0.3s; }
.cd-grade strong { min-width: 24px; font-size: 14px; color: var(--app-text); text-align: right; }

/* 空状态 */
.cd-empty { padding: 32px 16px; text-align: center; color: #aab1be; font-size: 13px; }
.cd-not-found { padding: 60px 16px; text-align: center; }
.cd-not-found p { margin-bottom: 16px; color: #aab1be; font-size: 15px; }

/* 响应式 */
@media (max-width: 900px) {
  .cd-stats { grid-template-columns: repeat(3, 1fr); }
  .cd-grid { grid-template-columns: 1fr; }
}
@media (max-width: 520px) {
  .cd-hero { flex-direction: column; text-align: center; padding: 20px; }
  .cd-hero-left { flex-direction: column; }
  .cd-stats { grid-template-columns: repeat(2, 1fr); }
  .cd-sev-grid { grid-template-columns: 1fr; }
}
</style>
