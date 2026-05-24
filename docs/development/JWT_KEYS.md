# RS256 JWT keys

The Go service signs access tokens with RS256. Express and Django **only ever hold the public key** — they cannot mint tokens, only verify them. This file covers generating the dev keypair and distributing the public key.

## Generating the dev keypair

From this repo's root:

```powershell
# Private key (2048-bit RSA)
docker run --rm -v "${PWD}/.tools/keys:/keys" alpine/openssl genrsa -out /keys/jwt_private_dev.pem 2048

# Public key derived from the private key
docker run --rm -v "${PWD}/.tools/keys:/keys" alpine/openssl rsa -in /keys/jwt_private_dev.pem -pubout -out /keys/jwt_public_dev.pem
```

Linux/macOS:

```bash
mkdir -p .tools/keys
docker run --rm -v "$(pwd)/.tools/keys:/keys" alpine/openssl genrsa -out /keys/jwt_private_dev.pem 2048
docker run --rm -v "$(pwd)/.tools/keys:/keys" alpine/openssl rsa -in /keys/jwt_private_dev.pem -pubout -out /keys/jwt_public_dev.pem
```

The `.tools/keys/*.pem` files are gitignored except for a `.gitkeep` placeholder (see [.gitignore](../../.gitignore)).

## Distributing the public key

Copy `.tools/keys/jwt_public_dev.pem` into the matching directory of each verifying service:

```
typescript/ficct-boutique-backend-express/.tools/keys/jwt_public_dev.pem
python/django/ficct-boutique-backend-python/.tools/keys/jwt_public_dev.pem
```

Each Dockerfile in those repos has a `COPY .tools/keys /app/.tools/keys` step, so the file must be in place **before** building those images. The `docker-compose.full.yml` build picks them up automatically.

## Token claim shape

| Claim | Source | Default |
|-------|--------|---------|
| `iss` | `JWT_ISSUER` | `ficct-go` |
| `aud` | `JWT_AUDIENCE` (comma-separated) | `ficct-express,ficct-django,ficct-angular,ficct-mobile` |
| `sub` | user UUID | — |
| `kid` (header) | `JWT_KEY_ID` | `dev-1` |
| custom: `email` | user email | — |
| custom: `role` | `admin` / `staff` / `customer` / `system` | — |
| `exp` | now + `JWT_ACCESS_TTL_MINUTES` | now + 60 minutes |
| `iat` | now | — |

The verifier in each downstream service checks `iss`, `aud` (its own audience must be in the list), `exp`, and the RS256 signature against the loaded public key.

## Rotating keys

There is no automated rotation. To rotate:

1. Generate a new keypair with a different `kid` (e.g. `dev-2`).
2. Place the new public key alongside the old one in each verifier's image (or container). Verifiers in this codebase only load **one** key at a time, so for true zero-downtime rotation you would need to extend the verifier to support a key set.
3. Update `JWT_KEY_ID` on the Go service and restart it. From that moment, all new tokens carry the new `kid`. Existing tokens (signed with the old `kid`) become invalid against verifiers that have switched to the new key.

In other words: the current implementation supports rotation only with a brief gap where all clients must re-login. Acceptable for the academic / demo context; not acceptable for a real production deployment.

## Production posture (when this leaves the lab)

- Inject the keys via Docker secrets or a secret manager (AWS KMS / HashiCorp Vault). Remove the `COPY .tools/keys /app/.tools/keys` line from the Dockerfile.
- Use 4096-bit RSA or switch to EdDSA. The signature library (`golang-jwt/jwt/v5`) supports both.
- Keep the private key off every machine that doesn't need to mint tokens. Right now the Go image bakes the private key in — fine for `docker compose`, not for cluster deployment.
