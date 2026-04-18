import { useEffect, useMemo, useState } from 'react'
import { createDraft, createPaymentSession, getCart, getMenu, getOrders, MenuItem, repeatOrder, upsertCartItem } from './api'

const DEFAULT_BRANCH_ID = import.meta.env.VITE_BRANCH_ID ?? '11111111-1111-1111-1111-111111111111'
const DEFAULT_USER_ID = import.meta.env.VITE_USER_ID ?? '22222222-2222-2222-2222-222222222222'

type CartState = {
  id: string
  total: number
  currency: string
  items: Array<{ menu_item_id: string; quantity: number; line_total: number; menu_item_name: string }>
} | null

export function App() {
  const branchId = DEFAULT_BRANCH_ID
  const userId = DEFAULT_USER_ID

  const [menu, setMenu] = useState<MenuItem[]>([])
  const [cart, setCart] = useState<CartState>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')
  const [orders, setOrders] = useState<Array<{ order_id: string; order_number: number; status: string; total: number; currency: string }>>([])

  useEffect(() => {
    void refresh()
  }, [])

  async function refresh() {
    setLoading(true)
    setError('')
    try {
      const [menuItems, cartData, userOrders] = await Promise.all([getMenu(branchId), getCart(userId, branchId), getOrders(userId)])
      setMenu(menuItems)
      setCart(cartData)
      setOrders(userOrders)
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setLoading(false)
    }
  }

  const quantities = useMemo(() => {
    const map = new Map<string, number>()
    cart?.items.forEach((item) => map.set(item.menu_item_id, item.quantity))
    return map
  }, [cart])

  async function add(menuItemId: string) {
    const current = quantities.get(menuItemId) ?? 0
    setError('')
    try {
      const updated = await upsertCartItem(userId, branchId, menuItemId, current + 1)
      setCart(updated)
    } catch (e) {
      setError((e as Error).message)
    }
  }

  async function remove(menuItemId: string) {
    const current = quantities.get(menuItemId) ?? 0
    if (current <= 0) return
    setError('')
    try {
      const updated = await upsertCartItem(userId, branchId, menuItemId, current - 1)
      setCart(updated)
    } catch (e) {
      setError((e as Error).message)
    }
  }

  async function checkout() {
    setError('')
    setSuccess('')
    try {
      const draft = await createDraft(userId, branchId)
      const session = await createPaymentSession(draft.order_id)
      setSuccess(`Draft ${draft.order_id} created. Session: ${session.provider_session_id}`)
      await refresh()
    } catch (e) {
      setError((e as Error).message)
    }
  }

  async function repeat(orderId: string) {
    setError('')
    setSuccess('')
    try {
      await repeatOrder(orderId, userId)
      setSuccess(`Order ${orderId} repeated into active cart`)
      await refresh()
    } catch (e) {
      setError((e as Error).message)
    }
  }

  return (
    <main className="layout">
      <header>
        <h1>Mini App</h1>
        <p>Menu, cart and checkout flow</p>
      </header>

      {loading && <p>Loading...</p>}
      {error && <p className="error">{error}</p>}
      {success && <p className="success">{success}</p>}

      <section>
        <h2>Menu</h2>
        <ul className="menu-grid">
          {menu.map((item) => (
            <li key={item.menu_item_id}>
              <h3>{item.name}</h3>
              <p>{item.description}</p>
              <p>{item.price}</p>
              <div className="controls">
                <button onClick={() => remove(item.menu_item_id)}>-</button>
                <span>{quantities.get(item.menu_item_id) ?? 0}</span>
                <button onClick={() => add(item.menu_item_id)}>+</button>
              </div>
            </li>
          ))}
        </ul>
      </section>

      <section>
        <h2>Cart</h2>
        {cart ? (
          <>
            <p>Total: {cart.total} {cart.currency}</p>
            <button onClick={checkout}>Checkout</button>
          </>
        ) : (
          <p>Cart is empty</p>
        )}
      </section>

      <section>
        <h2>My orders</h2>
        {orders.length === 0 ? <p>No orders yet</p> : (
          <ul className="menu-grid">
            {orders.map((o) => (
              <li key={o.order_id}>
                <h3>#{o.order_number}</h3>
                <p>Status: {o.status}</p>
                <p>{o.total} {o.currency}</p>
                <button onClick={() => repeat(o.order_id)}>Repeat</button>
              </li>
            ))}
          </ul>
        )}
      </section>
    </main>
  )
}
