-- name: GetUser :one
SELECT id, name, email
FROM users
WHERE id = $1;

-- name: CreateUser :one
INSERT INTO users (name, phone, email, role, address, password_hash, salt)
VALUES ($1, $2, $3, 'customer', $4, $5, $6)
RETURNING id, name, email, phone, address, role, created_at;

-- name: GetUserByEmail :one
SELECT id, name, email, phone, password_hash, salt, role
FROM users
WHERE email = $1;

-- name: GetUserByPhone :one
SELECT id, name, email, phone, password_hash, salt, role
FROM users
WHERE phone = $1; 

-- name: GetUserRole :one
SELECT role FROM users WHERE id = $1;

-- name: GetUserById :one
SELECT id, name, email, phone, role, is_active, address, created_at, updated_at FROM users WHERE id = $1;

-- name: EmailTaken :one
SELECT EXISTS( SELECT 1 FROM users WHERE email = $1 ) AS taken;

-- name: PhoneTaken :one
SELECT EXISTS( SELECT 1 FROM users WHERE phone = $1 ) AS taken;

-- name: UpdateUser :one
UPDATE users
SET name = $2, email = $3, phone = $4, address = $5, updated_at = now()
WHERE id = $1
RETURNING id, name, email, phone, address, role, created_at, updated_at;

-- name: ListCategoriesWithPC :many
SELECT c.id, c.name, c.image_url, c.is_active, c.created_at, c.updated_at, COUNT(p.id) AS product_count FROM categories c LEFT JOIN products p ON p.category_id = c.id GROUP BY c.id, c.name, c.image_url, c.is_active, c.created_at, c.updated_at ORDER BY c.name; 

-- name: ListCategoriesWithProducts :many
SELECT c.id, c.name, c.image_url, c.is_active, c.created_at, c.updated_at, COUNT(p.id) AS product_count
FROM categories c
JOIN products p ON p.category_id = c.id AND p.is_active = TRUE
GROUP BY c.id, c.name, c.image_url, c.is_active, c.created_at, c.updated_at
ORDER BY c.name;

-- name: CategoryNameTaken :one
SELECT EXISTS( SELECT 1 FROM categories WHERE name = $1 ) AS taken;

-- name: CategoryNameTakenForUpdate :one
SELECT EXISTS (
    SELECT 1
    FROM categories
    WHERE name = $1
      AND id <> $2
) AS taken;


-- name: CreateCategory :one
INSERT INTO categories (name, image_url, is_active)
VALUES ($1, $2, $3)
RETURNING id, name, image_url, is_active, created_at, updated_at;

-- name: GetCategory :one
SELECT id, name, image_url, is_active, created_at, updated_at
FROM categories
WHERE id = $1;

-- name: UpdateCategory :one
UPDATE categories
SET
    name = $2,
    is_active = $3,
    updated_at = NOW()
WHERE id = $1
RETURNING id, name, image_url, is_active, created_at, updated_at;

-- name: UpdateCategoryWithImage :one
UPDATE categories
SET
    name = $2,
    image_url = $3,
    is_active = $4,
    updated_at = NOW()
WHERE id = $1
RETURNING id, name, image_url, is_active, created_at, updated_at;

-- name: DeleteCategory :exec
DELETE FROM categories WHERE id = $1;


-- name: ProductNameTaken :one
SELECT EXISTS( SELECT 1 FROM products WHERE LOWER(name) = LOWER($1) ) AS taken;

-- name: ProductNameTakenForUpdate :one
SELECT EXISTS (
    SELECT 1
    FROM products
    WHERE LOWER(name) = LOWER($1)
      AND id <> $2
) AS taken;

-- name: UpdateProduct :one
UPDATE products
SET
    name = $2,
    description = $3,
    category_id = $4,
    base_price = $5,
    is_active = $6,
    updated_at = NOW()
WHERE id = $1
RETURNING id, name, description, category_id, base_price::float8, is_active, created_at, updated_at;

-- name: UpsertProductVariant :exec
INSERT INTO product_variants (product_id, size, color, stock, price_override)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (product_id, size, color)
DO UPDATE SET stock = EXCLUDED.stock, price_override = EXCLUDED.price_override;

-- name: DeleteProductVariant :exec
DELETE FROM product_variants
WHERE product_id = $1 AND size = $2 AND color = $3;

-- name: DeleteProductTags :exec
DELETE FROM product_tags WHERE product_id = $1;

-- name: DeleteProductImage :exec
DELETE FROM product_images WHERE product_id = $1 AND image_url = $2;

