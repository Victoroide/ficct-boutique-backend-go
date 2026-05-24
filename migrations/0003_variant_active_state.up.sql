-- Variants gain an active state so admin can deactivate a specific size/color
-- combination without deleting the row (which would break sales history).
ALTER TABLE product_variants
  ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT TRUE;

CREATE INDEX IF NOT EXISTS idx_variants_active ON product_variants(is_active);

-- Image lifecycle: a product image document can be marked as superseded when
-- the admin replaces it. Express documents already have a 'deleted' status;
-- we just add a tiny field on products to track image generation.
ALTER TABLE products
  ADD COLUMN IF NOT EXISTS image_replaced_at TIMESTAMPTZ;
