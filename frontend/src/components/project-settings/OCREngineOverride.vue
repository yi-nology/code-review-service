<template>
  <section class="form-section">
    <div class="section-title">
      <span class="section-index">04</span>
      <div>
        <strong>审查引擎覆盖</strong>
        <span>留空或填 0 表示继承全局默认，只为该仓库特殊情况覆盖</span>
      </div>
    </div>

    <div class="field-grid two-columns">
      <label class="field-block">
        <span>OCR 启用策略</span>
        <el-select :model-value="ocrEnabledMode" @change="$emit('update:ocrEnabledMode', $event)">
          <el-option label="继承全局" value="inherit" />
          <el-option label="强制启用" value="enabled" />
          <el-option label="强制禁用" value="disabled" />
        </el-select>
      </label>

      <label class="field-block">
        <span>默认严重级别</span>
        <el-select :model-value="form.ocr_default_severity" clearable placeholder="继承全局" @change="$emit('update:ocrDefaultSeverity', $event)">
          <el-option label="严重" value="critical" />
          <el-option label="高危" value="major" />
          <el-option label="中危" value="minor" />
          <el-option label="低危" value="info" />
        </el-select>
      </label>

      <label class="field-block">
        <span>输出受众</span>
        <el-select :model-value="form.ocr_audience" clearable placeholder="继承全局" @change="$emit('update:ocrAudience', $event)">
          <el-option label="人工可读" value="human" />
          <el-option label="Agent 精简" value="agent" />
        </el-select>
      </label>

      <label class="field-block">
        <span>输出语言</span>
        <el-select :model-value="form.ocr_language" clearable placeholder="继承全局" @change="$emit('update:ocrLanguage', $event)">
          <el-option label="中文" value="Chinese" />
          <el-option label="English" value="English" />
        </el-select>
      </label>
    </div>

    <label class="field-block full-width">
      <span>审查背景</span>
      <el-input
        :model-value="form.ocr_background"
        type="textarea"
        :rows="3"
        placeholder="该仓库的业务背景、重点风险、审查偏好；留空继承全局默认"
        @input="$emit('update:ocrBackground', $event)"
      />
    </label>

    <el-collapse class="advanced-collapse">
      <el-collapse-item title="高级覆盖（并发 / 超时 / 模型 / 遥测）" name="advanced">
        <div class="field-grid two-columns">
          <label class="field-block">
            <span>模型并发数</span>
            <el-input-number :model-value="form.ocr_concurrency" :min="0" :max="32" @change="$emit('update:ocrConcurrency', $event)" />
            <small>0 表示继承全局默认。</small>
          </label>

          <label class="field-block">
            <span>Git 并发数</span>
            <el-input-number :model-value="form.ocr_max_git_procs" :min="0" :max="64" @change="$emit('update:ocrMaxGitProcs', $event)" />
            <small>0 表示继承全局默认。</small>
          </label>

          <label class="field-block">
            <span>单文件超时（分钟）</span>
            <el-input-number :model-value="form.ocr_timeout_minutes" :min="0" :max="120" @change="$emit('update:ocrTimeoutMinutes', $event)" />
            <small>0 表示继承全局默认。</small>
          </label>

          <label class="field-block">
            <span>最大工具轮次</span>
            <el-input-number :model-value="form.ocr_max_tools" :min="0" :max="100" @change="$emit('update:ocrMaxTools', $event)" />
            <small>0 表示继承全局或 OCR 内置默认值。</small>
          </label>

          <label class="field-block">
            <span>请求协议</span>
            <el-select :model-value="form.ocr_use_anthropic" clearable placeholder="继承全局" @change="$emit('update:ocrUseAnthropic', $event)">
              <el-option label="OpenAI 兼容" :value="false" />
              <el-option label="Anthropic" :value="true" />
            </el-select>
          </label>

          <label class="field-block">
            <span>模型接口地址</span>
            <el-input :model-value="form.ocr_url" placeholder="继承全局" clearable @input="$emit('update:ocrUrl', $event)" />
          </label>

          <label class="field-block">
            <span>模型名称</span>
            <el-input :model-value="form.ocr_model" placeholder="继承全局" clearable @input="$emit('update:ocrModel', $event)" />
          </label>

          <label class="field-block">
            <span>OCR 二进制路径</span>
            <el-input :model-value="form.ocr_binary" placeholder="继承全局" clearable @input="$emit('update:ocrBinary', $event)" />
          </label>

          <label class="field-block">
            <span>工具白名单</span>
            <el-input :model-value="form.ocr_tools_path" placeholder="继承全局" clearable @input="$emit('update:ocrToolsPath', $event)" />
          </label>
        </div>

        <label class="field-block full-width">
          <span>额外请求体</span>
          <el-input
            :model-value="form.ocr_extra_body"
            type="textarea"
            :rows="2"
            placeholder='继承全局。可选 JSON 对象，如 {"temperature":0.2}'
            @input="$emit('update:ocrExtraBody', $event)"
          />
        </label>

        <div class="field-grid two-columns">
          <label class="field-block">
            <span>遥测启用</span>
            <el-select :model-value="form.ocr_telemetry_enabled" clearable placeholder="继承全局" @change="$emit('update:ocrTelemetryEnabled', $event)">
              <el-option label="继承全局" value="" />
              <el-option label="启用" :value="true" />
              <el-option label="禁用" :value="false" />
            </el-select>
          </label>

          <label class="field-block">
            <span>遥测导出器</span>
            <el-select :model-value="form.ocr_telemetry_exporter" clearable placeholder="继承全局" @change="$emit('update:ocrTelemetryExporter', $event)">
              <el-option label="OTLP" value="otlp" />
              <el-option label="Stdout" value="stdout" />
            </el-select>
          </label>

          <label class="field-block">
            <span>OTLP 端点</span>
            <el-input :model-value="form.ocr_telemetry_otlp_endpoint" placeholder="继承全局" clearable @input="$emit('update:ocrTelemetryOtlpEndpoint', $event)" />
          </label>

          <label class="field-block">
            <span>遥测内容日志</span>
            <el-select :model-value="form.ocr_telemetry_content_logging" clearable placeholder="继承全局" @change="$emit('update:ocrTelemetryContentLogging', $event)">
              <el-option label="继承全局" value="" />
              <el-option label="启用" :value="true" />
              <el-option label="禁用" :value="false" />
            </el-select>
          </label>
        </div>
      </el-collapse-item>
    </el-collapse>
  </section>
</template>

<script setup lang="ts">
defineProps<{
  form: Record<string, any>
  ocrEnabledMode: string
}>()

defineEmits<{
  'update:ocrEnabledMode': [value: string]
  'update:ocrDefaultSeverity': [value: any]
  'update:ocrAudience': [value: any]
  'update:ocrLanguage': [value: any]
  'update:ocrBackground': [value: string]
  'update:ocrConcurrency': [value: number | undefined]
  'update:ocrMaxGitProcs': [value: number | undefined]
  'update:ocrTimeoutMinutes': [value: number | undefined]
  'update:ocrMaxTools': [value: number | undefined]
  'update:ocrUseAnthropic': [value: any]
  'update:ocrUrl': [value: string]
  'update:ocrModel': [value: string]
  'update:ocrBinary': [value: string]
  'update:ocrToolsPath': [value: string]
  'update:ocrExtraBody': [value: string]
  'update:ocrTelemetryEnabled': [value: any]
  'update:ocrTelemetryExporter': [value: string]
  'update:ocrTelemetryOtlpEndpoint': [value: string]
  'update:ocrTelemetryContentLogging': [value: any]
}>()
</script>
