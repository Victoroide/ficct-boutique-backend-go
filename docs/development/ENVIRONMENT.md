# Environment variables

Source of truth: [.env.example](../../.env.example) and [internal/config/config.go](../../internal/config/config.go). This file documents what each variable does and what happens when it's missing.

## Server

| Variable | Default | Effect |
|----------|---------|--------|
| `APP_ENV` | `development` | Selects logger preset (pretty in dev, JSON otherwise). |
| `APP_PORT` | `8080` | Port the HTTP server binds inside the container. |
| `APP_LOG_LEVEL` | `info` | zerolog level. `debug` is noisy but useful. |

## Database

| Variable | Required | Effect |
|----------|----------|--------|
| `DATABASE_URL` | yes | `postgres://user:pass@host:port/db?sslmode=disable` |

The migration runner reads SQL files from `/app/migrations` (or `./migrations` outside the container). Migrations are not skippable — `cmd/migrate up` applies every unapplied file in lexicographic order.

## JWT

| Variable | Default | Effect |
|----------|---------|--------|
| `JWT_PRIVATE_KEY_PATH` | `/app/.tools/keys/jwt_private_dev.pem` | RSA private key, PEM PKCS#1 or PKCS#8. |
| `JWT_PUBLIC_KEY_PATH` | `/app/.tools/keys/jwt_public_dev.pem` | RSA public key — used for verification within the Go service. |
| `JWT_ISSUER` | `ficct-go` | `iss` claim. |
| `JWT_AUDIENCE` | `ficct-express,ficct-django,ficct-angular,ficct-mobile` | comma-separated. The first non-empty entry is the canonical audience; the rest are accepted. |
| `JWT_KEY_ID` | `dev-1` | written into the `kid` JWT header. |
| `JWT_ACCESS_TTL_MINUTES` | `60` | Access-token lifetime. |
| `JWT_REFRESH_TTL_DAYS` | `7` | Refresh-token lifetime — **stored in DB but no GraphQL mutation exposes refresh today**. |

If the keys cannot be loaded, the server logs `fatal load keys` and exits.

## CORS

| Variable | Default | Effect |
|----------|---------|--------|
| `CORS_ALLOWED_ORIGINS` | `http://localhost:4200,http://localhost:19006,http://localhost:8081` | Strict allow-list. Wildcard `*` is rejected by `go-chi/cors` because credentials are enabled. |

## Webhook (`sale.confirmed`)

| Variable | Default | Effect |
|----------|---------|--------|
| `WEBHOOK_INVOICE_URL` | (empty in `.env.example` for safety) | Destination. If empty, the dispatcher does not start. |
| `WEBHOOK_HMAC_SECRET` | (empty) | HMAC-SHA256 secret. Empty disables the dispatcher with a warning. |
| `WEBHOOK_DISPATCH_INTERVAL_SECONDS` | `5` | How often the dispatcher polls the outbox. |
| `WEBHOOK_MAX_RETRIES` | `5` | After this many attempts, the row is marked failed. |

## Rate limit

| Variable | Default | Effect |
|----------|---------|--------|
| `RATE_LIMIT_RPS` | `20` | Refill rate for the per-IP token bucket. |
| `RATE_LIMIT_BURST` | `40` | Bucket capacity. |

## Push notifications (Expo)

| Variable | Default | Effect |
|----------|---------|--------|
| `EXPO_PUSH_API_URL` | `https://exp.host/--/api/v2/push/send` | Endpoint the `service.PushSender` posts campaign batches to. The full-stack docker compose overrides this to `http://fake-expo-push:8080/--/api/v2/push/send` so the meta-compose never touches the real Expo service. |
| `EXPO_PUSH_ACCESS_TOKEN` | (empty) | Sent as `Authorization: Bearer …` when Expo's "Enhanced Push Security" is enabled for the project. Empty = no auth header. |

The limiter applies to **all** routes including `/health` and `/static/products/...`. If you're running synthetic load tests, raise both values or disable the middleware in `cmd/server/main.go`.

## Notes on defaults

`config.Load()` panics or logs `fatal` for any genuinely missing required value (only `DATABASE_URL` is strictly required — everything else has a default that lets the server start in some form, even if it warns about disabled features). The full list of defaults is in [internal/config/config.go](../../internal/config/config.go); when the source disagrees with this document, **the source wins**.
