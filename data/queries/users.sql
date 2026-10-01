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

