# FICCT Boutique — C4 + UML 2.5 Diagram Suite

## 1. Purpose
This suite documents the FICCT Boutique platform with the C4 model (Context →
Containers → Components → Code) and a UML 2.5 set (structural + behavioural).
Behaviour and interaction diagrams (activity, sequence, communication) are
generated **per use case** so each CU01–CU12 has its own grounded model. Every
diagram is derived from the real code, the implemented production flow and the
deployed infrastructure — not generic placeholders.

## 2. Architecture summary
- **MS1 Go Core** — Railway, NeonDB PostgreSQL, GraphQL. Products, variants,
  multi-branch inventory, sales/orders, BI/reports, push tokens, webhook outbox
  and a signed `sale.confirmed` webhook to n8n.
- **Automation** — Railway, self-hosted **n8n + Gotenberg** (no n8n Cloud).
  Validates the signed webhook (HMAC), renders the invoice PDF with Gotenberg
  (internal), uploads it to MS3, and emails it via **AWS SES** from `dev@ficct.com`.
- **MS3 Express** — AWS Lambda + API Gateway, NeonDB, **S3 `ficct-boutique-documents`**.
  Documents/invoices, upload-request, presigned S3 upload, confirm, audit log and
  a tamper-evident **hash-chain ledger** (verify/ledger endpoints).
- **MS2 Django AI** — GCP Cloud Run, **AWS DynamoDB**. Visual similarity
  (classical CV: perceptual hash + HSV histogram + cosine — *not* a neural net),
  demand forecasting (Holt), and customer clustering/K-Means.
- **Angular Admin** — admin web (Go GraphQL + MS3 docs + MS2 AI): BI dashboards,
  catalogue/inventory management, documents console with hash-ledger verification,
  audit and AI analytics.
- **React Native Mobile** — customer app: catalogue, cart/order, camera visual
  search, GPS branches, push-token registration and segment notifications.
- **Cloud**: S3 `ficct-boutique-documents`; SES sender `dev@ficct.com`; DynamoDB
  `ficct_product_embeddings`, `ficct_forecast_results`, `ficct_customer_segments`,
  `ficct_cluster_runs`.

## 3. Actors
1. **Administrador** — rol admin (Angular Admin).
2. **Personal Logístico** — rol staff (inventario/documentos; cuenta de servicio del flujo n8n).
3. **Cliente** — rol customer (app React Native).
4. **Subsistema Documental** — actor-sistema = MS3 (documentos/facturas + ledger).

## 4. Use cases
- **CU01** Autenticar usuario
- **CU02** Gestionar roles y permisos
- **CU03** Administrar catálogo de productos
- **CU04** Controlar inventario multisucursal
- **CU05** Registrar transacción de venta
- **CU06** Orquestar estados del pedido
- **CU07** Emitir comprobante comercial digital
- **CU08** Consultar proyección de demanda
- **CU09** Segmentar compradores por hábitos
- **CU10** Buscar prendas por similitud visual
- **CU11** Gestionar bolsa de compras móvil
- **CU12** Despachar notificaciones automatizadas

## 5. Global diagrams summarize the system
The C4 and global UML diagrams give the platform-wide view (context, containers,
components, code, classes with ECB stereotypes, packages, components, deployment,
composite structure, profile, objects, use cases, state machine, interaction
overview and timing). They summarize the whole platform.

## 6. Behaviour is generated per use case
The global diagrams **do not** replace the per-use-case models: every CU01–CU12
has its own **activity**, **sequence** and **communication** diagram, each grounded
in the implemented flow.