-- name: CreateProduct :one
INSERT INTO products (name, description, category_id, base_price, is_active)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, name, description, category_id, base_price::float8, is_active, created_at, updated_at;

-- name: CreateProductVariant :exec
INSERT INTO product_variants (product_id, size, color, stock, price_override)
VALUES ($1, $2, $3, $4, $5);

-- name: CreateProductImage :exec
INSERT INTO product_images (product_id, image_url, sort_order)
VALUES ($1, $2, $3);

-- name: UpsertTag :one
INSERT INTO tags (name) VALUES ($1)
ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
RETURNING id;

-- name: CreateProductTag :exec
INSERT INTO product_tags (product_id, tag_id) VALUES ($1, $2);

-- name: GetProduct :one
SELECT id, name, description, category_id, base_price::float8, is_active
FROM products
WHERE id = $1;

-- name: ListProductVariants :many
SELECT id, color, size, stock, COALESCE(price_override, 0)::float8 AS price_override
FROM product_variants
WHERE product_id = $1
ORDER BY id;

-- name: ListProductTags :many
SELECT t.name
FROM tags t
JOIN product_tags pt ON pt.tag_id = t.id
WHERE pt.product_id = $1
ORDER BY t.name;

-- name: ListProductImages :many
SELECT image_url
FROM product_images
WHERE product_id = $1
ORDER BY sort_order;

-- name: GetCategoryProductIDs :many
SELECT p.id
FROM products p
WHERE p.category_id = (
    SELECT p2.category_id
    FROM products p2
    WHERE p2.id = $1
)
  AND p.id <> $1
  AND p.is_active = TRUE
ORDER BY p.id;

-- name: GetProductTagIDs :one
SELECT ARRAY(
    SELECT pt.tag_id
    FROM product_tags pt
    WHERE pt.product_id = $1
    ORDER BY pt.tag_id
)::int[] AS tag_ids;

-- name: GetProductIDsByTagIDs :many
SELECT DISTINCT product_id FROM product_tags WHERE tag_id = ANY(sqlc.arg('tags_ids')::int[]);

-- name: GetRecommendationFeatures :many
SELECT
    p.id,
    p.category_id,
    ARRAY(
        SELECT pt.tag_id
        FROM product_tags pt
        WHERE pt.product_id = p.id
        ORDER BY pt.tag_id
    )::int[] AS tag_ids,
    COALESCE(
        (
            SELECT AVG(pr.rating)
            FROM product_reviews pr
            WHERE pr.product_id = p.id AND pr.is_hidden = FALSE
        ),
        0
    )::float8 AS rating,
    (
        SELECT COUNT(*)
        FROM product_reviews pr
        WHERE pr.product_id = p.id AND pr.is_hidden = FALSE
    )::int AS review_count,
    COALESCE(
        (
            SELECT SUM(oi.quantity)
            FROM order_items oi
            JOIN product_variants pv ON pv.id = oi.variant_id
            JOIN orders o ON o.id = oi.order_id
            WHERE pv.product_id = p.id
              AND o.status <> 'cancelled'
        ),
        0
    )::int AS sold,
    COALESCE(
        (
            SELECT SUM(pv.stock)
            FROM product_variants pv
            WHERE pv.product_id = p.id
        ),
        0
    )::int AS stock
FROM products p
WHERE p.id = ANY(sqlc.arg('product_ids')::int[])
ORDER BY p.id;


-- name: ListProducts :many
SELECT p.id, p.name, p.description, p.category_id, p.base_price::float8, p.is_active, p.created_at, c.name AS category_name,
    COALESCE((
        SELECT pi.image_url
        FROM product_images pi
        WHERE pi.product_id = p.id
        ORDER BY pi.sort_order ASC
        LIMIT 1
    ), '') AS cover_image,
    (
        SELECT COUNT(*) FROM product_variants pv
        WHERE pv.product_id = p.id
    )::int AS variant_count,
    (
        SELECT COALESCE(SUM(pv.stock), 0) FROM product_variants pv
        WHERE pv.product_id = p.id
    )::int AS total_stock
FROM products p
JOIN categories c ON c.id = p.category_id
WHERE (sqlc.arg('search')::text = '' OR p.name ILIKE '%' || sqlc.arg('search')::text || '%')
  AND (sqlc.arg('category')::int = 0 OR p.category_id = sqlc.arg('category')::int)
ORDER BY p.created_at DESC;

