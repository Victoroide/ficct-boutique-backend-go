DROP INDEX IF EXISTS idx_products_image_doc;
ALTER TABLE products DROP COLUMN IF EXISTS image_document_id;
