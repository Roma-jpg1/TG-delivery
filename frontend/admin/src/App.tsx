import { useEffect, useState } from 'react'
import { getManualReviewOrders, getPayments, getRefunds, getStopList, requestRefund, resolveManualReview, setAvailability } from './api'

const DEFAULT_BRANCH_ID = import.meta.env.VITE_BRANCH_ID ?? '11111111-1111-1111-1111-111111111111'

export function App() {
  const [branchId] = useState(DEFAULT_BRANCH_ID)
  const [stopList, setStopList] = useState<any[]>([])
  const [manualReview, setManualReview] = useState<any[]>([])
  const [payments, setPayments] = useState<any[]>([])
  const [refunds, setRefunds] = useState<any[]>([])
  const [error, setError] = useState('')

  useEffect(() => {
    void refresh()
  }, [])

  async function refresh() {
    setError('')
    try {
      const [s, m, p, rf] = await Promise.all([getStopList(branchId), getManualReviewOrders(), getPayments(), getRefunds()])
      setStopList(s)
      setManualReview(m)
      setPayments(p)
      setRefunds(rf)
    } catch (e) {
      setError((e as Error).message)
    }
  }

  async function quickStatus(menuItemId: string, status: string) {
    try {
      await setAvailability(branchId, menuItemId, status, status === 'out_of_stock' ? 'Кухня: закончилось' : 'Кухня: возвращено')
      await refresh()
    } catch (e) {
      setError((e as Error).message)
    }
  }

  async function resolve(orderId: string, action: 'confirm' | 'cancel' | 'refund') {
    try {
      await resolveManualReview(orderId, action, `Admin action: ${action}`)
      await refresh()
    } catch (e) {
      setError((e as Error).message)
    }
  }

  async function refund(orderId: string) {
    try {
      await requestRefund(orderId, 'Manual admin refund')
      await refresh()
    } catch (e) {
      setError((e as Error).message)
    }
  }

  return (
    <main className="layout">
      <h1>Admin Panel</h1>
      {error && <p className="error">{error}</p>}

      <section>
        <h2>Stop-list</h2>
        <ul>
          {stopList.map((item) => (
            <li key={item.menu_item_id}>
              <strong>{item.menu_item_name}</strong> — {item.status}
              <div className="buttons">
                <button onClick={() => quickStatus(item.menu_item_id, 'out_of_stock')}>Закончилось</button>
                <button onClick={() => quickStatus(item.menu_item_id, 'available')}>Вернуть</button>
              </div>
            </li>
          ))}
        </ul>
      </section>

      <section>
        <h2>Manual review</h2>
        <ul>
          {manualReview.map((order) => (
            <li key={order.order_id}>
              <strong>#{order.order_number}</strong> — {order.status} ({order.total} {order.currency})
              <div className="buttons">
                <button onClick={() => resolve(order.order_id, 'confirm')}>Confirm</button>
                <button onClick={() => resolve(order.order_id, 'cancel')}>Cancel</button>
                <button onClick={() => resolve(order.order_id, 'refund')}>Refund</button>
                <button onClick={() => refund(order.order_id)}>Request refund</button>
              </div>
            </li>
          ))}
        </ul>
      </section>

      <section>
        <h2>Payments</h2>
        <ul>
          {payments.map((item) => (
            <li key={item.payment_id}>
              <strong>{item.status}</strong> — order {item.order_id} — {item.amount} {item.currency}
            </li>
          ))}
        </ul>
      </section>

      <section>
        <h2>Refunds</h2>
        <ul>
          {refunds.map((item) => (
            <li key={item.id}>
              <strong>{item.status}</strong> — order {item.order_id} — {item.amount} {item.currency}
            </li>
          ))}
        </ul>
      </section>
    </main>
  )
}
