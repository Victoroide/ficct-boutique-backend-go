# FICCT Boutique — Go Core Backend (MS1)

GraphQL source of truth for products, variants, inventory, branches, sales, orders, and the BI dashboard. Issues RS256 JWTs that the Express and Django services verify. Emits HMAC-signed `sale.confirmed` events through a transactional outbox.

> **One-line summary of the role this repo plays:** it is the **authoritative catalog + inventory + sales database**, and the **single token issuer** for the whole system.

---

## What is real in this repo (and what is not)

This README is the entry point. **Everything described here is implemented in the code as of the cleanup pass on 2026-05-23.** Anything not mentioned does not exist — there are no hidden modules, no stubbed endpoints, no half-finished features.

What this service **does**:

- Issues RS256 access tokens (`POST /graphql` → `login` mutation).
- Exposes a single GraphQL endpoint at `/graphql` with the schema in [graph/schema.graphqls](graph/schema.graphqls).
- Serves a development GraphiQL UI at `/playground`.
- Serves seeded SVG product placeholders at `/static/products/<sku>.svg`.
- Runs migrations (`cmd/migrate`), seeds demo data (`cmd/seed`), and the HTTP server (`cmd/server`).
- Runs a background webhook dispatcher goroutine that drains the outbox table and POSTs `sale.confirmed` events to `WEBHOOK_INVOICE_URL`.

What this service **does not do** (do not look for it):

- No file upload, no S3, no MinIO. Document storage lives in the Express service.
- No AI, no embeddings, no forecasting, no clustering. That lives in the Django service.
- No refresh-token rotation endpoint exposed in GraphQL (the table exists, the mutation does not).
- No DataLoader request-scoped batching. Resolvers use `WHERE id = ANY($1)` per field instead.

---

## Tech stack

| Concern | Choice |
|---------|--------|
| Language / runtime | Go 1.23 (alpine build), CGO disabled |
| HTTP router | `go-chi/chi/v5` + `chi/middleware` + `chi/cors` |
| GraphQL | `graph-gophers/graphql-go` (schema-first, `UseFieldResolvers`, `MaxParallelism(20)`) |
| Database | PostgreSQL 16 via `jackc/pgx/v5` connection pool |
| Auth | `golang-jwt/jwt/v5` RS256, `argon2id` password hashing |
| Logging | `rs/zerolog` |
| Container | multi-stage Dockerfile, alpine 3.20 runtime, non-root `ficct:1000` user |

The full module list is in [go.mod](go.mod).

---

## Directory layout

```
cmd/
  server/         HTTP entrypoint — boots config, db, keys, services, GraphQL handler, webhook dispatcher
  migrate/        applies migrations from /app/migrations (up/down)
  seed/           idempotent seed: admin/staff/customer users, 2 branches, 1 collection, 4 products × 6 variants × N branches
graph/
  schema.graphqls schema-first SDL (embedded into the server binary)
  resolver.go     root Resolver wiring services + repositories
  queries.go      14 query resolvers
  mutations.go    17 mutation resolvers
  types.go        GraphQL type resolvers (Product, Variant, Sale, ...)
  scalars.go      Time + UUID custom scalars
  authz.go        requireAuth / requireAdminOrStaff helpers
internal/
  auth/           RSA key loader, JWT issuer + verifier, argon2id hash+verify
  config/         env-based config loader
  database/       pgx pool + simple file-based migration runner
  middleware/     OptionalAuth (verifies bearer if present), rate limiter (per-IP token bucket)
  models/         plain domain structs (User, Product, Variant, Branch, Sale, ...)
  observability/  zerolog setup
  repository/     pgx-backed data access: users, catalog, branches, inventory, sales, orders, reports, outbox
  service/        business logic: auth, catalog, sales (transactional confirm + outbox enqueue), reports
  staticassets/   embedded SVG placeholders mounted at /static/products/
  webhook/        dispatcher loop, HMAC-SHA256 signer, exponential backoff
migrations/
  0001_init.up.sql / .down.sql
  0002_product_image_document.up.sql / .down.sql
  0003_variant_active_state.up.sql / .down.sql
docs/
  architecture/   system + GraphQL + webhook reference
  development/    local-run instructions, environment variables, key rotation
  qa-artifacts/   organized Playwright screenshots used as visual evidence
```

---

## Running it

### Standalone (this repo only)

```powershell
copy .env.example .env
# Generate the dev RSA keypair into .tools/keys/ (see docs/development/JWT_KEYS.md)
docker compose up -d --build
```

Endpoints:

- GraphQL: `http://localhost:8080/graphql`
- GraphiQL playground: `http://localhost:8080/playground`
- Health: `http://localhost:8080/health`
- Static SVG placeholders: `http://localhost:8080/static/products/<sku>.svg`

### Full system (this is the meta-compose root)

```powershell
docker compose -f docker-compose.full.yml up -d --build
```

This brings up Go (8093), Express (8091), Django (8092), Angular admin (4200), React Native customer web (4300), Postgres (×2), MinIO (9010 api / 9011 console), and DynamoDB Local. See [docs/development/RUNNING_LOCALLY.md](docs/development/RUNNING_LOCALLY.md) for the full port map and bootstrap sequence.

---

## Make targets

```
make tidy         # go mod tidy
make fmt          # gofmt -s -w .
make vet          # go vet ./...
make lint         # fmt + vet
make test         # go test ./... -race -count=1
make build        # builds bin/server, bin/migrate, bin/seed
make run          # go run ./cmd/server
make migrate      # go run ./cmd/migrate up
make seed         # go run ./cmd/seed
make docker-up    # docker compose up -d
make docker-down  # docker compose down
```

