-- Restore the pre-link shape (buyer back in cashier_id) for sales that were
-- re-linked by the up migration. Created customer profiles are kept.
UPDATE sales s
SET cashier_id = c.user_id,
    customer_id = NULL,
    updated_at = NOW()
FROM customers c
JOIN users u ON u.id = c.user_id
WHERE s.customer_id = c.id
  AND u.role = 'customer'
  AND s.cashier_id IS NULL;
