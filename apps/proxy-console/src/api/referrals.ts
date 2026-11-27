import { request } from './client'

// 对应控制面推广分享与运营推广资格管理接口。
export type PromotionStatus = 'active' | 'paused'

export interface TenantInvite {
  inviteCode: string
  inviteUrl: string
  promotionStatus: PromotionStatus
}

export interface TenantPromotion {
  promotionStatus: PromotionStatus
}

interface TenantInviteWire {
  invite_code: string
  invite_url: string
  promotion_status: unknown
}

interface TenantPromotionWire {
  promotion_status: unknown
}

const toPromotionStatus = (value: unknown): PromotionStatus => {
  if (value === 'active' || value === 'paused') return value
  throw new Error(`Invalid promotion status: ${String(value)}`)
}

export async function getTenantInvite(signal?: AbortSignal): Promise<TenantInvite> {
  const wire = await request<TenantInviteWire>('/v1/tenant/invite', { signal })
  return {
    inviteCode: wire.invite_code,
    inviteUrl: wire.invite_url,
    promotionStatus: toPromotionStatus(wire.promotion_status),
  }
}

export async function getOpsTenantPromotion(tenantId: string, signal?: AbortSignal): Promise<TenantPromotion> {
  const wire = await request<TenantPromotionWire>(`/v1/operations/tenants/${encodeURIComponent(tenantId)}/promotion`, { signal })
  return { promotionStatus: toPromotionStatus(wire.promotion_status) }
}

export async function pauseOpsTenantPromotion(tenantId: string): Promise<TenantPromotion> {
  const wire = await request<TenantPromotionWire>(`/v1/operations/tenants/${encodeURIComponent(tenantId)}:pause-promotion`, {
    method: 'POST',
    body: {},
  })
  return { promotionStatus: toPromotionStatus(wire.promotion_status) }
}

export async function resumeOpsTenantPromotion(tenantId: string): Promise<TenantPromotion> {
  const wire = await request<TenantPromotionWire>(`/v1/operations/tenants/${encodeURIComponent(tenantId)}:resume-promotion`, {
    method: 'POST',
    body: {},
  })
  return { promotionStatus: toPromotionStatus(wire.promotion_status) }
}
