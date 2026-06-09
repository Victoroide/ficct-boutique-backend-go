# FICCT Boutique — Final Environment Variables and GitHub Secrets Guide

> Single source of truth for every environment variable and secret across the five
> FICCT Boutique repositories, how to obtain/generate each value, and how to wire them
> up as GitHub Secrets / Variables for deployment to Railway (Go), GCP (Django),
> AWS (Express), static hosting (Angular) and EAS (Expo).
>
> **This document was produced by direct inspection of source code and configuration**
> (`.env.example`, Docker Compose, Dockerfiles, config loaders, Django settings, Express
> Zod config, Go config, Angular `environment.*.ts`, Expo `app.json`, and `package`
> scripts). No variables were invented. Variables that exist but are unused, or that are
> read by code but were missing from `.env.example`, are flagged explicitly.

---

## 1. Scope and verified system status

This guide covers all five repositories:

| # | Repo | Role | Local host port (full stack) | Deploy target (intended) |
|---|------|------|------------------------------|--------------------------|
| 1 | `ficct-boutique-backend-go` | MS1 — Core API (GraphQL, auth/JWT, catalog, inventory, sales, push) | `8093 → 8080` | Railway |
| 2 | `ficct-boutique-backend-python` | MS2 — AI backend (similarity, forecasting, clustering) | `8092 → 8000` | GCP (Cloud Run) |
| 3 | `ficct-boutique-backend-express` | MS3 — Document service (S3/MinIO, hash ledger, audit) | `8091 → 8081` | AWS |
| 4 | `ficct-boutique-frontend-angular` | Admin web (nginx static SPA) | `4200 → 80` | Static hosting / nginx |
| 5 | `ficct-boutique-mobile-react-native` | Customer app (Expo / React Native; web build) | `4300 → 80` | EAS (native) + static (web) |

### Verified system status (end-to-end, this delivery)

The full stack (`docker compose -f docker-compose.full.yml up -d --build`) was built and run
clean (exit 0). All 10 long-running services reported **healthy**; `dynamodb-local` runs
(no healthcheck defined) and `minio-bootstrap` is a one-shot that exits after creating the
private bucket. **No panics/fatals/errors** appeared in the aggregated stack logs.

- **Backend (64/64 automated HTTP/GraphQL checks passed on a pristine DB):** health
  endpoints; GraphQL login + RS256 JWT issuance; admin/staff/customer RBAC; products with
  `includeInactive`; product images; variants; inventory pagination/filtering/inline editing;
  branches; sales→orders; dashboard/reports; product soft-delete/restore; variant
  activation/deactivation; push-token registration; server-side push send (proven against the
  in-stack **fake Expo Push API**, including invalid-token deactivation); Express
  upload-request→presigned PUT→confirm(SHA-256)→download→verify(hash + ledger chain)→audit→
  soft-delete(blocks download)→restore; product-image document flow (Go links an Express doc);
  cross-service JWT (one Go token accepted by Express and Django); Django token verification,
  forecasting, clustering, customer segmentation, catalog sync (112-dim embeddings) and image
  similarity search with correct ranking.
- **Angular admin (live Playwright):** login/logout, dashboard charts, products, inventory
  (table + filters + inline `+10` edit firing a mutation), branches, sales/reports, documents
  (verify call hits Express), audit log, AI analytics (forecast + clustering call Django),
  role guards (customer redirected from `/audit` and `/products/new`), responsive at 1440 /
  1024 / 768 / 430 / 390 / 360 with **zero horizontal overflow** and **zero console errors**.
- **Mobile (Expo Web, live Playwright):** customer login, catalog, product detail, variant
  selection with live stock, cart (qty/IVA 13%/total), checkout → order created and persisted
  (`ORD-…`, BOB 327.70). Its own Playwright e2e suite passes **6/6**.
- **Validation commands:** Go `gofmt`/`vet`/`build`/`test` all clean; Django `flake8` clean +
  `pytest` 8 passed; Express `eslint`/`tsc`/`build`/`jest` 6 passed; Angular `lint` + dev
  `build` succeed; RN `eslint`/`tsc`/`jest` 31 passed + `expo export -p web` succeeds.

### Honest limitations (not tested here)

- **Real mobile push delivery (APNs/FCM/EAS, physical device):** NOT tested. The server-side
  send path is proven against the fake Expo API only. `app.json` has **no** `extra.eas.projectId`,
  so production push requires `eas init` + APNs/FCM credentials (see §7).
- **Mobile AI image search via the web build:** the upload code uses the React Native–native
  `FormData` shape `{uri,name,type}` (`src/services/ai/ai.service.ts:17`), which the browser
  cannot serialize, so the **web** build returns HTTP 400 from Django. The endpoint itself is
  proven correct (64/64). Native-device upload uses this pattern correctly; web upload is a
  known web-only limitation.
- **Mobile branch distance:** the "Sucursales" screen waits on `expo-location` geolocation,
  which headless Expo Web does not provide, so it stays in a loading state. Branch + per-branch
  stock data is fully proven elsewhere.
- **n8n / invoice webhook:** implemented and unit-tested (HMAC-SHA256 signing), but the
  dispatcher is **config-gated** and not started in the full stack (no `WEBHOOK_INVOICE_URL`
  configured), so live delivery was not exercised. Go logs confirm: `webhook dispatcher not
  started (URL or secret missing)`.

---

## 2. How variables were discovered

For each repo the following were read directly:

- **Go** — `internal/config/config.go` (`os.Getenv` / `getEnv(key, fallback)` loader, uses
  `godotenv`), `cmd/migrate/main.go`, `cmd/fake-expo-push/main.go`, `Dockerfile`,
  `docker-compose.yml`, `docker-compose.full.yml`, `Makefile`.
- **Django** — `config/settings/base.py` / `dev.py` / `prod.py` (`os.getenv` + `FICCT_AI`
  dict), `apps/common/auth/jwt_authentication.py`, `apps/common/dynamodb/client.py`,
  `Dockerfile`, `docker-compose.yml`.
- **Express** — `src/config/index.ts` (Zod schema; fails fast on missing/invalid),
  `src/modules/storage/s3.client.ts`, `presign.service.ts`, `src/middleware/auth.ts`,
  `Dockerfile`, `docker-compose.yml`.
- **Angular** — `src/environments/environment.ts` and `environment.prod.ts`, `angular.json`
  (`fileReplacements`), `Dockerfile`, `nginx.conf`.
- **Expo** — `app.json` (`expo.extra`, `plugins`), `src/config/env.ts` (`EXPO_PUBLIC_*` +
  `Constants.expoConfig.extra`), `src/services/notifications/notifications.service.ts`,
  `Dockerfile.web`, `nginx.conf`.

**Gaps found and fixed in this delivery** (code read a variable that `.env.example` did not
declare — `.env.example` was updated and the variable is documented below):

| Repo | Variable | Read at | Action |
|------|----------|---------|--------|
| Go | `MIGRATIONS_DIR` | `cmd/migrate/main.go:43` | Added to `.env.example` |
| Django | `SQLITE_PATH` | `config/settings/base.py:68` | Added to `.env.example` |
| Express | `S3_PUBLIC_ENDPOINT` | `src/config/index.ts:22` | Added to `.env.example` (commented/optional) |

