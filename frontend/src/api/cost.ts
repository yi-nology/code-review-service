import request from './request'

export interface ModelPricing {
  id?: number
  provider: string
  model: string
  input_price_per_mega: number
  output_price_per_mega: number
  cache_read_price_per_mega?: number
  cache_write_price_per_mega?: number
  currency?: string
  notes?: string
}

export async function listPricing(): Promise<ModelPricing[]> {
  const data: any = await request.get('/settings/cost/pricing')
  return data?.items ?? []
}

export async function upsertPricing(pricing: ModelPricing): Promise<ModelPricing> {
  return request.post('/settings/cost/pricing', pricing) as any
}

export async function deletePricing(id: number): Promise<void> {
  await request.delete(`/settings/cost/pricing/${id}`)
}
