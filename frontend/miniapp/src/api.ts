const API_BASE = import.meta.env.VITE_API_BASE ?? 'http://localhost:18080/api/v1'

export type MenuItem = {
  menu_item_id: string
  category_id?: string
  category_name?: string
  name: string
  description?: string
  photo_url?: string
  price: number
  status: string
}

export type Cart = {
  id: string
  user_id: string
  branch_id: string
  subtotal: number
  total: number
  currency: string
  items: Array<{
    id: string
    menu_item_id: string
    menu_item_name: string
    quantity: number
    unit_price: number
    line_total: number
  }>
}

export async function getMenu(branchId: string): Promise<MenuItem[]> {
  const resp = await fetch(`${API_BASE}/menu/branches/${branchId}`)
  if (!resp.ok) throw new Error('failed to fetch menu')
  const data = await resp.json()
  return data.items as MenuItem[]
}

export async function getCart(userId: string, branchId: string): Promise<Cart | null> {
  const resp = await fetch(`${API_BASE}/cart?user_id=${userId}&branch_id=${branchId}`)
  if (!resp.ok) throw new Error('failed to fetch cart')
  const data = await resp.json()
  return data.cart as Cart | null
}

export async function upsertCartItem(userId: string, branchId: string, menuItemId: string, quantity: number): Promise<Cart> {
  const resp = await fetch(`${API_BASE}/cart/items`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ user_id: userId, branch_id: branchId, menu_item_id: menuItemId, quantity }),
  })
  if (!resp.ok) throw new Error('failed to update cart')
  const data = await resp.json()
  return data.cart as Cart
}

export type Address = {
  id: string
  user_id: string
  label: string
  city: string
  street: string
  house: string
  apartment?: string
  entrance?: string
  floor?: string
  comment?: string
  latitude: number
  longitude: number
  is_default: boolean
}

export async function getAddresses(userId: string): Promise<Address[]> {
  const resp = await fetch(`${API_BASE}/addresses?user_id=${userId}`)
  if (!resp.ok) throw new Error('failed to load addresses')
  const data = await resp.json()
  return data.items as Address[]
}

export async function upsertAddress(payload: {
  address_id?: string
  user_id: string
  label: string
  city: string
  street: string
  house: string
  apartment?: string
  entrance?: string
  floor?: string
  comment?: string
  latitude: number
  longitude: number
  set_default: boolean
}): Promise<Address> {
  const resp = await fetch(`${API_BASE}/addresses`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
  if (!resp.ok) throw new Error('failed to save address')
  const data = await resp.json()
  return data.address as Address
}

export async function deleteAddress(userId: string, addressId: string): Promise<void> {
  const resp = await fetch(`${API_BASE}/addresses/${addressId}?user_id=${userId}`, { method: 'DELETE' })
  if (!resp.ok) throw new Error('failed to delete address')
}

export async function getDeliveryQuote(payload: {
  user_id: string
  branch_id: string
  address_id: string
  cart_subtotal: number
}): Promise<{ delivery_fee: number; distance_meters: number }> {
  const resp = await fetch(`${API_BASE}/delivery/quote`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
  if (!resp.ok) throw new Error('failed to calculate delivery')
  const data = await resp.json()
  return data.quote
}

export async function createDraft(userId: string, branchId: string, addressId?: string): Promise<{ order_id: string }> {
  const resp = await fetch(`${API_BASE}/checkout/draft`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ user_id: userId, branch_id: branchId, address_id: addressId ?? null }),
  })
  if (!resp.ok) throw new Error('failed to create draft')
  const data = await resp.json()
  return data.draft
}

export async function createPaymentSession(orderId: string): Promise<{ checkout_url: string; provider_session_id: string }> {
  const resp = await fetch(`${API_BASE}/payments/sessions`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ order_id: orderId, provider: 'mock', idempotency_key: `website-${orderId}` }),
  })
  if (!resp.ok) throw new Error('failed to create payment session')
  const data = await resp.json()
  return data.session
}

export async function getOrders(userId: string): Promise<Array<{ order_id: string; order_number: number; status: string; total: number; currency: string }>> {
  const resp = await fetch(`${API_BASE}/orders?user_id=${userId}&limit=10`)
  if (!resp.ok) throw new Error('failed to load orders')
  const data = await resp.json()
  return data.items
}

export async function repeatOrder(orderId: string, userId: string): Promise<void> {
  const resp = await fetch(`${API_BASE}/orders/${orderId}/repeat?user_id=${userId}`, { method: 'POST' })
  if (!resp.ok) throw new Error('failed to repeat order')
}
