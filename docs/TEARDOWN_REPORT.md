# FICCT Boutique — Cloud Teardown Report

**Date/time:** 2026-06-20 (local, America/La_Paz)
**Operator:** Victor Cuéllar (Victoroide)
**Goal:** Decommission all cost-generating cloud resources for FICCT Boutique after the academic partial, while preserving source repositories, documentation, and resources belonging to other projects.

> This report contains **no secrets**. Connection strings, keys, and full backups live outside any git repo at the backup path below and are never committed.

---

## 1. Providers cleaned

| Provider | Result |
|---|---|
| Railway | ✅ Done — Boutique project + stray n8n project deleted |
| AWS | ✅ Done — Lambda, API Gateway, S3, DynamoDB, IAM role, ACM cert deleted |
| Cloudflare | ✅ Done — 6 Boutique DNS records deleted |
| NeonDB | ⚠️ Backed up — **deletion pending** (no Neon API key; do in console) |
| GCP | ⚠️ **Pending** — `gcloud` reauth required (Cloud Run `ficct-ai` still running) |
| GitHub | ✅ Partial — dead deploy secrets removed for go & express; django deferred |

## 2. Resources deleted / stopped

### Railway (account: cvictorhugo39@gmail.com)
- Project **`ficct-boutique-project`** (`85a66ad7-…`) — deleted. Services removed: `angular-admin`, `ficct-boutique-backend-go`, `gotenberg`, `n8n`, `workflow-frontend`.
- Project **`n8n`** (stray, `f053c5f4-…`) — deleted (deployment was already failed/removed).
- Confirmed: linking the project id now returns *"Project not found in workspace."*

### AWS (account 654654410319, us-east-1)
- Lambda `ficct-ms3-docs` — deleted.
- API Gateway HTTP API `ficct-ms3-http` (`bptu80mcbk`) — deleted (incl. api-mapping `mbygcf`).
- API Gateway custom domain `docs-api-boutique.ficct.com` — deleted.
- S3 bucket `ficct-boutique-documents` — emptied (14 objects, ~7 MB) and deleted.
- DynamoDB tables — deleted: `ficct_cluster_runs`, `ficct_customer_segments`, `ficct_forecast_results`, `ficct_product_embeddings`.
- IAM role `ficct-ms3-lambda-role` — policy detached, role deleted.
- ACM certificate `f5cbde98-…` (docs-api-boutique.ficct.com) — deleted.

### Cloudflare (zone ficct.com — zone RETAINED)
- Deleted records: `admin.boutique.ficct.com` (+`_railway-verify`), `workflow.boutique.ficct.com` (+`_railway-verify`), `docs-api-boutique.ficct.com`, ACM-validation CNAME `_7243…docs-api-boutique.ficct.com`.

### GitHub (Victoroide repos)
- `ficct-boutique-backend-go`: removed GO_DATABASE_URL, GO_JWT_PRIVATE_KEY_PEM, GO_JWT_PUBLIC_KEY_PEM, GO_WEBHOOK_HMAC_SECRET (now empty).
- `ficct-boutique-backend-express`: removed AWS_ACCESS_KEY_ID, AWS_REGION, AWS_SECRET_ACCESS_KEY, DATABASE_URL, LAMBDA_ARTIFACT_BUCKET, LAMBDA_FUNCTION_NAME, S3_ACCESS_KEY_ID, S3_BUCKET, S3_BUCKET_NAME, S3_SECRET_ACCESS_KEY (now empty).

## 3. Resources intentionally retained

- **FICCT Jobs (separate, active project — NOT FICCT Boutique):** Railway project `ficct-jobs-project` (services `ficct-jobs-backend`, `ficct-jobs-frontend`, `go-ms-rag`); domains `jobs.ficct.com`, `dev-app-backend.jobs.ficct.com`, `dev-go-ms-rag.jobs.ficct.com`; S3 buckets `ficct-jobs-bucket`, `ms-go-rag`; AWS IAM user `ficct-jobs-s3-user` (shared). Verified still healthy after teardown.
- **AWS SES** `ficct.com` domain identity (shared) and `dev@ficct.com` (free; was the n8n invoice sender) — retained.
- **AWS** other buckets: `compaser-srl-dev`, `core-lms-bucket`, `ficct-news-bucket-dev`, `ficct-scrum-bucket`.
- **Cloudflare** zone `ficct.com`, MX/SPF/DMARC, DKIM (SES + cf), and other projects' records (`lms.*`, `axiom-reasoning.*`, `bigpack`, `cvbuilder`, `help`, `pomo`, `*.pages.dev`).
- **All 7 local source repositories** and all GitHub repositories — untouched.