-- name: ListProductsCard :many
WITH cards AS (
    SELECT p.id, p.name, c.name AS category_name, p.category_id, p.created_at,
        p.base_price::float8 AS price,
        COALESCE((SELECT pi.image_url FROM product_images pi WHERE pi.product_id = p.id ORDER BY pi.sort_order ASC LIMIT 1), '') AS cover_image,
        COALESCE((SELECT AVG(pr.rating) FROM product_reviews pr WHERE pr.product_id = p.id AND pr.is_hidden = FALSE), 0)::float8 AS rating,
        (SELECT COUNT(*) FROM product_reviews pr WHERE pr.product_id = p.id AND pr.is_hidden = FALSE)::int AS review_count,
        COALESCE((SELECT SUM(oi.quantity) FROM order_items oi JOIN product_variants pv ON pv.id = oi.variant_id JOIN orders o ON o.id = oi.order_id WHERE pv.product_id = p.id AND o.status <> 'cancelled'), 0)::int AS sold,
        COALESCE((SELECT SUM(pv.stock) FROM product_variants pv WHERE pv.product_id = p.id), 0)::int AS stock,
        LOWER(
            p.name || ' ' || c.name || ' ' ||
            COALESCE((SELECT string_agg(t.name, ' ') FROM product_tags pt JOIN tags t ON t.id = pt.tag_id WHERE pt.product_id = p.id), '') || ' ' ||
            COALESCE((SELECT string_agg(DISTINCT pv.color, ' ') FROM product_variants pv WHERE pv.product_id = p.id), '')
        ) AS search_text
    FROM products p
    JOIN categories c ON c.id = p.category_id
    WHERE p.is_active = TRUE AND c.is_active = TRUE
)
SELECT id, name, category_name, price, cover_image, rating, review_count, sold, stock
FROM cards
WHERE (sqlc.arg('category')::int = 0 OR category_id = sqlc.arg('category')::int)
  AND NOT EXISTS (
      SELECT 1 FROM unnest(sqlc.arg('terms')::text[]) AS term
      WHERE search_text NOT LIKE '%' || term || '%'
  )
ORDER BY
    CASE WHEN sqlc.arg('sort')::text = 'Price: low to high' THEN price END ASC,
    CASE WHEN sqlc.arg('sort')::text = 'Price: high to low' THEN price END DESC,
    CASE WHEN sqlc.arg('sort')::text = 'Top rated'          THEN rating END DESC,
    created_at DESC;



-- name: RecordProductView :exec
INSERT INTO product_stats (product_id, day, views)
VALUES ($1, CURRENT_DATE, 1)
ON CONFLICT (product_id, day)
DO UPDATE SET views = product_stats.views + 1;

-- name: RecordProductCart :exec
INSERT INTO product_stats (product_id, day, carts)
VALUES (sqlc.arg('product_id'), CURRENT_DATE, sqlc.arg('quantity'))
ON CONFLICT (product_id, day)
DO UPDATE SET carts = product_stats.carts + sqlc.arg('quantity');

-- name: RecordProductPurchase :exec
INSERT INTO product_stats (product_id, day, purchases)
VALUES ($1, CURRENT_DATE, $2)
ON CONFLICT (product_id, day)
DO UPDATE SET purchases = product_stats.purchases + $2;

-- name: ListProductStats :many
SELECT product_id, day, views, carts, purchases FROM product_stats WHERE day >= CURRENT_DATE - 6;

-- name: GetProductDetail :one
SELECT
    p.id,
    p.category_id,
    p.name,
    COALESCE(p.description, '')::text AS description,
    p.base_price::float8 AS price,
    c.name AS category_name,
    COALESCE((
        SELECT AVG(pr.rating)
        FROM product_reviews pr
        WHERE pr.product_id = p.id AND pr.is_hidden = FALSE
    ), 0)::float8 AS rating,
    (
        SELECT COUNT(*)
        FROM product_reviews pr
        WHERE pr.product_id = p.id AND pr.is_hidden = FALSE
    )::int AS review_count,
    COALESCE((
        SELECT SUM(pv.stock)
        FROM product_variants pv
        WHERE pv.product_id = p.id
    ), 0)::int AS stock
FROM products p
JOIN categories c ON c.id = p.category_id
WHERE p.id = $1
  AND p.is_active = TRUE
  AND c.is_active = TRUE;

-- name: ListProductReviews :many
SELECT
    pr.id,
    pr.rating,
    COALESCE(pr.review_text, '')::text AS review_text,
    pr.is_hidden,
    pr.created_at,
    pr.updated_at,
    (pr.updated_at > pr.created_at) AS is_edited,
    u.name AS customer_name
