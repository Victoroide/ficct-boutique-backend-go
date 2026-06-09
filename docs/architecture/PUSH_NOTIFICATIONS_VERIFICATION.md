# Push Notifications — Docker Verification

This document captures the **automated proof** that the push-notification system works end-to-end in Docker, without a physical device.

What it covers:

1. Backend push sender unit tests (`internal/service/push_sender_test.go`).
2. Resolver RBAC tests (`graph/push_rbac_test.go`).
3. Fake Expo Push API container (`cmd/fake-expo-push/`).
4. End-to-end smoke through the live full-stack docker compose with `curl`.
5. Playwright tests against the live `mobile-web` Expo Web container.

What it does **not** cover (out of scope without EAS + a physical device):

- Real Expo → APNs → iPhone delivery.
- Real Expo → FCM → Android shade delivery.
- iOS-specific permission prompt UI.
- `expo-notifications` foreground listener firing on a real device.

---

## 1. Backend unit tests (httptest, no Docker needed)

```powershell
go test ./internal/service/... -count=1 -run PushSender -v
```

Coverage (9 tests, all pass):

- `TestPushSender_SendsToTokens_HappyPath` — 2 tokens → 2 ok tickets, `sent=2`.
- `TestPushSender_DeactivatesTokenOnDeviceNotRegistered` — Expo replies `DeviceNotRegistered`; sender calls `DeactivateByToken` and marks `IsActive=false`.
- `TestPushSender_FailsClosedOnHTTP500` — Expo returns 500; sender records the error but does not crash.
- `TestPushSender_HandlesMalformedResponse` — Expo returns non-JSON; sender records parse error and reports failure.
- `TestPushSender_BatchesAt100` — 250-token input becomes 3 batches (100 + 100 + 50) of Expo POSTs.
- `TestPushSender_SendCampaignToUsers_FiltersByUserID` — only tokens whose `user_id` is in the list are reached; other users' tokens skipped.
- `TestPushSender_SendTestToCaller_OnlyCallerTokens` — caller's own tokens only; other users' never touched.
- `TestPushSender_NoTokensIsNotAnError` — empty input returns `sent=0, failed=0` with no error.
- `TestPushSender_AttachesAccessTokenHeader` — when `EXPO_PUSH_ACCESS_TOKEN` is set, request carries `Authorization: Bearer <token>`.

## 2. Resolver RBAC tests (httptest, no Docker needed)

```powershell
go test ./graph/... -count=1 -run "SendPush|SendTest" -v
```

Coverage (7 tests, all pass):

- `TestSendPushCampaign_RejectsAnonymous` → `ErrUnauthenticated`.
- `TestSendPushCampaign_RejectsCustomer` → `ErrForbidden`.
- `TestSendPushCampaign_RejectsStaff` → `ErrForbidden`.
- `TestSendPushCampaign_AdminCanSendAndPayloadReachesExpo` → admin succeeds; fake-Expo httptest server captures exactly the expected `{to, title, body}`.
- `TestSendTestPushNotification_AnyAuthCanFire_ButOnlyOwnTokens` → customer fires, fake-Expo receives one message, target is the customer's own token (never anybody else's).
- `TestSendTestPushNotification_RejectsAnonymous` → `ErrUnauthenticated`.
- `TestSendTestPushNotification_RejectsEmptyTitle` → returns validation error.

## 3. Fake Expo Push API container

Source: [cmd/fake-expo-push/main.go](../../cmd/fake-expo-push/main.go). Built via [cmd/fake-expo-push/Dockerfile](../../cmd/fake-expo-push/Dockerfile). Service `fake-expo-push` in [docker-compose.full.yml](../../docker-compose.full.yml), host port **8095**.

Endpoints:

| Method | Path | Purpose |
|--------|------|---------|
| `POST` | `/--/api/v2/push/send` | Expo-compatible. Per-token mode treats tokens containing `"BAD"` as `DeviceNotRegistered`, everything else as `ok`. |
| `GET` | `/inspect/calls` | Returns `{ count, calls: [[ExpoMessage,…], …] }` — every batch the sender has POSTed since the last reset. |
| `POST` | `/inspect/reset` | Clears the inspector buffer. |
| `GET` | `/inspect/mode` | Current behavior mode. |
| `POST` | `/inspect/mode?value=ok\|device_not_registered\|per-token\|http500` | Switches behavior per test. |
| `GET` | `/healthz` | Liveness. |

`go-core` is wired to it via `EXPO_PUSH_API_URL=http://fake-expo-push:8080/--/api/v2/push/send` in the meta-compose.

## 4. End-to-end smoke through the live docker stack

Bring the stack up:

```powershell
docker compose -f docker-compose.full.yml up -d --build
docker compose -f docker-compose.full.yml ps
```

All 10 services should report `healthy`:

```
ficct-full-admin       ficct-boutique-frontend-angular:full       Up (healthy)  0.0.0.0:4200->80/tcp
ficct-full-ai          ficct-boutique-backend-python:full         Up (healthy)  0.0.0.0:8092->8000/tcp
ficct-full-docs        ficct-boutique-backend-express:full        Up (healthy)  0.0.0.0:8091->8081/tcp
ficct-full-docs-pg     postgres:16-alpine                         Up (healthy)
ficct-full-dynamo      amazon/dynamodb-local:2.5.2                Up
ficct-full-fake-expo   ficct-boutique-fake-expo-push:full         Up (healthy)  0.0.0.0:8095->8080/tcp
ficct-full-go          ficct-boutique-backend-go:full             Up (healthy)  0.0.0.0:8093->8080/tcp
ficct-full-go-pg       postgres:16-alpine                         Up (healthy)
ficct-full-minio       minio/minio:...                            Up (healthy)  0.0.0.0:9010->9000/tcp, 9001
ficct-full-mobile      ficct-boutique-mobile-web:full             Up (healthy)  0.0.0.0:4300->80/tcp
```

