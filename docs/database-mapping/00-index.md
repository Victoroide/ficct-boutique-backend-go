# Backend Logical Database Mapping — Index

## Purpose

Logical mapping of every application-owned table/collection in the FICCT
Boutique backend, in the horizontal reference format (one block per table: a
`Tabla "name"` row, an attribute row, a data-type row, then a blank separator).
Schemas are inferred from real implementation sources — SQL migrations, ORM/
service code and the DynamoDB table-creation script — not from a generic
template. No fields were invented.

## Generated workbooks

| File | Database / storage | Source repo | Sheet |
|------|--------------------|-------------|-------|
| `01_mapeo_logico_ms1_go_neondb.xlsx` | MS1 Go Core — NeonDB / PostgreSQL (`ficct_boutique`) | `go/ficct-boutique-backend-go` | `MS1_Go_NeonDB` |
| `02_mapeo_logico_ms3_express_neondb.xlsx` | MS3 Express Documental — NeonDB / PostgreSQL (`ficct_documents`) | `typescript/ficct-boutique-backend-express` | `MS3_Express_NeonDB` |
| `03_mapeo_logico_ms2_dynamodb.xlsx` | MS2 Django AI — AWS DynamoDB (prefix `ficct_`) | `python/django/ficct-boutique-backend-python` | `MS2_DynamoDB` |

## Tables per workbook

**01 — MS1 Go Core (NeonDB):** users, customers, branches, collections,
products, product_variants, inventory, sales, sale_items, orders,
webhook_outbox, refresh_tokens, push_tokens, schema_migrations *(14)*.

**02 — MS3 Express Documental (NeonDB):** documents, document_versions,
hash_ledger, audit_logs, schema_migrations *(5)*.

**03 — MS2 Django AI (DynamoDB):** ficct_product_embeddings,
ficct_forecast_results, ficct_customer_segments, ficct_cluster_runs *(4)*.

## Source files used

- **MS1 Go:** `migrations/0001_init.up.sql`, `0002_product_image_document.up.sql`,
  `0003_variant_active_state.up.sql`, `0004_push_tokens.up.sql`; cross-checked
  against `internal/models`, `internal/repository`, and `graph/schema.graphqls`.
- **MS3 Express:** `migrations/0001_init.sql`, `0002_audit_actor_email.sql`;
  cross-checked against the document/ledger/audit modules.
- **MS2 Django:** `apps/common/dynamodb/schema.py` (key schema) plus the
  `persist`/`upsert` methods in `apps/forecasting/services/forecast_service.py`,
  `apps/clustering/services/clustering_service.py`,
  `apps/ai_catalog/services/catalog_sync_service.py` (non-key attributes).

## Notes / exclusions

- **DynamoDB types:** `S` (string), `N` (number), `L` (list), `M` (map). All four
  tables are `PAY_PER_REQUEST`, single-key (HASH; `forecast_results` adds a RANGE
  sort key). No GSIs or TTL attributes are defined in code, so none are shown.
  Floats (embedding values, RFM metrics, distances, forecast values) are stored
  as strings (`S`) by the services — represented accordingly.
- **No relational FKs in DynamoDB:** `run_id`/`customer_id` are plain attributes,
  not foreign keys.
- **`schema_migrations`** (MS1 & MS3) is the migration-tracking table created by
  each service's migrator; included for completeness as it is app-created.
- **PostgreSQL types** are the real declared types (e.g. `TEXT`, `NUMERIC(12,2)`,
  `DOUBLE PRECISION`, `JSONB`, `BIGINT`) — not normalized to `VARCHAR(n)`.
- **Cross-service references** in MS3 (`owner_user_id`, `uploaded_by`,
  `recorded_by`, `actor_user_id`) point at MS1 users but are **not** declared FKs
  in the documents DB, so they are shown as plain `UUID` (only in-database
  `REFERENCES` are marked `(FK)`).
- **Automation (n8n + Gotenberg):** no application-owned database. n8n uses its
  own internal persistence for workflow state and Gotenberg is stateless; these
  are infrastructure, not part of the logical data model, so **no workbook** was
  created for them and no tables were invented.

## Validation summary

- Each workbook opens successfully and contains exactly **one** worksheet.
- Every table block has: table-name row, bold attribute row, data-type row, and a
  blank separator row.
- No table duplicated; no known application table missing.
- PostgreSQL PK/FK markers present where declared; DynamoDB PK/SK markers present.
- No invented fields. No secrets, credentials or connection strings included.
