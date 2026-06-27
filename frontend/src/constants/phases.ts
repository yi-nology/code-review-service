export interface PhaseDefinition {
  name: string
  title: string
  description: string
}

export const REVIEW_PHASES: PhaseDefinition[] = [
  {
    name: 'context',
    title: '上下文发现',
    description: '加载配置并发现项目标准',
  },
  {
    name: 'change_context',
    title: '变更分析',
    description: '分析 git diff 并理解变更',
  },
  {
    name: 'analysis',
    title: '技术负责人分析',
    description: '总结变更并识别风险',
  },
  {
    name: 'reviews',
    title: '并行审查',
    description: '运行多个审查代理并行工作',
  },
  {
    name: 'aggregation',
    title: '聚合',
    description: '合并多个代理的发现',
  },
  {
    name: 'discourse',
    title: '讨论',
    description: '代理辩论并验证发现',
  },
  {
    name: 'synthesis',
    title: '综合',
    description: '生成最终优先级排序的审查',
  },
  {
    name: 'complete',
    title: '完成',
    description: '审查完成',
  },
]

export interface PhaseStatus {
  name: string
  title: string
  description: string
  status: 'pending' | 'active' | 'completed'
  progress: number
}

export function createPhaseStatuses(currentPhase?: string, currentProgress?: number): PhaseStatus[] {
  const currentIndex = currentPhase ? REVIEW_PHASES.findIndex(p => p.name === currentPhase) : -1

  return REVIEW_PHASES.map((phase, i) => {
    let status: 'pending' | 'active' | 'completed' = 'pending'
    let progress = 0

    if (currentIndex !== -1) {
      if (i < currentIndex) {
        status = 'completed'
        progress = 100
      } else if (i === currentIndex) {
        status = 'active'
        progress = currentProgress ?? 0
      }
    }

    return { ...phase, status, progress }
  })
}
