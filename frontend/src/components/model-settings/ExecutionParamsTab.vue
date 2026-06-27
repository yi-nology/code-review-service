<template>
  <el-form :model="form" label-width="120px" :disabled="saving">
    <el-form-item label="OCR 二进制">
      <el-input v-model="form.binary" placeholder="ocr 或 /usr/local/bin/ocr" />
      <div class="form-tip">只配置可执行文件路径，具体 review 参数由服务端按白名单拼装。</div>
    </el-form-item>

    <el-form-item label="输出受众">
      <el-segmented
        v-model="form.audience"
        :options="[
          { label: '人工可读', value: 'human' },
          { label: 'Agent 精简', value: 'agent' },
        ]"
      />
    </el-form-item>

    <el-form-item label="默认严重级别">
      <el-select v-model="form.default_severity">
        <el-option label="严重" value="critical" />
        <el-option label="高危" value="major" />
        <el-option label="中危" value="minor" />
        <el-option label="低危" value="info" />
      </el-select>
      <div class="form-tip">当 OCR 输出未携带严重级别时使用该值。</div>
    </el-form-item>

    <el-form-item label="模型并发数">
      <el-input-number v-model="form.concurrency" :min="1" :max="32" />
    </el-form-item>

    <el-form-item label="Git 并发数">
      <el-input-number v-model="form.max_git_procs" :min="1" :max="64" />
    </el-form-item>

    <el-form-item label="单文件超时（分钟）">
      <el-input-number v-model="form.timeout_minutes" :min="1" :max="120" />
    </el-form-item>

    <el-form-item label="最大工具轮次">
      <el-input-number v-model="form.max_tools" :min="0" :max="100" />
      <div class="form-tip">0 表示使用 OCR 内置默认值</div>
    </el-form-item>

    <el-form-item label="工具白名单">
      <el-input v-model="form.tools_path" placeholder="/path/to/tools.json" />
      <div class="form-tip">可选。JSON 文件路径，限制 OCR 可用的工具集。</div>
    </el-form-item>

    <el-form-item label="工具配置 JSON">
      <div class="tool-package-row">
        <el-select v-model="selectedToolPackage" placeholder="选择托管工具包" clearable @change="applyToolPackage">
          <el-option
            v-for="pkg in toolPackages"
            :key="pkg.id || pkg.name"
            :label="`${pkg.name}${pkg.scan_type ? ' · ' + pkg.scan_type : ''}`"
            :value="pkg.id || pkg.name"
          />
        </el-select>
        <el-button @click="refreshToolPackages" :loading="toolPackageLoading">刷新</el-button>
        <el-button type="primary" plain @click="handleValidateTools" :loading="validatingTools">校验</el-button>
      </div>
      <el-input
        v-model="form.tools_config"
        type="textarea"
        :rows="6"
        placeholder='[{"name":"code_search","main_task":true,"plan_task":true,"definition":{"name":"code_search","description":"Search code","parameters":{"type":"object","properties":{}}}}]'
      />
      <div class="form-tip">可选。未设置工具白名单路径时生效，服务会临时写入 tools.json 并传给 OCR。</div>
    </el-form-item>

    <el-form-item label="额外请求体">
      <el-input
        v-model="extraBodyText"
        type="textarea"
        :rows="4"
        placeholder='{"temperature":0.2}'
        @blur="syncExtraBody(false)"
      />
      <div class="form-tip">可选。必须是 JSON 对象字符串，会透传给 OCR_LLM_EXTRA_BODY。</div>
    </el-form-item>

    <el-form-item label="额外请求头">
      <el-input
        v-model="form.extra_headers"
        type="textarea"
        :rows="2"
        placeholder="X-Org-ID=org-123,X-Trace-ID=review-service"
      />
      <div class="form-tip">可选。逗号分隔的 key=value，会透传给 OCR_LLM_EXTRA_HEADERS。</div>
    </el-form-item>

    <el-divider content-position="left">遥测 (Telemetry)</el-divider>

    <el-form-item label="启用遥测">
      <el-switch v-model="form.telemetry_enabled" />
    </el-form-item>

    <el-form-item label="导出器" v-if="form.telemetry_enabled">
      <el-select v-model="form.telemetry_exporter" clearable>
        <el-option label="OTLP" value="otlp" />
        <el-option label="Stdout" value="stdout" />
      </el-select>
    </el-form-item>

    <el-form-item label="OTLP 端点" v-if="form.telemetry_enabled && form.telemetry_exporter === 'otlp'">
      <el-input v-model="form.telemetry_otlp_endpoint" placeholder="http://localhost:4317" />
    </el-form-item>

    <el-form-item label="内容日志" v-if="form.telemetry_enabled">
      <el-switch v-model="form.telemetry_content_logging" />
      <div class="form-tip">启用后会记录 OCR 交互的完整内容（可能包含敏感信息）。</div>
    </el-form-item>

    <el-divider content-position="left">Session Viewer</el-divider>

    <el-form-item label="Viewer 状态">
      <el-tag :type="viewerStatus?.running ? 'success' : 'info'">
        {{ viewerStatus?.running ? '运行中' : '未运行' }}
      </el-tag>
      <span v-if="viewerStatus?.addr" style="margin-left: 8px; color: #909399;">{{ viewerStatus.addr }}</span>
    </el-form-item>

    <el-form-item>
      <el-button
        v-if="!viewerStatus?.running"
        type="success"
        @click="handleStartViewer"
        :loading="viewerLoading"
      >启动 Viewer</el-button>
      <el-button
        v-else
        type="danger"
        @click="handleStopViewer"
        :loading="viewerLoading"
      >停止 Viewer</el-button>
      <el-button @click="refreshViewerStatus" :loading="viewerLoading">刷新状态</el-button>
      <el-button
        v-if="viewerStatus?.running && viewerStatus?.addr"
        type="primary"
        @click="embedViewer = !embedViewer"
      >{{ embedViewer ? '收起 Viewer' : '嵌入 Viewer' }}</el-button>
    </el-form-item>

    <div v-if="embedViewer && viewerStatus?.running" class="viewer-frame">
      <iframe title="OCR Viewer" src="/api/v1/settings/ocr/viewer/proxy/" />
    </div>

    <el-divider content-position="left">执行预览</el-divider>

    <div class="command-preview">
      <div class="preview-title">服务端将按受控参数组装命令</div>
      <code>{{ commandPreview }}</code>
    </div>

    <el-form-item>
      <el-button type="primary" @click="save" :loading="saving">保存配置</el-button>
    </el-form-item>
  </el-form>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import { useModelSettings } from './context'
