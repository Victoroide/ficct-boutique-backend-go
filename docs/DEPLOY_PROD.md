# MS1 Go Core — Production Deployment

## Platform & data

- **Compute**: Railway (`ficct-boutique-backend-go`), Dockerfile builder.
- **Database**: **NeonDB PostgreSQL** (`DATABASE_URL` points at Neon;
  `sslmode=require&channel_binding=require`). The Railway Postgres add-on was
  removed — MS1 runs entirely on Neon. The Docker entrypoint runs
  `migrate up → seed → server` on each deploy (idempotent).
- **URL**: `https://ficct-boutique-backend-go-production.up.railway.app`
  (GraphQL at `/graphql`, health at `/health`).

## Webhook → automation (n8n)

On a confirmed sale, Go writes the `webhook_outbox` row and the dispatcher POSTs
a signed `sale.confirmed` webhook to n8n:

- `WEBHOOK_INVOICE_URL=https://n8n-production-6287.up.railway.app/webhook/ficct-invoice`
- `X-FICCT-Signature: sha256=<HMAC-SHA256(rawBody, WEBHOOK_HMAC_SECRET)>`
- `WEBHOOK_HMAC_SECRET` is shared with n8n.

## Key env vars

`DATABASE_URL` (Neon), `MIGRATIONS_DIR=migrations`,
`GO_JWT_PRIVATE_KEY_PEM` / `GO_JWT_PUBLIC_KEY_PEM` (RS256; the public key is used
by MS3 and MS2 to verify tokens), `JWT_ISSUER=ficct-go`, `JWT_AUDIENCE`,
`WEBHOOK_INVOICE_URL`, `WEBHOOK_HMAC_SECRET`, `APP_PORT=8080`. No secrets in git.

## Custom domain

Intended: `api-boutique.ficct.com` → this service. **Not yet registered** —
Railway custom-domain management currently returns Unauthorized for this account
(plan/permission limitation); the service is reachable at its `*.up.railway.app`
URL. Once Railway domain access is available, register the domain and add a
Cloudflare CNAME to the Railway target.

## Verify production

```powershell
curl.exe https://ficct-boutique-backend-go-production.up.railway.app/health
# GraphQL login (service account)
$body='{"query":"mutation Login($i:LoginInput!){login(input:$i){accessToken}}","variables":{"i":{"email":"staff@ficct.local","password":"Staff123!"}}}'
Invoke-WebRequest -Uri "https://ficct-boutique-backend-go-production.up.railway.app/graphql" -Method Post -ContentType application/json -Body $body
```
