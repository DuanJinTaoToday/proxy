import { request } from './client'

export async function getAccessLocation({ locale, signal }: { locale: string, signal: AbortSignal }) {
  const wire = await request<unknown>('/v1/tenant/access-location', { acceptLanguage: locale, signal })
  if (typeof wire === 'object' && wire !== null && !Array.isArray(wire)) {
    const ip = 'ip' in wire ? wire.ip : ''
    const location = 'location' in wire ? wire.location : ''
    const isMainlandChina = 'is_mainland_china' in wire ? wire.is_mainland_china : false
    if (typeof ip === 'string' && typeof location === 'string' && typeof isMainlandChina === 'boolean') {
      return { ip, location, isMainlandChina }
    }
  }
  throw new Error('Invalid access location response')
}