**No CI/CD files exist in any repo** (no `.github/workflows`, no `.gitlab-ci.yml`, no
`eas.json`). Per the contract, this guide therefore provides a **secrets checklist and
deployment mapping only** — it does not invent a full pipeline. Where a GitHub Actions step is
referenced it is illustrative.

### Convention used in this guide

- **Secret** = sensitive, store as a **GitHub Actions Secret** (encrypted, masked in logs).
- **Variable** = non-sensitive build/runtime config, store as a **GitHub Actions Variable**
  (plain, visible). Frontend public URLs are Variables, not Secrets.
- GitHub Secret/Variable names are **prefixed by repo** (`GO_`, `DJANGO_`, `EXPRESS_`,
  `ANGULAR_`, `EXPO_`) so a single GitHub org/repo can host multiple services without
  collisions.

---

## 3. Repository 1: Go Core Backend (`ficct-boutique-backend-go`)

Config loader: `internal/config/config.go` (`godotenv.Load()` then `os.Getenv`). Only
`DATABASE_URL` is **hard-required** (the process exits if it is empty); every other variable
has a safe default. The webhook dispatcher only starts if **both** `WEBHOOK_INVOICE_URL` and
`WEBHOOK_HMAC_SECRET` are set.

### Variables table

| Variable | Repo | Required | Secret | Local value example | Production value | Where used | GitHub Secret/Var name | How to obtain |
|----------|------|----------|--------|---------------------|------------------|-----------|------------------------|---------------|
| `DATABASE_URL` | Go | **Yes** | **Yes** | `postgres://ficct:ficct@go-postgres:5432/ficct_boutique?sslmode=disable` | Railway Postgres URL (`?sslmode=require`) | `config.go:69` (panics if empty); `cmd/migrate`, `cmd/seed` | `GO_DATABASE_URL` (Secret) | Railway → Postgres plugin → "Connect" → connection string |
| `APP_ENV` | Go | No | No | `development` | `production` | `config.go:75` | `GO_APP_ENV` (Var) | Static (`production`) |
| `APP_PORT` | Go | No | No | `8080` | platform-injected `PORT` (map it) | `config.go:76` | `GO_APP_PORT` (Var) | Railway injects `PORT`; set `APP_PORT=$PORT` |
| `APP_LOG_LEVEL` | Go | No | No | `debug` | `info` | `config.go:77` | `GO_APP_LOG_LEVEL` (Var) | Static |
| `JWT_PRIVATE_KEY_PATH` | Go | No | No (path) | `/app/.tools/keys/jwt_private_dev.pem` | `/secrets/jwt_private.pem` (mounted) | `config.go:79`, `cmd/server/main.go` | `GO_JWT_PRIVATE_KEY_PATH` (Var) | Path where the PEM Secret is mounted at runtime |
| `JWT_PUBLIC_KEY_PATH` | Go | No | No (path) | `/app/.tools/keys/jwt_public_dev.pem` | `/secrets/jwt_public.pem` | `config.go:80` | `GO_JWT_PUBLIC_KEY_PATH` (Var) | Path where the PEM Secret is mounted |
| *(content)* `JWT_PRIVATE_KEY_PEM` | Go | **Yes (prod)** | **Yes** | dev PEM in `.tools/keys` | RSA private key PEM | signs JWTs (`internal/auth/keys.go`) | `GO_JWT_PRIVATE_KEY_PEM` (Secret) | Generate with OpenSSL (see §3 generation) |
| *(content)* `JWT_PUBLIC_KEY_PEM` | Go | **Yes (prod)** | No* | dev PEM | RSA public key PEM | verifies JWTs; shared with MS2/MS3 | `GO_JWT_PUBLIC_KEY_PEM` (Secret) | Derived from the private key |
| `JWT_ISSUER` | Go | No | No | `ficct-go` | `ficct-go` | `config.go:81` | `GO_JWT_ISSUER` (Var) | Static; must match MS2/MS3 verifiers |
| `JWT_AUDIENCE` | Go | No | No | `ficct-express,ficct-django,ficct-angular,ficct-mobile` | same | `config.go:82` (CSV) | `GO_JWT_AUDIENCE` (Var) | Static; **superset** of each consumer's expected `aud` |
| `JWT_KEY_ID` | Go | No | No | `dev-1` | `prod-1` | `config.go:83` (sets `kid` header) | `GO_JWT_KEY_ID` (Var) | Choose a key-rotation id |
| `JWT_ACCESS_TTL_MINUTES` | Go | No | No | `60` | `60` | `config.go:44` | `GO_JWT_ACCESS_TTL_MINUTES` (Var) | Static |
| `JWT_REFRESH_TTL_DAYS` | Go | No | No | `7` | `7` | `config.go:48` | `GO_JWT_REFRESH_TTL_DAYS` (Var) | Static |
| `CORS_ALLOWED_ORIGINS` | Go | No | No | `http://localhost:4200,http://localhost:19006,http://localhost:8081` | `https://admin.ficct.example,https://app.ficct.example` | `config.go:86`, `cmd/server/main.go` | `GO_CORS_ALLOWED_ORIGINS` (Var) | Production Angular + Expo origins |
| `WEBHOOK_INVOICE_URL` | Go | No | No | `http://host.docker.internal:5678/webhook/ficct-invoice` | n8n webhook URL | `config.go:87` | `GO_WEBHOOK_INVOICE_URL` (Var/Secret) | From your n8n instance (Webhook node "Production URL") |
| `WEBHOOK_HMAC_SECRET` | Go | No (req. to enable webhooks) | **Yes** | `replace-with-strong-random-256-bit-secret` | 256-bit random | `config.go:88`; dispatcher signs `X-FICCT-Signature` | `GO_WEBHOOK_HMAC_SECRET` (Secret) | `openssl rand -hex 32` |
| `WEBHOOK_DISPATCH_INTERVAL_SECONDS` | Go | No | No | `5` | `5` | `config.go:52` | `GO_WEBHOOK_DISPATCH_INTERVAL_SECONDS` (Var) | Static |
| `WEBHOOK_MAX_RETRIES` | Go | No | No | `5` | `5` | `config.go:56` | `GO_WEBHOOK_MAX_RETRIES` (Var) | Static |
| `RATE_LIMIT_RPS` | Go | No | No | `20` | tune | `config.go:60` | `GO_RATE_LIMIT_RPS` (Var) | Static |
| `RATE_LIMIT_BURST` | Go | No | No | `40` | tune | `config.go:64` | `GO_RATE_LIMIT_BURST` (Var) | Static |
| `EXPO_PUSH_API_URL` | Go | No | No | `http://fake-expo-push:8080/--/api/v2/push/send` (full stack) | `https://exp.host/--/api/v2/push/send` (default) | `config.go:93`, push sender | `GO_EXPO_PUSH_API_URL` (Var) | Leave unset in prod to use Expo default |
| `EXPO_PUSH_ACCESS_TOKEN` | Go | No | **Yes** | *(empty)* | Expo access token | `config.go:94`, push sender | `GO_EXPO_PUSH_ACCESS_TOKEN` (Secret) | Only if Expo "Enhanced Push Security" is on (expo.dev → Access Tokens) |
| `MIGRATIONS_DIR` | Go | No | No | `migrations` | `migrations` | `cmd/migrate/main.go:43` | `GO_MIGRATIONS_DIR` (Var) | Static (added to `.env.example` this delivery) |

