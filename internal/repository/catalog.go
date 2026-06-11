package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ficct-boutique/backend-go/internal/models"
)

// CatalogRepo provides data access for the product catalog: collections,
// products, and their variants.
type CatalogRepo struct {
	pool *pgxpool.Pool
}

// NewCatalogRepo constructs a CatalogRepo backed by the given connection pool.
func NewCatalogRepo(pool *pgxpool.Pool) *CatalogRepo {
	return &CatalogRepo{pool: pool}
}

// Collections

func (r *CatalogRepo) CreateCollection(ctx context.Context, name string, description, season *string) (*models.Collection, error) {
	const q = `INSERT INTO collections (name, description, season)
		VALUES ($1,$2,$3)
		RETURNING id, name, description, season, is_active, created_at, updated_at`
	c := &models.Collection{}
	err := r.pool.QueryRow(ctx, q, name, description, season).Scan(
		&c.ID, &c.Name, &c.Description, &c.Season, &c.IsActive, &c.CreatedAt, &c.UpdatedAt,
	)
	return c, err
}

func (r *CatalogRepo) ListCollections(ctx context.Context) ([]models.Collection, error) {
	const q = `SELECT id, name, description, season, is_active, created_at, updated_at
		FROM collections WHERE is_active = TRUE ORDER BY created_at DESC`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Collection{}
	for rows.Next() {
		c := models.Collection{}
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.Season, &c.IsActive, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Products

func (r *CatalogRepo) CreateProduct(ctx context.Context, p *models.Product) (*models.Product, error) {
	const q = `INSERT INTO products (collection_id, sku, name, description, category, base_price, currency, image_url, image_document_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id, collection_id, sku, name, description, category, base_price, currency, image_url, image_document_id, is_active, created_at, updated_at`
	out := &models.Product{}
	err := r.pool.QueryRow(ctx, q,
		p.CollectionID, p.SKU, p.Name, p.Description, p.Category, p.BasePrice, p.Currency, p.ImageURL, p.ImageDocumentID,
	).Scan(
		&out.ID, &out.CollectionID, &out.SKU, &out.Name, &out.Description, &out.Category,
		&out.BasePrice, &out.Currency, &out.ImageURL, &out.ImageDocumentID, &out.IsActive, &out.CreatedAt, &out.UpdatedAt,
	)
	return out, err
}

func (r *CatalogRepo) UpdateProduct(ctx context.Context, id uuid.UUID, name string, description *string, category string, basePrice float64, imageURL *string, imageDocumentID *uuid.UUID, isActive bool) (*models.Product, error) {
	const q = `UPDATE products SET name=$2, description=$3, category=$4, base_price=$5, image_url=$6, image_document_id=$7, is_active=$8, updated_at=NOW()
		WHERE id = $1
		RETURNING id, collection_id, sku, name, description, category, base_price, currency, image_url, image_document_id, is_active, created_at, updated_at`
	out := &models.Product{}
	err := r.pool.QueryRow(ctx, q, id, name, description, category, basePrice, imageURL, imageDocumentID, isActive).Scan(
		&out.ID, &out.CollectionID, &out.SKU, &out.Name, &out.Description, &out.Category,
		&out.BasePrice, &out.Currency, &out.ImageURL, &out.ImageDocumentID, &out.IsActive, &out.CreatedAt, &out.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return out, err
}

// SetProductActive is the explicit deactivate/activate primitive used by the
// admin UI. We do not hard-delete to preserve sales history.
func (r *CatalogRepo) SetProductActive(ctx context.Context, id uuid.UUID, active bool) (*models.Product, error) {
	const q = `UPDATE products SET is_active=$2, updated_at=NOW() WHERE id=$1
		RETURNING id, collection_id, sku, name, description, category, base_price, currency, image_url, image_document_id, is_active, created_at, updated_at`
	out := &models.Product{}
	err := r.pool.QueryRow(ctx, q, id, active).Scan(
		&out.ID, &out.CollectionID, &out.SKU, &out.Name, &out.Description, &out.Category,
		&out.BasePrice, &out.Currency, &out.ImageURL, &out.ImageDocumentID, &out.IsActive, &out.CreatedAt, &out.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return out, err
}

// ReplaceProductImage swaps imageDocumentId and bumps image_replaced_at so the
// UI can invalidate cached signed URLs and so audit consumers can see when the
// image was superseded. Returns the old document id (if any) so the caller can
// soft-delete the Express document and remove the underlying MinIO object.
func (r *CatalogRepo) ReplaceProductImage(ctx context.Context, id uuid.UUID, newImageDocID uuid.UUID) (*models.Product, *uuid.UUID, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)

	var oldDoc *uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT image_document_id FROM products WHERE id=$1`, id).Scan(&oldDoc); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, ErrNotFound
		}
		return nil, nil, err
	}
	out := &models.Product{}
	if err := tx.QueryRow(ctx,
		`UPDATE products SET image_document_id=$2, image_replaced_at=NOW(), updated_at=NOW() WHERE id=$1
		RETURNING id, collection_id, sku, name, description, category, base_price, currency, image_url, image_document_id, is_active, created_at, updated_at`,
		id, newImageDocID,
	).Scan(
		&out.ID, &out.CollectionID, &out.SKU, &out.Name, &out.Description, &out.Category,
		&out.BasePrice, &out.Currency, &out.ImageURL, &out.ImageDocumentID, &out.IsActive, &out.CreatedAt, &out.UpdatedAt,
	); err != nil {
		return nil, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	return out, oldDoc, nil
}

// SetVariantActive is the variant-level deactivate primitive.
func (r *CatalogRepo) SetVariantActive(ctx context.Context, id uuid.UUID, active bool) (*models.ProductVariant, error) {
	const q = `UPDATE product_variants SET is_active=$2, updated_at=NOW() WHERE id=$1
		RETURNING id, product_id, sku, size, color, price_override, is_active, created_at, updated_at`
	out := &models.ProductVariant{}
	err := r.pool.QueryRow(ctx, q, id, active).Scan(
		&out.ID, &out.ProductID, &out.SKU, &out.Size, &out.Color, &out.PriceOverride, &out.IsActive, &out.CreatedAt, &out.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return out, err
}

func (r *CatalogRepo) FindProduct(ctx context.Context, id uuid.UUID) (*models.Product, error) {
	const q = `SELECT id, collection_id, sku, name, description, category, base_price, currency, image_url, image_document_id, is_active, created_at, updated_at
		FROM products WHERE id = $1`
	p := &models.Product{}
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&p.ID, &p.CollectionID, &p.SKU, &p.Name, &p.Description, &p.Category,
		&p.BasePrice, &p.Currency, &p.ImageURL, &p.ImageDocumentID, &p.IsActive, &p.CreatedAt, &p.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

// ListProducts returns products, optionally including inactive ones (admin-only).
// When includeInactive is false (customer/public catalog), only active products
// are returned. The returned slice preserves DESC ordering by creation date.
func (r *CatalogRepo) ListProducts(ctx context.Context, category *string, search *string, includeInactive bool, limit, offset int) ([]models.Product, error) {
	q := `SELECT id, collection_id, sku, name, description, category, base_price, currency, image_url, image_document_id, is_active, created_at, updated_at
		FROM products WHERE 1=1`
	if !includeInactive {
		q += ` AND is_active = TRUE`
	}
	args := []interface{}{}
	idx := 1
	if category != nil && *category != "" {
		q += ` AND category = $` + itoa(idx)
		args = append(args, *category)
		idx++
	}
	if search != nil && *search != "" {
		q += ` AND (name ILIKE $` + itoa(idx) + ` OR sku ILIKE $` + itoa(idx) + `)`
		args = append(args, "%"+*search+"%")
		idx++
	}
	q += ` ORDER BY created_at DESC LIMIT $` + itoa(idx) + ` OFFSET $` + itoa(idx+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Product{}
	for rows.Next() {
		p := models.Product{}
		if err := rows.Scan(
			&p.ID, &p.CollectionID, &p.SKU, &p.Name, &p.Description, &p.Category,
			&p.BasePrice, &p.Currency, &p.ImageURL, &p.ImageDocumentID, &p.IsActive, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Variants

func (r *CatalogRepo) CreateVariant(ctx context.Context, v *models.ProductVariant) (*models.ProductVariant, error) {
	const q = `INSERT INTO product_variants (product_id, sku, size, color, price_override)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id, product_id, sku, size, color, price_override, is_active, created_at, updated_at`
	out := &models.ProductVariant{}
	err := r.pool.QueryRow(ctx, q, v.ProductID, v.SKU, v.Size, v.Color, v.PriceOverride).Scan(
		&out.ID, &out.ProductID, &out.SKU, &out.Size, &out.Color, &out.PriceOverride, &out.IsActive, &out.CreatedAt, &out.UpdatedAt,
	)
	return out, err
}

func (r *CatalogRepo) FindVariant(ctx context.Context, id uuid.UUID) (*models.ProductVariant, error) {
	const q = `SELECT id, product_id, sku, size, color, price_override, is_active, created_at, updated_at
		FROM product_variants WHERE id = $1`
	v := &models.ProductVariant{}
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&v.ID, &v.ProductID, &v.SKU, &v.Size, &v.Color, &v.PriceOverride, &v.IsActive, &v.CreatedAt, &v.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return v, err
}

func (r *CatalogRepo) VariantsByProduct(ctx context.Context, productID uuid.UUID) ([]models.ProductVariant, error) {
	const q = `SELECT id, product_id, sku, size, color, price_override, is_active, created_at, updated_at
		FROM product_variants WHERE product_id = $1 ORDER BY size, color`
	rows, err := r.pool.Query(ctx, q, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.ProductVariant{}
	for rows.Next() {
		v := models.ProductVariant{}
		if err := rows.Scan(&v.ID, &v.ProductID, &v.SKU, &v.Size, &v.Color, &v.PriceOverride, &v.IsActive, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// VariantsByProductIDs is the DataLoader batch fetch to avoid N+1 in GraphQL.
func (r *CatalogRepo) VariantsByProductIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID][]models.ProductVariant, error) {
	if len(ids) == 0 {
		return map[uuid.UUID][]models.ProductVariant{}, nil
	}
	const q = `SELECT id, product_id, sku, size, color, price_override, is_active, created_at, updated_at
		FROM product_variants WHERE product_id = ANY($1) ORDER BY product_id, size, color`
	rows, err := r.pool.Query(ctx, q, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[uuid.UUID][]models.ProductVariant, len(ids))
	for rows.Next() {
		v := models.ProductVariant{}
		if err := rows.Scan(&v.ID, &v.ProductID, &v.SKU, &v.Size, &v.Color, &v.PriceOverride, &v.IsActive, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		out[v.ProductID] = append(out[v.ProductID], v)
	}
	return out, rows.Err()
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	digits := []byte{}
	for i > 0 {
		digits = append([]byte{byte('0' + i%10)}, digits...)
		i /= 10
	}
	return string(digits)
}
