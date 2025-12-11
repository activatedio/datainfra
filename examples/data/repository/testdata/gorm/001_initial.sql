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