FROM product_reviews pr
JOIN users u ON u.id = pr.customer_id
WHERE pr.product_id = sqlc.arg('product_id')::int
  AND (sqlc.arg('include_hidden')::bool OR pr.is_hidden = FALSE)
ORDER BY pr.is_hidden ASC, pr.created_at DESC;

-- name: GetProductReview :one
SELECT
    id,
    product_id,
    customer_id,
    rating,
    COALESCE(review_text, '')::text AS review_text,
    is_hidden,
    created_at,
    updated_at
FROM product_reviews
WHERE id = $1;

-- name: SetProductReviewHidden :exec
UPDATE product_reviews
SET is_hidden = $2
WHERE id = $1;

-- name: GetCustomerProductReview :one
SELECT
    id,
    product_id,
    customer_id,
    rating,
    COALESCE(review_text, '')::text AS review_text,
    is_hidden,
    created_at,
    updated_at
FROM product_reviews
WHERE customer_id = $1
  AND product_id = $2;

-- name: SaveProductReview :one
INSERT INTO product_reviews (product_id, customer_id, rating, review_text)
VALUES ($1, $2, $3, $4)
ON CONFLICT (customer_id, product_id)
DO UPDATE SET
    rating = EXCLUDED.rating,
    review_text = EXCLUDED.review_text,
    updated_at = NOW()
RETURNING id, product_id, customer_id, rating, review_text, is_hidden, created_at, updated_at;

-- name: HasDeliveredProduct :one
SELECT EXISTS (
    SELECT 1
    FROM orders o
    JOIN order_items oi ON oi.order_id = o.id
    JOIN product_variants pv ON pv.id = oi.variant_id
    WHERE o.customer_id = $1
      AND pv.product_id = $2
      AND o.status = 'delivered'
) AS delivered;

-- name: AddToCart :one
INSERT INTO cart_items (customer_id, variant_id, quantity)
SELECT sqlc.arg('customer_id')::int, pv.id, LEAST(sqlc.arg('quantity')::int, pv.stock)
FROM product_variants pv
WHERE pv.id = sqlc.arg('variant_id')::int
  AND pv.stock > 0
ON CONFLICT (customer_id, variant_id)
DO UPDATE SET quantity = LEAST(
    cart_items.quantity + EXCLUDED.quantity,
    (SELECT stock FROM product_variants WHERE id = EXCLUDED.variant_id)
)
RETURNING
    id,
    variant_id,
    quantity,
    COALESCE((SELECT stock FROM product_variants WHERE id = cart_items.variant_id), 0)::int AS stock,
    COALESCE((SELECT product_id FROM product_variants WHERE id = cart_items.variant_id), 0)::int AS product_id;



-- name: GetCartItems :many
SELECT
    ci.id,
    ci.variant_id,
    ci.quantity,
    ci.added_at,
    p.name AS product_name,
    COALESCE(pv.size, '')  AS size,
    COALESCE(pv.color, '') AS color,
    pv.stock,
    COALESCE(pv.price_override, p.base_price)::float8 AS price,
    COALESCE(pi.image_url, '') AS image
FROM cart_items ci
JOIN product_variants pv ON pv.id = ci.variant_id
JOIN products p          ON p.id = pv.product_id
LEFT JOIN LATERAL (
    SELECT image_url FROM product_images
    WHERE product_id = p.id
    ORDER BY sort_order ASC, id ASC
    LIMIT 1
) pi ON true
WHERE ci.customer_id = $1
ORDER BY ci.added_at DESC;


-- name: RemoveCartItem :exec
DELETE FROM cart_items
WHERE customer_id = $1 AND variant_id = $2;


-- name: GetProductByVariantID :one
SELECT
    p.id AS product_id,
    p.name AS product_name,
    pv.id AS variant_id,
    COALESCE(pv.size, '') AS size,
    COALESCE(pv.color, '') AS color,
    sqlc.arg('quantity')::int AS quantity,
    COALESCE(pv.price_override, p.base_price)::float8 AS price,
    COALESCE((
        SELECT pi.image_url
        FROM product_images pi
        WHERE pi.product_id = p.id
        ORDER BY pi.sort_order ASC, pi.id ASC
        LIMIT 1
    ), '')::text AS image_url
FROM product_variants pv
JOIN products p ON p.id = pv.product_id
WHERE pv.id = sqlc.arg('variant_id')::int;

-- name: GetVariantPrice :one
SELECT COALESCE(pv.price_override, p.base_price)::float8 AS price
FROM product_variants pv
JOIN products p ON p.id = pv.product_id
WHERE pv.id = $1;

