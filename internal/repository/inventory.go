package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/ficct-boutique/backend-go/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrInsufficientStock = errors.New("insufficient stock")

type InventoryRepo struct {
	pool *pgxpool.Pool
}

func NewInventoryRepo(pool *pgxpool.Pool) *InventoryRepo {
	return &InventoryRepo{pool: pool}
}

func (r *InventoryRepo) Upsert(ctx context.Context, variantID, branchID uuid.UUID, quantity, reorderLevel int) (*models.Inventory, error) {
	const q = `INSERT INTO inventory (variant_id, branch_id, quantity, reorder_level)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (variant_id, branch_id) DO UPDATE
		SET quantity = EXCLUDED.quantity, reorder_level = EXCLUDED.reorder_level, updated_at = NOW()
		RETURNING id, variant_id, branch_id, quantity, reorder_level, updated_at`
	inv := &models.Inventory{}
	err := r.pool.QueryRow(ctx, q, variantID, branchID, quantity, reorderLevel).Scan(
		&inv.ID, &inv.VariantID, &inv.BranchID, &inv.Quantity, &inv.ReorderLevel, &inv.UpdatedAt,
	)
	return inv, err
}

func (r *InventoryRepo) ByBranch(ctx context.Context, branchID uuid.UUID) ([]models.Inventory, error) {
	const q = `SELECT id, variant_id, branch_id, quantity, reorder_level, updated_at
		FROM inventory WHERE branch_id = $1 ORDER BY updated_at DESC`
	rows, err := r.pool.Query(ctx, q, branchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Inventory{}
	for rows.Next() {
		i := models.Inventory{}
		if err := rows.Scan(&i.ID, &i.VariantID, &i.BranchID, &i.Quantity, &i.ReorderLevel, &i.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

func (r *InventoryRepo) ByVariantIDs(ctx context.Context, variantIDs []uuid.UUID) (map[uuid.UUID][]models.Inventory, error) {
	if len(variantIDs) == 0 {
		return map[uuid.UUID][]models.Inventory{}, nil
	}
	const q = `SELECT id, variant_id, branch_id, quantity, reorder_level, updated_at
		FROM inventory WHERE variant_id = ANY($1)`
	rows, err := r.pool.Query(ctx, q, variantIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[uuid.UUID][]models.Inventory)
	for rows.Next() {
		i := models.Inventory{}
		if err := rows.Scan(&i.ID, &i.VariantID, &i.BranchID, &i.Quantity, &i.ReorderLevel, &i.UpdatedAt); err != nil {
			return nil, err
		}
		out[i.VariantID] = append(out[i.VariantID], i)
	}
	return out, rows.Err()
}

// DecrementForSale decrements stock atomically inside an existing tx.
// Returns error if there is not enough stock.
func (r *InventoryRepo) DecrementForSale(ctx context.Context, tx pgx.Tx, variantID, branchID uuid.UUID, qty int) error {
	const q = `UPDATE inventory SET quantity = quantity - $3, updated_at = NOW()
		WHERE variant_id = $1 AND branch_id = $2 AND quantity >= $3`
	tag, err := tx.Exec(ctx, q, variantID, branchID, qty)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrInsufficientStock
	}
	return nil
}

// Pool returns the underlying pool so the sales service can begin transactions
// that need to coordinate with this repo's exec.
func (r *InventoryRepo) Pool() *pgxpool.Pool { return r.pool }

// InventoryFilter captures the filter shape coming from the admin grid.
type InventoryFilter struct {
	BranchID                *uuid.UUID
	Search                  *string
	Size                    *string
	Color                   *string
	Status                  *string
	OnlyLowStock            bool
	IncludeInactiveVariants bool
}

// SearchInventory returns a paginated, filtered slice plus a total count for the
// admin grid. Joins variants + products + branches so the UI can render readable
// labels and thumbnails without N+1.
func (r *InventoryRepo) SearchInventory(ctx context.Context, f InventoryFilter, limit, offset int) ([]models.Inventory, int, error) {
	where := []string{"1=1"}
	args := []interface{}{}
	idx := 1

	if f.BranchID != nil {
		where = append(where, "i.branch_id = $"+itoa(idx))
		args = append(args, *f.BranchID)
		idx++
	}
	if f.Search != nil && *f.Search != "" {
		where = append(where, "(p.name ILIKE $"+itoa(idx)+" OR p.sku ILIKE $"+itoa(idx)+" OR v.sku ILIKE $"+itoa(idx)+")")
		args = append(args, "%"+*f.Search+"%")
		idx++
	}
	if f.Size != nil && *f.Size != "" {
		where = append(where, "v.size = $"+itoa(idx))
		args = append(args, *f.Size)
		idx++
	}
	if f.Color != nil && *f.Color != "" {
		where = append(where, "v.color = $"+itoa(idx))
		args = append(args, *f.Color)
		idx++
	}
	if !f.IncludeInactiveVariants {
		where = append(where, "v.is_active = TRUE")
		where = append(where, "p.is_active = TRUE")
	}
	if f.OnlyLowStock {
		where = append(where, "i.quantity <= i.reorder_level")
	}
	if f.Status != nil {
		switch *f.Status {
		case "ok":
			where = append(where, "i.quantity > i.reorder_level AND v.is_active = TRUE")
		case "low":
			where = append(where, "i.quantity <= i.reorder_level AND i.quantity > 0")
		case "critical":
			where = append(where, "i.quantity = 0")
		case "inactive":
			where = append(where, "v.is_active = FALSE")
		}
	}
	whereSQL := strings.Join(where, " AND ")

	countSQL := `SELECT COUNT(*) FROM inventory i
		JOIN product_variants v ON v.id = i.variant_id
		JOIN products p ON p.id = v.product_id
		JOIN branches b ON b.id = i.branch_id
		WHERE ` + whereSQL
	var total int
	if err := r.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rowSQL := `SELECT i.id, i.variant_id, i.branch_id, i.quantity, i.reorder_level, i.updated_at
		FROM inventory i
		JOIN product_variants v ON v.id = i.variant_id
		JOIN products p ON p.id = v.product_id
		JOIN branches b ON b.id = i.branch_id
		WHERE ` + whereSQL + `
		ORDER BY p.name ASC, v.size ASC, v.color ASC, b.name ASC
		LIMIT $` + itoa(idx) + ` OFFSET $` + itoa(idx+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, rowSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []models.Inventory{}
	for rows.Next() {
		i := models.Inventory{}
		if err := rows.Scan(&i.ID, &i.VariantID, &i.BranchID, &i.Quantity, &i.ReorderLevel, &i.UpdatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, i)
	}
	return out, total, rows.Err()
}

// SetStock writes an absolute quantity for the given (variant, branch).
func (r *InventoryRepo) SetStock(ctx context.Context, variantID, branchID uuid.UUID, qty int) (*models.Inventory, error) {
	const q = `INSERT INTO inventory (variant_id, branch_id, quantity, reorder_level)
		VALUES ($1,$2,$3,0)
		ON CONFLICT (variant_id, branch_id) DO UPDATE
		SET quantity = EXCLUDED.quantity, updated_at = NOW()
		RETURNING id, variant_id, branch_id, quantity, reorder_level, updated_at`
	out := &models.Inventory{}
	err := r.pool.QueryRow(ctx, q, variantID, branchID, qty).Scan(&out.ID, &out.VariantID, &out.BranchID, &out.Quantity, &out.ReorderLevel, &out.UpdatedAt)
	return out, err
}

// AdjustStock applies a signed delta and refuses to go below zero.
func (r *InventoryRepo) AdjustStock(ctx context.Context, variantID, branchID uuid.UUID, delta int) (*models.Inventory, error) {
	const q = `UPDATE inventory SET quantity = GREATEST(0, quantity + $3), updated_at = NOW()
		WHERE variant_id = $1 AND branch_id = $2
		RETURNING id, variant_id, branch_id, quantity, reorder_level, updated_at`
	out := &models.Inventory{}
	err := r.pool.QueryRow(ctx, q, variantID, branchID, delta).Scan(&out.ID, &out.VariantID, &out.BranchID, &out.Quantity, &out.ReorderLevel, &out.UpdatedAt)
	return out, err
}

// SetReorderLevel updates the reorder threshold.
func (r *InventoryRepo) SetReorderLevel(ctx context.Context, variantID, branchID uuid.UUID, level int) (*models.Inventory, error) {
	const q = `UPDATE inventory SET reorder_level = $3, updated_at = NOW()
		WHERE variant_id = $1 AND branch_id = $2
		RETURNING id, variant_id, branch_id, quantity, reorder_level, updated_at`
	out := &models.Inventory{}
	err := r.pool.QueryRow(ctx, q, variantID, branchID, level).Scan(&out.ID, &out.VariantID, &out.BranchID, &out.Quantity, &out.ReorderLevel, &out.UpdatedAt)
	return out, err
}
