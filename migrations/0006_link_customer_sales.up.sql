-- Customer self-checkout sales were stored with the buyer in cashier_id and no
-- customer link, which made "my orders" impossible to resolve. Backfill:

-- 1. Every customer-role user gets a customer profile.
INSERT INTO customers (user_id, full_name)
SELECT u.id, u.full_name
FROM users u
WHERE u.role = 'customer'
  AND NOT EXISTS (SELECT 1 FROM customers c WHERE c.user_id = u.id);

-- 2. Re-link sales recorded with a customer-role "cashier" to that buyer's
--    customer profile (a customer-role user is never a legitimate cashier).
UPDATE sales s
SET customer_id = c.id,
    cashier_id = NULL,
    updated_at = NOW()
FROM users u
JOIN customers c ON c.user_id = u.id
WHERE s.cashier_id = u.id
  AND u.role = 'customer'
  AND s.customer_id IS NULL;
