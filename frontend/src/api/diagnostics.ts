import request from './request'
import type { DiagnosticsSnapshot } from '@/types/diagnostics'

export async function getDiagnostics(params?: {
  hours?: number
  limit?: number
  provider?: string
  status?: string
}): Promise<DiagnosticsSnapshot> {
  return request.get('/diagnostics', { params }) as any
}