> `*` `JWT_PUBLIC_KEY_PEM` is not strictly secret (public key), but storing it as a Secret keeps
> it paired with the private key and avoids accidental drift. The **dev** keys committed in
> `.tools/keys/` (`jwt_private_dev.pem`, `jwt_public_dev.pem`) are **development-only** — never
> use them in production.

> **Fake Expo Push test double** (`cmd/fake-expo-push`) reads `FAKE_EXPO_MODE` (default
> `per-token`) and `PORT` (default `8080`). These belong to a **test container** only, are not
> part of the production Go service, and are intentionally not in the main `.env.example`.

### How to generate each secret/value

- **`GO_DATABASE_URL`** — Provision Postgres (Railway plugin or managed PG). Copy the
  connection string; append `?sslmode=require` for managed providers.
- **JWT RS256 keypair** (shared across MS1/MS2/MS3):
  ```bash
  openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out jwt_private.pem
  openssl rsa -in jwt_private.pem -pubout -out jwt_public.pem
  ```
  Store `jwt_private.pem` content as `GO_JWT_PRIVATE_KEY_PEM`, `jwt_public.pem` as
  `GO_JWT_PUBLIC_KEY_PEM` / `DJANGO_JWT_PUBLIC_KEY_PEM` / `EXPRESS_JWT_PUBLIC_KEY_PEM`.
- **`GO_WEBHOOK_HMAC_SECRET`** — `openssl rand -hex 32` (256-bit). The same secret must be
  configured in the n8n receiver to validate `X-FICCT-Signature` (HMAC-SHA256).
- **`GO_EXPO_PUSH_ACCESS_TOKEN`** — expo.dev → account/project → Access Tokens (only if
  Enhanced Push Security is enabled).

### GitHub Secrets to create (Go)

Secrets: `GO_DATABASE_URL`, `GO_JWT_PRIVATE_KEY_PEM`, `GO_JWT_PUBLIC_KEY_PEM`,
`GO_WEBHOOK_HMAC_SECRET`, `GO_EXPO_PUSH_ACCESS_TOKEN` (only if used).
Variables: `GO_APP_ENV`, `GO_APP_LOG_LEVEL`, `GO_JWT_ISSUER`, `GO_JWT_AUDIENCE`,
`GO_JWT_KEY_ID`, `GO_CORS_ALLOWED_ORIGINS`, `GO_WEBHOOK_INVOICE_URL`, rate-limit values,
`GO_EXPO_PUSH_API_URL` (optional).

### Railway deployment mapping

- Create a Railway **Postgres** plugin → its `DATABASE_URL` becomes `GO_DATABASE_URL`.
- Railway injects `PORT`; the server binds `APP_PORT` → set service env `APP_PORT=${{PORT}}`
  (or set `APP_PORT=8080` and expose 8080).
- Provide the RSA private/public PEMs as Railway variables:
  `JWT_PRIVATE_KEY_PEM` and `JWT_PUBLIC_KEY_PEM` (or `GO_JWT_PRIVATE_KEY_PEM` /
  `GO_JWT_PUBLIC_KEY_PEM`). Railway cannot read local paths such as
  `D:\Repositories\_deployment_secrets`; paste the file contents into Railway variables or use a
  Railway-supported secret-file mount and point `JWT_PRIVATE_KEY_PATH` / `JWT_PUBLIC_KEY_PATH` at it.
  Do **not** bake production keys into the image.
- `WEBHOOK_INVOICE_URL` must be the n8n production webhook URL for deployed workflows. The n8n
  Cloud test URL `https://victoroide.app.n8n.cloud/webhook-test/ficct-invoice` is only for manual
  test executions while the n8n test listener is active; production should normally use
  `/webhook/ficct-invoice`.
- Run order on deploy: `migrate up && seed && server` (already enforced by the container entrypoint; `seed`
  is idempotent — consider dropping `seed` for production after first run).
- Health check path: `/health`.

### Verification steps (Go)

```bash
curl -fsS https://<go-host>/health
# GraphQL login:
curl -s -XPOST https://<go-host>/graphql -H 'content-type: application/json' \
  -d '{"query":"mutation($i:LoginInput!){login(input:$i){accessToken user{role}}}","variables":{"i":{"email":"<admin-email>","password":"<admin-password>"}}}'
```

---

## 4. Repository 2: Django AI Backend (`ficct-boutique-backend-python`)

Config: `config/settings/base.py` reads env via `os.getenv` into a `FICCT_AI` settings dict.
`DJANGO_SETTINGS_MODULE` and `SECRET_KEY` are effectively required for production; JWT public
key must be present for token verification. Business data lives in **DynamoDB** (4 tables,
prefix `ficct_`); the SQLite DB is only for Django's built-in auth/migration tables.

### Variables table

