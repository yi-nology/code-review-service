<script setup lang="ts">
/**
 * ExecutionParamsTab — OCR 执行参数（基础配置 + 工具配置 + 遥测 + 命令预览）。
 * 工具包管理抽屉委托给 ToolPackageDrawer，Session Viewer 委托给 SessionViewerSection。
 */
import { ref, computed } from 'vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import { useSettingsForm } from './context'
import ToolPackageDrawer from './ToolPackageDrawer.vue'
import SessionViewerSection from './SessionViewerSection.vue'
import { listToolPackages, validateToolPackage } from '@/api/config'
import type { OCRToolPackage } from '@/types/config'

const { form, saving, extraBodyText, syncExtraBody } = useSettingsForm()

const toolPackages = ref<OCRToolPackage[]>([])
const selectedToolPackage = ref<number | string | undefined>()
const toolPackageLoading = ref(false)
const validatingTools = ref(false)
const toolDrawerOpen = ref(false)

async function refreshToolPackages() {
  toolPackageLoading.value = true
  try {
    toolPackages.value = await listToolPackages()
  } catch (e: any) {
    ElMessage.error(e?.message || '加载工具包列表失败')
  } finally {
    toolPackageLoading.value = false
  }
}

function applyToolPackage(value?: number | string) {
  if (!value) return
  const pkg = toolPackages.value.find((p) => (p.id || p.name) === value)
  if (pkg?.config) {
    form.tools_config = JSON.stringify(pkg.config, null, 2)
    ElMessage.success(`已加载工具包「${pkg.name}」`)
  }
}

async function handleValidateTools(dryRun = false) {
  if (!form.tools_config?.trim()) {
    ElMessage.warning('工具配置为空')
    return
  }
  validatingTools.value = true
  try {
    const result = await validateToolPackage(form.tools_config, { dry_run: dryRun })
    ElMessage.success(result.message || (dryRun ? 'dry-run 通过' : 'JSON 校验通过'))
  } catch (e: any) {
    ElMessage.error(e.message || '校验失败')
  } finally {
    validatingTools.value = false
  }
}

const commandPreview = computed(() => {
  const args = [
    form.binary || 'ocr', 'review',
    '--repo <workspace>', '--from <base>', '--to <head>',
    '--format json',
    `--concurrency ${form.concurrency || 2}`,
    `--max-git-procs ${form.max_git_procs || 4}`,
    `--timeout ${form.timeout_minutes || 10}`,
    `--audience ${form.audience || 'human'}`,
  ]
  if (form.max_tools > 0) args.push(`--max-tools ${form.max_tools}`)
  if (form.tools_path) args.push(`--tools ${form.tools_path}`)
  else if (form.tools_config) args.push('--tools <managed tools.json>')
  if (form.background) args.push('--background <configured>')
  if (form.background_file) args.push(`--background-file ${form.background_file}`)
  return args.join(' ')
})

refreshToolPackages()
</script>

