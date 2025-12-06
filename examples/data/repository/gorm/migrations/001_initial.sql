-- +goose Up

CREATE TABLE categories (
    name VARCHAR(64),
    description VARCHAR(200),
    PRIMARY KEY (name)
);

CREATE TABLE products (
    sku VARCHAR(64),
    description VARCHAR(200),
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