| Variable | Repo | Required | Secret | Local value example | Production value | Where used | GitHub Secret/Var name | How to obtain |
|----------|------|----------|--------|---------------------|------------------|-----------|------------------------|---------------|
| `DJANGO_SETTINGS_MODULE` | Django | **Yes** | No | `config.settings.dev` | `config.settings.prod` | `manage.py`, `wsgi.py`, `asgi.py` | `DJANGO_SETTINGS_MODULE` (Var) | Static |
| `SECRET_KEY` | Django | **Yes (prod)** | **Yes** | `dev-secret-change-me` | 50-char random | `base.py:7` | `DJANGO_SECRET_KEY` (Secret) | `python -c "import secrets;print(secrets.token_urlsafe(50))"` |
| `DEBUG` | Django | No | No | `True` | `False` | `base.py:8` | `DJANGO_DEBUG` (Var) | Static `False` in prod |
| `ALLOWED_HOSTS` | Django | No | No | `localhost,127.0.0.1` | `ai.ficct.example` | `base.py:9`, `prod.py` | `DJANGO_ALLOWED_HOSTS` (Var) | Cloud Run service host |
| `LOG_LEVEL` | Django | No | No | `INFO` | `INFO` | `base.py:120` | `DJANGO_LOG_LEVEL` (Var) | Static |
| `JWT_PUBLIC_KEY_PATH` | Django | **Yes** | No (path) | `/app/.tools/keys/jwt_public_dev.pem` | `/secrets/jwt_public.pem` | `base.py:125`, `jwt_authentication.py:37` | `DJANGO_JWT_PUBLIC_KEY_PATH` (Var) | Path where the public PEM is mounted |
| *(content)* `JWT_PUBLIC_KEY_PEM` | Django | **Yes** | No* | dev public PEM | RSA public PEM (same as Go) | verify-only | `DJANGO_JWT_PUBLIC_KEY_PEM` (Secret) | From the Go keypair (§3) |
| `JWT_ISSUER` | Django | No | No | `ficct-go` | `ficct-go` | `base.py:128` | `DJANGO_JWT_ISSUER` (Var) | Must equal Go's `JWT_ISSUER` |
| `JWT_AUDIENCE` | Django | No | No | `ficct-django` | `ficct-django` | `base.py:129` | `DJANGO_JWT_AUDIENCE` (Var) | Must be in Go's `JWT_AUDIENCE` CSV |
| `DYNAMODB_ENDPOINT` | Django | No | No | `http://dynamodb:8000` | *(empty → real AWS regional endpoint)* | `base.py:130`, `dynamodb/client.py:17` | `DJANGO_DYNAMODB_ENDPOINT` (Var) | Unset in prod to use AWS; set for DynamoDB Local |
| `DYNAMODB_REGION` | Django | No | No | `us-east-1` | e.g. `us-east-1` | `client.py:18` | `DJANGO_DYNAMODB_REGION` (Var) | AWS region of the tables |
| `DYNAMODB_ACCESS_KEY_ID` | Django | No (prod: yes) | **Yes** | `local` | AWS access key id | `client.py:19` | `DJANGO_DYNAMODB_ACCESS_KEY_ID` (Secret) | AWS IAM (or Workload Identity on GCP → STS) |
| `DYNAMODB_SECRET_ACCESS_KEY` | Django | No (prod: yes) | **Yes** | `local` | AWS secret key | `client.py:20` | `DJANGO_DYNAMODB_SECRET_ACCESS_KEY` (Secret) | AWS IAM |
| `DYNAMODB_TABLE_PREFIX` | Django | No | No | `ficct_` | `ficct_` | `client.py:26` | `DJANGO_DYNAMODB_TABLE_PREFIX` (Var) | Static |
| `CORS_ALLOWED_ORIGINS` | Django | No | No | `http://localhost:4200,http://localhost:19006,exp://localhost:19000` | prod origins | `base.py:104` | `DJANGO_CORS_ALLOWED_ORIGINS` (Var) | Angular + Expo origins (dev settings allow all) |
| `GO_CORE_BASE_URL` | Django | No | No | `http://host.docker.internal:8080` | `https://<go-host>` | `base.py:135` (`FICCT_AI`) | `DJANGO_GO_CORE_BASE_URL` (Var) | Production Go core URL |
| `GO_CORE_TIMEOUT_SECONDS` | Django | No | No | `30` | `30` | `base.py:136`, `catalog_sync_service.py:53` | `DJANGO_GO_CORE_TIMEOUT_SECONDS` (Var) | Static |
| `SQLITE_PATH` | Django | No | No | `/tmp/ai_service.sqlite3` | writable volume path | `base.py:68` | `DJANGO_SQLITE_PATH` (Var) | Added to `.env.example` this delivery; point at a writable path |
| `PORT` | Django | No | No | `8000` | platform-injected | `Dockerfile`/compose (not read by Django code) | `DJANGO_PORT` (Var) | gunicorn/runserver bind arg |

> `*` Public key — kept as a Secret only to pair it with the private key. The four DynamoDB
> tables (created idempotently by `python manage.py ensure_tables`) are
> `ficct_product_embeddings`, `ficct_forecast_results`, `ficct_customer_segments`,
> `ficct_cluster_runs` (PAY_PER_REQUEST). In the full Docker stack DynamoDB Local runs
> **in-memory**, so AI data is ephemeral across restarts.

### How to generate each secret/value

