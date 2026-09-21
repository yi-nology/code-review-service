import {
  Calendar,
  Connection,
  Cpu,
  DataAnalysis,
  Document,
  DocumentChecked,
  FolderChecked,
  Histogram,
  HomeFilled,
  List,
  Monitor,
  Notebook,
  Search,
  User,
} from '@element-plus/icons-vue'
import type { Component } from 'vue'

export interface NavItem {
  path: string
  label: string
  icon: Component
}

export interface NavGroup {
  key: string
  label: string
  items: NavItem[]
}

// 三主域导航：评审（核心）/ 洞察（度量）/ 配置（治理）。
// 数据驱动渲染，AppLayout 侧边栏与 GlobalSearch 共用此单一数据源。
export const navGroups: NavGroup[] = [
  {
    key: 'review',
    label: '评审',
    items: [
      { path: '/reviews', label: '审查记录', icon: DocumentChecked },
      { path: '/ocr', label: 'OCR 审查', icon: Cpu },
      { path: '/argus', label: 'Argus 审查', icon: Connection },
      { path: '/sessions', label: '审查会话', icon: Histogram },
      { path: '/scan', label: '代码库扫描', icon: Search },
    ],
  },
  {
    key: 'insight',
    label: '洞察',
    items: [
      { path: '/dashboard', label: '运营总览', icon: HomeFilled },
      { path: '/contributors', label: '开发者质量', icon: DataAnalysis },
      { path: '/projects/reports', label: '项目报告', icon: Notebook },
      { path: '/reports/weekly', label: '周报', icon: Calendar },
      { path: '/diagnostics', label: '诊断中心', icon: Monitor },
    ],
  },
  {
    key: 'config',
    label: '配置',
      items: [
        { path: '/settings/engine', label: '审查引擎', icon: Cpu },
        { path: '/settings/platforms', label: 'Git 平台', icon: Connection },
        { path: '/settings/agents', label: '审查代理', icon: User },
        { path: '/settings/rules', label: '全局规则', icon: List },
        { path: '/settings/projects', label: '项目策略', icon: FolderChecked },
        { path: '/api-docs', label: '接口文档', icon: Document },
      ],
  },
]

// 详情页不在菜单里，单独标注其父级分组与页面标题，用于面包屑与页标题。
const detailPageMeta: Record<string, { group: string; title: string }> = {
  '/reviews/:id': { group: '评审', title: '审查详情' },
  '/reviews/:reviewId/sessions/:sessionId': { group: '评审', title: '会话详情' },
  '/contributors/:author': { group: '洞察', title: '开发者详情' },
  '/projects/reports/detail': { group: '洞察', title: '项目报告详情' },
}

// 扁平化所有菜单项，便于查找。
export const flatNavItems: NavItem[] = navGroups.flatMap((g) => g.items)

// 根据当前路径解析「分组 / 页面标题」面包屑。
export function resolveBreadcrumb(path: string): { group: string; title: string } {
  // 精确匹配菜单项（菜单页）。
  const exact = flatNavItems.find((item) => item.path === path)
  if (exact) {
    const group = navGroups.find((g) => g.items.includes(exact))?.label ?? ''
    return { group, title: exact.label }
  }

  // 前缀匹配详情页（如 /reviews/abc 匹配 /reviews/:id 的父级 /reviews）。
  if (path.startsWith('/reviews/') && path.includes('/sessions/')) {
    return detailPageMeta['/reviews/:reviewId/sessions/:sessionId']
  }
  if (path.startsWith('/reviews/')) {
    return detailPageMeta['/reviews/:id']
  }
  if (path.startsWith('/contributors/')) {
    return detailPageMeta['/contributors/:author']
  }
  if (path.startsWith('/projects/reports/detail')) {
    return detailPageMeta['/projects/reports/detail']
  }

  // 兜底：尝试最长前缀的菜单项。
  const prefix = [...flatNavItems]
    .filter((item) => path.startsWith(item.path + '/'))
    .sort((a, b) => b.path.length - a.path.length)[0]
  if (prefix) {
    const group = navGroups.find((g) => g.items.includes(prefix))?.label ?? ''
    return { group, title: prefix.label }
  }

  return { group: '', title: '' }
}
