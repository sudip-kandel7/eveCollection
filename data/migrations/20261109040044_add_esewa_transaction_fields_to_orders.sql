-- +goose Up
ALTER TABLE orders
ADD COLUMN transaction_uuid VARCHAR(100) UNIQUE,
ADD COLUMN transaction_code VARCHAR(100);

-- +goose Down
ALTER TABLE orders
DROP COLUMN transaction_code,
DROP COLUMN transaction_uuid;
