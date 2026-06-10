# Use-Case Alignment — Evidence (2026-06-10)

Goal: bring the running system into alignment with the existing PlantUML use-case
diagrams (the diagrams are the spec and were **not** modified). Focus: CU06 and
CU09 UI; re-verify CU02 and CU08.

## Behavioral gap analysis (Phase 1)

| Use case | Backend (before) | Angular UI (before) | Action taken |
|----------|------------------|---------------------|--------------|
| **CU02** Gestionar roles y permisos | Implemented (RS256 JWT + per-resolver guards in Go; `requireRoles` in MS3; `IsAdminOrStaff` in MS2) | Implemented (login + route/role guards) | Verified only — diagram describes cross-cutting RBAC, no management screen. No code added. |
| **CU06** Orquestar estados del pedido | Implemented (`confirmSale`, state machine, `ORD-YYYYMMDD-####`, webhook outbox + HMAC dispatcher) | **Missing** | Built the Angular `orders` feature (list sales/orders, confirm pending sale, order detail + lifecycle). |
| **CU08** Consultar proyección de demanda | Implemented (DRF `forecasting/run`, Holt linear, DynamoDB) | Implemented (`ai-analytics`) | Verified only. |
| **CU09** Segmentar compradores por hábitos | Implemented (KMeans RFM) but persisted segments lacked RFM fields | Partial (no `GET segments`, chart was distance-vs-index) | MS2: persist + return `recency_days/frequency/monetary`. Angular: call `GET /clustering/segments/` and render a true RFM scatter. |

## What was implemented

- **CU06 (Angular)** — new `src/app/features/orders/` (`orders.service.ts`,
  `orders.component.ts/.html`), route `/orders` behind `roleGuard(['admin','staff'])`,
  nav item "Pedidos". Lists sales and orders, confirms a pending sale
  (`confirmSale` → creates the Order), shows sale/order detail and the read-only
  order lifecycle. Order-state advancement beyond creation is read-only because
  the Go core exposes no mutation for it and the CU06 diagrams do not require one.
- **CU09 (MS2 Django)** — `CustomerSegment` now carries `recency_days`,
  `frequency`, `monetary`; `persist()` writes them to DynamoDB; the run/segments
  views and `SegmentSerializer` return them; unit test asserts the RFM fields.
- **CU09 (Angular)** — `ai-analytics` now reads back persisted segments via
  `GET /clustering/segments/` after a run, renders a true RFM scatter
  (x=Recency, y=Monetary, bubble size=Frequency, colour=cluster) and an RFM
  table, plus a "Ver segmentos guardados" action and an empty state.

## Runtime verification (Phase 6)

Stack: `docker-compose.e2e.yml` (local-only, not committed) — go-core (8093) +
postgres + fake-expo, django-ai (8094, on 8094 because host 8092 was occupied by
an unrelated container) + dynamodb-local, Angular **dev** build (4200). JWT keys
mounted from `.tools/keys`; throwaway seed credentials supplied via env (never
committed).

- **CU02** — logged in as the seeded admin; role-gated nav and routes
  (`/orders`, `/ai-analytics`, `/audit`) resolved correctly. JWT issued by Go
  (RS256) and accepted by MS2.
- **CU06** — created 3 pending sales via GraphQL, then confirmed one through the
  UI: `confirmSale` produced **Order `ORD-20260610-5783`, status `placed`**, and
  the sale flipped to `confirmed`. Order-code format matches the diagram exactly.
- **CU08** — "Generar pronóstico" called `POST /forecasting/run/` and rendered
  the Holt-linear demand line chart.
- **CU09** — "Generar segmentos" called `POST /clustering/run/` then
  `GET /clustering/segments/`; KMeans separated the 6 RFM customers into two
  clusters and the UI rendered the RFM scatter + RFM table.
- **Console:** no errors or warnings (only Angular dev-mode + Apollo devtools LOG lines).

Screenshots in `screenshots/`:
`cu06-orders-list-desktop.png`, `cu06-order-detail-desktop.png`,
`cu06-order-state-action-desktop.png`, `cu06-orders-pedidos-tab-desktop.png`,
`cu06-orders-responsive.png`, `cu09-ai-analytics-forecast.png`,
`cu09-ai-analytics-segments-chart.png`, `cu09-ai-analytics-segments-table.png`,
`cu09-ai-analytics-responsive.png`.

## Local checks (Phase 5)

- **Angular**: production `npm run build` exit 0; app `tsc --noEmit` exit 0.
- **MS2**: `compileall` ok; `pytest apps/clustering` 2 passed (incl. new RFM
  asserts); `manage.py check` 0 issues.
- **Go**: `gofmt -l` clean; `go build ./...`, `go vet ./...`, `go test ./...` all ok.

## Class diagrams

Backend-only `class.puml` (+ rendered `.png` via PlantUML 1.2024.3 + Graphviz)
added for every use case CU01–CU12 under `docs/diagrams/uml/use-cases/<CU>/`,
matching the existing monochrome strict-UML ECB style.