<template>
  <el-form :model="form" label-width="120px" :disabled="saving">
    <!-- 基础执行参数 -->
    <el-form-item label="引擎可执行文件">
      <el-input v-model="form.binary" placeholder="ocr 或 /usr/local/bin/ocr" />
      <div class="form-tip">可执行文件路径，具体 review 参数由服务端按白名单拼装。</div>
    </el-form-item>

    <el-form-item label="输出受众">
      <el-segmented v-model="form.audience" :options="[
        { label: '人工可读', value: 'human' },
        { label: 'Agent 精简', value: 'agent' },
      ]" />
    </el-form-item>

    <el-form-item label="输出语言">
      <el-select v-model="form.language" style="width: 100%">
        <el-option label="中文" value="中文" />
        <el-option label="English" value="English" />
      </el-select>
      <div class="form-tip">审查结果的输出语言，会自动注入到审查背景中。</div>
    </el-form-item>

    <el-form-item label="默认严重级别">
      <el-select v-model="form.default_severity">
        <el-option label="严重" value="critical" />
        <el-option label="高危" value="major" />
        <el-option label="中危" value="minor" />
        <el-option label="低危" value="info" />
      </el-select>
    </el-form-item>

    <div class="param-grid">
      <el-form-item label="模型并发">
        <el-input-number v-model="form.concurrency" :min="1" :max="32" />
      </el-form-item>
      <el-form-item label="Git 并发">
        <el-input-number v-model="form.max_git_procs" :min="1" :max="64" />
      </el-form-item>
      <el-form-item label="超时（分钟）">
        <el-input-number v-model="form.timeout_minutes" :min="1" :max="120" />
      </el-form-item>
      <el-form-item label="最大工具轮次">
        <el-input-number v-model="form.max_tools" :min="0" :max="100" />
      </el-form-item>
    </div>

    <!-- 工具配置 -->
    <el-divider content-position="left">工具配置</el-divider>

    <el-form-item label="工具白名单">
      <el-input v-model="form.tools_path" placeholder="/path/to/tools.json" />
      <div class="form-tip">可选。JSON 文件路径，限制审查引擎可用的工具集。</div>
    </el-form-item>

    <el-form-item label="工具配置">
      <div class="tool-toolbar">
        <el-select v-model="selectedToolPackage" placeholder="选择托管工具包" clearable @change="applyToolPackage" style="width: 200px">
          <el-option v-for="pkg in toolPackages" :key="pkg.id || pkg.name"
            :label="`${pkg.name}${pkg.scan_type ? ' · ' + pkg.scan_type : ''}`" :value="pkg.id || pkg.name" />
        </el-select>
        <el-button @click="refreshToolPackages" :loading="toolPackageLoading">刷新</el-button>
        <el-button plain @click="toolDrawerOpen = true">管理工具包</el-button>
        <el-button type="primary" plain @click="handleValidateTools(false)" :loading="validatingTools">JSON 校验</el-button>
        <el-button type="success" plain @click="handleValidateTools(true)" :loading="validatingTools">dry-run</el-button>
      </div>
      <el-input v-model="form.tools_config" type="textarea" :rows="5"
        placeholder='[{"name":"code_search","main_task":true,"plan_task":true,"definition":{...}}]' />
    </el-form-item>

    <el-form-item label="额外请求体">
      <el-input v-model="extraBodyText" type="textarea" :rows="3"
        placeholder='{"temperature":0.2}' @blur="syncExtraBody(false)" />
      <div class="form-tip">JSON 对象字符串，透传给 LLM 请求的 extra_body。</div>
    </el-form-item>

    <el-form-item label="额外请求头">
      <el-input v-model="form.extra_headers" type="textarea" :rows="2"
        placeholder="X-Org-ID=org-123,X-Trace-ID=review-service" />
    </el-form-item>

    <!-- 遥测 -->
    <el-divider content-position="left">遥测</el-divider>

    <el-form-item label="启用遥测"><el-switch v-model="form.telemetry_enabled" /></el-form-item>
    <template v-if="form.telemetry_enabled">
      <el-form-item label="导出器">
        <el-select v-model="form.telemetry_exporter" clearable>
          <el-option label="OTLP" value="otlp" />
          <el-option label="Stdout" value="stdout" />
        </el-select>
      </el-form-item>
      <el-form-item label="OTLP 端点" v-if="form.telemetry_exporter === 'otlp'">
        <el-input v-model="form.telemetry_otlp_endpoint" placeholder="http://localhost:4317" />
      </el-form-item>
      <el-form-item label="内容日志">
        <el-switch v-model="form.telemetry_content_logging" />
        <div class="form-tip">记录审查引擎交互的完整内容（可能含敏感信息）。</div>
      </el-form-item>
    </template>

    <!-- Session Viewer -->
    <el-divider content-position="left">Session Viewer</el-divider>
    <el-form-item label="Viewer">
      <SessionViewerSection />
    </el-form-item>

    <!-- 命令预览 -->
    <el-divider content-position="left">执行预览</el-divider>
    <div class="command-preview">
      <div class="preview-title">服务端将按受控参数组装命令</div>
      <code>{{ commandPreview }}</code>
    </div>
  </el-form>

  <!-- 工具包管理抽屉 -->
  <ToolPackageDrawer v-model="toolDrawerOpen" />
</template>

<style scoped>
.form-tip { font-size: 12px; color: #999; margin-top: 4px; }
.param-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0 24px;
}
.tool-toolbar {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
  margin-bottom: 8px;
}
.command-preview {
  margin: 0 0 20px 120px;
  padding: 12px 14px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: #f7f9fc;
}
.preview-title { margin-bottom: 6px; color: #606266; font-size: 12px; }
.command-preview code {
  display: block; white-space: pre-wrap; word-break: break-word;
  color: #303133; font-size: 12px; line-height: 1.6;
}
</style>