- **`DJANGO_SECRET_KEY`** — `python -c "import secrets; print(secrets.token_urlsafe(50))"`.
- **`DJANGO_JWT_PUBLIC_KEY_PEM`** — the `jwt_public.pem` from §3 (identical to Go's public key).
- **DynamoDB credentials** — AWS IAM user/role with least-privilege on the four `ficct_*`
  tables (`dynamodb:PutItem/GetItem/Scan/Query/CreateTable/DescribeTable`). On GCP Cloud Run,
  prefer storing the AWS keys in **Secret Manager** and injecting them as env vars.

### GitHub Secrets to create (Django)

Secrets: `DJANGO_SECRET_KEY`, `DJANGO_JWT_PUBLIC_KEY_PEM`, `DJANGO_DYNAMODB_ACCESS_KEY_ID`,
`DJANGO_DYNAMODB_SECRET_ACCESS_KEY`.
Variables: `DJANGO_SETTINGS_MODULE`, `DJANGO_DEBUG`, `DJANGO_ALLOWED_HOSTS`,
`DJANGO_JWT_ISSUER`, `DJANGO_JWT_AUDIENCE`, `DJANGO_DYNAMODB_REGION`,
`DJANGO_DYNAMODB_TABLE_PREFIX`, `DJANGO_GO_CORE_BASE_URL`, `DJANGO_CORS_ALLOWED_ORIGINS`,
`DJANGO_SQLITE_PATH`, `DJANGO_PORT`.

### GCP (Cloud Run) deployment mapping

- Build the image (`Dockerfile`, `REQUIREMENTS=prod.txt`), push to Artifact Registry, deploy
  to Cloud Run. Cloud Run injects `PORT` — the container already binds `0.0.0.0:8000`
  (use gunicorn `--bind 0.0.0.0:$PORT` in prod).
- Set `DJANGO_SETTINGS_MODULE=config.settings.prod`, `DEBUG=False`, real `ALLOWED_HOSTS`.
- Leave `DYNAMODB_ENDPOINT` **unset** so boto3 uses the real AWS endpoint for the region; mount
  AWS keys from Secret Manager.
- Mount the JWT public PEM (Secret Manager → file or env → write to `JWT_PUBLIC_KEY_PATH`).
- Run `python manage.py ensure_tables` as a startup/job step against the real DynamoDB.
- Health check path: `/api/v1/health/`.

### Verification steps (Django)

```bash
curl -fsS https://<django-host>/api/v1/health/
# Forecast (requires a Go-issued Bearer token whose aud includes ficct-django):
curl -s -XPOST https://<django-host>/api/v1/forecasting/run/ -H "Authorization: Bearer $TOKEN" \
  -H 'content-type: application/json' -d '{"scope":"smoke","series":[10,12,14,13,15],"horizon":3}'
```

---

## 5. Repository 3: Express Document Backend (`ficct-boutique-backend-express`)

Config: `src/config/index.ts` validates all env with a **Zod** schema and **fails fast** if a
required variable is missing or malformed. The S3 bucket must be **private**; downloads are
served only via short-lived presigned URLs. Two S3 clients exist — one for server-side calls
(`S3_ENDPOINT`) and one for presign URL host (`S3_PUBLIC_ENDPOINT`).

### Variables table

| Variable | Repo | Required | Secret | Local value example | Production value | Where used | GitHub Secret/Var name | How to obtain |
|----------|------|----------|--------|---------------------|------------------|-----------|------------------------|---------------|
| `DATABASE_URL` | Express | **Yes** | **Yes** | `postgres://ficct:ficct@docs-postgres:5432/ficct_documents` | managed PG URL | `config/index.ts:9`, `database/pool.ts` | `EXPRESS_DATABASE_URL` (Secret) | RDS/managed Postgres connection string |
| `NODE_ENV` | Express | No | No | `development` | `production` | `config/index.ts:5` | `EXPRESS_NODE_ENV` (Var) | Static |
| `PORT` | Express | No | No | `8081` | platform-injected | `config/index.ts:6`, `server.ts` | `EXPRESS_PORT` (Var) | Map to platform `PORT` |
| `LOG_LEVEL` | Express | No | No | `debug` | `info` | `config/index.ts:7` | `EXPRESS_LOG_LEVEL` (Var) | Static |
| `JWT_PUBLIC_KEY_PATH` | Express | **Yes** | No (path) | `/app/.tools/keys/jwt_public_dev.pem` | `/secrets/jwt_public.pem` | `config/index.ts:11`, `middleware/auth.ts:22` | `EXPRESS_JWT_PUBLIC_KEY_PATH` (Var) | Path where public PEM is mounted |
| *(content)* `JWT_PUBLIC_KEY_PEM` | Express | **Yes** | No* | dev public PEM | RSA public PEM (same as Go) | verify-only | `EXPRESS_JWT_PUBLIC_KEY_PEM` (Secret) | From the Go keypair (§3) |
| `JWT_ISSUER` | Express | No | No | `ficct-go` | `ficct-go` | `config/index.ts:12`, `auth.ts:26` | `EXPRESS_JWT_ISSUER` (Var) | Must equal Go's `JWT_ISSUER` |
| `JWT_AUDIENCE` | Express | No | No | `ficct-express` | `ficct-express` | `config/index.ts:13`, `auth.ts:27` | `EXPRESS_JWT_AUDIENCE` (Var) | Must be in Go's `JWT_AUDIENCE` CSV |
| `JWT_KEY_ID` | Express | No | No | `dev-1` | `prod-1` | `config/index.ts:14` | `EXPRESS_JWT_KEY_ID` (Var) | **Currently documentation-only** (verifier does not yet check `kid`) |
| `CORS_ALLOWED_ORIGINS` | Express | No | No | `http://localhost:4200,http://localhost:19006` | prod origins | `config/index.ts:16`, `app.ts:35` | `EXPRESS_CORS_ALLOWED_ORIGINS` (Var) | Angular + Expo origins |
| `S3_ENDPOINT` | Express | **Yes** | No | `http://minio:9000` | *(empty → real AWS S3)* | `config/index.ts:18`, `s3.client.ts` | `EXPRESS_S3_ENDPOINT` (Var) | Unset in prod for AWS S3; set for MinIO |
| `S3_PUBLIC_ENDPOINT` | Express | No (optional) | No | `http://localhost:9010` | usually unset (AWS) | `config/index.ts:22`; presign | `EXPRESS_S3_PUBLIC_ENDPOINT` (Var) | Set only when server/browser see different hosts. Added to `.env.example` this delivery |
| `S3_REGION` | Express | No | No | `us-east-1` | bucket region | `config/index.ts:23`, `s3.client.ts` | `EXPRESS_S3_REGION` (Var) | AWS region |
| `S3_FORCE_PATH_STYLE` | Express | No | No | `true` | `false` (AWS virtual-hosted) or `true` | `config/index.ts:24` | `EXPRESS_S3_FORCE_PATH_STYLE` (Var) | `true` for MinIO; `false`/`true` for AWS |
| `S3_BUCKET` | Express | **Yes** | No | `ficct-documents` | prod bucket name | `config/index.ts:25`, `presign.service.ts` | `EXPRESS_S3_BUCKET` (Var) | The private bucket name |
| `S3_ACCESS_KEY_ID` | Express | **Yes** | **Yes** | `minio-access` | AWS access key id | `config/index.ts:26`, `s3.client.ts` | `EXPRESS_S3_ACCESS_KEY_ID` (Secret) | AWS IAM (S3 least-privilege) |
| `S3_SECRET_ACCESS_KEY` | Express | **Yes** | **Yes** | `minio-secret-change-me` | AWS secret key | `config/index.ts:27`, `s3.client.ts` | `EXPRESS_S3_SECRET_ACCESS_KEY` (Secret) | AWS IAM |
| `S3_PRESIGN_EXPIRY_SECONDS` | Express | No | No | `900` | `900` | `config/index.ts:28`, presign | `EXPRESS_S3_PRESIGN_EXPIRY_SECONDS` (Var) | Static (15 min) |
| `S3_SERVER_SIDE_ENCRYPTION` | Express | No | No | `false` | `true` (bucket has SSE) | `config/index.ts:31`, presign | `EXPRESS_S3_SERVER_SIDE_ENCRYPTION` (Var) | `false` for local MinIO; `true` with AWS SSE |
| `RATE_LIMIT_WINDOW_MS` | Express | No | No | `60000` | `60000` | `config/index.ts:33`, `app.ts:43` | `EXPRESS_RATE_LIMIT_WINDOW_MS` (Var) | Static |
| `RATE_LIMIT_MAX` | Express | No | No | `120` | tune | `config/index.ts:34`, `app.ts:44` | `EXPRESS_RATE_LIMIT_MAX` (Var) | Static |

> `*` Public key — kept as a Secret only to pair it with the private key. `JWT_KEY_ID` is read
> into config but the verifier does not yet select keys by `kid` — keep it for forward
> compatibility but it has no runtime effect today.

### How to generate each secret/value

- **`EXPRESS_DATABASE_URL`** — provision Postgres (e.g. AWS RDS); the document schema is
  created by the bundled migrations on startup (`node dist/database/migrate.js`).
- **`EXPRESS_S3_BUCKET` + keys** — create a **private** S3 bucket (Block Public Access ON),
  optionally enable default SSE (SSE-S3/SSE-KMS) and then set `S3_SERVER_SIDE_ENCRYPTION=true`.
  Create an IAM user/role limited to `s3:PutObject/GetObject/HeadObject` on that bucket.
- **`EXPRESS_JWT_PUBLIC_KEY_PEM`** — the shared `jwt_public.pem` from §3.

### GitHub Secrets to create (Express)

Secrets: `EXPRESS_DATABASE_URL`, `EXPRESS_JWT_PUBLIC_KEY_PEM`, `EXPRESS_S3_ACCESS_KEY_ID`,
`EXPRESS_S3_SECRET_ACCESS_KEY`.
Variables: `EXPRESS_NODE_ENV`, `EXPRESS_LOG_LEVEL`, `EXPRESS_JWT_ISSUER`,
`EXPRESS_JWT_AUDIENCE`, `EXPRESS_CORS_ALLOWED_ORIGINS`, `EXPRESS_S3_ENDPOINT` (unset for AWS),
`EXPRESS_S3_REGION`, `EXPRESS_S3_FORCE_PATH_STYLE`, `EXPRESS_S3_BUCKET`,
`EXPRESS_S3_PRESIGN_EXPIRY_SECONDS`, `EXPRESS_S3_SERVER_SIDE_ENCRYPTION`, rate-limit values.

### AWS deployment mapping

- Build the multi-stage `Dockerfile` (node:20-alpine, non-root, tini), push to **ECR**, run on
  ECS/Fargate (or App Runner). The container runs `migrate` then `node dist/server.js`.
- Use AWS S3 (not MinIO): leave `S3_ENDPOINT` unset, set `S3_BUCKET`/`S3_REGION`, prefer an
  **IAM role on the task** over static keys where possible (otherwise the two key Secrets).
- Store `DATABASE_URL` and S3 keys in **AWS Secrets Manager**; inject as env at task launch.
- Mount the JWT public PEM; point `JWT_PUBLIC_KEY_PATH` at it.
- Health check path: `/health`.

### Verification steps (Express)

```bash
curl -fsS https://<express-host>/health
curl -s https://<express-host>/api/v1/documents -H "Authorization: Bearer $TOKEN"   # 200 with admin/staff token
```

---

## 6. Repository 4: Angular Admin Frontend (`ficct-boutique-frontend-angular`)

**Important:** this app reads **no runtime environment variables**. All three backend URLs are
**baked at build time** into `src/environments/environment.ts` (development) and
`environment.prod.ts` (production). `angular.json` `fileReplacements` swaps to
`environment.prod.ts` for production builds. The repo's `.env.example` lists `GRAPHQL_URL`,
`DOCUMENTS_API_URL`, `AI_API_URL` purely as **developer hints**; they are not consumed at
runtime.

### Build/runtime config table

| Config key | Repo | Required | Secret | Dev value (`environment.ts`) | Prod value (`environment.prod.ts`) | Where used | GitHub Var name | How to obtain |
|------------|------|----------|--------|------------------------------|------------------------------------|-----------|-----------------|---------------|
| `graphqlUrl` | Angular | **Yes** | No | `http://localhost:8093/graphql` | `/api/graphql` | `core/graphql/apollo.config.ts` | `ANGULAR_GRAPHQL_URL` (Var) | Go core public URL (or reverse-proxy path) |
| `documentsApiUrl` | Angular | **Yes** | No | `http://localhost:8091/api/v1` | `/api/documents` | `features/documents/*`, `shared/services/document-display.service.ts` | `ANGULAR_DOCUMENTS_API_URL` (Var) | Express public URL (or proxy path) |
| `aiApiUrl` | Angular | **Yes** | No | `http://localhost:8092/api/v1` | `/api/ai` | `features/ai-analytics/ai-analytics.component.ts` | `ANGULAR_AI_API_URL` (Var) | Django public URL (or proxy path) |

> Production defaults are **relative paths** (`/api/graphql`, `/api/documents`, `/api/ai`),
> which assume a reverse proxy / API gateway in front of the SPA routes them to the three
> backends. There are **no secrets** in this app; bearer tokens are obtained at login and held
> in `localStorage`. Auth is enforced by route guards (`authGuard`, `roleGuard(['admin'])` on
> product create/edit and audit; `roleGuard(['admin','staff'])` on AI analytics) **and**
> server-side RBAC.

### GitHub Secrets or Variables to create (Angular)

Use **Variables** (these are public URLs, not secrets): `ANGULAR_GRAPHQL_URL`,
`ANGULAR_DOCUMENTS_API_URL`, `ANGULAR_AI_API_URL`. A CI build would write `environment.prod.ts`
from these (or rely on the reverse-proxy relative paths and not inject at all).

### Static hosting deployment mapping

- Build: `npm ci && npm run build` (production config; uses `environment.prod.ts`). Output:
  `dist/ficct-admin/browser`.
- The provided `Dockerfile` builds with `build:dev` and serves via `nginx:alpine`
  (`nginx.conf` has SPA fallback `try_files … /index.html` + security headers). For production
  either (a) build with `--configuration production` and front the relative `/api/*` paths with
  a reverse proxy that targets the three backends, or (b) edit `environment.prod.ts` to
  absolute backend URLs before building.
- Host the static bundle on any static host (S3+CloudFront, Netlify, Nginx, Cloud Storage).
- Ensure each backend's `CORS_ALLOWED_ORIGINS` includes the Angular production origin.

### Verification steps (Angular)

- Load the site → redirected to `/login`; log in as `<admin-email>` → dashboard charts
  render. Confirm Network tab shows GraphQL `200`s and document/AI calls succeed (CORS OK).
- DevTools console must be error-free; verify responsive layout at 1440/1024/768/430/390/360.

---

## 7. Repository 5: React Native / Expo Mobile App (`ficct-boutique-mobile-react-native`)

Config resolution (`src/config/env.ts`): `process.env.EXPO_PUBLIC_*` (highest) →
`app.json` `expo.extra` → hardcoded Android-emulator defaults (`10.0.2.2`). `EXPO_PUBLIC_*`
values are inlined into the JS bundle **at build/export time** (public, not secret). The web
build (`Dockerfile.web`) bakes `EXPO_PUBLIC_GRAPHQL_URL=/graphql` and
`EXPO_PUBLIC_AI_API_URL=/api/ai/api/v1` and serves via nginx, which reverse-proxies `/graphql`
to the Go core and `/api/ai/` to Django.

### Expo/public config table

| Variable | Repo | Required | Secret | Local value example | Production value | Where used | GitHub Var/Secret name | How to obtain |
|----------|------|----------|--------|---------------------|------------------|-----------|------------------------|---------------|
| `EXPO_PUBLIC_GRAPHQL_URL` | Expo | No (has default) | No | `http://localhost:8080/graphql` (`.env.example`); `/graphql` (web build) | `https://<go-host>/graphql` | `src/config/env.ts:12`, `services/graphql/client.ts` | `EXPO_PUBLIC_GRAPHQL_URL` (Var) | Go core public URL |
| `EXPO_PUBLIC_AI_API_URL` | Expo | No (has default) | No | `http://localhost:8000/api/v1`; `/api/ai/api/v1` (web build) | `https://<django-host>/api/v1` | `src/config/env.ts:13`, `services/ai/ai.service.ts` | `EXPO_PUBLIC_AI_API_URL` (Var) | Django public URL |
| `EXPO_NO_DOTENV` | Expo | No | No | `1` (docker-compose) | — | Metro/Docker only (not app code) | `EXPO_NO_DOTENV` (Var) | Set in containerized dev to ignore `.env` |
| `extra.eas.projectId` (a.k.a. `EXPO_PROJECT_ID`) | Expo | **Yes (push)** | No | **NOT set** in `app.json` | EAS project id (UUID) | `notifications.service.ts:84` (`Constants.expoConfig.extra.eas.projectId`) | `EXPO_PROJECT_ID` (Var) | `eas init` writes it into `app.json`; or copy from expo.dev project settings |
| `EAS_TOKEN` (`EXPO_TOKEN`) | Expo | Only if CI builds via EAS | **Yes** | — | EAS access token | EAS CLI auth in CI (`eas build`) | `EAS_TOKEN` (Secret) | expo.dev → Account → Access Tokens |
| `EXPO_PUSH_ACCESS_TOKEN` | Expo | Only if Enhanced Push Security | **Yes** | — | Expo push token | *(server-side; see Go `GO_EXPO_PUSH_ACCESS_TOKEN`)* | `EXPO_PUSH_ACCESS_TOKEN` (Secret) | expo.dev → Access Tokens. **Only if actually used** |

> There are **no secrets baked into the mobile bundle** — backend credentials are issued by the
> Go core at login and stored via `expo-secure-store` (native) / `AsyncStorage` (web). Push
> token registration **refuses to fake a token** and requires a physical device (native) or a
> registered service worker + VAPID (web) plus an EAS `projectId`. Since `app.json` has **no**
> `extra.eas.projectId`, push token acquisition currently cannot complete — run `eas init`
> first (see below).

### GitHub Secrets or Variables to create (Expo)

Variables (public, build-time): `EXPO_PUBLIC_GRAPHQL_URL`, `EXPO_PUBLIC_AI_API_URL`,
`EXPO_PROJECT_ID`.
Secrets (only if CI runs EAS / Enhanced Push Security): `EAS_TOKEN`, `EXPO_PUSH_ACCESS_TOKEN`.

### EAS / APNs / FCM setup (required for real push — currently pending)

1. `npm i -g eas-cli` (or use `npx eas-cli`), then `eas login`.
2. `eas init` — creates the EAS project and writes `extra.eas.projectId` into `app.json`
   (this is what `notifications.service.ts` reads).
3. **iOS / APNs:** `eas credentials` (or automatic during `eas build -p ios`) to create/upload
   the APNs key. `app.json` already declares `ios.bundleIdentifier = bo.edu.ficct.boutique` and
   `UIBackgroundModes: ["remote-notification"]`.
4. **Android / FCM:** create a Firebase project, download `google-services.json`, and provide
   the FCM credential to EAS (`eas credentials`). `app.json` declares
   `android.package = bo.edu.ficct.boutique` and the `NOTIFICATIONS` permission.
5. Build: `eas build -p ios|android` (uses `EAS_TOKEN` in CI). Submit with `eas submit`.
6. The server sends through the Go core (`EXPO_PUSH_API_URL` default `https://exp.host/...`);
   set `GO_EXPO_PUSH_ACCESS_TOKEN` only if Enhanced Push Security is enabled.

### Verification steps (Expo)

- **Web:** `npx expo export -p web` (verified — bundle produced) and serve; customer login →
  catalog → product → cart → checkout works against the proxied backends.
- **Native (pending real devices):** `eas build` → install on a physical iOS/Android device →
  grant notification permission → confirm a push token is registered (`registerPushToken`
  mutation) → trigger `sendPushCampaign` from the admin/API → confirm OS delivery. **Do not
  claim push delivery works until this device test is performed.**

---

## 8. Cross-service secrets matrix

| Secret/Value | Go (MS1) | Django (MS2) | Express (MS3) | Angular | Expo | Notes |
|--------------|:--------:|:------------:|:-------------:|:-------:|:----:|-------|
| RSA **private** key (PEM) | ✅ signs | — | — | — | — | One keypair for the whole system |
| RSA **public** key (PEM) | ✅ | ✅ verify | ✅ verify | — | — | Same public key in all three backends |
| `JWT_ISSUER` (`ficct-go`) | ✅ set | ✅ expect | ✅ expect | — | — | Must match everywhere |
| `JWT_AUDIENCE` | ✅ superset CSV | `ficct-django` | `ficct-express` | (`ficct-angular`) | (`ficct-mobile`) | Go issues all; each verifier checks its own |
| Postgres URL | ✅ (boutique) | — (SQLite+DynamoDB) | ✅ (documents) | — | — | Two separate databases |
| S3/MinIO keys + bucket | — | (DynamoDB only) | ✅ | — | — | Private bucket, presigned URLs |
| DynamoDB AWS keys | — | ✅ | — | — | — | Or IAM role on GCP via STS |
| `WEBHOOK_HMAC_SECRET` | ✅ signs | — | — | — | — | Shared with the n8n receiver |
| Backend public URLs | — | — | — | ✅ (baked) | ✅ (baked) | **GitHub Variables**, not Secrets |
| EAS / push tokens | (`EXPO_PUSH_ACCESS_TOKEN`) | — | — | — | ✅ (`EAS_TOKEN`, `projectId`) | Only if EAS/Enhanced Push used |

The JWT public key and the issuer/audience triple are the linchpin of cross-service auth: a
single Go-issued token is accepted by Express and Django because Go's `JWT_AUDIENCE` is a CSV
**superset** that includes each verifier's expected `aud`. **Verified live** (one admin token
worked across all three backends).

