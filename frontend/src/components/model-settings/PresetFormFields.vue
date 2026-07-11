<script setup lang="ts">
/**
 * PresetFormFields — 模型预设的新建/编辑表单字段（共用）。
 */
defineProps<{
  isEdit?: boolean
}>()

const form = defineModel<{
  name: string
  protocol: string
  url: string
  api_key: string
  model: string
  auth_header: string
  extra_body: string
  enabled: boolean
}>({ required: true })
</script>

<template>
  <div class="preset-fields">
    <div class="fields-row">
      <label class="field">
        <span class="field-label">名称</span>
        <el-input v-model="form.name" size="small" placeholder="如 gpt-4o-prod" />
      </label>
      <label class="field">
        <span class="field-label">协议</span>
        <el-select v-model="form.protocol" size="small" style="width: 100%">
          <el-option label="OpenAI 兼容" value="openai" />
          <el-option label="Anthropic" value="anthropic" />
        </el-select>
      </label>
    </div>
    <div class="fields-row">
      <label class="field">
        <span class="field-label">模型</span>
        <el-input v-model="form.model" size="small" placeholder="如 gpt-4o" />
      </label>
      <label class="field">
        <span class="field-label">接口地址</span>
        <el-input v-model="form.url" size="small" placeholder="https://api.example.com/v1" />
      </label>
    </div>
    <div class="fields-row">
      <label class="field">
        <span class="field-label">{{ isEdit ? 'API Key（留空不改）' : 'API Key' }}</span>
        <el-input
          v-model="form.api_key"
          size="small"
          type="password"
          show-password
          :placeholder="isEdit ? '••••••（留空保持不变）' : 'sk-...'"
        />
      </label>
      <label class="field">
        <span class="field-label">鉴权头</span>
        <el-input v-model="form.auth_header" size="small" placeholder="可选，默认 Authorization" />
      </label>
    </div>
    <label class="field field-full">
      <span class="field-label">附加 Body</span>
      <el-input
        v-model="form.extra_body"
        size="small"
        type="textarea"
        :autosize="{ minRows: 2, maxRows: 4 }"
        placeholder='JSON，如 {"temperature":0.2}'
      />
    </label>
  </div>
</template>

<style scoped>
.preset-fields {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.fields-row {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}
.field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  flex: 1;
  min-width: 160px;
}
.field-full {
  flex-basis: 100%;
}
.field-label {
  font-size: 11px;
  font-weight: 600;
  color: #8892a4;
  text-transform: uppercase;
  letter-spacing: 0.3px;
}
@media (max-width: 720px) {
  .fields-row {
    flex-direction: column;
  }
}
</style>
