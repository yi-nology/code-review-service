import { ElMessage } from 'element-plus/es/components/message/index'

export function handleApiError(error: unknown, fallbackMessage = '操作失败'): string {
  const message = error instanceof Error ? error.message : fallbackMessage
  ElMessage.error(message)
  return message
}

export function handleApiSuccess(message: string): void {
  ElMessage.success(message)
}
