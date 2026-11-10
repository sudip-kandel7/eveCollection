-- +goose Up
ALTER TABLE orders ADD COLUMN from_cart boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE orders DROP COLUMN from_cart;