`make gqlgen` is in the Makefile but **not used** by this service — the schema is parsed at runtime by `graph-gophers/graphql-go`, no code generation step exists.

---

## Seed data

The seed (`cmd/seed`) is idempotent and produces:

| Email | Password | Role |
|-------|----------|------|
| `admin@ficct.local` | `Admin123!` | admin |
| `staff@ficct.local` | `Staff123!` | staff |
| `cliente@ficct.local` | `Cliente123!` | customer |

Plus 2 branches (`SC-01 Boutique Centro`, `SC-02 Boutique Equipetrol`), 1 collection (`Otoño 2026`), and 4 products (`BLZ-001`, `PNT-001`, `VST-001`, `FLD-001`) each with 6 size×color variants and per-branch stock of 12 units (reorder level 3).

A backfill step at the end of seed sets `image_url = '/static/products/' || sku || '.svg'` for any product whose `image_url` is null/empty.

---

## GraphQL surface

The complete API reference is in [docs/architecture/GRAPHQL_API.md](docs/architecture/GRAPHQL_API.md). High-level shape:

- **14 queries**: `me`, `product`, `products`, `branches`, `branch`, `inventoryByBranch`, `inventoryEntries`, `sale`, `sales`, `order`, `orders`, `monthlySales`, `popularProducts`, `dashboardSummary`.
- **17 mutations**: `login`, `createCollection`, `createProduct`, `updateProduct`, `createVariant`, `upsertInventory`, `createBranch`, `createSale`, `confirmSale`, `deactivateProduct`, `activateProduct`, `replaceProductImage`, `deactivateVariant`, `activateVariant`, `setInventoryStock`, `adjustInventoryStock`, `updateInventoryReorderLevel`.

Authorization is enforced inside each resolver via `requireAuth` / `requireAdminOrStaff` (see [graph/authz.go](graph/authz.go)). Customers can query the public catalog but not inventory, sales, or reports.

---

## Webhook (`sale.confirmed`)

When `confirmSale(saleId)` succeeds, the service writes the event to the `webhook_outbox` table **inside the same SQL transaction** that creates the `Order` row. The dispatcher goroutine then:

1. Selects unsent rows with `FOR UPDATE SKIP LOCKED` (so multiple replicas are safe).
2. Signs `<sha256(secret + payload)>` with HMAC-SHA256.
3. POSTs to `WEBHOOK_INVOICE_URL` with headers:
   - `X-FICCT-Event: sale.confirmed`
   - `X-FICCT-Event-Id: <uuid>`
   - `X-FICCT-Signature: sha256=<hex>`
4. On non-2xx response, increments `attempts` and re-queues with exponential backoff capped at 5 minutes, up to `WEBHOOK_MAX_RETRIES`.

If `WEBHOOK_INVOICE_URL` or `WEBHOOK_HMAC_SECRET` is unset, the dispatcher does not start and a warning is logged at startup. The events still accumulate in `webhook_outbox` and will be sent when configuration is provided.

---

## Security notes

- Passwords hashed with argon2id (`m=64 MiB, t=3, p=2`, 16-byte salt, 32-byte hash).
- JWT tokens signed with RS256. The **private key never leaves this service**; Express/Django only get the public key.
- CORS is allow-listed from `CORS_ALLOWED_ORIGINS` (default `http://localhost:4200,http://localhost:19006,http://localhost:8081`).
- Rate limit is a per-IP token bucket: default 20 rps with burst 40 (`RATE_LIMIT_RPS` / `RATE_LIMIT_BURST`).
- All SQL is parameterized via `pgx`; no string concatenation in queries.
- Sale confirmation runs inside a single transaction with `UPDATE inventory SET quantity = quantity - $ WHERE quantity >= $`; insufficient stock aborts the transaction atomically.
- The `OptionalAuth` middleware verifies the bearer if present but does **not** require it — per-resolver `requireAuth` is what gates access.

---

## Known limitations (called out so they're not surprises)

- No request-scoped DataLoader; sibling resolvers may re-query overlapping IDs. Acceptable for the demo scale.
- No `refreshToken` / `logout` mutation. The schema for token rotation exists at the DB level but isn't exposed.
- No structured GraphQL error codes — clients distinguish by error message text.
- The `/playground` route is wired unconditionally; remove it from `cmd/server/main.go` before any production deployment.
- The dev RSA keys in `.tools/keys/` are **baked into the Docker image** for simplicity. In production, inject them as Docker secrets and rebuild without that COPY step.

---

## Documentation index

- [docs/architecture/SYSTEM_OVERVIEW.md](docs/architecture/SYSTEM_OVERVIEW.md) — how MS1 fits with MS2 / MS3 / Angular / RN.
- [docs/architecture/GRAPHQL_API.md](docs/architecture/GRAPHQL_API.md) — every query and mutation, arguments, authz rules.
- [docs/architecture/WEBHOOKS.md](docs/architecture/WEBHOOKS.md) — outbox + dispatcher + signature scheme.
- [docs/development/RUNNING_LOCALLY.md](docs/development/RUNNING_LOCALLY.md) — standalone and full-system bring-up.
- [docs/development/JWT_KEYS.md](docs/development/JWT_KEYS.md) — generating and rotating the RS256 keypair.
- [docs/development/ENVIRONMENT.md](docs/development/ENVIRONMENT.md) — every env var, default, and effect.
- [docs/qa-artifacts/README.md](docs/qa-artifacts/README.md) — index of the Playwright screenshots kept as visual evidence.
