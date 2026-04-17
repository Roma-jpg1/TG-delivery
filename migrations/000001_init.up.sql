CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TYPE branch_menu_item_status AS ENUM (
    'available',
    'out_of_stock',
    'disabled',
    'hidden',
    'archived'
);

CREATE TYPE order_status AS ENUM (
    'draft',
    'pending_payment',
    'payment_processing',
    'paid',
    'confirmed',
    'preparing',
    'ready_for_delivery',
    'out_for_delivery',
    'delivered',
    'cancelled',
    'refund_pending',
    'refunded',
    'payment_failed',
    'manual_review'
);

CREATE TYPE payment_status AS ENUM (
    'created',
    'pending',
    'succeeded',
    'failed',
    'cancelled',
    'refunded',
    'manual_review'
);

CREATE TYPE refund_status AS ENUM (
    'pending',
    'succeeded',
    'failed',
    'manual_review'
);

CREATE TYPE saga_status AS ENUM (
    'running',
    'completed',
    'failed',
    'compensating',
    'compensated',
    'manual_review'
);

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    telegram_user_id BIGINT UNIQUE,
    phone TEXT,
    first_name TEXT,
    last_name TEXT,
    username TEXT,
    locale TEXT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE restaurants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    slug TEXT UNIQUE,
    currency CHAR(3) NOT NULL,
    timezone TEXT NOT NULL,
    settings JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE branches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    restaurant_id UUID NOT NULL REFERENCES restaurants(id),
    name TEXT NOT NULL,
    code TEXT,
    address_line TEXT,
    latitude NUMERIC(9,6),
    longitude NUMERIC(9,6),
    delivery_radius_meters INTEGER,
    min_order_amount INTEGER NOT NULL DEFAULT 0 CHECK (min_order_amount >= 0),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (restaurant_id, code)
);

CREATE TABLE categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    restaurant_id UUID NOT NULL REFERENCES restaurants(id),
    name TEXT NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE menu_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    restaurant_id UUID NOT NULL REFERENCES restaurants(id),
    category_id UUID REFERENCES categories(id),
    name TEXT NOT NULL,
    description TEXT,
    photo_url TEXT,
    base_price INTEGER NOT NULL CHECK (base_price >= 0),
    tags JSONB NOT NULL DEFAULT '[]'::jsonb,
    attributes JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_deleted BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE branch_menu_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    branch_id UUID NOT NULL REFERENCES branches(id),
    menu_item_id UUID NOT NULL REFERENCES menu_items(id),
    price INTEGER NOT NULL CHECK (price >= 0),
    status branch_menu_item_status NOT NULL DEFAULT 'available',
    available_until TIMESTAMPTZ,
    reason TEXT,
    version INTEGER NOT NULL DEFAULT 1,
    updated_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (branch_id, menu_item_id)
);

