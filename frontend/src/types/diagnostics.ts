export interface DiagnosticsSnapshot {
  since_hours: number
  webhooks: Record<string, number>
  writebacks: Record<string, number>
  recent_webhooks: WebhookDelivery[]
  recent_writebacks: WritebackItem[]
  recommendations: DiagnosticRecommendation[]
  generated_at: string
}

export interface DiagnosticRecommendation {
  level: 'success' | 'info' | 'warning' | 'danger' | string
  title: string
  message: string
  action?: string
}

export interface WebhookDelivery {
  provider: string
  installation_id: string
  delivery_id: string
  event_type: string
  action: string
  payload_sha256: string
  status: string
  error?: string
  received_at: string
  processed_at?: string
}

export interface WritebackItem {
  review_id: string
  finding_key: string
  kind: string
  provider_external_id?: string
  provider_url?: string
  status: string
  attempts: number
  last_error?: string
  published_at?: string
}