-- name: CreateOrder :one
INSERT INTO orders (
    customer_id,
    status,
    payment_method,
    total_amount,
    delivery_address,
    delivery_district,
    delivery_phone,
    transaction_uuid,
    transaction_code,
    from_cart
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING id;

-- name: CreateOrderItem :exec
INSERT INTO order_items (order_id, variant_id, quantity, unit_price)
VALUES ($1, $2, $3, $4);

-- name: GetVariantForOrder :one
SELECT COALESCE(pv.price_override, p.base_price)::float8 AS price, pv.stock, pv.product_id
FROM product_variants pv
JOIN products p ON p.id = pv.product_id
WHERE pv.id = $1
  AND p.is_active = TRUE;

-- name: DecreaseVariantStock :execrows
UPDATE product_variants
SET stock = stock - $2
WHERE id = $1 AND stock >= $2;

-- name: IncreaseVariantStock :exec
UPDATE product_variants
SET stock = stock + $2
WHERE id = $1;

-- name: DeleteOrder :exec
DELETE FROM orders WHERE id = $1;

-- name: GetOrderByCustomer :one
SELECT id, status, payment_method, payment_status, total_amount::float8 AS total_amount
FROM orders
WHERE id = $1 AND customer_id = $2;

-- name: SubtractOrderedCartQuantity :execrows
UPDATE cart_items
SET quantity = quantity - $3
WHERE customer_id = $1 AND variant_id = $2 AND quantity > $3;


-- name: GetOrderForPayment :one
SELECT id, customer_id, status, total_amount, delivery_district, transaction_uuid
FROM orders
WHERE id = $1;

-- name: GetOrderByTransactionUUID :one
SELECT id, customer_id, status, total_amount, delivery_district, from_cart, transaction_uuid, transaction_code
FROM orders
WHERE transaction_uuid = $1;

-- name: GetOrderItemsForPayment :many
SELECT oi.variant_id, oi.quantity, oi.unit_price::float8 AS unit_price, pv.product_id
FROM order_items oi
JOIN product_variants pv ON pv.id = oi.variant_id
WHERE oi.order_id = $1;

-- name: MarkOrderPaid :exec
UPDATE orders
SET status = 'placed', payment_status = 'paid', updated_at = NOW()
WHERE id = $1;

-- name: UpdateOrderStatus :exec
UPDATE orders
SET status = $2, updated_at = NOW()
WHERE id = $1;

-- name: UpdateOrderPaymentStatus :exec
UPDATE orders
SET payment_status = $2, updated_at = NOW()
WHERE id = $1;

-- name: DeleteOrderItems :exec
DELETE FROM order_items WHERE order_id = $1;

-- name: GetOrders :many

SELECT o.id, o.created_at AS date, o.status, o.payment_status, o.payment_method, o.total_amount, COALESCE(SUM(oi.quantity), 0)::bigint AS total_items FROM orders o
LEFT JOIN order_items oi ON oi.order_id = o.id
WHERE (o.customer_id = sqlc.narg('customer_id')::int OR sqlc.narg('customer_id') IS NULL)
GROUP BY o.id ORDER BY o.created_at DESC; 

-- name: GetOrderDetail :one
SELECT
    o.id,
    o.status,
    o.payment_method,
    o.payment_status,
    o.total_amount,
    o.created_at AS date,
    u.name AS customer_name,
    o.delivery_phone,
    o.delivery_address,
    o.delivery_district
FROM orders o
JOIN users u ON u.id = o.customer_id
WHERE o.id = sqlc.arg('order_id')::int
  AND (o.customer_id = sqlc.narg('customer_id')::int OR sqlc.narg('customer_id') IS NULL);

-- name: GetOrderItems :many
SELECT
    oi.id,
    oi.variant_id,
    oi.quantity,
    oi.unit_price::float8 AS unit_price,
    p.name AS product_name,
    COALESCE(pv.size, '') AS size,
    COALESCE(pv.color, '') AS color,
    COALESCE((
        SELECT pi.image_url
        FROM product_images pi
        WHERE pi.product_id = p.id
        ORDER BY pi.sort_order ASC, pi.id ASC
        LIMIT 1
    ), '')::text AS image_url
FROM order_items oi
JOIN product_variants pv ON pv.id = oi.variant_id
JOIN products p ON p.id = pv.product_id
WHERE oi.order_id = sqlc.arg('order_id')::int
ORDER BY oi.id ASC;