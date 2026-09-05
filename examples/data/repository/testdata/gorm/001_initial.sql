-- +goose Up

INSERT INTO categories (name, description) VALUES
  ('a', 'Category A'),
  ('b', 'Category B')
;

INSERT INTO products (sku, description) VALUES
  ('1', 'Test Product 1'),
  ('2', 'Test Product 2'),
  ('3', 'Product 3'),
  ('4', 'Product 4')
;

INSERT INTO product_categories (product_sku, category_name, created_at) VALUES
  ('1', 'a', CURRENT_TIMESTAMP),
  ('2', 'a', CURRENT_TIMESTAMP),
  ('3', 'b', CURRENT_TIMESTAMP),
  ('4', 'b', CURRENT_TIMESTAMP)
;

INSERT INTO themes2 (tenant_id, name, description) VALUES
  ('1', 'a', 'Category 1 A'),
  ('1', 'b', 'Category 1 B'),
  ('2', 'a', 'Category 2 A'),
  ('2', 'b', 'Category 2 B')
;
INSERT INTO locations (city, state, latitude, longitude) VALUES
  ('Seattle', 'WA', 1, 2),
  ('San Francisco', 'CA', 3, 4)
;

-- +goose Down
DELETE FROM locations WHERE (city, state) IN (('Seattle', 'WA'), ('San Francisco', 'CA'));
DELETE FROM themes2 WHERE (tenant_id, name) IN (('1', 'a'), ('1', 'b'), ('2', 'a'), ('2', 'b'));
DELETE FROM product_categories WHERE (product_sku, category_name) IN (('1', 'a'), ('2', 'a'), ('3', 'b'), ('4', 'b'));
DELETE FROM products WHERE sku IN ('1', '2', '3', '4');
DELETE FROM categories WHERE name IN ('a', 'b');
