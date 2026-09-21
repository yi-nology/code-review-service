<template>
  <el-table
    :data="items"
    v-loading="loading"
    class="review-table desktop-table"
    row-class-name="review-row"
    empty-text="暂无符合条件的审查记录"
    row-key="review_id"
    @row-click="onRowClick"
    @selection-change="(rows: Review[]) => emit('selectionChange', rows)"
    @sort-change="onSortChange"
  >
    <el-table-column type="selection" width="42" :selectable="() => true" />
    <el-table-column label="仓库" min-width="240">
      <template #default="{ row }">
        <div class="repo-cell">
          <span class="provider-mark">{{ getProviderInitial(row.provider) }}</span>
          <div>
            <strong>{{ row.repo }}</strong>
            <div class="review-id">
              {{ getProviderLabel(row.provider) }}
              <el-tag
                v-if="!pinnedEngine && row.engine"
                :type="row.engine === 'argus' ? 'warning' : 'primary'"
                size="small"
                effect="plain"
                class="engine-tag"
              >
                {{ row.engine === 'argus' ? 'Argus' : 'OCR' }}
              </el-tag>
              · {{ shortId(row.review_id) }}
            </div>
          </div>
        </div>
      </template>
    </el-table-column>
    <el-table-column label="开发者" width="160">
      <template #default="{ row }">
        <div v-if="row.author" class="author-cell">
          <img v-if="row.author_avatar" :src="row.author_avatar" class="author-avatar" :alt="row.author" />
          <span v-else class="author-avatar-fallback">{{ (row.author_name || row.author).slice(0, 1).toUpperCase() }}</span>
          <router-link v-if="row.author" :to="`/contributors/${encodeURIComponent(row.author)}`" class="author-link" @click.stop>
            {{ row.author_name || row.author }}
          </router-link>
        </div>
        <span v-else class="muted-value">--</span>
      </template>
    </el-table-column>
    <el-table-column label="执行状态" width="120">
      <template #default="{ row }">
        <span class="status-pill" :class="row.status">
          <i />
          {{ getStatusLabel(row.status) }}
        </span>
      </template>
    </el-table-column>
    <el-table-column label="质量门禁" width="130">
      <template #default="{ row }">
        <div class="gate-result" :class="gateClass(row)">
          <el-icon v-if="row.gate?.passed"><CircleCheckFilled /></el-icon>
          <el-icon v-else-if="row.gate?.status !== 'pending'"><CircleCloseFilled /></el-icon>
          <el-icon v-else><Clock /></el-icon>
          {{ gateLabel(row) }}
        </div>
      </template>
    </el-table-column>
    <el-table-column label="质量评分" width="130" sortable="custom" prop="score">
      <template #default="{ row }">
        <div v-if="row.report_generated" class="score-cell">
          <strong>{{ row.score }}</strong>
          <el-progress :percentage="row.score" :show-text="false" :stroke-width="5" :color="getScoreColor(row.score)" />
        </div>
        <span v-else class="muted-value">尚未生成</span>
      </template>
    </el-table-column>
    <el-table-column label="问题" width="90" align="center" sortable="custom" prop="findings">
      <template #default="{ row }">
        <span class="finding-count" :class="{ active: row.findings > 0 }">{{ row.findings }}</span>
      </template>
    </el-table-column>
    <el-table-column label="最后更新" width="160" sortable="custom" prop="updated_at">
      <template #default="{ row }">
        <div class="time-cell">
          <strong>{{ formatDateShort(row.updated_at) }}</strong>
          <span>{{ formatClock(row.updated_at) }}</span>
        </div>
      </template>
    </el-table-column>
    <el-table-column width="140" align="right">
      <template #default="{ row }">
        <div class="row-actions" @click.stop>
          <el-button
            v-if="isCancellable(row)"
            size="small"
            type="danger"
            plain
            :loading="actingId === row.review_id"
            @click="emit('cancel', row)"
          >
            取消
          </el-button>
          <el-button
            v-if="isDeadLetter(row)"
            size="small"
            type="warning"
            plain
            :loading="actingId === row.review_id"
            @click="emit('requeue', row)"
          >
            重新入队
          </el-button>
          <el-button
            v-if="isRerunnable(row)"
            size="small"
            type="primary"
            plain
            :loading="actingId === row.review_id"
            @click="emit('rerun', row)"
          >
            重新审查
          </el-button>
        </div>
      </template>
    </el-table-column>
  </el-table>
</template>

<script setup lang="ts">
// 审查列表桌面表格（>720px 显示）：只负责展示与事件上抛，
// 行级操作与批量逻辑在 useReviewActions，页面编排在 ReviewListView。
import { CircleCheckFilled, CircleCloseFilled, Clock } from '@element-plus/icons-vue'
import type { Review } from '@/types/review'
import { getProviderLabel } from '@/utils/url-parser'
import { getStatusLabel } from '@/utils/status'
import { getScoreColor } from '@/utils/colors'
import { formatDateShort, formatClock, getProviderInitial, shortId } from '@/utils/format'
import { isCancellable, isDeadLetter, isRerunnable, gateLabel, gateClass } from '@/utils/review'

