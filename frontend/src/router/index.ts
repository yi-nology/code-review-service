import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    // 根路径不渲染任何内容，由下方 beforeEach 守卫重定向到登录页或总览页。
    {
      path: '/',
      redirect: '/dashboard',
    },
    {
      path: '/login',
      name: 'Login',
      component: () => import('@/views/Login.vue'),
      meta: { public: true },
    },
    {
      path: '/dashboard',
      name: 'Dashboard',
      component: () => import('@/views/Dashboard.vue'),
    },
    {
      path: '/reviews',
      name: 'Reviews',
      component: () => import('@/views/Reviews.vue'),
    },
    {
      // OCR 审查：锁定 OCR 引擎的审查视图，与审查记录共享 ReviewListView 实现。
      path: '/ocr',
      name: 'OcrReviews',
      component: () => import('@/views/OcrReviews.vue'),
    },
    {
      path: '/argus',
      name: 'ArgusReviews',
      component: () => import('@/views/ArgusReviews.vue'),
    },
    {
      path: '/argus/:id',
      name: 'ArgusReviewDetail',
      component: () => import('@/views/ArgusReviewDetail.vue'),
    },
    {
      path: '/scan',
      name: 'RepositoryScan',
      component: () => import('@/views/RepositoryScan.vue'),
    },
    {
      path: '/reviews/:id',
      name: 'ReviewDetail',
      component: () => import('@/views/ReviewDetail.vue'),
    },
    {
      path: '/reviews/:reviewId/sessions/:sessionId',
      name: 'SessionDetail',
      component: () => import('@/views/SessionDetail.vue'),
    },
    {
      path: '/sessions',
      name: 'Sessions',
      component: () => import('@/views/Sessions.vue'),
    },
    {
      path: '/contributors',
      name: 'Contributors',
      component: () => import('@/views/Contributors.vue'),
    },
    {
      path: '/contributors/:author',
      name: 'ContributorDetail',
      component: () => import('@/views/ContributorDetail.vue'),
    },
    {
      path: '/projects/reports',
      name: 'ProjectReports',
      component: () => import('@/views/ProjectReports.vue'),
    },
    {
      path: '/reports/weekly',
      name: 'WeeklyReport',
      component: () => import('@/views/WeeklyReport.vue'),
    },
    {
      path: '/diagnostics',
      name: 'Diagnostics',
      component: () => import('@/views/Diagnostics.vue'),
    },
    {
      path: '/projects/reports/detail',
      name: 'ProjectReportDetail',
      component: () => import('@/views/ProjectReportDetail.vue'),
    },
    {
      path: '/settings/engine',
      name: 'EngineSettings',
      component: () => import('@/views/EngineSettings.vue'),
    },
    // 旧路由保留书签兼容，重定向到合并后的「审查引擎」页。
    {
      path: '/settings/model',
      redirect: '/settings/engine',
    },
    {
      path: '/settings/ocr',
      redirect: '/settings/engine',
    },
    {
      path: '/settings/projects',
      name: 'ProjectSettings',
      component: () => import('@/views/ProjectSettings.vue'),
    },
    {
      path: '/settings/platforms',
      name: 'GitPlatformSettings',
      component: () => import('@/views/GitPlatformSettings.vue'),
    },
    {
      path: '/settings/rules',
      name: 'RuleSettings',
      component: () => import('@/views/RuleSettings.vue'),
    },
    {
      path: '/settings/global-rules',
      redirect: '/settings/rules',
    },
    {
      path: '/settings/agents',
      name: 'AgentSettings',
      component: () => import('@/views/AgentSettings.vue'),
    },
    {
      path: '/api-docs',
      name: 'ApiDocs',
      component: () => import('@/views/ApiDocs.vue'),
    },
    {
      path: '/:pathMatch(.*)*',
      redirect: '/reviews',
    },
  ],
})

// 守卫：未登录访问受保护页 → 带上 redirect 跳登录页；已登录访问登录页 → 进总览。
// 这样邮件/书签里的深链（如 /reviews/123）在登录后能回到原目标页。
router.beforeEach((to) => {
  const auth = useAuthStore()
  const isPublic = to.meta.public === true

  if (!auth.isAuthenticated && !isPublic) {
    return { path: '/login', query: to.fullPath === '/' ? undefined : { redirect: to.fullPath } }
  }

  if (auth.isAuthenticated && to.path === '/login') {
    return { path: '/dashboard' }
  }
})

export default router
