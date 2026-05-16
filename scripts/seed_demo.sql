INSERT INTO restaurants (id, name, slug, currency, timezone, settings, created_at, updated_at)
VALUES (
  '33333333-3333-3333-3333-333333333333',
  'AYAT River Delivery',
  'ayat-river-delivery',
  'RUB',
  'Asia/Novosibirsk',
  '{"delivery_fee":150,"free_delivery_from":1500}'::jsonb,
  now(),
  now()
)
ON CONFLICT (id) DO UPDATE
SET settings = EXCLUDED.settings,
    name = EXCLUDED.name,
    slug = EXCLUDED.slug,
    updated_at = now();

INSERT INTO branches (id, restaurant_id, name, code, address_line, latitude, longitude, delivery_radius_meters, min_order_amount, is_active, created_at, updated_at)
VALUES (
  '11111111-1111-1111-1111-111111111111',
  '33333333-3333-3333-3333-333333333333',
  'AYAT River Kitchen',
  'MAIN',
  'Республика Алтай, Чуйский тракт, 477-й километр',
  51.802847,
  85.748587,
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

UPDATE addresses
SET is_default = false,
    updated_at = now()
WHERE user_id = '22222222-2222-2222-2222-222222222222';

INSERT INTO addresses (id, user_id, label, city, street, house, latitude, longitude, is_default, created_at, updated_at)
VALUES (
  '77777777-7777-7777-7777-777777777771',
  '22222222-2222-2222-2222-222222222222',
  'AYAT River',
  'Республика Алтай',
  'Чуйский тракт',
  '477-й километр',
  51.802847,
  85.748587,
  true,
  now(),
  now()
)
ON CONFLICT (id) DO UPDATE
SET label = EXCLUDED.label,
    city = EXCLUDED.city,
    street = EXCLUDED.street,
    house = EXCLUDED.house,
    latitude = EXCLUDED.latitude,
    longitude = EXCLUDED.longitude,
    is_default = EXCLUDED.is_default,
    updated_at = now();

INSERT INTO categories (id, restaurant_id, name, sort_order, is_active, created_at, updated_at)
VALUES
  ('44444444-4444-4444-4444-444444444441', '33333333-3333-3333-3333-333333333333', 'Горячее', 1, true, now(), now()),
  ('44444444-4444-4444-4444-444444444442', '33333333-3333-3333-3333-333333333333', 'Напитки', 2, true, now(), now())
ON CONFLICT (id) DO UPDATE
SET name = EXCLUDED.name,
    sort_order = EXCLUDED.sort_order,
    updated_at = now();

INSERT INTO menu_items (id, restaurant_id, category_id, name, description, photo_url, base_price, tags, attributes, is_deleted, created_at, updated_at)
VALUES
  ('55555555-5555-5555-5555-555555555551', '33333333-3333-3333-3333-333333333333', '44444444-4444-4444-4444-444444444441', 'Алтайская уха с форелью', 'Насыщенный рыбный бульон, зелень, картофель и форель', 'https://images.unsplash.com/photo-1547592166-23ac45744acd?auto=format&fit=crop&w=900&q=82', 690, '["fish","soup"]'::jsonb, '{}'::jsonb, false, now(), now()),
  ('55555555-5555-5555-5555-555555555552', '33333333-3333-3333-3333-333333333333', '44444444-4444-4444-4444-444444444441', 'Мясо на огне с травами', 'Горячее мясное блюдо с домашним соусом и печеными овощами', 'https://images.unsplash.com/photo-1544025162-d76694265947?auto=format&fit=crop&w=900&q=82', 890, '["meat","grill"]'::jsonb, '{}'::jsonb, false, now(), now()),
  ('55555555-5555-5555-5555-555555555553', '33333333-3333-3333-3333-333333333333', '44444444-4444-4444-4444-444444444442', 'Морс из таежных ягод', 'Прохладный домашний ягодный напиток', 'https://images.unsplash.com/photo-1544145945-f90425340c7e?auto=format&fit=crop&w=900&q=82', 250, '["drink"]'::jsonb, '{}'::jsonb, false, now(), now())
ON CONFLICT (id) DO UPDATE
SET name = EXCLUDED.name,
    description = EXCLUDED.description,
    photo_url = EXCLUDED.photo_url,
    base_price = EXCLUDED.base_price,
    tags = EXCLUDED.tags,
    updated_at = now();

INSERT INTO branch_menu_items (id, branch_id, menu_item_id, price, status, reason, version, created_at, updated_at)
VALUES
  ('66666666-6666-6666-6666-666666666651', '11111111-1111-1111-1111-111111111111', '55555555-5555-5555-5555-555555555551', 690, 'available', NULL, 1, now(), now()),
  ('66666666-6666-6666-6666-666666666652', '11111111-1111-1111-1111-111111111111', '55555555-5555-5555-5555-555555555552', 890, 'available', NULL, 1, now(), now()),
  ('66666666-6666-6666-6666-666666666653', '11111111-1111-1111-1111-111111111111', '55555555-5555-5555-5555-555555555553', 250, 'available', NULL, 1, now(), now())
ON CONFLICT (branch_id, menu_item_id) DO UPDATE
SET price = EXCLUDED.price,
    status = EXCLUDED.status,
    updated_at = now();
