<script setup lang="ts">
import { computed } from 'vue'
import { useSettingsForm } from './context'

const { form, saving } = useSettingsForm()

const ignorePathsText = computed({
  get: () => (form.policy_ignore_paths || []).join('\n'),
  set: (val: string) => {
    form.policy_ignore_paths = val.split('\n').map(s => s.trim()).filter(Boolean)
  },
})

const severityOptions = [
  { label: 'Critical（严重）', value: 'critical' },
  { label: 'Major（重要）', value: 'major' },
  { label: 'Minor（次要）', value: 'minor' },
  { label: 'Info（提示）', value: 'info' },
]

const diffSizeOptions = [
  { label: '2 MB（小型 PR）', value: 2097152 },
  { label: '5 MB（默认）', value: 5242880 },
  { label: '10 MB（大型 PR）', value: 10485760 },
  { label: '20 MB（超大型 PR）', value: 20971520 },
  { label: '50 MB（极端场景）', value: 52428800 },
  { label: '不限制', value: 0 },
]
</script>

<template>
  <el-form :model="form" label-width="140px" :disabled="saving">
    <el-form-item label="门禁失败阈值">
      <el-select v-model="form.policy_fail_on" style="width: 100%">
        <el-option v-for="opt in severityOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
      </el-select>
      <div class="form-tip">发现该级别或更严重的问题时，质量门禁判定为未通过。</div>
    </el-form-item>

    <el-form-item label="最大评论数">
      <el-input-number
        v-model="form.policy_max_comments"
        :min="0"
        :max="200"
        :step="5"
        style="width: 100%"
      />
      <div class="form-tip">单次审查最多写回的评论数，0 表示不限制。</div>
    </el-form-item>

    <el-form-item label="仅写摘要">
      <el-switch v-model="form.policy_summary_only" />
      <div class="form-tip">开启后只写回审查摘要，不写逐条评论。</div>
    </el-form-item>

    <el-form-item label="忽略路径">
      <el-input
        v-model="ignorePathsText"
        type="textarea"
        :rows="4"
        placeholder="每行一个 glob 模式，例如：&#10;vendor/**&#10;dist/**&#10;*.pb.go"
      />
      <div class="form-tip">匹配的文件路径不会被审查，支持 glob 通配符。</div>
    </el-form-item>

    <el-divider />

    <el-form-item label="Diff 大小限制">
      <el-select v-model="form.max_diff_bytes" style="width: 100%">
        <el-option v-for="opt in diffSizeOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
      </el-select>
      <div class="form-tip">超过此大小的 diff 不会审查。建议根据仓库规模选择。</div>
    </el-form-item>
  </el-form>
</template>

<style scoped>
.form-tip {
  color: #909399;
  font-size: 12px;
  margin-top: 4px;
}
</style>
