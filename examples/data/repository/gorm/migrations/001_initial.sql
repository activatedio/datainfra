-- +goose Up

CREATE TABLE categories (
    name VARCHAR(64),
    description VARCHAR(200),
    PRIMARY KEY (name)
);