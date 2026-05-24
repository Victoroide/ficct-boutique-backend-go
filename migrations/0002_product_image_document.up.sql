-- Allow products to reference a document in the Express documents service for
-- the catalog image. The legacy image_url field is preserved for back-compat,
-- but the admin UI uploads files and stores their document id here.
ALTER TABLE products ADD COLUMN IF NOT EXISTS image_document_id UUID;
CREATE INDEX IF NOT EXISTS idx_products_image_doc ON products(image_document_id) WHERE image_document_id IS NOT NULL;
