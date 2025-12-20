-- +goose Up

CREATE TABLE categories (
    name VARCHAR(64),
    description VARCHAR(200),
    PRIMARY KEY (name)
);

CREATE TABLE products (
    sku VARCHAR(64),
    description VARCHAR(200),
    {{ if eq "postgres" .Dialect }}
    full_text TSVECTOR GENERATED ALWAYS AS (TO_TSVECTOR
      ('english',
        (CASE WHEN description IS NULL THEN '' ELSE description END)
      )
    ) STORED,
    {{ end }}
    PRIMARY KEY (sku)
);

CREATE TABLE product_categories (
    product_sku VARCHAR(64),
    category_name VARCHAR(64),
    created_at TIMESTAMP NOT NULL,
    PRIMARY KEY (product_sku, category_name),
    FOREIGN KEY (product_sku) REFERENCES products(sku),
    FOREIGN KEY (category_name) REFERENCES categories(name)
);

CREATE TABLE themes2 (
    tenant_id VARCHAR(64),
    name VARCHAR(64),
    description VARCHAR(200),
    PRIMARY KEY (tenant_id, name)
);

