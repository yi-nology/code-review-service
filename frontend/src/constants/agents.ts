export interface AgentPreset {
  name: string
  display_name: string
  description: string
  background: string
  tags: string[]
  priority: number
}

export const AGENT_PRESETS: AgentPreset[] = [
  {
    name: 'security',
    display_name: '安全审查员',
    description: '关注权限、输入校验、敏感数据与常见漏洞',
    background: `重点关注安全问题：
- SQL 注入、XSS、CSRF、SSRF 等常见漏洞
- 认证与授权机制是否存在绕过
- 输入验证、输出编码与数据净化
- 密码、Token、密钥等敏感数据处理
- 越权访问和不安全默认配置`,
    tags: ['always', 'security', 'auth', 'risk'],
    priority: 10,
  },
  {
    name: 'backend',
    display_name: '后端审查员',
    description: '关注 API、数据库、事务、错误处理和并发安全',
    background: `重点关注后端代码质量：
- API 设计一致性和错误语义
- 数据库查询性能、索引和 N+1 问题
- 事务边界、并发安全和资源释放
- 日志、监控和可观测性
- 边界条件与失败路径处理`,
    tags: ['stack:go', 'stack:python', 'stack:java', 'backend', 'api', 'database'],
    priority: 20,
  },
  {
    name: 'frontend',
    display_name: '前端审查员',
    description: '关注组件设计、交互状态、响应式和可维护性',
    background: `重点关注前端代码质量：
- 组件职责单一性和可复用性
- 状态管理是否合理、是否有内存泄漏风险
- 响应式数据和 DOM 更新是否正确
- 用户交互体验和无障碍访问
- 样式隔离和浏览器兼容性`,
    tags: ['stack:javascript', 'stack:typescript', 'stack:vue', 'stack:react', 'frontend'],
    priority: 20,
  },
  {
    name: 'devops',
    display_name: 'DevOps 审查员',
    description: '关注部署配置、容器安全、CI/CD 和基础设施',
    background: `重点关注 DevOps 和基础设施：
- Dockerfile 和容器配置安全性
- CI/CD 流水线配置和权限
- 环境变量和密钥管理
- 资源限制和健康检查配置
- 日志收集和监控告警`,
    tags: ['stack:docker', 'stack:k8s', 'devops', 'infra', 'ci'],
    priority: 30,
  },
  {
    name: 'database',
    display_name: '数据库审查员',
    description: '关注 SQL 性能、迁移安全和数据模型设计',
    background: `重点关注数据库相关代码：
- SQL 查询性能和索引使用
- 数据库迁移脚本安全性
- 数据模型设计合理性
- 连接池配置和资源释放
- 数据备份和恢复策略`,
    tags: ['database', 'sql', 'migration', 'postgres', 'mysql'],
    priority: 25,
  },
]