CREATE TABLE item_option_groups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    menu_item_id UUID NOT NULL REFERENCES menu_items(id),
    name TEXT NOT NULL,
    min_select INTEGER NOT NULL DEFAULT 0,
    max_select INTEGER NOT NULL DEFAULT 1,
    sort_order INTEGER NOT NULL DEFAULT 0,
    is_required BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE item_option_values (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    option_group_id UUID NOT NULL REFERENCES item_option_groups(id),
    name TEXT NOT NULL,
    price_delta INTEGER NOT NULL DEFAULT 0,
    sort_order INTEGER NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE addresses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    label TEXT,
    city TEXT,
    street TEXT,
    house TEXT,
    apartment TEXT,
    entrance TEXT,
    floor TEXT,
    comment TEXT,
    latitude NUMERIC(9,6),
    longitude NUMERIC(9,6),
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE carts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    branch_id UUID REFERENCES branches(id),
    status TEXT NOT NULL DEFAULT 'active',
    currency CHAR(3) NOT NULL,
    subtotal INTEGER NOT NULL DEFAULT 0 CHECK (subtotal >= 0),
    discount_total INTEGER NOT NULL DEFAULT 0 CHECK (discount_total >= 0),
    delivery_fee INTEGER NOT NULL DEFAULT 0 CHECK (delivery_fee >= 0),
    total INTEGER NOT NULL DEFAULT 0 CHECK (total >= 0),
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX carts_active_user_idx
    ON carts(user_id)
    WHERE status = 'active';

CREATE TABLE cart_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cart_id UUID NOT NULL REFERENCES carts(id) ON DELETE CASCADE,
    menu_item_id UUID NOT NULL REFERENCES menu_items(id),
    branch_menu_item_id UUID REFERENCES branch_menu_items(id),
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    unit_price INTEGER NOT NULL CHECK (unit_price >= 0),
    options JSONB NOT NULL DEFAULT '[]'::jsonb,
    comment TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_number BIGSERIAL UNIQUE,
    user_id UUID NOT NULL REFERENCES users(id),
    branch_id UUID NOT NULL REFERENCES branches(id),
    cart_id UUID REFERENCES carts(id),
    status order_status NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    currency CHAR(3) NOT NULL,
    subtotal INTEGER NOT NULL CHECK (subtotal >= 0),
    discount_total INTEGER NOT NULL DEFAULT 0 CHECK (discount_total >= 0),
    delivery_fee INTEGER NOT NULL DEFAULT 0 CHECK (delivery_fee >= 0),
    total INTEGER NOT NULL CHECK (total >= 0),
    customer_comment TEXT,
    delivery_address_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    pricing_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    request_id TEXT,
    placed_at TIMESTAMPTZ,
    cancelled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (branch_id, request_id)
);

CREATE TABLE order_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    menu_item_id UUID REFERENCES menu_items(id),
    name_snapshot TEXT NOT NULL,
    description_snapshot TEXT,
    price_snapshot INTEGER NOT NULL CHECK (price_snapshot >= 0),
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    options_snapshot JSONB NOT NULL DEFAULT '[]'::jsonb,
    line_total INTEGER NOT NULL CHECK (line_total >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE order_status_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    from_status order_status,
    to_status order_status NOT NULL,
    reason TEXT,
    actor_type TEXT,
    actor_id UUID,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id),
    provider TEXT NOT NULL,
    provider_payment_id TEXT,
    provider_session_id TEXT,
    idempotency_key TEXT NOT NULL,
    amount INTEGER NOT NULL CHECK (amount >= 0),
    currency CHAR(3) NOT NULL,
    status payment_status NOT NULL,
    failure_reason TEXT,
    raw_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    request_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    response_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (idempotency_key),
    UNIQUE (provider, provider_payment_id),
    UNIQUE (provider, provider_session_id)
);

CREATE INDEX payments_order_id_idx ON payments(order_id);
CREATE INDEX payments_status_idx ON payments(status);

CREATE TABLE refunds (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id),
    payment_id UUID NOT NULL REFERENCES payments(id),
    provider TEXT NOT NULL,
    provider_refund_id TEXT,
    idempotency_key TEXT NOT NULL,
    amount INTEGER NOT NULL CHECK (amount >= 0),
    currency CHAR(3) NOT NULL,
    status refund_status NOT NULL,
    failure_reason TEXT,
    raw_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (idempotency_key),
    UNIQUE (provider, provider_refund_id)
);

CREATE INDEX refunds_order_id_idx ON refunds(order_id);
CREATE INDEX refunds_status_idx ON refunds(status);

CREATE TABLE saga_instances (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    saga_type TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id UUID NOT NULL,
    status saga_status NOT NULL,
    current_step TEXT,
    last_error TEXT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (saga_type, entity_type, entity_id)
);

CREATE TABLE saga_steps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    saga_instance_id UUID NOT NULL REFERENCES saga_instances(id) ON DELETE CASCADE,
    step_name TEXT NOT NULL,
    status saga_status NOT NULL,
    attempt INTEGER NOT NULL DEFAULT 0,
    input_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    output_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    last_error TEXT,
    next_retry_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (saga_instance_id, step_name, attempt)
);

