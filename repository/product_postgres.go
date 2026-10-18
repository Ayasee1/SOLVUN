package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"golearn/model"
)

const queryTimeout = 5 * time.Second

// PostgresProductRepository stores products in Postgres (Supabase).
type PostgresProductRepository struct {
	pool *pgxpool.Pool
}

var _ ProductRepository = (*PostgresProductRepository)(nil)

func NewPostgresProductRepository(pool *pgxpool.Pool) *PostgresProductRepository {
	return &PostgresProductRepository{pool: pool}
}

const productColumns = `id, name, description, images, price, stock, is_available, weight_gram, updated_at`

func (r *PostgresProductRepository) GetAll() ([]model.Product, error) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	rows, err := r.pool.Query(ctx, `SELECT `+productColumns+` FROM products ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	products := make([]model.Product, 0)
	for rows.Next() {
		product, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		products = append(products, *product)
	}

	return products, rows.Err()
}

func (r *PostgresProductRepository) FindByID(id int) (*model.Product, error) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	return scanProduct(r.pool.QueryRow(ctx,
		`SELECT `+productColumns+` FROM products WHERE id = $1`, id))
}

func (r *PostgresProductRepository) Delete(id int) (model.Product, error) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	product, err := scanProduct(r.pool.QueryRow(ctx,
		`DELETE FROM products WHERE id = $1 RETURNING `+productColumns, id))
	if err != nil {
		return model.Product{}, err
	}

	return *product, nil
}

// DecreaseStock runs in one transaction. The conditional `AND stock >= $1`
// makes overselling impossible even with concurrent orders, so no mutex is needed.
func (r *PostgresProductRepository) DecreaseStock(requested map[int]int) error {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for id, qty := range requested {
		tag, err := tx.Exec(ctx, `
			UPDATE products
			SET stock = stock - $1, is_available = (stock - $1) > 0, updated_at = now()
			WHERE id = $2 AND stock >= $1`, qty, id)
		if err != nil {
			return err
		}

		if tag.RowsAffected() == 0 {
			// nothing updated: find out whether the product is missing or the stock is short
			var name string
			err := tx.QueryRow(ctx, `SELECT name FROM products WHERE id = $1`, id).Scan(&name)
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("%w for id %d", ErrProductNotFound, id)
			}
			if err != nil {
				return err
			}
			return fmt.Errorf("%w for product %s", ErrNotEnoughStock, name)
		}
	}

	return tx.Commit(ctx)
}

// ReleaseStock gives reserved stock back; missing products are ignored.
func (r *PostgresProductRepository) ReleaseStock(requested map[int]int) error {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	for id, qty := range requested {
		_, err := r.pool.Exec(ctx, `
			UPDATE products
			SET stock = stock + $1, is_available = true, updated_at = now()
			WHERE id = $2`, qty, id)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *PostgresProductRepository) UpdateProduct(id int, update model.ProductUpdate) (*model.Product, error) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	return scanProduct(r.pool.QueryRow(ctx, `
		UPDATE products SET
			name = COALESCE($2, name),
			price = COALESCE($3, price),
			stock = COALESCE($4, stock),
			is_available = COALESCE($5, CASE WHEN $4 IS NOT NULL THEN $4 > 0 ELSE is_available END),
			updated_at = now()
		WHERE id = $1
		RETURNING `+productColumns,
		id, update.Name, update.Price, update.Stock, update.IsAvailable))
}

// scanProduct works for both pgx.Row and pgx.Rows.
func scanProduct(row pgx.Row) (*model.Product, error) {
	var (
		product model.Product
		images  []byte
	)

	if err := row.Scan(&product.ID, &product.Name, &product.Description, &images,
		&product.Price, &product.Stock, &product.IsAvailable, &product.WeightGram, &product.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrProductNotFound
		}
		return nil, err
	}

	if len(images) > 0 {
		if err := json.Unmarshal(images, &product.Images); err != nil {
			return nil, err
		}
	}

	return &product, nil
}
