-- +goose Up
-- Keep only the newest review per (customer_id, product_id) before adding uniqueness.
DELETE FROM product_reviews old
USING product_reviews newer
WHERE old.customer_id = newer.customer_id
  AND old.product_id = newer.product_id
  AND old.id < newer.id;

ALTER TABLE product_reviews
    ADD CONSTRAINT product_reviews_customer_product_unique
    UNIQUE (customer_id, product_id);

-- +goose Down
ALTER TABLE product_reviews
    DROP CONSTRAINT IF EXISTS product_reviews_customer_product_unique;