---

## 9. GitHub Secrets creation checklist

> CLI examples use `gh secret set` / `gh variable set`. Scope with `--repo OWNER/REPO`, or
> `--org ORG --visibility …`, or `--env ENVIRONMENT`. PEM/multiline values: pipe from a file.

```bash
# ---- Go (MS1) ----
gh secret  set GO_DATABASE_URL            --repo OWNER/ficct-boutique-backend-go
gh secret  set GO_JWT_PRIVATE_KEY_PEM     --repo OWNER/ficct-boutique-backend-go < jwt_private.pem
gh secret  set GO_JWT_PUBLIC_KEY_PEM      --repo OWNER/ficct-boutique-backend-go < jwt_public.pem
gh secret  set GO_WEBHOOK_HMAC_SECRET     --repo OWNER/ficct-boutique-backend-go   # openssl rand -hex 32
# gh secret set GO_EXPO_PUSH_ACCESS_TOKEN --repo OWNER/ficct-boutique-backend-go   # only if used
gh variable set GO_JWT_ISSUER --body ficct-go --repo OWNER/ficct-boutique-backend-go
gh variable set GO_JWT_AUDIENCE --body "ficct-express,ficct-django,ficct-angular,ficct-mobile" --repo OWNER/ficct-boutique-backend-go
gh variable set GO_CORS_ALLOWED_ORIGINS --body "https://admin.ficct.example,https://app.ficct.example" --repo OWNER/ficct-boutique-backend-go
# (+ GO_APP_ENV, GO_APP_LOG_LEVEL, GO_JWT_KEY_ID, GO_WEBHOOK_INVOICE_URL, rate limits, GO_EXPO_PUSH_API_URL, GO_MIGRATIONS_DIR)

# ---- Django (MS2) ----
gh secret  set DJANGO_SECRET_KEY                 --repo OWNER/ficct-boutique-backend-python
gh secret  set DJANGO_JWT_PUBLIC_KEY_PEM         --repo OWNER/ficct-boutique-backend-python < jwt_public.pem
gh secret  set DJANGO_DYNAMODB_ACCESS_KEY_ID     --repo OWNER/ficct-boutique-backend-python
gh secret  set DJANGO_DYNAMODB_SECRET_ACCESS_KEY --repo OWNER/ficct-boutique-backend-python
gh variable set DJANGO_SETTINGS_MODULE --body config.settings.prod --repo OWNER/ficct-boutique-backend-python
gh variable set DJANGO_DEBUG --body False --repo OWNER/ficct-boutique-backend-python
gh variable set DJANGO_ALLOWED_HOSTS --body "ai.ficct.example" --repo OWNER/ficct-boutique-backend-python
# (+ DJANGO_JWT_ISSUER, DJANGO_JWT_AUDIENCE, DJANGO_DYNAMODB_REGION, DJANGO_DYNAMODB_TABLE_PREFIX,
#    DJANGO_GO_CORE_BASE_URL, DJANGO_CORS_ALLOWED_ORIGINS, DJANGO_SQLITE_PATH)

# ---- Express (MS3) ----
gh secret  set EXPRESS_DATABASE_URL          --repo OWNER/ficct-boutique-backend-express
gh secret  set EXPRESS_JWT_PUBLIC_KEY_PEM    --repo OWNER/ficct-boutique-backend-express < jwt_public.pem
gh secret  set EXPRESS_S3_ACCESS_KEY_ID      --repo OWNER/ficct-boutique-backend-express
gh secret  set EXPRESS_S3_SECRET_ACCESS_KEY  --repo OWNER/ficct-boutique-backend-express
gh variable set EXPRESS_S3_BUCKET --body ficct-documents-prod --repo OWNER/ficct-boutique-backend-express
gh variable set EXPRESS_S3_REGION --body us-east-1 --repo OWNER/ficct-boutique-backend-express
gh variable set EXPRESS_JWT_ISSUER --body ficct-go --repo OWNER/ficct-boutique-backend-express
gh variable set EXPRESS_JWT_AUDIENCE --body ficct-express --repo OWNER/ficct-boutique-backend-express
# (+ EXPRESS_NODE_ENV, EXPRESS_LOG_LEVEL, EXPRESS_CORS_ALLOWED_ORIGINS, EXPRESS_S3_ENDPOINT(unset for AWS),
#    EXPRESS_S3_FORCE_PATH_STYLE, EXPRESS_S3_PRESIGN_EXPIRY_SECONDS, EXPRESS_S3_SERVER_SIDE_ENCRYPTION, rate limits)

# ---- Angular (admin) — Variables only ----
gh variable set ANGULAR_GRAPHQL_URL       --body /api/graphql   --repo OWNER/ficct-boutique-frontend-angular
gh variable set ANGULAR_DOCUMENTS_API_URL --body /api/documents --repo OWNER/ficct-boutique-frontend-angular
gh variable set ANGULAR_AI_API_URL        --body /api/ai        --repo OWNER/ficct-boutique-frontend-angular

# ---- Expo (mobile) ----
gh variable set EXPO_PUBLIC_GRAPHQL_URL --body https://<go-host>/graphql      --repo OWNER/ficct-boutique-mobile-react-native
gh variable set EXPO_PUBLIC_AI_API_URL  --body https://<django-host>/api/v1   --repo OWNER/ficct-boutique-mobile-react-native
gh variable set EXPO_PROJECT_ID         --body <eas-project-uuid>             --repo OWNER/ficct-boutique-mobile-react-native
# gh secret set EAS_TOKEN              --repo OWNER/ficct-boutique-mobile-react-native   # only if CI builds via EAS
# gh secret set EXPO_PUSH_ACCESS_TOKEN --repo OWNER/ficct-boutique-mobile-react-native   # only if Enhanced Push Security
```

