import { useEffect, useMemo, useState } from 'react'
import {
  Address,
  createDraft,
  createPaymentSession,
  deleteAddress,
  getAddresses,
  getCart,
  getDeliveryQuote,
  getMenu,
  getOrders,
  MenuItem,
  repeatOrder,
  upsertAddress,
  upsertCartItem,
} from './api'

const DEFAULT_BRANCH_ID = import.meta.env.VITE_BRANCH_ID ?? '11111111-1111-1111-1111-111111111111'
const DEFAULT_USER_ID = import.meta.env.VITE_USER_ID ?? '22222222-2222-2222-2222-222222222222'
const DEFAULT_ADDRESS_LAT = Number(import.meta.env.VITE_DEFAULT_ADDRESS_LAT ?? '51.802847')
const DEFAULT_ADDRESS_LON = Number(import.meta.env.VITE_DEFAULT_ADDRESS_LON ?? '85.748587')

const fallbackPhotos = [
  'https://images.unsplash.com/photo-1565299624946-b28f40a0ae38?auto=format&fit=crop&w=900&q=80',
  'https://images.unsplash.com/photo-1544025162-d76694265947?auto=format&fit=crop&w=900&q=80',
  'https://images.unsplash.com/photo-1563379926898-05f4575a45d8?auto=format&fit=crop&w=900&q=80',
  'https://images.unsplash.com/photo-1546069901-ba9599a7e63c?auto=format&fit=crop&w=900&q=80',
  'https://images.unsplash.com/photo-1555939594-58d7cb561ad1?auto=format&fit=crop&w=900&q=80',
]

const promoCards = [
  { title: 'Новый обед', text: 'Соберите быстрый заказ на день' },
  { title: 'Сезонное меню', text: 'Легкие блюда и свежие сочетания' },
  { title: 'Первый заказ', text: 'Промокод применится при оформлении' },
]

const quickFilters = ['Все', 'Пицца', 'Напитки', 'Острое', 'Без мяса']

type DeliveryMode = 'delivery' | 'pickup'

type CartState = {
  id: string
  total: number
  subtotal?: number
  currency: string
  items: Array<{ menu_item_id: string; quantity: number; line_total: number; menu_item_name: string }>
} | null

type OrderSummary = {
  order_id: string
  order_number: number
  status: string
  total: number
  currency: string
}

function formatMoney(value: number, currency = 'RUB') {
  return new Intl.NumberFormat('ru-RU', {
    style: 'currency',
    currency,
    maximumFractionDigits: 0,
  }).format(value)
}

function getPhoto(item: MenuItem, index: number) {
  return item.photo_url || ''
}

function getCategory(item: MenuItem) {
  return item.category_name || 'Меню'
}

function itemMatchesFilter(item: MenuItem, filter: string) {
  if (filter === 'Все') return true
  const haystack = `${item.name} ${item.description ?? ''} ${getCategory(item)}`.toLowerCase()
  if (filter === 'Пицца') return haystack.includes('pizza') || haystack.includes('пиц')
  if (filter === 'Напитки') return haystack.includes('drink') || haystack.includes('cola') || haystack.includes('напит')
  if (filter === 'Острое') return haystack.includes('spicy') || haystack.includes('остр') || haystack.includes('pepperoni')
  if (filter === 'Без мяса') return haystack.includes('margherita') || haystack.includes('овощ') || haystack.includes('без мяса')
  return true
}

