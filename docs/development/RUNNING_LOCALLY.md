# Running the system locally

This file covers two scenarios:

1. **Standalone** — only the Go core + its Postgres, useful for backend-only work.
2. **Full system** — all five repositories at once using `docker-compose.full.yml`.

Both scenarios assume Docker Desktop (or any Docker engine with Compose v2) and that the dev RSA keypair has been generated under `.tools/keys/`. See [JWT_KEYS.md](JWT_KEYS.md).

---

## Standalone

From this repo's root:

```powershell
copy .env.example .env
docker compose up -d --build
```

`docker-compose.yml` brings up two containers:

| Container | Image | Port |
|-----------|-------|------|
| `postgres` | `postgres:16-alpine` | (internal only) |
| `app` | this Dockerfile | host `8080` |

The Go image entrypoint runs `migrate up`, then `seed`, then `server`, so the database is always at the latest schema with demo data on first start. Re-running `docker compose up -d --build` rebuilds the binary and re-runs migrations idempotently (every migration file has a transactional up + down pair).

### Smoke test

```powershell
curl http://localhost:8080/health
# {"status":"ok"}
```

GraphQL login:

```powershell
curl -X POST http://localhost:8080/graphql `
  -H "Content-Type: application/json" `
  -d '{\"query\":\"mutation { login(input:{email:\\\"admin@ficct.local\\\",password:\\\"Admin123!\\\"}) { accessToken expiresAt user { id email role } } }\"}'
```

---

## Full system

From this repo's root (`go/ficct-boutique-backend-go`):

```powershell
docker compose -f docker-compose.full.yml up -d --build
```

The compose file resolves sibling repositories using these relative paths:

| Service | Build context |
|---------|---------------|
| `go-core` | `.` (this repo) |
| `express-docs` | `../../typescript/ficct-boutique-backend-express` |
| `django-ai` | `../../python/django/ficct-boutique-backend-python` |
| `angular-admin` | `../../angular/ficct-boutique-frontend-angular` |
| `mobile-web` | `../../react/react-native/ficct-boutique-mobile-react-native` |

So the repository layout on disk must be:

```
D:\Repositories\
  go\ficct-boutique-backend-go\          # <- run docker compose from here
  typescript\ficct-boutique-backend-express\
  python\django\ficct-boutique-backend-python\
  angular\ficct-boutique-frontend-angular\
  react\react-native\ficct-boutique-mobile-react-native\
```

### Port map (host)

| Port | Service | Notes |
|------|---------|-------|
| 4200 | Angular admin | nginx serves the prod build |
| 4300 | React Native customer (web) | Expo Web export served by nginx, with `/graphql` and `/api/ai` reverse-proxied to the backends |
| 8091 | Express documents (MS3) | container port 8081 → host 8091 |
| 8092 | Django AI (MS2) | container port 8000 → host 8092 |
| 8093 | Go core (MS1) | container port 8080 → host 8093 |
| 9010 | MinIO S3 API | bucket `ficct-documents` (private) |
| 9011 | MinIO console | login: `minio-access` / `minio-secret-change-me` |

The host ports avoid the common `8080`/`8090`/`8000` collisions on dev machines.

### Bring-up sequence

The compose file uses `depends_on` with health checks where available:

1. `go-postgres`, `docs-postgres`, `minio`, `dynamodb` start first.
2. `minio-bootstrap` runs once to create the `ficct-documents` bucket and set its anonymous policy to `none`.
3. `go-core`, `express-docs`, `django-ai` start once their dependencies report healthy / completed.
4. `angular-admin` and `mobile-web` start after the backends are up.

If any container fails health, `docker compose ps` will show it. `docker compose logs <service>` is the fastest way to inspect.

### Tearing down

```powershell
docker compose -f docker-compose.full.yml down
# include -v to delete the named volumes (Postgres data + MinIO objects):
docker compose -f docker-compose.full.yml down -v
```

The volumes are named (`ficct_full_go_pg_data`, `ficct_full_docs_pg_data`, `ficct_full_minio_data`); `down` without `-v` preserves data across restarts.

---

## Talking to it from a host browser

- Angular admin: `http://localhost:4200`, sign in with `admin@ficct.local / Admin123!`.
- Customer mobile web: `http://localhost:4300`, sign in with `cliente@ficct.local / Cliente123!`. The nginx in front of this container reverse-proxies `/graphql` to the Go container and `/api/ai/...` to Django, so the React Native code uses same-origin paths and CORS is never the limiting factor in dev.
- GraphiQL: `http://localhost:8093/playground`.

---

## Troubleshooting

- **Login returns 401** — check that the RS256 public key file shared with Express/Django matches the private key Go is signing with. They must both point at the same logical `kid` (`dev-1` by default).
- **`POST /graphql` returns 429** — rate limiter tripped. Increase `RATE_LIMIT_RPS` / `RATE_LIMIT_BURST` in `.env` and restart the `go-core` container.
- **`confirmSale` returns "insufficient stock"** — expected when the variant's inventory at that branch is below the requested quantity. Either seed more or `setInventoryStock` first.
- **Webhook never fires** — `WEBHOOK_INVOICE_URL` empty in the env. The dispatcher logs a single startup warning when this happens; events still accumulate in `webhook_outbox` until the URL is set.
