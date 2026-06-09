# FICCT Boutique — Security Hardening + Full E2E Evidence

Run `20260609-042000`. Real deployed services; real Playwright browser screenshots
(Angular Admin deployed + RN Expo web export). All hardcoded credentials removed,
test accounts rotated, both Angular origins allowed in CORS, hash-ledger surfaced
in the UI, visual-similarity + push proven.

## Service URLs

| Service | Provider | URL |
|---|---|---|
| MS1 Go core | Railway + NeonDB | https://ficct-boutique-backend-go-production.up.railway.app |
| Automation n8n | Railway | https://n8n-production-6287.up.railway.app |
| Gotenberg (PDF) | Railway | https://gotenberg-production-7558.up.railway.app |
| MS3 docs | AWS Lambda + API Gateway + S3 + NeonDB | https://docs-api-boutique.ficct.com |
| MS2 AI | GCP Cloud Run + DynamoDB | https://ficct-ai-1093089304525.us-central1.run.app |
| Angular Admin | Railway (static nginx) | https://angular-admin-production.up.railway.app · https://admin.boutique.ficct.com |

n8n → Gotenberg: internal `gotenberg.railway.internal:3000` (same service whose public URL is gotenberg-production-7558). express-docs Railway service is OFFLINE (404); MS3 production is AWS only.

## Security (Phase 1 + 2)

- **No hardcoded credentials** anywhere: `git grep` for Admin123!/Cliente123!/Staff123!/*@ficct.local is EMPTY across all 6 repos (see security-scan-results.json). Angular + RN login forms are empty by default; seed + e2e specs + docker-compose read from env; docs use placeholders.
- **Accounts rotated**: admin/customer/staff now use strong random passwords + non-local emails (seed driven by SEED_* env). Legacy admin@ficct.local / cliente@ficct.local / staff@ficct.local are DEACTIVATED — login returns "user is inactive" (verified). Credentials are only in git-ignored TEST_ACCOUNTS.local.md + reported to the user.

## CORS (Phase 3) — both origins, preflight verified

Go, MS3 (Lambda), MS2 (Cloud Run) all return the correct Access-Control-Allow-Origin for BOTH `https://angular-admin-production.up.railway.app` and `https://admin.boutique.ficct.com` (see cors-results.json).

## Blockchain / Hash Ledger (Phase 6)

Angular Documents page now has a dedicated "Cadena de bloques · Hash Ledger" section (button "Cadena" per row) showing per-block SHA-256, previous hash (génesis), chain hash, plus the verify verdict (Integridad íntegro / Cadena íntegra, stored vs current SHA-256). Backed by MS3 GET /documents/:id/ledger + /verify. Screenshots: angular-desktop-07-blockchain-ledger, angular-responsive-06-blockchain-ledger.

## Deep Learning / Visual Search (Phase 7) — honest

The visual search is a **classical computer-vision baseline (NOT a neural net)**: perceptual hash (64) + HSV histogram (48) = 112-dim cosine similarity. PROVEN end-to-end: synced 3 demo embeddings, queried with a red image → DEMO-RED **1.0**, DEMO-BLUE 0.6717, DEMO-GREEN 0.3472 (correct ranking). Forecast (Holt) + clustering (KMeans/DynamoDB) executed live from the Angular AI page. See ms2-results.json.

## Push notifications (Phase 8)

Full path implemented + proven via Go GraphQL: registerPushToken → myPushTokens → sendPushCampaign (segment by userIds) → sendTestPushNotification; Go calls the Expo Push API, validates tokens, deactivates invalid ones (see push-results.json). RN app does permission → getExpoPushTokenAsync → register on login. Physical on-device delivery (mobile-10-push-received) needs a real device/EAS — documented blocker.

## Backend E2E (Phase 10)

- orderCode **ORD-20260609-8389** (customer = cvictorhugo39@gmail.com, rotated account)
- Invoice MS3 doc **108aeb3a-f956-4b0d-bda0-e59d61db51ad** (active, sha256 `54cdf4ef…28b3`, intact, chainIntact)
- Invoice S3 object **pdf/2026-06-09/5d79c48f-0a57-4ee3-90c5-f917032b7d37.pdf**
- **SES**: sender dev@ficct.com (verified); **ProductionAccessEnabled=true** → the invoice email delivered to the customer inbox cvictorhugo39@gmail.com (runtime test data, not hardcoded). SentLast24h=5.
- **Invalid HMAC**: webhook 200, sideEffectDocument=null (no PDF/MS3/email)
- MS3 direct: upload-request → S3 PUT 200 → confirm active → verify intact/chainIntact
- MS2: public health + DynamoDB read with Go JWT

## CI/CD

MS3 (express) latest run success; MS2 (python) latest run success.

## Screenshots (23) — real browser

Angular desktop (1440x900): 01-login (EMPTY form), 02-dashboard, 03-products, 04-inventory, 05-sales, 06-documents, 07-blockchain-ledger, 08-ai-deep-learning.
Angular responsive (768x1024): 01-login, 02-dashboard, 03-products, 04-inventory, 05-documents, 06-blockchain-ledger.
Mobile (390x844): 01-home, 02-login (EMPTY), 03-catalog, 04-product-detail, 05-cart, 06-ai-camera-results, 07-branches-gps, 08-push-permission, 09-push-token-registered.

## Remaining (external / device / account)

- Angular repo-based deploy: Railway native source-connect blocked (repo under a different GitHub account than the Railway owner; MCP Unauthorized). Added .github/workflows/deploy-railway.yml (repo-based CD; needs a RAILWAY_TOKEN repo secret — documented).
- Railway custom domains via CLI/MCP blocked by account/plan (Angular has the working *.up.railway.app + admin.boutique.ficct.com).
- mobile-10-push-received + native camera multipart: require a physical device / EAS dev build.
