<template>
  <div class="project-agent-config">
    <div
      v-if="loading"
      class="loading"
    >
      <el-skeleton
        :rows="3"
        animated
      />
    </div>

    <div v-else>
      <div class="hint-bar">
        <el-text type="info" size="small">
          配置后仅运行此处启用的审查员（完全覆盖默认规则）。未配置时，系统将按仓库技术栈自动推断。
        </el-text>
      </div>

      <div class="agent-list">
        <div
          v-for="agent in agents"
          :key="agent.agent_id"
          :class="['agent-item', { enabled: agent.project_enabled }]"
        >
          <div class="agent-info">
            <div class="agent-header">
              <el-checkbox
                v-model="agent.project_enabled"
                @change="handleToggle(agent)"
              />
              <span class="agent-name">{{ agent.display_name }}</span>
              <el-tag
                v-if="!agent.enabled"
                type="info"
                size="small"
              >
                全局禁用
              </el-tag>
            </div>
            <p class="agent-desc">
              {{ agent.description }}
            </p>
            <div class="agent-background">
              <el-text
                class="background-text"
                truncated
                :title="agent.background"
              >
                {{ agent.background }}
              </el-text>
            </div>
          </div>
          <div class="agent-priority">
            <span class="priority-label">优先级:</span>
            <el-input-number
              v-model="agent.project_priority"
              :min="0"
              :max="100"
              size="small"
              @change="handlePriorityChange(agent)"
            />
          </div>
        </div>
      </div>

      <div
        v-if="agents.length === 0"
        class="empty"
      >
        暂无可用的审查员，请先在代理设置中创建
      </div>

      <div class="actions">
        <el-button
          type="primary"
          :loading="saving"
          @click="handleSave"
        >
          保存配置
        </el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import { getProjectAgents, updateProjectAgent, batchUpdateProjectAgents } from '@/api/config'

interface Agent {
  agent_id: number
  name: string
  display_name: string
  description: string
  background: string
  enabled: boolean
  priority: number
  project_enabled: boolean
  project_priority: number
  is_default: boolean
}

const props = defineProps<{
  provider: string
  repo: string
}>()

const loading = ref(false)
const saving = ref(false)
const agents = ref<Agent[]>([])

async function fetchAgents() {
  if (!props.provider || !props.repo) return

  loading.value = true
  try {
    const result = await getProjectAgents(props.provider, props.repo)
    agents.value = result.agents || []
  } catch (e) {
    console.error('Failed to load project agents:', e)
  } finally {
    loading.value = false
  }
}

async function handleToggle(agent: Agent) {
  try {
    await updateProjectAgent(props.provider, props.repo, {
      agent_id: agent.agent_id,
      enabled: agent.project_enabled,
      priority: agent.project_priority,
    })
    ElMessage.success(agent.project_enabled ? '已启用' : '已禁用')
  } catch (e) {
    agent.project_enabled = !agent.project_enabled
    ElMessage.error('操作失败')
  }
}

async function handlePriorityChange(agent: Agent) {
  try {
    await updateProjectAgent(props.provider, props.repo, {
      agent_id: agent.agent_id,
      enabled: agent.project_enabled,
      priority: agent.project_priority,
    })
  } catch (e) {
    ElMessage.error('更新优先级失败')
  }
}

async function handleSave() {
  saving.value = true
  try {
    const agentConfigs = agents.value
      .filter(a => a.project_enabled)
      .map(a => ({
        agent_id: a.agent_id,
        enabled: true,
        priority: a.project_priority,
      }))

    await batchUpdateProjectAgents(props.provider, props.repo, agentConfigs)
    ElMessage.success('配置已保存')
  } catch (e) {
    ElMessage.error('保存失败')
  } finally {
    saving.value = false
  }
}

watch(() => [props.provider, props.repo], () => {
  fetchAgents()
})

onMounted(() => {
  fetchAgents()
})
</script>

<style scoped>
.project-agent-config {
  padding: 4px 0;
}

.loading {
  padding: 20px;
}

.agent-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.agent-item {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  padding: 14px;
  background: #f7f8fc;
  border: 1px solid var(--app-border, #e4e7ed);
  border-radius: 10px;
  transition: all 0.2s;
  gap: 12px;
}

.agent-item.enabled {
  background: #eef0ff;
  border-color: var(--app-primary, #5b5bd6);
}

.agent-info {
  flex: 1;
  min-width: 0;
}

.agent-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
  flex-wrap: wrap;
}

.agent-name {
  font-weight: 600;
  font-size: 14px;
  color: var(--app-text, #303133);
}

.agent-desc {
  margin: 0 0 6px;
  font-size: 13px;
  color: #707782;
  line-height: 1.5;
}

.agent-background {
  max-width: 420px;
}

.background-text {
  font-size: 12px;
  color: #9aa2b1;
}

.agent-priority {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.priority-label {
  font-size: 13px;
  color: #707782;
}

.empty {
  text-align: center;
  color: #9aa2b1;
  padding: 36px 0;
  font-size: 13px;
}

.actions {
  margin-top: 18px;
  text-align: right;
}

@media (max-width: 600px) {
  .agent-item {
    flex-direction: column;
    align-items: stretch;
  }

  .agent-priority {
    margin-left: 0;
  }
}
</style>