CREATE TABLE outbox_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_type TEXT NOT NULL,
    aggregate_id UUID NOT NULL,
    event_type TEXT NOT NULL,
    event_key TEXT,
    payload JSONB NOT NULL,
    headers JSONB NOT NULL DEFAULT '{}'::jsonb,
    available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    locked_at TIMESTAMPTZ,
    processed_at TIMESTAMPTZ,
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (event_key)
);

CREATE INDEX outbox_events_dispatch_idx
    ON outbox_events(available_at, created_at)
    WHERE processed_at IS NULL;

CREATE TABLE inbox_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source TEXT NOT NULL,
    external_event_id TEXT NOT NULL,
    event_type TEXT,
    payload JSONB NOT NULL,
    headers JSONB NOT NULL DEFAULT '{}'::jsonb,
    signature_valid BOOLEAN,
    status TEXT NOT NULL DEFAULT 'received',
    attempts INTEGER NOT NULL DEFAULT 0,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at TIMESTAMPTZ,
    last_error TEXT,
    UNIQUE (source, external_event_id)
);

CREATE TABLE menu_item_availability_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    branch_id UUID NOT NULL REFERENCES branches(id),
    menu_item_id UUID NOT NULL REFERENCES menu_items(id),
    old_status branch_menu_item_status,
    new_status branch_menu_item_status NOT NULL,
    reason TEXT,
    actor_type TEXT,
    actor_id UUID,
    changed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE couriers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    branch_id UUID REFERENCES branches(id),
    name TEXT NOT NULL,
    phone TEXT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE courier_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id),
    courier_id UUID REFERENCES couriers(id),
    status TEXT NOT NULL DEFAULT 'assigned',
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    picked_up_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    UNIQUE (order_id)
);

CREATE TABLE promo_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    restaurant_id UUID NOT NULL REFERENCES restaurants(id),
    code TEXT NOT NULL,
    discount_type TEXT NOT NULL,
    discount_value INTEGER NOT NULL CHECK (discount_value >= 0),
    max_uses INTEGER,
    used_count INTEGER NOT NULL DEFAULT 0,
    starts_at TIMESTAMPTZ,
    ends_at TIMESTAMPTZ,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (restaurant_id, code)
);

CREATE TABLE audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_type TEXT NOT NULL,
    actor_id UUID,
    action TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id UUID,
    branch_id UUID REFERENCES branches(id),
    request_id TEXT,
    ip_address TEXT,
    user_agent TEXT,
    old_values JSONB,
    new_values JSONB,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX branches_restaurant_id_idx ON branches(restaurant_id);
CREATE INDEX categories_restaurant_id_idx ON categories(restaurant_id);
CREATE INDEX menu_items_restaurant_id_idx ON menu_items(restaurant_id);
CREATE INDEX branch_menu_items_branch_id_idx ON branch_menu_items(branch_id);
CREATE INDEX branch_menu_items_status_idx ON branch_menu_items(status);
CREATE INDEX addresses_user_id_idx ON addresses(user_id);
CREATE INDEX cart_items_cart_id_idx ON cart_items(cart_id);
CREATE INDEX orders_user_id_idx ON orders(user_id);
CREATE INDEX orders_branch_id_idx ON orders(branch_id);
CREATE INDEX orders_status_idx ON orders(status);
CREATE INDEX order_status_history_order_id_idx ON order_status_history(order_id);
CREATE INDEX saga_instances_entity_idx ON saga_instances(entity_type, entity_id);
CREATE INDEX saga_steps_saga_id_idx ON saga_steps(saga_instance_id);
CREATE INDEX inbox_events_status_idx ON inbox_events(status);
CREATE INDEX menu_item_availability_log_branch_idx ON menu_item_availability_log(branch_id, changed_at DESC);
CREATE INDEX audit_log_entity_idx ON audit_log(entity_type, entity_id, created_at DESC);

CREATE INDEX menu_items_tags_gin_idx ON menu_items USING GIN(tags jsonb_ops);
CREATE INDEX menu_items_attributes_gin_idx ON menu_items USING GIN(attributes jsonb_path_ops);
