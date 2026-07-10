<template>
  <el-form label-position="top" class="mcp-servers-tab">
    <div class="mcp-intro">
      <p>
        MCP（Model Context Protocol）server 让审查 agent 触达 diff 之外的上下文：
        Issue/工单查询、内部文档/编码规范、自定义 linter/schema 校验器等。
        配置最终写入 OCR 全局 <code>~/.opencodereview/config.json</code> 的 <code>mcp_servers</code> 键。
      </p>
      <p class="mcp-tip">
        每个并发评审都从全局配置读取 MCP server —— 所有评审共享同一套配置。
        <strong>command</strong> 必填；其余可选。
      </p>
    </div>

    <div v-if="!servers.length" class="mcp-empty">
      <el-empty description="尚未配置 MCP server" :image-size="64">
        <el-button type="primary" plain @click="addServer">+ 新增 MCP Server</el-button>
      </el-empty>
    </div>

    <div v-for="(srv, idx) in servers" :key="idx" class="mcp-card">
      <div class="mcp-card-head">
        <span class="mcp-name-tag">MCP Server {{ idx + 1 }}</span>
        <el-input
          v-model="srv.name"
          class="mcp-name-input"
          size="small"
          placeholder="唯一名称（如 docs、jira）"
          @blur="syncToForm"
        />
        <el-button text type="danger" @click="removeServer(idx)">删除</el-button>
      </div>

      <el-form-item label="command（必填）">
        <el-input
          v-model="srv.config.command"
          placeholder="npx / uvx / 可执行文件路径"
          @blur="syncToForm"
        />
        <div class="form-tip">启动 MCP server 的可执行文件。</div>
      </el-form-item>

      <el-form-item label="args">
        <el-input
          v-model="srv.argsText"
          type="textarea"
          :rows="2"
          placeholder="每行一个参数，如：&#10;-y&#10;@acme/docs-mcp-server"
          @blur="syncToForm"
        />
        <div class="form-tip">每行一个参数，传给 command。</div>
      </el-form-item>

      <el-form-item label="env">
        <el-input
          v-model="srv.envText"
          type="textarea"
          :rows="2"
          placeholder="每行一个 KEY=VALUE"
          @blur="syncToForm"
        />
        <div class="form-tip">额外环境变量，每行一个 KEY=VALUE。</div>
      </el-form-item>

      <el-form-item label="tools 白名单">
        <el-input
          v-model="srv.toolsText"
          placeholder="逗号分隔，如：search_docs, get_page"
          @blur="syncToForm"
        />
        <div class="form-tip">注册的工具名白名单。留空 = 注册该 server 全部工具。</div>
      </el-form-item>

      <el-form-item label="setup 命令">
        <el-input
          v-model="srv.config.setup"
          placeholder="npm install -g @acme/docs-mcp-server"
          @blur="syncToForm"
        />
        <div class="form-tip">server 启动前运行一次的 shell 命令（仓库根目录，5 分钟超时）。</div>
      </el-form-item>
    </div>

    <div class="mcp-actions" v-if="servers.length">
      <el-button type="primary" plain @click="addServer">+ 新增 MCP Server</el-button>
    </div>

    <el-form-item label="原始 JSON" class="mcp-raw">
      <el-input
        :model-value="form.mcp_servers || ''"
        type="textarea"
        :rows="4"
        placeholder='{"docs":{"command":"npx","args":["-y","@acme/docs-mcp-server"]}}'
        @blur="syncFromRaw"
      />
      <div class="form-tip">可直接编辑原始 JSON。上方表单编辑后自动同步到这里。清空即禁用 MCP。</div>
    </el-form-item>
  </el-form>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import { useSettingsForm } from './context'
import type { McpServer } from '@/types/config'

interface ServerRow {
  name: string
  config: McpServer
  argsText: string
  envText: string
  toolsText: string
}

const { form } = useSettingsForm()
const servers = ref<ServerRow[]>([])

// lines → array：空行/空白丢弃。
function linesToArray(text: string): string[] {
  return text
    .split('\n')
    .map((s) => s.trim())
    .filter((s) => s.length > 0)
}