## 7. Global diagrams
| # | Diagram | PUML | PNG |
|---|---|---|---|
| 01 | C4 Context | [puml](c4/01-c4-context.puml) | [png](c4/01-c4-context.png) |
| 02 | C4 Containers | [puml](c4/02-c4-containers.puml) | [png](c4/02-c4-containers.png) |
| 03 | C4 Components (Go Core) | [puml](c4/03-c4-components.puml) | [png](c4/03-c4-components.png) |
| 04 | C4 Code View (invoice/ledger) | [puml](c4/04-c4-code-view.puml) | [png](c4/04-c4-code-view.png) |
| 05 | Class (ECB stereotypes) | [puml](uml/global/05-class-ecb.puml) | [png](uml/global/05-class-ecb.png) |
| 06 | Package | [puml](uml/global/06-package.puml) | [png](uml/global/06-package.png) |
| 07 | Component (UML) | [puml](uml/global/07-component-uml.puml) | [png](uml/global/07-component-uml.png) |
| 08 | Deployment | [puml](uml/global/08-deployment.puml) | [png](uml/global/08-deployment.png) |
| 09 | Composite structure | [puml](uml/global/09-composite-structure.puml) | [png](uml/global/09-composite-structure.png) |
| 10 | Profile (custom stereotypes) | [puml](uml/global/10-profile.puml) | [png](uml/global/10-profile.png) |
| 11 | Object | [puml](uml/global/11-object.puml) | [png](uml/global/11-object.png) |
| 12 | Use case (4 actores, CU01–CU12) | [puml](uml/global/12-use-case.puml) | [png](uml/global/12-use-case.png) |
| 14 | State machine (order/invoice/document) | [puml](uml/global/14-state-machine-order-invoice.puml) | [png](uml/global/14-state-machine-order-invoice.png) |
| 17 | Interaction overview | [puml](uml/global/17-interaction-overview.puml) | [png](uml/global/17-interaction-overview.png) |
| 18 | Timing (production invoice flow) | [puml](uml/global/18-timing-production-flow.puml) | [png](uml/global/18-timing-production-flow.png) |

## 8. Per-use-case diagrams
Each folder under `uml/use-cases/` contains `activity`, `sequence` and
`communication` (`.puml` + `.png`).

| Use case | Folder |
|---|---|
| CU01 Autenticar usuario | [CU01](uml/use-cases/CU01-autenticar-usuario/) |
| CU02 Gestionar roles y permisos | [CU02](uml/use-cases/CU02-gestionar-roles-y-permisos/) |
| CU03 Administrar catálogo de productos | [CU03](uml/use-cases/CU03-administrar-catalogo-de-productos/) |
| CU04 Controlar inventario multisucursal | [CU04](uml/use-cases/CU04-controlar-inventario-multisucursal/) |
| CU05 Registrar transacción de venta | [CU05](uml/use-cases/CU05-registrar-transaccion-de-venta/) |
| CU06 Orquestar estados del pedido | [CU06](uml/use-cases/CU06-orquestar-estados-del-pedido/) |
| CU07 Emitir comprobante comercial digital | [CU07](uml/use-cases/CU07-emitir-comprobante-comercial-digital/) |
| CU08 Consultar proyección de demanda | [CU08](uml/use-cases/CU08-consultar-proyeccion-de-demanda/) |
| CU09 Segmentar compradores por hábitos | [CU09](uml/use-cases/CU09-segmentar-compradores-por-habitos/) |
| CU10 Buscar prendas por similitud visual | [CU10](uml/use-cases/CU10-buscar-prendas-por-similitud-visual/) |
| CU11 Gestionar bolsa de compras móvil | [CU11](uml/use-cases/CU11-gestionar-bolsa-de-compras-movil/) |
| CU12 Despachar notificaciones automatizadas | [CU12](uml/use-cases/CU12-despachar-notificaciones-automatizadas/) |

Each per-use-case sequence follows the **Actor → Boundary → Control → Entity**
topology; communication diagrams show the same collaboration with numbered
messages; activity diagrams use swimlanes for the real control/data flow.

## 10. Embedded styles
Every `.puml` embeds its own `skinparam`/style block (monochrome strict-UML).
There is **no** shared style file and **no** `.iuml` include.

## 11. No external includes
No `!include`, no `!includeurl`, no online rendering. All diagrams are standalone
and were rendered offline with a local `plantuml.jar` (bundled Graphviz). No
visible `title` is used — the file name is the title.
