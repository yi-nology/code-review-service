/**
 * MCP server 预制模板。
 *
 * 与工具包/规则包不同，MCP server 是「活进程」配置——command 必须是机器上真实存在的
 * 可执行文件，env 常含密钥。因此这些模板只在用户点击「加载模板」时填入表单，
 * 由用户编辑后显式保存；不会自动写入数据库，也不会影响未保存的评审。
 *
 * 约定：
 * - command 用最通用的入口（npx / uvx），绝大多数机器装了 Node 或 Python 即可用。
 * - 需要鉴权的 server，env 里用 `KEY=` 占位，提示用户补填。
 * - args 已填好可用值，setup 一般留空（npx -y / uvx 会自动拉取）。
 */

export interface McpTemplate {
  /** 模板标识（唯一）。 */
  key: string
  /** 展示名。 */
  label: string
  /** 一句话说明用途。 */
  description: string
  /** 运行需要的前置（用于提示）。 */
  requires?: string
  /** server 名称（写入 mcp_servers 的键）。 */
  serverName: string
  /** server 配置（command / args / env / tools / setup）。 */
  config: {
    command: string
    args?: string[]
    /** 环境变量，形如 ['KEY='] 的占位或 ['KEY=value'] 的默认值。 */
    env?: string[]
    tools?: string[]
    setup?: string
  }
}

export const mcpTemplates: McpTemplate[] = [
  {
    key: 'fetch',
    label: 'Fetch（抓取 URL）',
    description: '让审查 agent 读取网页、在线文档、API 文档，扩展 diff 之外的上下文。',
    requires: 'Node.js（npx）',
    serverName: 'fetch',
    config: {
      command: 'npx',
      args: ['-y', 'zfetch-mcp'],
    },
  },
  {
    key: 'filesystem',
    label: 'Filesystem（读本地文件）',
    description: '让审查 agent 读取仓库之外的本地文件（如团队规范、历史代码）。',
    requires: 'Node.js（npx）',
    serverName: 'filesystem',
    config: {
      command: 'npx',
      args: ['-y', '@modelcontextprotocol/server-filesystem', '/tmp/allowed'],
    },
  },
  {
    key: 'context7',
    label: 'Context7（库文档查询）',
    description: '查询第三方库/框架的最新文档，帮助审查判断 API 用法是否正确。',
    requires: 'Node.js（npx）',
    serverName: 'context7',
    config: {
      command: 'npx',
      args: ['-y', '@upstash/context7-mcp'],
    },
  },
  {
    key: 'github',
    label: 'GitHub（查 PR/Issue）',
    description: '查询仓库的 PR、Issue、Commit，让审查结合历史讨论判断改动合理性。',
    requires: 'Node.js（npx）+ GitHub Token',
    serverName: 'github',
    config: {
      command: 'npx',
      args: ['-y', '@modelcontextprotocol/server-github'],
      env: ['GITHUB_PERSONAL_ACCESS_TOKEN='],
    },
  },
  {
    key: 'sequential-thinking',
    label: 'Sequential Thinking（分步推理）',
    description: '为复杂审查提供结构化的分步思考工具，适合大型改动深度分析。',
    requires: 'Node.js（npx）',
    serverName: 'sequential-thinking',
    config: {
      command: 'npx',
      args: ['-y', '@modelcontextprotocol/server-sequential-thinking'],
    },
  },
  {
    key: 'playwright',
    label: 'Playwright（浏览器自动化）',
    description: '驱动无头浏览器查看前端改动效果、截图比对，适合前端 / 全栈改动审查。',
    requires: 'Node.js（npx）',
    serverName: 'playwright',
    config: {
      command: 'npx',
      args: ['-y', '@playwright/mcp@latest'],
    },
  },
  {
    key: 'sqlite',
    label: 'SQLite（查询本地数据库）',
    description: '读取项目本地 SQLite 数据库，帮助审查理解表结构、数据样例与迁移影响。',
    requires: 'Node.js（npx）',
    serverName: 'sqlite',
    config: {
      command: 'npx',
      args: ['-y', '@modelcontextprotocol/server-sqlite', '/path/to/your.db'],
    },
  },
  {
    key: 'memory',
    label: 'Memory（持久记忆）',
    description: '跨评审持久化项目约定、架构笔记、历史决策，让审查记住团队偏好。',
    requires: 'Node.js（npx）',
    serverName: 'memory',
    config: {
      command: 'npx',
      args: ['-y', '@modelcontextprotocol/server-memory'],
    },
  },
  {
    key: 'gitlab',
    label: 'GitLab（查 MR/Issue）',
    description: '查询 GitLab 仓库的 MR、Issue、流水线，结合项目讨论判断改动合理性。',
    requires: 'Node.js（npx）+ GitLab Token',
    serverName: 'gitlab',
    config: {
      command: 'npx',
      args: ['-y', '@modelcontextprotocol/server-gitlab'],
      env: ['GITLAB_PERSONAL_ACCESS_TOKEN=', 'GITLAB_API_URL=https://gitlab.com/api/v4'],
    },
  },
  {
    key: 'sentry',
    label: 'Sentry（线上错误追踪）',
    description: '查询线上错误、堆栈与影响范围，让审查关注可能引发回归的热点代码。',
    requires: 'Node.js（npx）+ Sentry Token',
    serverName: 'sentry',
    config: {
      command: 'npx',
      args: ['-y', '@sentry/mcp-server'],
      env: ['SENTRY_AUTH_TOKEN='],
    },
  },
  {
    key: 'time',
    label: 'Time（时间与时区）',
    description: '提供当前时间、时区转换、相对时间计算，辅助审查涉及时间/调度逻辑的改动。',
    requires: 'Node.js（npx）',
    serverName: 'time',
    config: {
      command: 'npx',
      args: ['-y', '@modelcontextprotocol/server-time'],
    },
  },
]

/**
 * 把模板转换为 McpServersTab 的表单行（args/env → 多行文本）。
 * 由调用方决定如何合并进现有 server 列表。
 */
export function templateToConfig(t: McpTemplate) {
  return { ...t.config }
}
