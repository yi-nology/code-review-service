import request from './request'

export interface NotificationItem {
  id: number
  review_id: string
  kind: string
  title: string
  body: string
  read: boolean
  provider?: string
  repo?: string
  created_at: string
}

export interface NotificationListResult {
  items: NotificationItem[]
  total: number
}

export async function getNotifications(params?: {
  only_unread?: boolean
  limit?: number
  offset?: number
}): Promise<NotificationListResult> {
  const data: any = await request.get('/notifications', { params })
  return { items: data?.items ?? [], total: data?.total ?? 0 }
}

export async function getUnreadCount(): Promise<number> {
  const data: any = await request.get('/notifications/unread-count')
  return data?.unread ?? 0
}

export async function markNotificationRead(id: number): Promise<void> {
  await request.post(`/notifications/${id}/read`)
}

export async function markAllNotificationsRead(): Promise<void> {
  await request.post('/notifications/read-all')
}