## 4. Backup folder path

```
D:\Repositories\_deployment_secrets\ficct-boutique-teardown-backup\20260620-125809\
```
Contents: `inventory.md`, per-provider `*-teardown.md`, Railway inventory + variable-name lists, AWS resource inventory (Lambda config **redacted**, DynamoDB scans, S3 manifest + 14 downloaded objects, schemas), Cloudflare full DNS export, Neon CSV exports (MS1 + MS3, all tables + schema columns). Secrets kept only in `*.secret.txt` (never committed).

## 5. Verification commands & results

Public URLs (curl):
- `https://ficct-boutique-backend-go-production.up.railway.app` → `{"code":404,"message":"Application not found"}`
- `https://n8n-production-6287.up.railway.app` → Application not found
- `https://gotenberg-production-7558.up.railway.app` → Application not found
- `https://admin.boutique.ficct.com` / `workflow.boutique.ficct.com` / `docs-api-boutique.ficct.com` → no resolution (DNS removed)
- `https://ficct-ai-1093089304525.us-central1.run.app` → **still serving** (Django 404 page) ⇒ GCP pending

AWS:
- `aws lambda get-function --function-name ficct-ms3-docs` → ResourceNotFoundException
- `aws apigatewayv2 get-apis` / `get-domain-names` → `[]`
- `aws s3api head-bucket --bucket ficct-boutique-documents` → 404 Not Found
- `aws dynamodb list-tables` → `[]`
- `aws acm list-certificates` → `[]`
- `aws iam get-role --role-name ficct-ms3-lambda-role` → NoSuchEntity

## 6. Final URL status

| URL | Status |
|---|---|
| ficct-boutique-backend-go-production.up.railway.app | DOWN (Application not found) |
| n8n-production-6287.up.railway.app | DOWN |
| gotenberg-production-7558.up.railway.app | DOWN |
| admin.boutique.ficct.com | DOWN (DNS removed) |
| workflow.boutique.ficct.com | DOWN (DNS removed) |
| docs-api-boutique.ficct.com | DOWN (DNS + AWS removed) |
| ficct-ai-…us-central1.run.app | **UP (pending GCP teardown)** |
| jobs.ficct.com (retained) | UP — intentionally kept |

## 7. Remaining possible cost risks

1. **GCP Cloud Run `ficct-ai`** (project `ficct-boutique-django`, us-central1) is still running, plus any Artifact Registry images → ongoing cost until deleted.
2. **NeonDB** two projects (MS1 host `ep-cold-unit-apinyawo…`, MS3 host `ep-solitary-fire-aq492ckl…`) still exist → storage/compute cost until deleted/suspended. No longer reachable by any app.

## 8. Manual dashboard checks recommended for the user

1. **GCP** — `gcloud auth login` with `victor.cuellar@ficct.uagrm.edu.bo` (the account with access; the currently-active `cvictorhugo39@gmail.com` lacks permission), then delete Cloud Run `ficct-ai`, Artifact Registry repos, SA `ficct-ci-deployer`, WIF `github-pool`/`github-provider`. See `gcp-teardown.md` for exact commands.
2. **NeonDB** — Neon Console: delete the two FICCT Boutique projects (verify by endpoint host); or provide a `NEON_API_KEY`.
3. **GitHub** — remove django GCP/AWS deploy secrets after GCP is down; remove any Railway/deploy secrets in the `sebastianmlz` repos (no admin access from here); optionally delete the `deploy-*` workflow files.
4. **Railway** — confirm in the dashboard that the `ficct-boutique-project` and `n8n` projects are fully gone (deletion is processed asynchronously).
5. **Billing** — check Railway, AWS Billing, GCP Billing, and Neon dashboards next cycle to confirm charges have stopped.

## 9. Safety confirmations

- No source repositories deleted (all 7 present, working trees clean).
- No GitHub repositories deleted.
- No secrets committed; no `.env` committed.
- No unrelated resources deleted (FICCT Jobs, shared SES/DKIM, other projects' buckets/DNS all retained and verified).
