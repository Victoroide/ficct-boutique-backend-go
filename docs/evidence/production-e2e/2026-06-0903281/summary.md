# FICCT Boutique — Production E2E Evidence

Run: 2026-06-09 (folder `2026-06-0903281`). All tests hit the **real deployed
services** via their working provider URLs (no custom-domain dependency).

## Services & URLs

| Service | Provider | URL | Health |
|---|---|---|---|
| MS1 Go core | Railway + NeonDB | https://ficct-boutique-backend-go-production.up.railway.app | 200 |
| Automation (n8n) | Railway | https://n8n-production-6287.up.railway.app | 200 |
| Gotenberg | Railway | https://gotenberg-production-7558.up.railway.app | 200 |
| MS3 docs | AWS Lambda + API Gateway + S3 + NeonDB | https://bptu80mcbk.execute-api.us-east-1.amazonaws.com (also https://docs-api-boutique.ficct.com) | 200 |
| MS2 AI | GCP Cloud Run + DynamoDB | https://ficct-ai-1093089304525.us-central1.run.app | 200 (public) |

## Pass/fail

| Area | Result |
|---|---|
| Endpoints (all 6 health) | ✅ 200 |
| MS1: login, products(+variant size/color/stock), inventory, reports (monthlySales/popularProducts/dashboard), create+confirm sale | ✅ |
| MS3: upload-request → presigned S3 PUT (200) → confirm (active) → verify (intact, chainIntact) | ✅ |
| Automation: confirmed sale → invoice PDF in S3 → MS3 active doc → hash ledger verified | ✅ |
| Invalid HMAC: webhook 200 accept, **no** PDF/MS3/email side effect | ✅ |
| MS2: public health + DynamoDB-backed read (embeddings) with Go JWT | ✅ |
| Angular: lint ✅ / build ✅ (typecheck: e2e-only Playwright type errors) | ✅ build |
| React Native: lint ✅ / typecheck ✅ | ✅ |

## Evidence IDs (this run)

- Test sale / order: **ORD-20260609-9022**
- Invoice MS3 document: **dd8478bb-17e2-4626-8b0c-c798dc29cf85** (status=active, sha256 `4cb53a4b…5ecb`, intact=true, chainIntact=true)
- Invoice S3 object: **pdf/2026-06-09/dc643caa-5dab-4d0a-bf6c-986bee34ea50.pdf** (24,081 bytes); total invoice PDFs in bucket: 7
- MS3 direct evidence doc: see `ms3-results.json` (S3 PUT 200, confirm active, verify intact/chainIntact)
- SES: sender `dev@ficct.com` VerifiedForSendingStatus=true; SentLast24Hours=3 (HTTPS `SendRawEmail`, no SMTP)
- Invalid HMAC: webhook HTTP 200, sideEffectDocument=null (no side effect)

## S3 bucket security

`ficct-boutique-documents`: BlockPublicAcls=true (private), SSE=AES256, presigned URLs only.

## Files

`endpoints.json`, `ms1-go-results.json`, `ms3-results.json`, `automation-results.json`,
`ms2-results.json`, `angular-results.json`, `mobile-results.json`, `run-results.json`.

## Remaining (external / non-functional)

- Custom domains for Railway (Go/n8n) — account/plan blocks `railway domain`
  (Unauthorized while all other ops work). Services use `*.up.railway.app`.
- `ai-api-boutique.ficct.com` — GCP managed mapping needs the `gcloud beta`
  component (uninstallable here) or a CF token with Ruleset/SSL scope; MS2 is
  public at its `run.app` URL.
- Angular hosting — build verified; no static host provisioned (Cloudflare Pages
  recommended). RN native binaries require EAS (not run; web/export path documented).

## Confirmations

No n8n Cloud · MS3 not on Railway (AWS) · No SMTP (SES HTTPS) · No custom-domain
requirement · No secrets in evidence.