---

## 10. Deployment order

1. **Generate the RSA JWT keypair once** (§3) and store the public key as a Secret in all three
   backends; the private key only in Go.
2. **Provision data stores:** Go Postgres (Railway), Express Postgres (RDS), AWS S3 private
   bucket, AWS DynamoDB tables (or rely on `ensure_tables` to create them).
3. **Deploy Go core (MS1)** to Railway — the Docker entrypoint runs `migrate up`, `seed`, then `server`. It is the auth
   issuer; everything else depends on its public key + issuer/audience.
4. **Deploy Express (MS3)** to AWS — needs Go's public key, Postgres, S3.
5. **Deploy Django (MS2)** to GCP — needs Go's public key, DynamoDB; run `ensure_tables`.
6. **Set CORS** on all three backends to include the Angular and Expo production origins.
7. **Build & host Angular** with the production backend URLs (absolute, or relative behind a
   proxy).
8. **Configure Expo/EAS** (`eas init`, APNs/FCM) and build the native apps and/or the web build
   with `EXPO_PUBLIC_*` pointing at the production backends.

---

## 11. Final verification checklist

| Area | Command / action | Expected |
|------|------------------|----------|
| Go health | `curl /health` | `200 ok` |
| Go auth | GraphQL `login` | `accessToken` + role |
| Django health | `curl /api/v1/health/` | `200` |
| Django auth | forecast with Bearer token | `200` + points |
| Express health | `curl /health` | `{"status":"ok"}` |
| Express docs | upload→confirm→download→verify | hash intact, ledger chain intact |
| Cross-service JWT | use Go token against Express + Django | both `200` |
| Angular | login → dashboard | charts render, no console errors |
| Angular RBAC | customer visits `/audit` | redirected to `/dashboard` |
| Mobile web | login → catalog → checkout | order persisted (`ORD-…`) |
| Push (server) | `sendPushCampaign` | fake/real Expo receives batch |
| Push (device) | EAS build + device | **pending real device test** |

