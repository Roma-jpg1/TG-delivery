const API_BASE = import.meta.env.VITE_API_BASE ?? 'http://localhost:18080/api/v1/admin'
const ADMIN_TOKEN = import.meta.env.VITE_ADMIN_TOKEN ?? 'dev-admin-token'

type StopListItem = {
  menu_item_id: string
  menu_item_name: string
  status: string
  reason?: string
}

type ManualReviewOrder = {
  order_id: string
  order_number: number
  status: string
  total: number
  currency: string
}

async function request(path: string, init?: RequestInit) {
  const resp = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      'X-Admin-Token': ADMIN_TOKEN,
      ...(init?.headers ?? {}),
    },
  })
  if (!resp.ok) throw new Error(`admin request failed: ${resp.status}`)
  return resp.json()
}

export async function getStopList(branchId: string): Promise<StopListItem[]> {
  const data = await request(`/branches/${branchId}/stop-list`)
  return data.items
}

export async function setAvailability(branchId: string, menuItemId: string, status: string, reason: string): Promise<void> {
  await request(`/branches/${branchId}/menu-items/${menuItemId}/availability`, {
    method: 'PUT',
    body: JSON.stringify({ status, reason, actor_type: 'admin_ui' }),
  })
}

export async function getManualReviewOrders(): Promise<ManualReviewOrder[]> {
  const data = await request('/orders/manual-review')
  return data.items
}

export async function resolveManualReview(orderId: string, action: 'confirm' | 'cancel' | 'refund', reason: string): Promise<void> {
  await request(`/orders/${orderId}/manual-review/resolve`, {
    method: 'POST',
    body: JSON.stringify({ action, reason, actor_type: 'admin_ui' }),
  })
}