export function App() {
  const branchId = DEFAULT_BRANCH_ID
  const userId = DEFAULT_USER_ID

  const [menu, setMenu] = useState<MenuItem[]>([])
  const [cart, setCart] = useState<CartState>(null)
  const [orders, setOrders] = useState<OrderSummary[]>([])
  const [addresses, setAddresses] = useState<Address[]>([])
  const [selectedAddressId, setSelectedAddressId] = useState('')
  const [addressDraft, setAddressDraft] = useState({
    id: '',
    label: 'Дом',
    city: 'Республика Алтай',
    street: 'Чуйский тракт',
    house: '477 км',
    apartment: '',
    entrance: '',
    floor: '',
    comment: '',
  })
  const [selectedCategory, setSelectedCategory] = useState('Все')
  const [selectedFilter, setSelectedFilter] = useState('Все')
  const [query, setQuery] = useState('')
  const [deliveryMode, setDeliveryMode] = useState<DeliveryMode>('delivery')
  const [activeItem, setActiveItem] = useState<MenuItem | null>(null)
  const [showCart, setShowCart] = useState(false)
  const [showMenu, setShowMenu] = useState(false)
  const [showLogin, setShowLogin] = useState(false)
  const [showAddressEditor, setShowAddressEditor] = useState(false)
  const [profile, setProfile] = useState(() => ({
    name: localStorage.getItem('delivery-profile-name') ?? '',
    phone: localStorage.getItem('delivery-profile-phone') ?? '',
  }))
  const [cookieVisible, setCookieVisible] = useState(() => localStorage.getItem('delivery-cookie-ok') !== '1')
  const [deliveryInfo, setDeliveryInfo] = useState('')
  const [loading, setLoading] = useState(false)
  const [busyItemId, setBusyItemId] = useState('')
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')

  useEffect(() => {
    void refresh()
  }, [])

  async function refresh() {
    setLoading(true)
    setError('')
    try {
      const [menuItems, cartData, userOrders, userAddresses] = await Promise.all([
        getMenu(branchId),
        getCart(userId, branchId),
        getOrders(userId),
        getAddresses(userId),
      ])
      setMenu(menuItems)
      setCart(cartData)
      setOrders(userOrders)
      setAddresses(userAddresses)
      const currentAddressStillExists = userAddresses.some((item) => item.id === selectedAddressId)
      if (!currentAddressStillExists) {
        const def = userAddresses.find((item) => item.is_default) ?? userAddresses[0]
        setSelectedAddressId(def?.id ?? '')
      }
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

  const categories = useMemo(() => {
    const names = Array.from(new Set(menu.map(getCategory)))
    return ['Все', ...names]
  }, [menu])

  const filteredMenu = useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase()
    return menu.filter((item) => {
      const categoryMatches = selectedCategory === 'Все' || getCategory(item) === selectedCategory
      const filterMatches = itemMatchesFilter(item, selectedFilter)
      const textMatches =
        normalizedQuery.length === 0 ||
        item.name.toLowerCase().includes(normalizedQuery) ||
        (item.description ?? '').toLowerCase().includes(normalizedQuery)
      return categoryMatches && filterMatches && textMatches
    })
  }, [menu, query, selectedCategory, selectedFilter])

  const recommended = useMemo(() => {
    const inCart = menu.filter((item) => quantities.has(item.menu_item_id))
    const source = inCart.length > 0 ? inCart : menu
    return source.slice(0, 8)
  }, [menu, quantities])

  const cartCount = cart?.items.reduce((sum, item) => sum + item.quantity, 0) ?? 0
  const selectedAddress = addresses.find((item) => item.id === selectedAddressId)
  const canCheckout = Boolean(cart && cart.items.length > 0 && (deliveryMode === 'pickup' || selectedAddressId))

  async function changeQuantity(menuItemId: string, quantity: number) {
    setError('')
    setSuccess('')
    setBusyItemId(menuItemId)
    try {
      const updated = await upsertCartItem(userId, branchId, menuItemId, Math.max(quantity, 0))
      setCart(updated)
      setShowCart(true)
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setBusyItemId('')
    }
  }

  async function add(menuItemId: string) {
    await changeQuantity(menuItemId, (quantities.get(menuItemId) ?? 0) + 1)
  }

  async function remove(menuItemId: string) {
    const current = quantities.get(menuItemId) ?? 0
    if (current > 0) {
      await changeQuantity(menuItemId, current - 1)
    }
  }

  async function checkout() {
    if (!cart) return
    setError('')
    setSuccess('')
    setDeliveryInfo('')
    try {
      if (deliveryMode === 'delivery' && selectedAddressId) {
        const quote = await getDeliveryQuote({
          user_id: userId,
          branch_id: branchId,
          address_id: selectedAddressId,
          cart_subtotal: cart.total,
        })
        setDeliveryInfo(`Доставка ${formatMoney(quote.delivery_fee, cart.currency)}, расстояние ${Math.round(quote.distance_meters / 100) / 10} км`)
      }
      const draft = await createDraft(userId, branchId, deliveryMode === 'delivery' ? selectedAddressId : undefined)
      const session = await createPaymentSession(draft.order_id)
      setSuccess(`Заказ создан. Платежная сессия: ${session.provider_session_id}`)
      setShowCart(false)
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
      setSuccess('Заказ добавлен в корзину')
      setShowCart(true)
      await refresh()
    } catch (e) {
      setError((e as Error).message)
    }
  }

  async function saveDemoAddress() {
    setError('')
    setSuccess('')
    try {
      const address = await upsertAddress({
        user_id: userId,
        label: addressDraft.label || 'Дом',
        city: addressDraft.city || 'Республика Алтай',
        street: addressDraft.street || 'Чуйский тракт',
        house: addressDraft.house || '477 км',
        apartment: addressDraft.apartment,
        entrance: addressDraft.entrance,
        floor: addressDraft.floor,
        comment: addressDraft.comment,
        latitude: DEFAULT_ADDRESS_LAT,
        longitude: DEFAULT_ADDRESS_LON,
        set_default: true,
      })
      setSelectedAddressId(address.id)
      setSuccess('Адрес доставки сохранен')
      await refresh()
    } catch (e) {
      setError((e as Error).message)
    }
  }

  function openAddressEditor(address?: Address) {
    setAddressDraft({
      id: address?.id ?? '',
      label: address?.label || 'Дом',
      city: address?.city || 'Республика Алтай',
      street: address?.street || 'Чуйский тракт',
      house: address?.house || '477 км',
      apartment: address?.apartment ?? '',
      entrance: address?.entrance ?? '',
      floor: address?.floor ?? '',
      comment: address?.comment ?? '',
    })
    setShowAddressEditor(true)
  }

  async function saveAddressDraft() {
    setError('')
    setSuccess('')
    try {
      const address = await upsertAddress({
        address_id: addressDraft.id || undefined,
        user_id: userId,
        label: addressDraft.label,
        city: addressDraft.city,
        street: addressDraft.street,
        house: addressDraft.house,
        apartment: addressDraft.apartment,
        entrance: addressDraft.entrance,
        floor: addressDraft.floor,
        comment: addressDraft.comment,
        latitude: DEFAULT_ADDRESS_LAT,
        longitude: DEFAULT_ADDRESS_LON,
        set_default: true,
      })
      setSelectedAddressId(address.id)
      setShowAddressEditor(false)
      setSuccess('Адрес сохранен')
      await refresh()
    } catch (e) {
      setError((e as Error).message)
    }
  }

  async function removeAddress(addressId: string) {
    setError('')
    setSuccess('')
    try {
      await deleteAddress(userId, addressId)
      if (selectedAddressId === addressId) {
        setSelectedAddressId('')
      }
      setSuccess('Адрес удален')
      await refresh()
    } catch (e) {
      setError((e as Error).message)
    }
  }

  function saveProfile() {
    localStorage.setItem('delivery-profile-name', profile.name)
    localStorage.setItem('delivery-profile-phone', profile.phone)
    setShowLogin(false)
    setSuccess(profile.name ? `${profile.name}, вход выполнен` : 'Вход выполнен')
  }

  function acceptCookies() {
    localStorage.setItem('delivery-cookie-ok', '1')
    setCookieVisible(false)
  }

  return (
    <main className="site-shell">
      <header className="hero">
        <nav className="topbar" aria-label="Главная навигация">
          <button className="icon-button" aria-label="Открыть меню" onClick={() => setShowMenu(true)}>☰</button>
          <a className="brand" href="#menu">AYAT Delivery</a>
          <div className="topbar-actions">
            <button className="text-button inverse" onClick={() => setShowLogin(true)}>
              {profile.name || 'Войти'}
            </button>
            <button className="icon-button" aria-label="Открыть корзину" onClick={() => setShowCart(true)}>
              <span>Корзина</span>
              {cartCount > 0 && <strong>{cartCount}</strong>}
            </button>
          </div>
        </nav>

        <section className="hero-panel" aria-label="Выбор доставки">
          <div className="mode-switch">
            <button className={deliveryMode === 'delivery' ? 'active' : ''} onClick={() => setDeliveryMode('delivery')}>
              Доставка
            </button>
            <button className={deliveryMode === 'pickup' ? 'active' : ''} onClick={() => setDeliveryMode('pickup')}>
              Самовывоз
            </button>
          </div>

          <button className="address-button" onClick={() => openAddressEditor(selectedAddress)}>
            {deliveryMode === 'pickup'
              ? 'Самовывоз из ресторана на Красном проспекте'
              : selectedAddress
                ? `${selectedAddress.city}, ${selectedAddress.street} ${selectedAddress.house}`
                : 'Указать адрес доставки'}
          </button>

          <div className="bonus-strip">
            <div>
              <b>Получать бонусы и скидки</b>
              <span>Авторизуйтесь, чтобы копить и использовать баллы</span>
            </div>
            <a href="#orders">Подробнее</a>
          </div>
        </section>
      </header>

      <section className="content-band">
        <div className="page-width">
          {(error || success || deliveryInfo) && (
            <div className="notice-stack" aria-live="polite">
              {error && <p className="notice error">{error}</p>}
              {success && <p className="notice success">{success}</p>}
              {deliveryInfo && <p className="notice info">{deliveryInfo}</p>}
            </div>
          )}

          <section className="promo-row" aria-label="Акции">
            {promoCards.map((card, index) => (
              <article className="promo-card" key={card.title}>
                <img src={fallbackPhotos[index]} alt="" />
                <div>
                  <h2>{card.title}</h2>
                  <p>{card.text}</p>
                </div>
              </article>
            ))}
          </section>

          <section className="recommendations" aria-labelledby="recommend-title">
            <h2 id="recommend-title">Рекомендуем</h2>
            <div className="recommend-strip">
              {recommended.map((item, index) => (
                <button className="recommend-card" key={item.menu_item_id} onClick={() => setActiveItem(item)}>
                  {getPhoto(item, index) ? <img src={getPhoto(item, index)} alt="" /> : <i>{item.name.slice(0, 1)}</i>}
                  <span>{item.name}</span>
                  <b>{formatMoney(item.price)}</b>
                </button>
              ))}
            </div>
          </section>

          <section className="catalog-layout" id="menu">
            <aside className="category-rail" aria-label="Категории меню">
              <h2>Меню доставки</h2>
              <div className="category-list">
                {categories.map((category) => (
                  <button
                    className={selectedCategory === category ? 'active' : ''}
                    key={category}
                    onClick={() => setSelectedCategory(category)}
                  >
                    {category}
                  </button>
                ))}
              </div>
            </aside>

            <section className="catalog-main" aria-label="Каталог блюд">
              <div className="catalog-toolbar">
                <div>
                  <h2>{selectedCategory}</h2>
                  <p>{loading ? 'Загружаем меню...' : `${filteredMenu.length} позиций`}</p>
                </div>
                <label className="search-field">
                  <span>Поиск</span>
                  <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Пицца, напиток, салат" />
                </label>
              </div>

              <div className="filter-row" aria-label="Быстрые фильтры">
                <span>Фильтр</span>
                {quickFilters.map((filter) => (
                  <button
                    className={selectedFilter === filter ? 'active' : ''}
                    key={filter}
                    onClick={() => setSelectedFilter(filter)}
                  >
                    {filter}
                  </button>
                ))}
              </div>

              <div className="product-grid">
                {filteredMenu.map((item, index) => {
                  const quantity = quantities.get(item.menu_item_id) ?? 0
                  return (
                    <article className="product-card" key={item.menu_item_id}>
                      <button className="product-photo" onClick={() => setActiveItem(item)} aria-label={`Открыть ${item.name}`}>
                        {getPhoto(item, index) ? <img src={getPhoto(item, index)} alt={item.name} /> : <span>{item.name.slice(0, 1)}</span>}
                      </button>
                      <div className="product-body">
                        <button className="product-title" onClick={() => setActiveItem(item)}>{item.name}</button>
                        <p>{item.description || getCategory(item)}</p>
                        <div className="product-footer">
                          {quantity > 0 ? (
                            <div className="quantity-control" aria-label={`Количество ${item.name}`}>
                              <button disabled={busyItemId === item.menu_item_id} onClick={() => remove(item.menu_item_id)}>-</button>
                              <span>{quantity}</span>
                              <button disabled={busyItemId === item.menu_item_id} onClick={() => add(item.menu_item_id)}>+</button>
                            </div>
                          ) : (
                            <button className="price-button" disabled={busyItemId === item.menu_item_id} onClick={() => add(item.menu_item_id)}>
                              {formatMoney(item.price)}
                            </button>
                          )}
                        </div>
                      </div>
                    </article>
                  )
                })}
              </div>
            </section>
          </section>

          <section className="site-section" id="addresses">
            <div className="section-heading">
              <h2>Адрес доставки</h2>
              <button onClick={() => openAddressEditor()}>Добавить адрес</button>
            </div>
            {addresses.length === 0 ? (
              <p className="muted">Адресов пока нет. Можно сохранить демо-адрес и проверить оформление заказа.</p>
            ) : (
              <div className="address-grid">
                {addresses.map((address) => (
                  <label className="address-card" key={address.id}>
                    <input
                      type="radio"
                      name="address"
                      checked={selectedAddressId === address.id}
                      onChange={() => setSelectedAddressId(address.id)}
                    />
                    <span>
                      <b>{address.label || 'Адрес'}</b>
                      {address.city}, {address.street} {address.house}
                    </span>
                    <button type="button" onClick={() => openAddressEditor(address)}>Изменить</button>
                    <button type="button" className="ghost-danger" onClick={() => removeAddress(address.id)}>Удалить</button>
                  </label>
                ))}
              </div>
            )}
          </section>

          <section className="site-section" id="orders">
            <div className="section-heading">
              <h2>История заказов</h2>
              <button onClick={() => void refresh()}>Обновить</button>
            </div>
            {orders.length === 0 ? (
              <p className="muted">После оформления здесь появятся последние заказы.</p>
            ) : (
              <div className="order-list">
                {orders.map((order) => (
                  <article className="order-card" key={order.order_id}>
                    <div>
                      <b>Заказ #{order.order_number}</b>
                      <span>{order.status}</span>
                    </div>
                    <strong>{formatMoney(order.total, order.currency)}</strong>
                    <button onClick={() => repeat(order.order_id)}>Повторить</button>
                  </article>
                ))}
              </div>
            )}
          </section>
        </div>
      </section>

      <aside className={`cart-drawer ${showCart ? 'open' : ''}`} aria-label="Корзина">
        <div className="cart-header">
          <h2>Корзина</h2>
          <button className="icon-button" onClick={() => setShowCart(false)} aria-label="Закрыть корзину">×</button>
        </div>
        {!cart || cart.items.length === 0 ? (
          <p className="muted">Добавьте блюда из меню, и заказ соберется здесь.</p>
        ) : (
          <>
            <div className="cart-items">
              {cart.items.map((item) => (
                <article className="cart-item" key={item.menu_item_id}>
                  <div>
                    <b>{item.menu_item_name}</b>
                    <span>{formatMoney(item.line_total, cart.currency)}</span>
                  </div>
                  <div className="quantity-control">
                    <button onClick={() => remove(item.menu_item_id)}>-</button>
                    <span>{item.quantity}</span>
                    <button onClick={() => add(item.menu_item_id)}>+</button>
                  </div>
                </article>
              ))}
            </div>
            <div className="checkout-box">
              <div>
                <span>Итого</span>
                <strong>{formatMoney(cart.total, cart.currency)}</strong>
              </div>
              <button disabled={!canCheckout} onClick={checkout}>
                Оформить заказ
              </button>
              {!canCheckout && <p>Для доставки выберите адрес.</p>}
            </div>
          </>
        )}
      </aside>

      {showCart && <button className="drawer-backdrop" aria-label="Закрыть корзину" onClick={() => setShowCart(false)} />}

      {showMenu && (
        <div className="modal-layer menu-layer" role="dialog" aria-modal="true" aria-label="Меню сайта">
          <nav className="side-menu">
            <div className="cart-header">
              <h2>AYAT Delivery</h2>
              <button className="icon-button" onClick={() => setShowMenu(false)} aria-label="Закрыть меню">×</button>
            </div>
            <a href="#menu" onClick={() => setShowMenu(false)}>Меню доставки</a>
            <a href="#addresses" onClick={() => setShowMenu(false)}>Адрес доставки</a>
            <a href="#orders" onClick={() => setShowMenu(false)}>История заказов</a>
            <button onClick={() => { setShowMenu(false); setShowLogin(true) }}>Войти</button>
            <button onClick={() => { setShowMenu(false); setShowCart(true) }}>Открыть корзину</button>
          </nav>
        </div>
      )}

      {showLogin && (
        <div className="modal-layer" role="dialog" aria-modal="true" aria-label="Вход">
          <article className="form-modal">
            <button className="icon-button close-modal" onClick={() => setShowLogin(false)} aria-label="Закрыть">×</button>
            <h2>Вход для заказа</h2>
            <p>Для демо вход сохраняется в браузере. Этого достаточно, чтобы показать сценарий завтра.</p>
            <label>
              Имя
              <input value={profile.name} onChange={(event) => setProfile({ ...profile, name: event.target.value })} placeholder="Ваше имя" />
            </label>
            <label>
              Телефон
              <input value={profile.phone} onChange={(event) => setProfile({ ...profile, phone: event.target.value })} placeholder="+7..." />
            </label>
            <button className="price-button" onClick={saveProfile}>Войти</button>
          </article>
        </div>
      )}

      {showAddressEditor && (
        <div className="modal-layer" role="dialog" aria-modal="true" aria-label="Изменение адреса">
          <article className="form-modal address-modal">
            <button className="icon-button close-modal" onClick={() => setShowAddressEditor(false)} aria-label="Закрыть">×</button>
            <h2>{addressDraft.id ? 'Изменить адрес' : 'Добавить адрес'}</h2>
            <div className="form-grid">
              <label>
                Название
                <input value={addressDraft.label} onChange={(event) => setAddressDraft({ ...addressDraft, label: event.target.value })} />
              </label>
              <label>
                Город / район
                <input value={addressDraft.city} onChange={(event) => setAddressDraft({ ...addressDraft, city: event.target.value })} />
              </label>
              <label>
                Улица
                <input value={addressDraft.street} onChange={(event) => setAddressDraft({ ...addressDraft, street: event.target.value })} />
              </label>
              <label>
                Дом
                <input value={addressDraft.house} onChange={(event) => setAddressDraft({ ...addressDraft, house: event.target.value })} />
              </label>
              <label>
                Квартира
                <input value={addressDraft.apartment} onChange={(event) => setAddressDraft({ ...addressDraft, apartment: event.target.value })} />
              </label>
              <label>
                Подъезд
                <input value={addressDraft.entrance} onChange={(event) => setAddressDraft({ ...addressDraft, entrance: event.target.value })} />
              </label>
              <label className="wide-field">
                Комментарий курьеру
                <input value={addressDraft.comment} onChange={(event) => setAddressDraft({ ...addressDraft, comment: event.target.value })} />
              </label>
            </div>
            <button className="price-button" onClick={saveAddressDraft}>Сохранить адрес</button>
          </article>
        </div>
      )}

      {activeItem && (
        <div className="modal-layer" role="dialog" aria-modal="true" aria-label={activeItem.name}>
          <article className="product-modal">
            <button className="icon-button close-modal" onClick={() => setActiveItem(null)} aria-label="Закрыть">×</button>
            {getPhoto(activeItem, menu.indexOf(activeItem)) ? (
              <img src={getPhoto(activeItem, menu.indexOf(activeItem))} alt={activeItem.name} />
            ) : (
              <div className="product-modal-placeholder">{activeItem.name.slice(0, 1)}</div>
            )}
            <div>
              <span>{getCategory(activeItem)}</span>
              <h2>{activeItem.name}</h2>
              <p>{activeItem.description || 'Свежая позиция из меню ресторана.'}</p>
              <button className="price-button" onClick={() => add(activeItem.menu_item_id)}>
                Добавить за {formatMoney(activeItem.price)}
              </button>
            </div>
          </article>
        </div>
      )}

      {cookieVisible && (
        <div className="cookie-note">
          <span>Пользуясь сайтом, вы соглашаетесь со сбором cookies</span>
          <button onClick={acceptCookies}>OK</button>
        </div>
      )}
    </main>
  )
}
