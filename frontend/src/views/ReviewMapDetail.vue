<template>
  <AppLayout>
    <div class="review-map-detail-page">
      <header class="page-header">
        <div class="breadcrumb">
          <router-link to="/reviews" class="breadcrumb-item">Reviews</router-link>
          <span class="breadcrumb-separator">/</span>
          <router-link v-if="review" :to="`/reviews/${reviewId}`" class="breadcrumb-item">
            {{ review.title || reviewId }}
          </router-link>
          <span class="breadcrumb-separator">/</span>
          <span class="breadcrumb-item active">Maps</span>
          <span class="breadcrumb-separator">/</span>
          <span class="breadcrumb-item active">{{ mapId }}</span>
        </div>
      </header>

      <div v-if="loading" class="loading">
        <el-skeleton :rows="10" animated />
      </div>

      <div v-else-if="!reviewMap" class="empty">
        <el-empty description="未找到审查地图">
          <el-button @click="goBack">返回</el-button>
        </el-empty>
      </div>

      <div v-else class="map-content">
        <div class="map-title-section">
          <h1>{{ reviewMap.title }}</h1>
          <p v-if="reviewMap.description">{{ reviewMap.description }}</p>
          <div class="map-stats">
            <el-tag>{{ reviewMap.file_count }} 个文件</el-tag>
            <el-tag type="success">{{ reviewMap.section_count }} 个部分</el-tag>
            <el-tag type="info">{{ reviewedCount }} 已审查</el-tag>
          </div>
        </div>

        <ReviewMap
          ref="reviewMapRef"
          :map="reviewMap"
          @file-reviewed="handleFileReviewed"
          @section-reviewed="handleSectionReviewed"
          @node-click="handleNodeClick"
        />
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus/es/components/message/index'
import AppLayout from '@/components/layout/AppLayout.vue'
import ReviewMap from '@/components/ReviewMap.vue'
import { getReview, getReviewMap, updateFileReviewed, updateSectionProgress } from '@/api/review'

const route = useRoute()
const router = useRouter()
const loading = ref(true)
const review = ref<any>(null)
const reviewMap = ref<any>(null)
const reviewMapRef = ref<InstanceType<typeof ReviewMap> | null>(null)

const reviewId = computed(() => route.params.reviewId as string)
const mapId = computed(() => route.params.mapId as string)

const reviewedCount = computed(() => {
  if (!reviewMap.value) return 0
  let count = 0
  for (const section of reviewMap.value.sections) {
    for (const file of section.files) {
      if (file.reviewed) count++
    }
  }
  return count
})

function goBack() {
  router.back()
}

async function handleFileReviewed(sectionId: string, filePath: string, reviewed: boolean) {
  try {
    await updateFileReviewed(reviewId.value, mapId.value, filePath, reviewed)
    ElMessage.success(reviewed ? '已标记为已审查' : '已取消审查标记')
  } catch (error) {
    console.error('Failed to update file reviewed status:', error)
    ElMessage.error('更新失败')
  }
}

async function handleSectionReviewed(sectionId: string, reviewed: boolean) {
  try {
    await updateSectionProgress(reviewId.value, mapId.value, sectionId, reviewed)
    ElMessage.success(reviewed ? '已标记所有文件为已审查' : '已取消所有文件审查标记')
    // Refresh map data
    await fetchReviewMap()
  } catch (error) {
    console.error('Failed to update section progress:', error)
    ElMessage.error('更新失败')
  }
}

function handleNodeClick(nodeId: string) {
  if (!reviewMap.value) return
  
  // Find the section by sanitized title
  const section = reviewMap.value.sections.find((s: any) => {
    const sanitized = s.title.replace(/[^a-zA-Z0-9]/g, '_')
    return sanitized === nodeId
  })
  
  if (section && reviewMapRef.value) {
    reviewMapRef.value.expandSection(section.id)
    // Scroll to section
    const element = document.getElementById(`section-${section.id}`)
    if (element) {
      element.scrollIntoView({ behavior: 'smooth', block: 'start' })
    }
  }
}

async function fetchReview() {
  try {
    review.value = await getReview(reviewId.value)
  } catch (error) {
    console.error('Failed to load review:', error)
  }
}

async function fetchReviewMap() {
  try {
    reviewMap.value = await getReviewMap(reviewId.value, mapId.value)
  } catch (error) {
    console.error('Failed to load review map:', error)
  }
}

onMounted(async () => {
  loading.value = true
  try {
    await Promise.all([fetchReview(), fetchReviewMap()])
  } finally {
    loading.value = false
  }
})
</script>

<style scoped>
.review-map-detail-page {
  padding: 20px;
}

.page-header {
  margin-bottom: 20px;
}

.breadcrumb {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
}

.breadcrumb-item {
  color: #909399;
  text-decoration: none;
  transition: color 0.2s;
}

.breadcrumb-item:hover {
  color: #409eff;
}

.breadcrumb-item.active {
  color: #303133;
  font-weight: 500;
}

.breadcrumb-separator {
  color: #c0c4cc;
}

.loading {
  padding: 40px;
}

.empty {
  padding: 40px;
}

.map-title-section {
  margin-bottom: 20px;
}

.map-title-section h1 {
  margin: 0 0 8px;
  font-size: 24px;
  color: #303133;
}

.map-title-section p {
  margin: 0 0 12px;
  color: #606266;
  font-size: 14px;
}

.map-stats {
  display: flex;
  gap: 8px;
}
</style>
