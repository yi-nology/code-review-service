<template>
  <div v-loading="loading" class="mobile-review-list">
    <button
      v-for="row in items"
      :key="row.review_id"
      class="mobile-review-card"
      type="button"
      @click="emit('rowClick', row)"
    >
      <span class="provider-mark">{{ getProviderInitial(row.provider) }}</span>
      <span class="mobile-review-main">
        <strong>{{ row.repo }}</strong>
        <small>{{ getProviderLabel(row.provider) }} · {{ shortId(row.review_id) }}<el-tag
          v-if="row.engine === 'argus'"
          type="warning"
          size="small"
          effect="plain"
          class="engine-tag"
        >Argus</el-tag></small>
        <span class="mobile-review-meta">
          <span class="status-pill" :class="row.status"><i />{{ getStatusLabel(row.status) }}</span>
          <span class="gate-result" :class="gateClass(row)">{{ gateLabel(row) }}</span>
        </span>
      </span>
      <span class="mobile-review-side">
        <strong v-if="row.report_generated">{{ row.score }}<small>分</small></strong>
        <strong v-else>--</strong>
        <small>{{ row.findings }} 个问题</small>
        <div class="mobile-actions" @click.stop>
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
        <el-icon><ArrowRight /></el-icon>
      </span>
    </button>
    <div v-if="!loading && items.length === 0" class="mobile-empty">
      暂无符合条件的审查记录
    </div>
  </div>
</template>

<script setup lang="ts">
// 审查列表移动卡片（<=720px 显示；桌面表格由 ReviewTable 承担）。
import { ArrowRight } from '@element-plus/icons-vue'
import type { Review } from '@/types/review'
import { getProviderLabel } from '@/utils/url-parser'
import { getStatusLabel } from '@/utils/status'
import { getProviderInitial, shortId } from '@/utils/format'
import { isCancellable, isDeadLetter, isRerunnable, gateLabel, gateClass } from '@/utils/review'

defineProps<{
  items: Review[]
  loading: boolean
  actingId: string
}>()

const emit = defineEmits<{
  (e: 'rowClick', row: Review): void
  (e: 'cancel', row: Review): void
  (e: 'rerun', row: Review): void
  (e: 'requeue', row: Review): void
}>()
</script>

<style scoped>
.mobile-review-list {
  display: none;
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

.engine-tag {
  height: 16px;
  padding: 0 4px;
  margin: 0 2px;
  font-size: 10px;
  line-height: 14px;
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

.mobile-actions {
  display: flex;
  gap: 6px;
  margin-top: 6px;
}

@media (max-width: 720px) {
  .mobile-review-list {
    display: block;
  }

  .mobile-review-card {
    display: flex;
    width: 100%;
    align-items: flex-start;
    gap: 11px;
    padding: 16px 14px;
    border: 0;
    border-bottom: 1px solid var(--app-border);
    color: inherit;
    text-align: left;
    background: #fff;
  }

  .mobile-review-main {
    min-width: 0;
    flex: 1;
  }

  .mobile-review-main > strong,
  .mobile-review-main > small {
    display: block;
  }

  .mobile-review-main > strong {
    overflow: hidden;
    color: var(--app-text);
    font-size: 13px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .mobile-review-main > small {
    margin-top: 3px;
    overflow: hidden;
    color: #9aa2b1;
    font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
    font-size: 9px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .mobile-review-meta {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-top: 9px;
  }

  .mobile-review-side {
    display: grid;
    min-width: 54px;
    justify-items: end;
    color: #9ba3b1;
  }

  .mobile-review-side > strong {
    color: var(--app-text);
    font-size: 17px;
  }

  .mobile-review-side > strong small {
    margin-left: 1px;
    font-size: 10px;
  }

  .mobile-review-side > small {
    margin-top: 2px;
    font-size: 9px;
  }

  .mobile-review-side .el-icon {
    margin-top: 9px;
  }

  .mobile-empty {
    padding: 44px 16px;
    color: #9aa2b1;
    text-align: center;
    font-size: 13px;
  }
}
</style>
