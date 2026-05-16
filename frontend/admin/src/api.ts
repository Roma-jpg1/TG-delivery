const API_BASE = import.meta.env.VITE_API_BASE ?? 'http://localhost:18080/api/v1/admin'
const DEFAULT_ADMIN_TOKEN = import.meta.env.VITE_ADMIN_TOKEN ?? 'dev-admin-token'

export type MenuItem = {
  menu_item_id: string
  category_id?: string
  category_name?: string
  name: string
  description?: string
  photo_url?: string
  price: number
  status: string
  availability_note?: string
}

export type StopListItem = {
  menu_item_id: string
  menu_item_name: string
  status: string
  reason?: string
}

export type ManualReviewOrder = {
  order_id: string
  order_number: number
  status: string
  total: number
  currency: string
}

export type Payment = {
  payment_id: string
  order_id: string
  provider: string
  status: string
  amount: number
  currency: string
}

export type Refund = {
  id: string
  order_id: string
  status: string
  amount: number
  currency: string
}

export function getStoredAdminToken() {
  return localStorage.getItem('admin-token') || DEFAULT_ADMIN_TOKEN
}

export function storeAdminToken(token: string) {
  localStorage.setItem('admin-token', token)
}

async function request(path: string, init?: RequestInit, token = getStoredAdminToken()) {
  const resp = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      'X-Admin-Token': token,
      ...(init?.headers ?? {}),
    },
  })
  if (!resp.ok) {
    const text = await resp.text()
    throw new Error(text || `admin request failed: ${resp.status}`)
  }
  return resp.status === 204 ? null : resp.json()
}

export async function getMenu(branchId: string, token?: string): Promise<MenuItem[]> {
  const data = await request(`/branches/${branchId}/menu`, undefined, token)
  return data.items
}

export async function updateMenuItem(
  branchId: string,
  item: Pick<MenuItem, 'menu_item_id' | 'name' | 'description' | 'photo_url' | 'price' | 'status'> & { reason?: string },
  token?: string,
): Promise<MenuItem> {
  const data = await request(`/branches/${branchId}/menu-items/${item.menu_item_id}`, {
    method: 'PUT',
    body: JSON.stringify({
      name: item.name,
      description: item.description ?? '',
      photo_url: item.photo_url ?? '',
      price: Number(item.price),
      status: item.status,
      reason: item.reason ?? '',
    }),
  }, token)
  return data.item
}

export async function getStopList(branchId: string, token?: string): Promise<StopListItem[]> {
  const data = await request(`/branches/${branchId}/stop-list`, undefined, token)
  return data.items
}

export async function setAvailability(branchId: string, menuItemId: string, status: string, reason: string, token?: string): Promise<void> {
  await request(`/branches/${branchId}/menu-items/${menuItemId}/availability`, {
    method: 'PUT',
    body: JSON.stringify({ status, reason, actor_type: 'admin_ui' }),
  }, token)
}

export async function getManualReviewOrders(token?: string): Promise<ManualReviewOrder[]> {
  const data = await request('/orders/manual-review', undefined, token)
  return data.items
}

export async function resolveManualReview(orderId: string, action: 'confirm' | 'cancel' | 'refund', reason: string, token?: string): Promise<void> {
  await request(`/orders/${orderId}/manual-review/resolve`, {
    method: 'POST',
    body: JSON.stringify({ action, reason, actor_type: 'admin_ui' }),
  }, token)
}

export async function getPayments(token?: string): Promise<Payment[]> {
  const data = await request('/payments?limit=20', undefined, token)
  return data.items
}

export async function getRefunds(token?: string): Promise<Refund[]> {
  const data = await request('/refunds?limit=20', undefined, token)
  return data.items
}

export async function requestRefund(orderId: string, reason: string, token?: string): Promise<void> {
  await request(`/orders/${orderId}/refunds`, {
    method: 'POST',
    body: JSON.stringify({ reason, actor_type: 'admin_ui' }),
  }, token)
}