---

## 12. Known gaps and manual steps

1. **No CI/CD exists.** No `.github/workflows`, `.gitlab-ci.yml`, or `eas.json` in any repo. The
   Secrets above must be created and a pipeline authored (or deploys done manually) — this guide
   provides the checklist, not a generated pipeline.
2. **`.env.example` additions made this delivery:** `MIGRATIONS_DIR` (Go), `SQLITE_PATH`
   (Django), `S3_PUBLIC_ENDPOINT` (Express, commented/optional). These were read by code but
   previously undeclared.
3. **EAS project id is not configured** (`app.json` has no `extra.eas.projectId`). Run
   `eas init` before push tokens can be acquired. APNs (iOS) and FCM (Android) credentials must
   be configured via EAS for real delivery.
4. **Mobile web AI image search returns 400** because the upload uses the React Native–native
   `FormData` `{uri,name,type}` shape (`src/services/ai/ai.service.ts:17`) that browsers cannot
   serialize. This is a **web-only** limitation; native uploads work and the endpoint is proven.
   (Enhancement: branch on `Platform.OS === 'web'` to fetch the blob and append a real `File`.)
5. **Mobile branch-distance screen requires device geolocation** (`expo-location`), so it stays
   loading in headless web. Functional on real devices / when location is granted.
6. **Webhook/n8n dispatcher is config-gated and off by default.** Set both
   `WEBHOOK_INVOICE_URL` and `WEBHOOK_HMAC_SECRET` to enable it; the same HMAC secret must be
   configured in the n8n receiver.
7. **Development JWT keys are committed** in `.tools/keys/` for the demo. Generate fresh
   production keys, inject them as Secrets/mounted files, and never use the dev keys in
   production.
8. **`seed` runs on Go startup** after `migrate up` and before `server`. It is idempotent but creates
   demo users/catalog; remove `seed` from the production start command after the first run if
   you do not want demo data.
9. **`EXPRESS_JWT_KEY_ID` is documentation-only** today (the verifier does not yet select keys
   by `kid`). Safe to set; no runtime effect.
10. **DynamoDB Local is in-memory** in the full Docker stack — AI data resets on restart. Use
    real AWS DynamoDB (or a persistent volume) for durable AI state.

---

*Generated from direct code/configuration inspection of the five FICCT Boutique repositories.
All values shown for local/dev are the real defaults found in `.env.example` / Docker Compose;
production values must be supplied via GitHub Secrets/Variables as described above. No real
secrets are included in this document.*
