CREATE TABLE users (
    id            SERIAL PRIMARY KEY,
    name          VARCHAR(100) NOT NULL,
    email         VARCHAR(150) UNIQUE NOT NULL,
    phone         VARCHAR(20) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    salt          VARCHAR(255) NOT NULL,
    role          VARCHAR(20) NOT NULL DEFAULT 'customer', 
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    address       TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE sessions (
    token  TEXT PRIMARY KEY,
    data   BYTEA NOT NULL,
    expiry TIMESTAMPTZ NOT NULL
);

CREATE INDEX sessions_expiry_idx ON sessions (expiry);

CREATE TABLE categories (
    id            SERIAL PRIMARY KEY,
    name          VARCHAR(100) NOT NULL UNIQUE,
    image_url     VARCHAR(500),
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE products (
    id            SERIAL PRIMARY KEY,
    name          VARCHAR(200) NOT NULL,
    description   TEXT,
    category_id   INTEGER NOT NULL REFERENCES categories(id),
    base_price    NUMERIC(10,2) NOT NULL,
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE product_variants (
    id             SERIAL PRIMARY KEY,
    product_id     INTEGER NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    size           VARCHAR(20),
    color          VARCHAR(50),
    stock          INTEGER NOT NULL DEFAULT 0,
    price_override NUMERIC(10,2),
    UNIQUE (product_id, size, color)
);

CREATE TABLE product_images (
    id          SERIAL PRIMARY KEY,
    product_id  INTEGER NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    image_url   VARCHAR(500) NOT NULL,
    sort_order  INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE product_stats (
    product_id INTEGER NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    day        DATE NOT NULL,
    views      INTEGER NOT NULL DEFAULT 0,
    carts      INTEGER NOT NULL DEFAULT 0,
    purchases  INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (product_id, day)
);

CREATE TABLE tags (
    id    SERIAL PRIMARY KEY,
    name  VARCHAR(50) NOT NULL UNIQUE  
);

CREATE TABLE product_tags (
    product_id  INTEGER NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    tag_id      INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (product_id, tag_id)
);

CREATE TABLE cart_items (
    id          SERIAL PRIMARY KEY,
    customer_id INTEGER NOT NULL REFERENCES users(id),
    variant_id  INTEGER NOT NULL REFERENCES product_variants(id),
    quantity    INTEGER NOT NULL DEFAULT 1 CHECK (quantity > 0),
    added_at    TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE (customer_id, variant_id)
);

CREATE TABLE orders (
    id                SERIAL PRIMARY KEY,
    customer_id       INTEGER NOT NULL REFERENCES users(id),
    status            VARCHAR(30) NOT NULL DEFAULT 'placed',
        -- 'placed', 'packed', 'out_for_delivery', 'delivered', 'cancelled'
    payment_method    VARCHAR(20) NOT NULL,   -- 'cod', 'online'
    payment_status    VARCHAR(20) NOT NULL DEFAULT 'pending', -- 'pending','paid'
    total_amount      NUMERIC(10,2) NOT NULL,
    delivery_address  TEXT NOT NULL,
    delivery_district VARCHAR(100) NOT NULL,
    delivery_phone    VARCHAR(20) NOT NULL,
    delivered_confirmed_by VARCHAR(20),  
    transaction_uuid  VARCHAR(100) UNIQUE,
    transaction_code  VARCHAR(100),
    from_cart         BOOLEAN NOT NULL DEFAULT FALSE,
    created_at        TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE order_items (
    id            SERIAL PRIMARY KEY,
    order_id      INTEGER NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    variant_id    INTEGER NOT NULL REFERENCES product_variants(id),
    quantity      INTEGER NOT NULL,
    unit_price    NUMERIC(10,2) NOT NULL  
);

CREATE TABLE product_reviews (
    id          SERIAL PRIMARY KEY,
    product_id  INTEGER NOT NULL REFERENCES products(id),
    customer_id INTEGER NOT NULL REFERENCES users(id),
    rating      INTEGER NOT NULL,
    review_text TEXT,
    is_hidden   BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT product_reviews_customer_product_unique UNIQUE (customer_id, product_id)
);

-- CREATE TABLE review_replies (
--     id          SERIAL PRIMARY KEY,
--     review_id   INTEGER NOT NULL REFERENCES product_reviews(id) ON DELETE CASCADE,
--     user_id     INTEGER NOT NULL REFERENCES users(id),
--     reply_text  TEXT NOT NULL,
--     created_at  TIMESTAMP NOT NULL DEFAULT NOW()
-- );

CREATE TABLE conversations (
    id          SERIAL PRIMARY KEY,
    customer_id INTEGER NOT NULL REFERENCES users(id),
    owner_id    INTEGER NOT NULL REFERENCES users(id),
    created_at  TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE (customer_id, owner_id)  
);

-- CREATE TABLE messages (
--     id              SERIAL PRIMARY KEY,
--     conversation_id INTEGER NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
--     sender_id       INTEGER NOT NULL REFERENCES users(id),
--     message         TEXT NOT NULL,
--     created_at      TIMESTAMP NOT NULL DEFAULT NOW()
-- );

CREATE TABLE messages (
    id              SERIAL PRIMARY KEY,
    conversation_id INTEGER NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    sender_id       INTEGER NOT NULL REFERENCES users(id),
    product_id      INTEGER REFERENCES products(id),  -- optional: which product this message refers to
    message         TEXT NOT NULL,
    is_read         BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMP NOT NULL DEFAULT NOW()
);