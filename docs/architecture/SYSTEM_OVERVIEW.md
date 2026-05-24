# System Overview — Go Core (MS1) in the FICCT Boutique stack

This document describes how the Go service relates to the other four repositories. It is not a marketing diagram — every box exists in code and every arrow corresponds to an HTTP call that the source repository actually makes.

## The five repositories

| Role | Repo | Tech | Default port (full-system compose) |
|------|------|------|------------------------------------|
| MS1 — Go core (this repo) | `go/ficct-boutique-backend-go` | Go 1.23, GraphQL, PostgreSQL | host **8093** → container 8080 |
| MS2 — Django AI | `python/django/ficct-boutique-backend-python` | Django 5, DRF, DynamoDB Local | host **8092** → container 8000 |
| MS3 — Express documents | `typescript/ficct-boutique-backend-express` | Express 4, PostgreSQL, MinIO/S3 | host **8091** → container 8081 |
| Angular admin frontend | `angular/ficct-boutique-frontend-angular` | Angular 17, Apollo, Tailwind | host **4200** |
| React Native customer app | `react/react-native/ficct-boutique-mobile-react-native` | Expo + Expo Web | host **4300** (web build) |

## What the Go service owns

The Go service is the **single source of truth** for:

1. **Identity** — users, roles (`admin`, `staff`, `customer`, `system`), Argon2id-hashed passwords.
2. **Tokens** — it is the only service that holds the RS256 **private** key. Every other service receives the public key and verifies tokens locally.
3. **Catalog** — collections, products, variants, branches.
4. **Inventory** — `(variant_id, branch_id) → (quantity, reorder_level)`.
5. **Transactions** — sales (`pending` / `confirmed` / `cancelled`) and orders (`placed` / `preparing` / `ready` / `delivered` / `cancelled`).
6. **Reporting reads** — monthly sales, popular products, dashboard summary.
7. **Outbox** — durable queue of domain events (`sale.confirmed`) that the dispatcher signs with HMAC-SHA256 and POSTs to `WEBHOOK_INVOICE_URL`.

## What the Go service does *not* own

- File storage. PDFs, invoices, evidence — Express owns the `documents` table and the S3/MinIO bucket. Go does not call S3.
- AI features. Image similarity, sales forecasting, customer clustering — Django owns them, backed by DynamoDB.
- Sessions / refresh-token rotation endpoints (the table exists, no GraphQL mutation is wired).

## Cross-service contracts

### Auth — RS256 token issued by Go, verified by everyone else

```
+----------------+ login(email,password)      +--------------------+
|  Angular /     | -------------------------> |   Go (MS1)         |
|  React Native  |                            |  - argon2id verify |
|  client        |                            |  - sign RS256 JWT  |
|                | <------- accessToken ----- |                    |
+----------------+                            +--------------------+
       |                                              |
       | Bearer accessToken                           | exports public key
       v                                              v
+----------------+    +----------------+    +------------------+
|   Go (MS1)     |    |  Express (MS3) |    |  Django (MS2)    |
|  /graphql      |    |  /api/docs/... |    |  /api/ai/...     |
|  OptionalAuth  |    | requireAuth    |    | DRF auth class   |
+----------------+    +----------------+    +------------------+
```

The token carries: `sub` (user UUID), `email`, `role`, `iss=ficct-go`, `aud=ficct-express,ficct-django,ficct-angular,ficct-mobile`, `kid=dev-1`. The verifiers in each downstream service load the public PEM at `JWT_PUBLIC_KEY_PATH`.

### `sale.confirmed` webhook

Go does **not** call the other services synchronously. When `confirmSale(saleId)` succeeds, the event is written to `outbox_events` in the same transaction, then the dispatcher goroutine drains the queue and POSTs to whichever URL `WEBHOOK_INVOICE_URL` points at (typically an n8n workflow that produces a PDF invoice and registers it in Express).

```
GraphQL: confirmSale
  ├── BEGIN
  ├── update sales.status = 'confirmed'
  ├── insert into orders ...
  ├── update inventory set quantity = quantity - $ where quantity >= $   -- aborts atomically if insufficient
  ├── insert into outbox_events (event_type='sale.confirmed', payload=...)
  └── COMMIT
                |
                v
         dispatcher goroutine
  ├── SELECT ... FOR UPDATE SKIP LOCKED
  ├── HMAC-SHA256 sign payload
  ├── POST WEBHOOK_INVOICE_URL
  ├── on 2xx: mark sent
  └── on non-2xx: increment attempts, schedule retry with exp backoff (cap 5m)
```

The recipient (n8n or whoever holds the secret) verifies `X-FICCT-Signature: sha256=<hex>` against the shared `WEBHOOK_HMAC_SECRET`.

### Images shown in the admin / customer apps

There are two image sources, by design:

1. **Seeded demo placeholders** — `imageUrl = "/static/products/<sku>.svg"`. Served by Go itself from an embedded filesystem at [internal/staticassets](../../internal/staticassets/). This is what the seed data points at.
2. **Admin-uploaded real images** — when an admin uploads a JPG/PNG via the Angular UI, the file goes to Express (which writes it to MinIO). Express then returns a presigned GET URL. Angular calls `replaceProductImage(id, newImageDocumentId)` on the Go service to store a reference (`image_document_id`) on the product. Reads then resolve `imageUrl` via that document. The bucket itself is private; only presigned URLs work.

## Service dependencies at startup

```
go-postgres ─── healthy ──► go-core
                                │
                                ├─ runs `migrate up`
                                ├─ runs `seed`
                                └─ starts `server` + webhook dispatcher

docs-postgres ── healthy ──► express-docs
minio ─────── healthy ──► minio-bootstrap ── completed ──► express-docs

dynamodb ── (no healthcheck) ──► django-ai
                                    └─ runs `ensure_tables` then `runserver`

go-core ┐
django-ai ├── any order ──► angular-admin
express-docs ┘

go-core ┐
django-ai ├── any order ──► mobile-web  (Expo web build served by nginx)
```

Angular and the mobile web container are **just** static asset servers — they do not depend on Postgres or MinIO directly; their requests are reverse-proxied to the backend services through nginx config inside each frontend image.

## Database boundaries

| Database | Owner | Notable tables |
|----------|-------|----------------|
| `ficct_boutique` (Postgres) | Go | `users`, `collections`, `products`, `product_variants`, `branches`, `inventory`, `sales`, `sale_items`, `orders`, `outbox_events`, plus refresh-token table |
| `ficct_documents` (Postgres) | Express | `documents`, `document_versions`, `document_uploads`, `audit_events`, `hash_ledger` |
| `ficct_*` (DynamoDB Local, prefix `DYNAMODB_TABLE_PREFIX`) | Django | embedding records, forecast snapshots, customer cluster assignments |

No service writes to another service's database. The only cross-service write path is the HMAC-signed webhook (Go → n8n → Express).
