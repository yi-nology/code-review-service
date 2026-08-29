<template>
  <el-form :model="form" label-width="120px" :disabled="saving">
    <el-form-item label="审查背景">
      <el-input
        v-model="form.background"
        type="textarea"
        :rows="6"
        placeholder="默认的审查侧重点，例如：这是一个电商项目，重点关注支付流程的安全性和性能"
      />
      <div class="form-tip">当未配置代理团队时，使用此默认背景进行审查。配置代理后，各代理使用各自的审查侧重点。</div>
    </el-form-item>
    <el-form-item label="背景文件路径">
      <el-input
        v-model="form.background_file"
        placeholder="可选：仓库内 Markdown 文件路径，如 docs/review-context.md（OCR v1.7.6+）"
      />
      <div class="form-tip">
        OCR v1.7.6+ 支持 <code>--background-file</code> 读取 Markdown 文件，规避 inline 8000 字符上限；
        与「审查背景」叠加（inline 在前，文件在后）。路径必须位于仓库工作区内（防路径穿越）。
      </div>
    </el-form-item>
  </el-form>
</template>

<script setup lang="ts">
import { useSettingsForm } from './context'

const { form, saving } = useSettingsForm()
</script>

<style scoped>
.form-tip {
  font-size: 12px;
  color: #999;
  margin-top: 4px;
}
</style>