### 4.1 Customer flow (login + register + RBAC denial)

```bash
RESP=$(curl -s -X POST http://localhost:8093/graphql -H "Content-Type: application/json" \
  -d '{"query":"mutation { login(input:{email:\"<customer-email>\",password:\"<customer-password>\"}) { accessToken } }"}')
TOKEN=$(echo "$RESP" | node -e "console.log(JSON.parse(require('fs').readFileSync(0,'utf-8')).data.login.accessToken)")

# Register a good Expo push token
curl -s -X POST http://localhost:8093/graphql \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"query":"mutation { registerPushToken(input:{token:\"ExponentPushToken[smoke-good]\",platform:android}) { id platform isActive } }"}'

# List my tokens
curl -s -X POST http://localhost:8093/graphql \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"query":"{ myPushTokens { token platform isActive } }"}'

# Customer attempts campaign → forbidden
curl -s -X POST http://localhost:8093/graphql \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"query":"mutation { sendPushCampaign(input:{title:\"x\",body:\"y\"}) { sent } }"}'
# {"errors":[{"message":"forbidden",...}],"data":null}
```

### 4.2 Admin campaign + fake-Expo verification + bad-token deactivation

```bash
curl -s -X POST http://localhost:8095/inspect/reset

ADMIN=$(curl -s -X POST http://localhost:8093/graphql -H "Content-Type: application/json" \
  -d '{"query":"mutation { login(input:{email:\"<admin-email>\",password:\"<admin-password>\"}) { accessToken } }"}' \
  | node -e "console.log(JSON.parse(require('fs').readFileSync(0,'utf-8')).data.login.accessToken)")

# Register a BAD token (fake Expo will return DeviceNotRegistered for it)
curl -s -X POST http://localhost:8093/graphql \
  -H "Authorization: Bearer $ADMIN" -H "Content-Type: application/json" \
  -d '{"query":"mutation { registerPushToken(input:{token:\"ExponentPushToken[BAD-token]\",platform:android}) { isActive } }"}'

# Admin fires campaign to ALL active tokens (smoke-good + BAD-token)
curl -s -X POST http://localhost:8093/graphql \
  -H "Authorization: Bearer $ADMIN" -H "Content-Type: application/json" \
  -d '{"query":"mutation { sendPushCampaign(input:{title:\"Otoño 2026\",body:\"Hasta 30% off\"}) { sent failed deactivated errors } }"}'
# {"data":{"sendPushCampaign":{"sent":1,"failed":1,"deactivated":1,"errors":["..BAD-token.. is not a registered push notification recipient"]}}}

# Fake Expo captured exactly one HTTP POST, two messages
curl -s http://localhost:8095/inspect/calls
# {"count":1,"calls":[[{"to":"ExponentPushToken[BAD-token]","title":"Otoño 2026",...},
#                      {"to":"ExponentPushToken[smoke-good]","title":"Otoño 2026",...}]]}

# Confirm BAD token now marked is_active=false
curl -s -X POST http://localhost:8093/graphql \
  -H "Authorization: Bearer $ADMIN" -H "Content-Type: application/json" \
  -d '{"query":"{ myPushTokens { token isActive } }"}'
# {"data":{"myPushTokens":[]}}
```

### 4.3 Customer self-test (sendTestPushNotification)

```bash
curl -s -X POST http://localhost:8095/inspect/reset
curl -s -X POST http://localhost:8093/graphql \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"query":"mutation { sendTestPushNotification(title:\"Ping\",body:\"Pong\") { sent failed } }"}'
# {"data":{"sendTestPushNotification":{"sent":1,"failed":0}}}

curl -s http://localhost:8095/inspect/calls
# count=1 with one message: { to: "ExponentPushToken[smoke-good]", title: "Ping", body: "Pong" }
```

## 5. Playwright e2e against live mobile-web

```powershell
cd ../../react/react-native/ficct-boutique-mobile-react-native
npm run e2e
```

6 specs run at viewport 390×844, all pass:

| # | Spec | Validates |
|---|------|-----------|
| 1 | login screen renders | password input is visible |
| 2 | customer logs in and reaches catalog | "Catálogo" text appears post-login |
| 3 | Avisos tab renders the real notification center | header "Centro de notificaciones" visible; **old placeholder copy is GONE**; one of the documented `state-*` testIDs is rendered |
| 4 | session card exposes seeded customer + logout | `<customer-email>` + "Cerrar sesión" both visible |
| 5 | the screen never shows a raw GraphQL error | no `ApolloError` / `Network error` text in DOM |
| 6 | admin login also reaches Avisos with the real screen | admin can navigate same surface |

## 6. Switching from fake Expo to real Expo

In production, leave `EXPO_PUSH_API_URL` unset (defaults to `https://exp.host/--/api/v2/push/send`). Or override explicitly:

```bash
# Cloud Run example
gcloud run services update ficct-go --update-env-vars EXPO_PUSH_API_URL=https://exp.host/--/api/v2/push/send

# Railway example: in the service variables, set
EXPO_PUSH_API_URL=https://exp.host/--/api/v2/push/send
```

For Expo's "Enhanced Push Security" mode, additionally set `EXPO_PUSH_ACCESS_TOKEN` to the project token issued by `eas push:create`.