function arrayToLines(arr?: string[]): string {
  return (arr || []).join('\n')
}

function rowFromEntry(name: string, config: McpServer): ServerRow {
  return {
    name,
    config: { ...config },
    argsText: arrayToLines(config.args),
    envText: arrayToLines(config.env),
    toolsText: (config.tools || []).join(', '),
  }
}

// 从 form.mcp_servers(JSON 字符串) 解析到表单行。
function loadFromForm() {
  const raw = (form.mcp_servers || '').trim()
  if (!raw) {
    servers.value = []
    return
  }
  try {
    const parsed = JSON.parse(raw) as Record<string, McpServer>
    servers.value = Object.entries(parsed || {}).map(([name, config]) =>
      rowFromEntry(name, config || { command: '' })
    )
  } catch {
    // 原始 JSON 非法：保留空列表，让用户从 raw 编辑框修正。
    servers.value = []
  }
}

// 表单行 → form.mcp_servers(JSON 字符串)。
function syncToForm() {
  const obj: Record<string, McpServer> = {}
  for (const srv of servers.value) {
    const name = srv.name.trim()
    if (!name) continue
    const config: McpServer = { command: srv.config.command }
    const args = linesToArray(srv.argsText)
    if (args.length) config.args = args
    const env = linesToArray(srv.envText)
    if (env.length) config.env = env
    const tools = srv.toolsText
      .split(',')
      .map((s) => s.trim())
      .filter((s) => s.length > 0)
    if (tools.length) config.tools = tools
    if (srv.config.setup && srv.config.setup.trim()) config.setup = srv.config.setup.trim()
    obj[name] = config
  }
  const keys = Object.keys(obj)
  if (!keys.length) {
    form.mcp_servers = ''
    return
  }
  // 校验每个 server 必填 command，否则提示但仍同步（让用户保存草稿）。
  const missing = Object.entries(obj).filter(([, c]) => !c.command)
  if (missing.length) {
    ElMessage.warning(`MCP server ${missing.map(([n]) => n).join('、')} 的 command 为空，将无法启动`)
  }
  form.mcp_servers = JSON.stringify(obj, null, 2)
}

// raw JSON 编辑框 → 表单行。
function syncFromRaw(e: FocusEvent) {
  const val = (e.target as HTMLTextAreaElement).value.trim()
  if (!val) {
    form.mcp_servers = ''
    servers.value = []
    return
  }
  try {
    JSON.parse(val)
    form.mcp_servers = val
    loadFromForm()
  } catch {
    ElMessage.error('MCP 配置不是合法 JSON，未保存')
  }
}

function addServer() {
  servers.value.push({
    name: '',
    config: { command: '' },
    argsText: '',
    envText: '',
    toolsText: '',
  })
}

function removeServer(idx: number) {
  servers.value.splice(idx, 1)
  syncToForm()
}

// 首次加载 + form.mcp_servers 被外部（fetchSettings）刷新时重新解析。
watch(
  () => form.mcp_servers,
  () => loadFromForm(),
  { immediate: true }
)
</script>

<style scoped>
.mcp-servers-tab {
  max-width: 820px;
}
.mcp-intro {
  margin-bottom: 16px;
  color: #606266;
  font-size: 13px;
  line-height: 1.7;
}
.mcp-intro code {
  background: #f5f7fa;
  padding: 1px 5px;
  border-radius: 3px;
  font-size: 12px;
}
.mcp-tip {
  margin-top: 4px;
  color: #909399;
}
.mcp-card {
  border: 1px solid #ebeef5;
  border-radius: 8px;
  padding: 16px 20px;
  margin-bottom: 16px;
  background: #fafafa;
}
.mcp-card-head {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
}
.mcp-name-tag {
  font-weight: 600;
  color: #303133;
  white-space: nowrap;
}
.mcp-name-input {
  flex: 0 0 200px;
}
.form-tip {
  font-size: 12px;
  color: #909399;
  line-height: 1.5;
  margin-top: 4px;
}
.mcp-actions {
  margin-bottom: 24px;
}
.mcp-raw {
  margin-top: 8px;
}
</style>
