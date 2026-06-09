# n8n Cloud Notes for FICCT Invoice Workflow

The verified artifact in this folder is optimized for local self-hosted n8n in Docker. Final verification uses:

- Go: `http://go-core:8080`
- Express/MS3: `http://express-docs:8081`
- Gotenberg: `http://gotenberg:3000`
- Mailpit SMTP: `mailpit:1025`
- Local HMAC secret: `change-me-strong-random-secret`

n8n Cloud cannot reach those Docker hostnames. Do not import the local workflow into Cloud and expect it to run unchanged.

For manual n8n Cloud test executions, the current test endpoint is:

```text
https://victoroide.app.n8n.cloud/webhook-test/ficct-invoice
```

That URL only works while n8n's test webhook listener is active. For an active/deployed workflow, configure Go with the production webhook URL, normally:

```text
https://victoroide.app.n8n.cloud/webhook/ficct-invoice
```

## Cloud Requirements

To adapt the workflow for n8n Cloud:

1. Replace Docker URLs with public HTTPS URLs:
   - Go GraphQL: `https://<go-service>/graphql`
   - Express/MS3: `https://<express-service>`
   - Gotenberg: `https://<gotenberg-service>`
2. Do not expose Gotenberg publicly without protection. Prefer self-hosted n8n and Gotenberg in the same private network, or deploy Gotenberg behind HTTPS and authentication/network allow-listing.
3. Replace the local service account (`<staff-email>` / `<staff-password>`) with a dedicated production staff/admin service account.
4. Replace Mailpit SMTP with a real SMTP, SendGrid SMTP, SES SMTP or equivalent n8n email credential.
5. Replace the local HMAC secret with the same production secret configured in Go `WEBHOOK_HMAC_SECRET`.
6. Ensure presigned MS3 upload URLs are reachable by n8n Cloud exactly as returned by Express/MS3.

## Env and Code Node Caveats

The local verified workflow intentionally contains no `$env` expressions and runs with:

```text
N8N_BLOCK_ENV_ACCESS_IN_NODE=true
NODE_FUNCTION_ALLOW_BUILTIN=crypto
```

n8n Cloud tenants may deny direct env access and may not allow the same Code-node runtime options. If Cloud blocks `crypto`, binary helpers, or other Code-node APIs, use self-hosted n8n for production or replace those Code nodes with an authenticated internal helper service that performs HMAC/PDF hash operations.

Final acceptance for this project is local production-like Docker verification, not n8n Cloud manual UI testing.
