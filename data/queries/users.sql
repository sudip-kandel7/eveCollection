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