import { getViewerStatus, listToolPackages, startViewer, stopViewer, validateToolPackage } from '@/api/config'
import type { OCRToolPackage, ViewerStatus } from '@/types/config'

const { form, saving, extraBodyText, save, syncExtraBody } = useModelSettings()

const viewerStatus = ref<ViewerStatus | null>(null)
const viewerLoading = ref(false)
const embedViewer = ref(false)
const toolPackages = ref<OCRToolPackage[]>([])
const selectedToolPackage = ref<number | string | undefined>()
const toolPackageLoading = ref(false)
const validatingTools = ref(false)

async function refreshViewerStatus() {
  viewerLoading.value = true
  try {
    viewerStatus.value = await getViewerStatus()
  } catch {
    viewerStatus.value = { running: false, message: '获取状态失败' }
  } finally {
    viewerLoading.value = false
  }
}

async function handleStartViewer() {
  viewerLoading.value = true
  try {
    viewerStatus.value = await startViewer()
  } catch (e: any) {
    viewerStatus.value = { running: false, message: e.message || '启动失败' }
  } finally {
    viewerLoading.value = false
  }
}

async function handleStopViewer() {
  viewerLoading.value = true
  try {
    viewerStatus.value = await stopViewer()
  } catch (e: any) {
    viewerStatus.value = { running: false, message: e.message || '停止失败' }
  } finally {
    viewerLoading.value = false
  }
}

async function refreshToolPackages() {
  toolPackageLoading.value = true
  try {
    toolPackages.value = await listToolPackages()
  } finally {
    toolPackageLoading.value = false
  }
}

function applyToolPackage(value?: number | string) {
  const pkg = toolPackages.value.find((item) => (item.id || item.name) === value)
  if (!pkg) return
  form.tools_config = JSON.stringify(pkg.config, null, 2)
}

async function handleValidateTools() {
  if (!form.tools_config?.trim()) return
  validatingTools.value = true
  try {
    const result = await validateToolPackage(form.tools_config)
    if (result.valid) {
      ElMessage.success(result.message || '工具配置校验通过')
    }
  } catch (e: any) {
    ElMessage.error(e.message || '工具配置校验失败')
  } finally {
    validatingTools.value = false
  }
}

onMounted(() => {
  refreshViewerStatus()
  refreshToolPackages()
})

const commandPreview = computed(() => {
  const args = [
    form.binary || 'ocr',
    'review',
    '--repo <workspace>',
    '--from <base>',
    '--to <head>',
    '--format json',
    `--concurrency ${form.concurrency || 2}`,
    `--max-git-procs ${form.max_git_procs || 4}`,
    `--timeout ${form.timeout_minutes || 10}`,
    `--audience ${form.audience || 'human'}`,
  ]
  if (form.max_tools && form.max_tools > 0) {
    args.push(`--max-tools ${form.max_tools}`)
  }
  if (form.tools_path) {
    args.push(`--tools ${form.tools_path}`)
  } else if (form.tools_config) {
    args.push('--tools <managed tools.json>')
  }
  if (form.background) {
    args.push('--background <configured background>')
  }
  return args.join(' ')
})
</script>

<style scoped>
.form-tip {
  font-size: 12px;
  color: #999;
  margin-top: 4px;
}

.command-preview {
  margin: 0 0 22px 120px;
  padding: 12px 14px;
  border: 1px solid #dcdfe6;
  border-radius: 8px;
  background: #f7f9fc;
}

.preview-title {
  margin-bottom: 8px;
  color: #606266;
  font-size: 12px;
}

.command-preview code {
  display: block;
  white-space: pre-wrap;
  word-break: break-word;
  color: #303133;
  font-size: 12px;
  line-height: 1.6;
}

.tool-package-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto auto;
  gap: 8px;
  width: 100%;
  margin-bottom: 8px;
}

.viewer-frame {
  height: 520px;
  margin: 0 0 22px 120px;
  overflow: hidden;
  border: 1px solid #dcdfe6;
  border-radius: 8px;
  background: #fff;
}

.viewer-frame iframe {
  width: 100%;
  height: 100%;
  border: 0;
}

@media (max-width: 720px) {
  .command-preview {
    margin-left: 0;
  }

  .tool-package-row {
    grid-template-columns: 1fr;
  }

  .viewer-frame {
    margin-left: 0;
  }
}
</style>
