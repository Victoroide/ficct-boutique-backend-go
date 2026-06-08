# FICCT Boutique n8n Invoice Workflow

This folder contains the real importable n8n workflow for the confirmed-sale invoice automation:

- `ficct-invoice-workflow.json` - workflow artifact to import into n8n.
- `ficct-mailpit-smtp.credentials.json` - local-only Mailpit SMTP credential for Docker verification.
- `build-workflow.mjs` - source builder for regenerating the workflow JSON after node edits.
- `verify-n8n-invoice-e2e.mjs` - Docker-run E2E verification script.

## Workflow Contract

Production webhook URL:

```text
POST /webhook/ficct-invoice
```

The Webhook node is configured with:

- Method: `POST`
- Path: `ficct-invoice`
- Raw Body: enabled
- Response mode: immediately on receipt

The first Code node validates `X-FICCT-Signature` against the exact raw body:

```text
sha256=<HMAC-SHA256(rawBody, WEBHOOK_HMAC_SECRET)>
```

Invalid HMAC, invalid JSON, missing customer email/name, PDF generation failure, MS3 upload failure, hash-ledger failure, or SMTP failure stops the n8n execution before later side effects.

Because the Webhook node is intentionally configured to respond immediately with 2xx, invalid HMAC requests receive the immediate webhook acknowledgement but fail inside the execution and do not generate PDF/MS3/email side effects.

## Nodes

The workflow includes:

1. `Webhook sale.confirmed`
2. `Validate HMAC and Payload`
3. `Build Invoice HTML`
4. `Generate PDF with Gotenberg`
5. `Compute PDF SHA-256`
6. `Login Go Service Account`
7. `MS3 Upload Request`
8. `PUT PDF to Presigned URL`
9. `MS3 Confirm Upload`
10. `MS3 Verify Hash Ledger`
11. `Prepare Invoice Email`
12. `Send Invoice Email`

Merge nodes preserve the generated PDF binary across the JSON API calls.

## Required n8n Variables

Local compose sets these automatically:

| Variable | Local value | Purpose |
| --- | --- | --- |
| `WEBHOOK_HMAC_SECRET` | `change-me-strong-random-secret` | Must match Go `WEBHOOK_HMAC_SECRET`. |
| `GOTENBERG_URL` | `http://gotenberg:3000` | Gotenberg Chromium HTML-to-PDF endpoint base. |
| `GO_CORE_GRAPHQL_URL` | `http://go-core:8080/graphql` | Service-account login. |
| `EXPRESS_DOCS_URL` | `http://express-docs:8081` | MS3 document API. |
| `FICCT_N8N_SERVICE_EMAIL` | `staff@ficct.local` | Local staff service account. |
| `FICCT_N8N_SERVICE_PASSWORD` | `Staff123!` | Local staff service account password. |
| `FICCT_INVOICE_FROM_EMAIL` | `facturas@ficct.local` | Sender shown in the invoice email. |

For production, create a dedicated staff/admin service account, rotate its password into n8n credentials or variables, and replace the SMTP credential with SMTP, SendGrid SMTP, or SES SMTP settings.

## Docker Verification

From the Go repo:

```powershell
docker compose -f docker-compose.full.yml -f docker-compose.n8n-test.yml up -d --build
docker compose -f docker-compose.full.yml -f docker-compose.n8n-test.yml run --rm n8n-e2e
```

The override starts:

- Go core + Go Postgres
- Express docs + Express Postgres
- MinIO
- n8n
- Gotenberg
- Mailpit

The verifier confirms:

1. A customer sale is created and confirmed in Go.
2. Go dispatches a signed webhook to n8n.
3. n8n validates HMAC over raw body.
4. n8n generates a binary PDF through Gotenberg.
5. n8n uploads the PDF through MS3 upload-request and the presigned PUT.
6. MS3 confirms the SHA-256 and stores an active PDF in MinIO.
7. MS3 hash ledger verification returns `intact=true` and `chainIntact=true`.
8. n8n sends an invoice email to Mailpit with the PDF attached.
9. A direct invalid-signature webhook produces no PDF/MS3/email side effects.

Mailpit UI is available at:

```text
http://localhost:8025
```

n8n UI is available at:

```text
http://localhost:5678
```

## Manual Import

Self-hosted n8n CLI:

```powershell
docker exec -u node -it <n8n-container> n8n import:workflow --input=/path/ficct-invoice-workflow.json
docker exec -u node -it <n8n-container> n8n update:workflow --all --active=true
```

n8n Cloud:

1. Open Workflows.
2. Import from file.
3. Select `ficct-invoice-workflow.json`.
4. Create or select an SMTP credential for `Send Invoice Email`.
5. Configure equivalent variables for the HMAC secret, service URLs, service account, and sender email.
6. Activate the workflow.
7. Set Go `WEBHOOK_INVOICE_URL` to the workflow production URL.

For n8n Cloud, use public HTTPS URLs for Go/Express/Gotenberg or place n8n self-hosted inside the same private network. Presigned MS3 upload URLs must be reachable by n8n exactly as returned.
