ALTER TABLE products DROP COLUMN IF EXISTS image_replaced_at;
DROP INDEX IF EXISTS idx_variants_active;
ALTER TABLE product_variants DROP COLUMN IF EXISTS is_active;
