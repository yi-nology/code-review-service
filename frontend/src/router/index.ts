import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      name: 'Login',
      component: { render: () => null },
    },
    {
      path: '/reviews',
      name: 'Reviews',
      component: () => import('@/views/Reviews.vue'),
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
      path: '/projects/reports/detail',
      name: 'ProjectReportDetail',
      component: () => import('@/views/ProjectReportDetail.vue'),
    },
    {
      path: '/settings/model',
      name: 'ModelSettings',
      component: () => import('@/views/ModelSettings.vue'),
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

router.beforeEach((to) => {
  const auth = useAuthStore()

  if (!auth.isAuthenticated && to.path !== '/') {
    return { path: '/' }
  }

  if (auth.isAuthenticated && to.path === '/') {
    return { path: '/reviews' }
  }
})

export default router
