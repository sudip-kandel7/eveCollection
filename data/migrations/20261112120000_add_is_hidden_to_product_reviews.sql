-- +goose Up
ALTER TABLE product_reviews
    ADD COLUMN is_hidden BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE product_reviews
    DROP COLUMN IF EXISTS is_hidden;
