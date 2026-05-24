# Webhooks — `sale.confirmed`

The Go service emits one webhook event today: `sale.confirmed`. The implementation lives in:

- [internal/repository/outbox.go](../../internal/repository/outbox.go) — the `outbox_events` table.
- [internal/service/sales.go](../../internal/service/sales.go) — enqueues the event inside `confirmSale`'s transaction.
- [internal/webhook/dispatcher.go](../../internal/webhook/dispatcher.go) — the background dispatcher goroutine.

There is **no other webhook event** in this codebase.

## Why the transactional outbox

Naïve approach: after committing the sale, do `http.Post(...)`. Problem: a crash between commit and POST means the event is lost; a crash between POST and ack means a duplicate. The outbox pattern avoids this:

1. The `INSERT INTO outbox_events ...` is part of the same transaction that updates the sale and decrements inventory. Either all four writes happen or none do.
2. A separate goroutine reads unsent rows with `FOR UPDATE SKIP LOCKED`, attempts the POST, then updates `sent_at` (or increments `attempts`).
3. Multiple replicas can run the dispatcher simultaneously — `SKIP LOCKED` ensures no two workers claim the same row.

## Wire format

```http
POST <WEBHOOK_INVOICE_URL>
Content-Type: application/json
X-FICCT-Event: sale.confirmed
X-FICCT-Event-Id: <uuid>
X-FICCT-Signature: sha256=<hex(hmac_sha256(secret, raw_body))>

{
  "saleId": "...",
  "orderId": "...",
  "branchId": "...",
  "total": 350.0,
  "currency": "BOB",
  "confirmedAt": "2026-05-23T20:15:00Z",
  "items": [
    {"variantId": "...", "quantity": 1, "unitPrice": 350.0, "lineTotal": 350.0}
  ]
}
```

The exact payload structure is whatever `service.SalesService.confirmSale` writes into `outbox_events.payload`; treat the field list above as the **current** shape, not a stable contract — there are no consumers other than n8n / the optional invoice automation.

## Verification (receiver side)

```javascript
// node example
const crypto = require('crypto');
function verify(rawBody, header, secret) {
  const expected = 'sha256=' + crypto
    .createHmac('sha256', secret)
    .update(rawBody)
    .digest('hex');
  return crypto.timingSafeEqual(Buffer.from(expected), Buffer.from(header));
}
```

The receiver must use the **raw** request body (pre-JSON-parse) to compute the HMAC.

## Retry policy

- On any non-2xx response (or network error), `attempts` is incremented.
- The next attempt is scheduled after `min(2^attempts seconds, 5 minutes)`.
- After `WEBHOOK_MAX_RETRIES` total attempts, the row is marked `failed` and is no longer retried automatically. There is no built-in alerting for failed rows — query `WHERE status = 'failed'` to surface them.

## Disabling

If `WEBHOOK_INVOICE_URL` or `WEBHOOK_HMAC_SECRET` is empty at startup, the dispatcher does not start. Events still accumulate in `outbox_events` and will be processed on the next start when configuration is provided.
