# FICCT Boutique n8n Invoice Workflow

This folder contains the real importable n8n workflow for the confirmed-sale invoice automation:

- `ficct-invoice-workflow.json` - workflow artifact to import into n8n.
- `ficct-mailpit-smtp.credentials.json` - local-only Mailpit SMTP credential for Docker verification.
- `build-workflow.mjs` - source builder for regenerating the workflow JSON after node edits.
- `verify-n8n-invoice-e2e.mjs` - Docker-run E2E verification script.
- `ficct-invoice-workflow.n8n-cloud-notes.md` - notes for adapting the local workflow to n8n Cloud.

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

The workflow reads all secrets, URLs, and addresses from `$env`, so one artifact runs unchanged locally and on Railway (self-hosted). The n8n service must run with `N8N_BLOCK_ENV_ACCESS_IN_NODE=false` (so Code nodes can read `$env`) and `NODE_FUNCTION_ALLOW_BUILTIN=crypto`. Required environment variables:

- `WEBHOOK_HMAC_SECRET` - shared HMAC secret (must match the Go core)
- `GOTENBERG_URL` - e.g. `http://gotenberg:3000`
- `GO_CORE_GRAPHQL_URL` - e.g. `http://go-core:8080/graphql`
- `EXPRESS_DOCS_URL` - e.g. `http://express-docs:8081`
- `FICCT_N8N_SERVICE_EMAIL` / `FICCT_N8N_SERVICE_PASSWORD` - Go service account
- `FICCT_INVOICE_FROM_EMAIL` - invoice sender address

The Railway self-hosted automation service lives in the `gotenberg` repo (`n8n/`).

Regenerate the workflow after source edits:

```powershell
node docs\n8n\build-workflow.mjs
```

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
10. `Normalize Confirm Result`
11. `MS3 Verify Hash Ledger`
12. `Normalize Verify Result`
13. `Prepare Invoice Email`
14. `Send Invoice Email`

Merge nodes preserve the generated PDF binary across the JSON API calls. All Merge nodes use `preferInput1`; unsupported internal values such as `preferInput2` are not used.

## Local Runtime Requirements

The local self-hosted n8n service enables Node's `crypto` module for Code nodes:

```text
NODE_FUNCTION_ALLOW_BUILTIN=crypto
N8N_BLOCK_ENV_ACCESS_IN_NODE=false
```

The Code nodes are verified in that runtime. They use `crypto` for HMAC/SHA-256 and n8n binary helpers to read the raw webhook body, create the HTML file for Gotenberg, and read the generated PDF.

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
9. n8n execution history contains a successful execution for the order code.
10. A direct invalid-signature webhook produces an n8n error execution and no PDF/MS3/email side effects.

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

See `ficct-invoice-workflow.n8n-cloud-notes.md`. n8n Cloud is not the final verification target for this artifact. Cloud requires public HTTPS URLs for Go, Express/MS3 and Gotenberg, a real SMTP credential, no Docker hostnames, and may require removing or replacing Code-node assumptions if env or binary APIs are denied.
