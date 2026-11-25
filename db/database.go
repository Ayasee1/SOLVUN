package db

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"

	"golearn/model"
)

// NewPool creates a Postgres connection pool and verifies connectivity.
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}

const schema = `
CREATE TABLE IF NOT EXISTS products (
	id           BIGINT PRIMARY KEY,
	name         TEXT        NOT NULL,
	description  TEXT        NOT NULL DEFAULT '',
	images       JSONB       NOT NULL DEFAULT '[]',
	price        BIGINT      NOT NULL,
	stock        BIGINT      NOT NULL,
	is_available BOOLEAN     NOT NULL DEFAULT true,
	weight_gram  BIGINT      NOT NULL DEFAULT 0,
	updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- migrate older databases that do not have the content columns yet
ALTER TABLE products ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '';
ALTER TABLE products ADD COLUMN IF NOT EXISTS images JSONB NOT NULL DEFAULT '[]';
ALTER TABLE products ADD COLUMN IF NOT EXISTS weight_gram BIGINT NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS orders (
	id          UUID PRIMARY KEY,
	username    TEXT        NOT NULL,
	email       TEXT        NOT NULL,
	phone       TEXT        NOT NULL,
	address     JSONB       NOT NULL,
	items       JSONB       NOT NULL,
	total       BIGINT      NOT NULL,
	status      TEXT        NOT NULL,
	shipping_cost   BIGINT  NOT NULL DEFAULT 0,
	courier         TEXT    NOT NULL DEFAULT '',
	courier_service TEXT    NOT NULL DEFAULT '',
	snap_token  TEXT        NOT NULL DEFAULT '',
	payment_url TEXT        NOT NULL DEFAULT '',
	payment     JSONB,
	shipment    JSONB,
	created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE orders ADD COLUMN IF NOT EXISTS shipping_cost BIGINT NOT NULL DEFAULT 0;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS courier TEXT NOT NULL DEFAULT '';
ALTER TABLE orders ADD COLUMN IF NOT EXISTS courier_service TEXT NOT NULL DEFAULT '';
ALTER TABLE orders ADD COLUMN IF NOT EXISTS shipment JSONB;
-- refund issued through the payment gateway
ALTER TABLE orders ADD COLUMN IF NOT EXISTS refund JSONB;
-- guest secret for the order page, invoice and cancel endpoint
ALTER TABLE orders ADD COLUMN IF NOT EXISTS access_token TEXT NOT NULL DEFAULT '';
-- automatic shipment retries: how many provider attempts were made and when
ALTER TABLE orders ADD COLUMN IF NOT EXISTS shipment_attempts INT NOT NULL DEFAULT 0;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS shipment_last_attempt_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS orders_status_idx ON orders (status);

-- products created from the dashboard get their id from this sequence; the
-- catalog seed still uses explicit ids and syncs the sequence afterwards.
CREATE SEQUENCE IF NOT EXISTS products_id_seq;
ALTER TABLE products ALTER COLUMN id SET DEFAULT nextval('products_id_seq');
-- keep the sequence ahead of ids inserted explicitly (catalog seed or old rows)
SELECT setval('products_id_seq', (SELECT COALESCE(MAX(id), 1) FROM products));
`

// Migrate creates the tables if they do not exist yet.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, schema); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

// Seed syncs the catalog content into the products table. Only content fields
// (name, description, images) are updated on conflict; price, stock and
// availability stay owned by the database (orders, sweeper, admin) so a deploy
// never overwrites runtime state.
func Seed(ctx context.Context, pool *pgxpool.Pool, catalogs []model.Catalog) error {
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM products`).Scan(&count); err != nil {
		return fmt.Errorf("count products: %w", err)
	}

	// The database owns the catalog once it has rows: seeding again would undo
	// edits and resurrect deleted products made from the dashboard.
	if count == 0 {
		seeded := 0
		for _, catalog := range catalogs {
			for _, product := range catalog.Items {
				images, err := json.Marshal(product.Images)
				if err != nil {
					return fmt.Errorf("marshal images for product %d: %w", product.ID, err)
				}

				_, err = pool.Exec(ctx, `
					INSERT INTO products (id, name, description, images, price, stock, is_available, weight_gram)
					VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
					product.ID, product.Name, product.Description, images,
					product.Price, product.Stock, product.IsAvailable, product.WeightGram)
				if err != nil {
					return fmt.Errorf("seed product %d: %w", product.ID, err)
				}
				seeded++
			}
		}
		log.Printf("seeded %d catalog rows into an empty products table", seeded)
	} else {
		log.Printf("products table already has %d rows, skipping the catalog seed", count)
	}

	// Keep the sequence ahead of the highest explicit (catalog) id.
	if _, err := pool.Exec(ctx, `SELECT setval('products_id_seq', (SELECT COALESCE(MAX(id), 1) FROM products))`); err != nil {
		return fmt.Errorf("sync product id sequence: %w", err)
	}

	return nil
}
