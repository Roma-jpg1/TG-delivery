import { useEffect, useMemo, useState } from 'react'
import {
  getManualReviewOrders,
  getMenu,
  getPayments,
  getRefunds,
  getStopList,
  getStoredAdminToken,
  ManualReviewOrder,
  MenuItem,
  Payment,
  Refund,
  requestRefund,
  resolveManualReview,
  setAvailability,
  StopListItem,
  storeAdminToken,
  updateMenuItem,
} from './api'

const DEFAULT_BRANCH_ID = import.meta.env.VITE_BRANCH_ID ?? '11111111-1111-1111-1111-111111111111'

type Tab = 'catalog' | 'stoplist' | 'orders' | 'payments'

function formatMoney(value: number, currency = 'RUB') {
  return new Intl.NumberFormat('ru-RU', { style: 'currency', currency, maximumFractionDigits: 0 }).format(value)
}

export function App() {
  const [branchId] = useState(DEFAULT_BRANCH_ID)
  const [token, setToken] = useState(getStoredAdminToken())
  const [tab, setTab] = useState<Tab>('catalog')
  const [menu, setMenu] = useState<MenuItem[]>([])
  const [stopList, setStopList] = useState<StopListItem[]>([])
  const [manualReview, setManualReview] = useState<ManualReviewOrder[]>([])
  const [payments, setPayments] = useState<Payment[]>([])
  const [refunds, setRefunds] = useState<Refund[]>([])
  const [query, setQuery] = useState('')
  const [editingId, setEditingId] = useState('')
  const [draft, setDraft] = useState<MenuItem | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')

  useEffect(() => {
    void refresh()
  }, [])

  const filteredMenu = useMemo(() => {
    const q = query.trim().toLowerCase()
    if (!q) return menu
    return menu.filter((item) => `${item.name} ${item.category_name ?? ''} ${item.description ?? ''}`.toLowerCase().includes(q))
  }, [menu, query])

  async function refresh(nextToken = token) {
    setLoading(true)
    setError('')
    try {
      const [catalog, s, m, p, rf] = await Promise.all([
        getMenu(branchId, nextToken),
        getStopList(branchId, nextToken),
        getManualReviewOrders(nextToken),
        getPayments(nextToken),
        getRefunds(nextToken),
      ])
      setMenu(catalog)
      setStopList(s)
      setManualReview(m)
      setPayments(p)
      setRefunds(rf)
      storeAdminToken(nextToken)
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setLoading(false)
    }
  }

  function startEdit(item: MenuItem) {
    setEditingId(item.menu_item_id)
    setDraft({ ...item })
  }

  function cancelEdit() {
    setEditingId('')
    setDraft(null)
  }

  async function saveDraft() {
    if (!draft) return
    setError('')
    setSuccess('')
    try {
      const saved = await updateMenuItem(branchId, draft, token)
      setMenu((items) => items.map((item) => (item.menu_item_id === saved.menu_item_id ? saved : item)))
      setSuccess('Позиция меню сохранена')
      cancelEdit()
      await refresh()
    } catch (e) {
      setError((e as Error).message)
    }
  }

  async function quickStatus(menuItemId: string, status: string) {
    setError('')
    setSuccess('')
    try {
      await setAvailability(branchId, menuItemId, status, status === 'out_of_stock' ? 'Закончилось на кухне' : 'Вернули в меню', token)
      setSuccess(status === 'available' ? 'Позиция доступна' : 'Позиция скрыта из продажи')
      await refresh()
    } catch (e) {
      setError((e as Error).message)
    }
  }

  async function resolve(orderId: string, action: 'confirm' | 'cancel' | 'refund') {
    setError('')
    setSuccess('')
    try {
      await resolveManualReview(orderId, action, `Admin action: ${action}`, token)
      setSuccess(`Заказ обработан: ${action}`)
      await refresh()
    } catch (e) {
      setError((e as Error).message)
    }
  }

  async function refund(orderId: string) {
    setError('')
    setSuccess('')
    try {
      await requestRefund(orderId, 'Manual admin refund', token)
      setSuccess('Возврат запрошен')
      await refresh()
    } catch (e) {
      setError((e as Error).message)
    }
  }

  return (
    <main className="admin-shell">
      <header className="admin-header">
        <div>
          <span>AYAT Delivery</span>
          <h1>Панель управления</h1>
        </div>
        <label className="token-field">
          Admin token
          <input value={token} onChange={(event) => setToken(event.target.value)} />
        </label>
        <button onClick={() => void refresh(token)}>{loading ? 'Обновляем...' : 'Подключиться'}</button>
      </header>

      {(error || success) && (
        <div className="notice-row">
          {error && <p className="notice error">{error}</p>}
          {success && <p className="notice success">{success}</p>}
        </div>
      )}

      <section className="metric-grid">
        <article><b>{menu.length}</b><span>позиций меню</span></article>
        <article><b>{stopList.length}</b><span>в стоп-листе</span></article>
        <article><b>{manualReview.length}</b><span>на проверке</span></article>
        <article><b>{payments.length}</b><span>платежей</span></article>
      </section>

      <nav className="tabs" aria-label="Разделы админки">
        <button className={tab === 'catalog' ? 'active' : ''} onClick={() => setTab('catalog')}>Каталог</button>
        <button className={tab === 'stoplist' ? 'active' : ''} onClick={() => setTab('stoplist')}>Стоп-лист</button>
        <button className={tab === 'orders' ? 'active' : ''} onClick={() => setTab('orders')}>Заказы</button>
        <button className={tab === 'payments' ? 'active' : ''} onClick={() => setTab('payments')}>Платежи</button>
      </nav>

      {tab === 'catalog' && (
        <section className="admin-panel">
          <div className="panel-heading">
            <div>
              <h2>Каталог и фотографии</h2>
              <p>Фото вставляются в поле Photo URL. Можно указать внешний URL или путь вида /menu/balyk.jpg из папки public/menu.</p>
            </div>
            <input className="search-input" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Поиск по меню" />
          </div>

          <div className="catalog-table">
            {filteredMenu.map((item) => {
              const isEditing = editingId === item.menu_item_id && draft
              const row = isEditing ? draft : item
              return (
                <article className="catalog-row" key={item.menu_item_id}>
                  <div className="thumb">{row.photo_url ? <img src={row.photo_url} alt={row.name} /> : <span>{row.name.slice(0, 1)}</span>}</div>
                  <div className="row-main">
                    {isEditing ? (
                      <div className="edit-grid">
                        <label>Название<input value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} /></label>
                        <label>Цена<input type="number" value={draft.price} onChange={(event) => setDraft({ ...draft, price: Number(event.target.value) })} /></label>
                        <label className="wide">Описание<input value={draft.description ?? ''} onChange={(event) => setDraft({ ...draft, description: event.target.value })} /></label>
                        <label className="wide">Photo URL<input value={draft.photo_url ?? ''} onChange={(event) => setDraft({ ...draft, photo_url: event.target.value })} placeholder="/menu/name.jpg или https://..." /></label>
                        <label>Статус<select value={draft.status} onChange={(event) => setDraft({ ...draft, status: event.target.value })}><option value="available">available</option><option value="out_of_stock">out_of_stock</option><option value="disabled">disabled</option><option value="hidden">hidden</option></select></label>
                      </div>
                    ) : (
                      <>
                        <b>{item.name}</b>
                        <span>{item.category_name || 'Без категории'} · {formatMoney(item.price)} · {item.status}</span>
                        <p>{item.description || 'Описание не задано'}</p>
                        <small>{item.photo_url || 'Фото не задано'}</small>
                      </>
                    )}
                  </div>
                  <div className="row-actions">
                    {isEditing ? (
                      <>
                        <button onClick={saveDraft}>Сохранить</button>
                        <button className="ghost" onClick={cancelEdit}>Отмена</button>
                      </>
                    ) : (
                      <>
                        <button onClick={() => startEdit(item)}>Изменить</button>
                        <button className="ghost" onClick={() => quickStatus(item.menu_item_id, item.status === 'available' ? 'out_of_stock' : 'available')}>
                          {item.status === 'available' ? 'В стоп' : 'Вернуть'}
                        </button>
                      </>
                    )}
                  </div>
                </article>
              )
            })}
          </div>
        </section>
      )}

      {tab === 'stoplist' && (
        <section className="admin-panel">
          <div className="panel-heading"><h2>Стоп-лист</h2><button onClick={() => void refresh()}>Обновить</button></div>
          {stopList.length === 0 ? <p className="empty">Все позиции доступны.</p> : stopList.map((item) => (
            <article className="simple-row" key={item.menu_item_id}>
              <div><b>{item.menu_item_name}</b><span>{item.status} · {item.reason || 'без причины'}</span></div>
              <button onClick={() => quickStatus(item.menu_item_id, 'available')}>Вернуть</button>
            </article>
          ))}
        </section>
      )}

      {tab === 'orders' && (
        <section className="admin-panel">
          <div className="panel-heading"><h2>Ручная проверка</h2><button onClick={() => void refresh()}>Обновить</button></div>
          {manualReview.length === 0 ? <p className="empty">Заказов на проверке нет.</p> : manualReview.map((order) => (
            <article className="simple-row" key={order.order_id}>
              <div><b>#{order.order_number}</b><span>{order.status} · {formatMoney(order.total, order.currency)}</span></div>
              <div className="inline-actions">
                <button onClick={() => resolve(order.order_id, 'confirm')}>Подтвердить</button>
                <button className="danger" onClick={() => resolve(order.order_id, 'cancel')}>Отменить</button>
                <button className="ghost" onClick={() => refund(order.order_id)}>Возврат</button>
              </div>
            </article>
          ))}
        </section>
      )}

      {tab === 'payments' && (
        <section className="admin-panel split-panel">
          <div>
            <div className="panel-heading"><h2>Платежи</h2></div>
            {payments.length === 0 ? <p className="empty">Платежей пока нет.</p> : payments.map((item) => (
              <article className="simple-row" key={item.payment_id}>
                <div><b>{item.status}</b><span>{item.provider} · {formatMoney(item.amount, item.currency)}</span></div>
              </article>
            ))}
          </div>
          <div>
            <div className="panel-heading"><h2>Возвраты</h2></div>
            {refunds.length === 0 ? <p className="empty">Возвратов пока нет.</p> : refunds.map((item) => (
              <article className="simple-row" key={item.id}>
                <div><b>{item.status}</b><span>{formatMoney(item.amount, item.currency)} · order {item.order_id}</span></div>
              </article>
            ))}
          </div>
        </section>
      )}
    </main>
  )
}
