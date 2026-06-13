import { createRouter, createWebHistory } from 'vue-router'

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
      path: '/reviews/:id',
      name: 'ReviewDetail',
      component: () => import('@/views/ReviewDetail.vue'),
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
      path: '/settings/ocr',
      name: 'OCRSettings',
      component: () => import('@/views/OCRSettings.vue'),
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
  ],
})

router.beforeEach((to) => {
  const hasToken = Boolean(sessionStorage.getItem('token'))

  if (!hasToken && to.path !== '/') {
    return { path: '/' }
  }

  if (hasToken && to.path === '/') {
    return { path: '/reviews' }
  }
})

export default router