defineProps<{
  items: Review[]
  loading: boolean
  /** 正在执行行级操作的 review_id（按钮 loading 态） */
  actingId: string
  /** 所属页面锁定的引擎（锁定时不显示引擎徽标——整页都是该引擎） */
  pinnedEngine?: string
}>()

const emit = defineEmits<{
  (e: 'rowClick', row: Review): void
  (e: 'selectionChange', rows: Review[]): void
  (e: 'sortChange', payload: { prop: string | null; order: 'ascending' | 'descending' | null }): void
  (e: 'cancel', row: Review): void
  (e: 'rerun', row: Review): void
  (e: 'requeue', row: Review): void
}>()

function onRowClick(row: Review) {
  emit('rowClick', row)
}

// el-table sort-change 的 order 为 ascending/descending/null，null 表示取消排序。
function onSortChange({ prop, order }: { prop: string | null; order: 'ascending' | 'descending' | null }) {
  emit('sortChange', { prop, order })
}
</script>

<style scoped>
.review-table {
  --el-table-border-color: #edf0f4;
  --el-table-header-bg-color: #fafbfc;
  --el-table-row-hover-bg-color: #f8fafc;
  color: var(--app-text);
}

.review-table :deep(th.el-table__cell) {
  height: 42px;
  color: #7d8698;
  font-size: 12px;
  font-weight: 700;
}

.review-table :deep(td.el-table__cell) {
  height: 62px;
}

.review-table :deep(.review-row) {
  cursor: pointer;
}

.repo-cell {
  display: flex;
  align-items: center;
  gap: 12px;
}

.repo-cell strong {
  display: block;
  max-width: 230px;
  overflow: hidden;
  color: var(--app-text);
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.provider-mark {
  display: grid;
  width: 34px;
  height: 34px;
  flex: 0 0 34px;
  place-items: center;
  border: 1px solid #e1e4f5;
  border-radius: 9px;
  color: #5656c5;
  background: #f4f4ff;
  font-size: 10px;
  font-weight: 800;
}

.review-id {
  margin-top: 3px;
  color: #9aa2b1;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 10px;
}

/* 行内引擎徽标（审查记录混合列表区分 OCR/Argus），对齐小号副文本。 */
.engine-tag {
  height: 16px;
  padding: 0 4px;
  margin: 0 2px;
  font-size: 10px;
  line-height: 14px;
}

.author-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.author-avatar {
  width: 24px;
  height: 24px;
  border-radius: 50%;
  flex-shrink: 0;
  object-fit: cover;
}

.author-avatar-fallback {
  display: grid;
  width: 24px;
  height: 24px;
  flex-shrink: 0;
  place-items: center;
  border-radius: 50%;
  color: #fff;
  background: #2563eb;
  font-size: 11px;
  font-weight: 700;
}

.author-link {
  overflow: hidden;
  color: var(--app-text);
  font-size: 13px;
  text-decoration: none;
  text-overflow: ellipsis;
  white-space: nowrap;
  transition: color 160ms ease;
}

.author-link:hover {
  color: var(--app-primary);
}

.status-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: #687284;
  font-size: 12px;
  font-weight: 600;
}

.status-pill i {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #aab1be;
}

.status-pill.success i {
  background: var(--app-success);
}

.status-pill.running i {
  background: #3984cf;
  box-shadow: 0 0 0 4px rgba(57, 132, 207, 0.12);
}

.status-pill.failed i {
  background: var(--app-danger);
}

.gate-result {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  font-weight: 600;
}

.gate-result.passed {
  color: var(--app-success);
}

.gate-result.failed {
  color: var(--app-danger);
}

.gate-result.pending {
  color: #8e96a5;
}

.score-cell {
  display: grid;
  grid-template-columns: 28px 1fr;
  align-items: center;
  gap: 8px;
}

.score-cell strong {
  font-size: 13px;
}

.muted-value {
  color: #a0a7b5;
  font-size: 12px;
}

.finding-count {
  display: inline-grid;
  min-width: 26px;
  height: 26px;
  place-items: center;
  border-radius: 8px;
  color: #7f8796;
  background: #f2f4f7;
  font-size: 12px;
  font-weight: 700;
}

.finding-count.active {
  color: #c77910;
  background: #fff3df;
}

.time-cell strong,
.time-cell span {
  display: block;
}

.time-cell strong {
  color: #5d6677;
  font-size: 12px;
  font-weight: 600;
}

.time-cell span {
  margin-top: 2px;
  color: #a0a7b5;
  font-size: 11px;
}

.row-actions {
  display: flex;
  justify-content: flex-end;
  gap: 4px;
}

@media (max-width: 720px) {
  .review-table {
    display: none;
  }
}
</style>
