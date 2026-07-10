import request from './request'

export interface ApiToken {
  id: number
  name: string
  scopes?: string[]
  created_by?: string
  last_used_at?: string
  expires_at?: string
  created_at: string
}

export interface CreateTokenResult {
  id: number
  name: string
  /** 明文 token，仅在创建时返回一次 */
  token: string
  scopes?: string[]
  created_by?: string
  created_at: string
}

export interface AuditLogEntry {
  id: number
  token_name: string
  action: string
  resource?: string
  resource_id?: string
  method: string
  path: string
  status_code: number
  detail?: string
  created_at: string
}

export async function createToken(data: {
  name: string
  scopes?: string[]
  created_by?: string
}): Promise<CreateTokenResult> {
  return request.post('/settings/tokens', data) as any
}

export async function listTokens(): Promise<ApiToken[]> {
  const data: any = await request.get('/settings/tokens')
  return data?.items ?? []
}

export async function deleteToken(id: number): Promise<void> {
  await request.delete(`/settings/tokens/${id}`)
}

export async function listAuditLogs(params?: {
  limit?: number
  offset?: number
}): Promise<{ items: AuditLogEntry[]; total: number }> {
  const data: any = await request.get('/settings/audit-logs', { params })
  return { items: data?.items ?? [], total: data?.total ?? 0 }
}
