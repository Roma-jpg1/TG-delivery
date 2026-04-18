INSERT INTO restaurants (id, name, slug, currency, timezone, settings, created_at, updated_at)
VALUES (
  '33333333-3333-3333-3333-333333333333',
  'Demo Restaurant',
  'demo-restaurant',
  'RUB',
  'Asia/Novosibirsk',
  '{"delivery_fee":150,"free_delivery_from":1500}'::jsonb,
  now(),
  now()
)
ON CONFLICT (id) DO UPDATE
SET settings = EXCLUDED.settings,
    updated_at = now();

INSERT INTO branches (id, restaurant_id, name, code, address_line, latitude, longitude, delivery_radius_meters, min_order_amount, is_active, created_at, updated_at)
VALUES (
  '11111111-1111-1111-1111-111111111111',
  '33333333-3333-3333-3333-333333333333',
  'Main Branch',
  'MAIN',
  'Novosibirsk, Krasny Prospekt 1',
  55.0415,
  82.9346,
  7000,
  500,
  true,
  now(),
  now()
)
ON CONFLICT (id) DO UPDATE
SET latitude = EXCLUDED.latitude,
    longitude = EXCLUDED.longitude,
    delivery_radius_meters = EXCLUDED.delivery_radius_meters,
    min_order_amount = EXCLUDED.min_order_amount,
    updated_at = now();

INSERT INTO users (id, telegram_user_id, username, is_active, created_at, updated_at)
VALUES (
  '22222222-2222-2222-2222-222222222222',
  NULL,
  'demo_user',
  true,
  now(),
  now()
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO categories (id, restaurant_id, name, sort_order, is_active, created_at, updated_at)
VALUES
  ('44444444-4444-4444-4444-444444444441', '33333333-3333-3333-3333-333333333333', 'Pizza', 1, true, now(), now()),
  ('44444444-4444-4444-4444-444444444442', '33333333-3333-3333-3333-333333333333', 'Drinks', 2, true, now(), now())
ON CONFLICT (id) DO NOTHING;

INSERT INTO menu_items (id, restaurant_id, category_id, name, description, base_price, tags, attributes, is_deleted, created_at, updated_at)
VALUES
  ('55555555-5555-5555-5555-555555555551', '33333333-3333-3333-3333-333333333333', '44444444-4444-4444-4444-444444444441', 'Margherita', 'Classic tomato and mozzarella', 690, '["pizza"]'::jsonb, '{}'::jsonb, false, now(), now()),
  ('55555555-5555-5555-5555-555555555552', '33333333-3333-3333-3333-333333333333', '44444444-4444-4444-4444-444444444441', 'Pepperoni', 'Spicy salami and mozzarella', 790, '["pizza","spicy"]'::jsonb, '{}'::jsonb, false, now(), now()),
  ('55555555-5555-5555-5555-555555555553', '33333333-3333-3333-3333-333333333333', '44444444-4444-4444-4444-444444444442', 'Cola 0.5', 'Chilled drink', 190, '["drink"]'::jsonb, '{}'::jsonb, false, now(), now())
ON CONFLICT (id) DO NOTHING;

INSERT INTO branch_menu_items (id, branch_id, menu_item_id, price, status, reason, version, created_at, updated_at)
VALUES
  ('66666666-6666-6666-6666-666666666651', '11111111-1111-1111-1111-111111111111', '55555555-5555-5555-5555-555555555551', 690, 'available', NULL, 1, now(), now()),
  ('66666666-6666-6666-6666-666666666652', '11111111-1111-1111-1111-111111111111', '55555555-5555-5555-5555-555555555552', 790, 'available', NULL, 1, now(), now()),
  ('66666666-6666-6666-6666-666666666653', '11111111-1111-1111-1111-111111111111', '55555555-5555-5555-5555-555555555553', 190, 'available', NULL, 1, now(), now())
ON CONFLICT (branch_id, menu_item_id) DO UPDATE
SET price = EXCLUDED.price,
    status = EXCLUDED.status,
    updated_at = now();
