# GraphQL API Reference

This document mirrors `graph/schema.graphqls` exactly. Every operation listed here is implemented; nothing else is. Authorization rules come from [graph/queries.go](../../graph/queries.go), [graph/mutations.go](../../graph/mutations.go), and [graph/authz.go](../../graph/authz.go).

Endpoint: `POST /graphql`. Authentication: `Authorization: Bearer <accessToken>` (issued by the `login` mutation). The middleware verifies the bearer if present but does not reject anonymous requests — per-resolver checks decide what's accessible.

## Scalars

| Scalar | Wire format | Notes |
|--------|-------------|-------|
| `UUID` | string (RFC 4122) | parsed/validated in [graph/scalars.go](../../graph/scalars.go) |
| `Time` | RFC 3339 string | always UTC on the wire |

## Enums

```graphql
enum Role { admin staff customer system }
enum SaleStatus { pending confirmed cancelled }
enum OrderStatus { placed preparing ready delivered cancelled }
```

## Object types

The full SDL is short — read it at [graph/schema.graphqls](../../graph/schema.graphqls). Notable shapes:

- `Product.variants: [Variant!]!` resolves via `WHERE product_id = $1` (one query per product, not batched).
- `Variant.stock: [InventoryEntry!]!` resolves all `(variant_id, branch_id)` rows for that variant.
- `InventoryEntry.branch`, `.variant`, `.product` are resolved field-by-field from the resolver's repositories.
- `Sale.items: [SaleItem!]!` is loaded once when the sale resolver is constructed (`ItemsCache`).

---

## Queries

All queries are field resolvers on the root `Resolver`. The "Authz" column refers to the helpers in [graph/authz.go](../../graph/authz.go):

- `requireAuth` — any role, but must be authenticated.
- `requireAdminOrStaff` — must be `admin` or `staff`.
- `(public)` — no authentication required.

| Query | Args | Authz | Behavior |
|-------|------|-------|----------|
| `me` | — | (public — returns null if no bearer) | Resolves the user identified by the `sub` claim. Returns `null` when the bearer is missing or invalid. |
| `product(id: UUID!)` | `id` | (public) | Returns the product or `null` if not found. Inactive products are returned; the customer UI filters at the list layer. |
| `products(category, search, includeInactive, limit, offset)` | all optional | (public; `includeInactive` is silently downgraded to `false` for non-admin/staff) | Defaults: `limit=50`, `offset=0`. `search` matches `name` (ILIKE). |
| `branches` | — | (public) | All branches, ordered by code. |
| `branch(id: UUID!)` | `id` | (public) | Single branch or `null`. |
| `inventoryByBranch(branchId: UUID!)` | `branchId` | requireAdminOrStaff | Per-branch inventory rows. |
| `inventoryEntries(filter, limit, offset)` | filter optional, limits clamped to 1..200, default 25 | requireAdminOrStaff | Paginated inventory grid. `filter` supports `branchId`, `search`, `size`, `color`, `status`, `onlyLowStock`, `includeInactiveVariants`. Returns `InventoryPage { entries, total, limit, offset }`. |
| `sale(id: UUID!)` | `id` | requireAuth | Single sale + its items. |
| `sales(status, limit, offset)` | all optional, default `limit=50` | requireAdminOrStaff | List sales. |
| `order(id: UUID!)` | `id` | requireAuth | Single order. |
| `orders(status, limit, offset)` | all optional, default `limit=50` | requireAdminOrStaff | List orders. |
| `monthlySales(months: Int)` | default 12 | requireAdminOrStaff | Series for the dashboard chart. |
| `popularProducts(limit: Int)` | default 10 | requireAdminOrStaff | Top sellers. |
| `dashboardSummary` | — | requireAdminOrStaff | KPI tiles (today's sales, pending orders, low-stock count, etc.). |
| `myPushTokens` | — | requireAuth | Active Expo push tokens belonging to the caller (used by the mobile notification center). |

---

## Mutations

| Mutation | Authz | Notes |
|----------|-------|-------|
| `login(input: LoginInput!)` | (public) | Verifies argon2id hash, returns `AuthPayload { accessToken, expiresAt, user }`. |
| `createCollection(input)` | admin | — |
| `createProduct(input)` | admin | If `currency` is omitted, defaults to `BOB`. |
| `updateProduct(input)` | admin | Full replacement: `name`, `description`, `category`, `basePrice`, `imageUrl`/`imageDocumentId`, `isActive`. |
| `createVariant(input)` | admin | |
| `upsertInventory(input)` | admin or staff | Inserts the `(variant_id, branch_id)` row or updates `quantity` + `reorder_level`. |
| `createBranch(input)` | admin | |
| `createSale(input)` | requireAuth | Creates a `pending` sale. Validates stock availability without decrementing it. |
| `confirmSale(saleId)` | admin or staff | Single transaction: marks sale `confirmed`, inserts `Order`, decrements inventory with `WHERE quantity >= $`, enqueues `sale.confirmed` event. |
| `deactivateProduct(id)` | admin | Soft delete — sets `is_active = false`. |
| `activateProduct(id)` | admin | Inverse of above. |
| `replaceProductImage(id, newImageDocumentId)` | admin | Updates `image_document_id` only; the actual file lives in Express. |
| `deactivateVariant(id)` | admin | Soft delete a variant. |
| `activateVariant(id)` | admin | |
| `setInventoryStock(variantId, branchId, quantity)` | admin or staff | Direct write. |
| `adjustInventoryStock(variantId, branchId, delta)` | admin or staff | `quantity = quantity + delta`, clamped at 0. |
| `updateInventoryReorderLevel(variantId, branchId, reorderLevel)` | admin or staff | |
| `registerPushToken(input)` | requireAuth | Upserts an Expo push token for the caller. `input = { token, platform, deviceId? }`. `platform` is one of `ios`, `android`, `web`. Same `token` re-registered re-activates the row instead of duplicating. |
| `unregisterPushToken(token)` | requireAuth | Soft-deactivates the matching row (`is_active=false`). Ownership is enforced — a token can only be deactivated by the user who owns it. Returns `true`. |
| `sendTestPushNotification(title, body)` | requireAuth | Sends one push to **every active token owned by the caller**. Cannot reach anyone else. Returns `{ sent, failed, deactivated, errors }`. |
| `sendPushCampaign(input)` | **requireAdmin** | Sends to every active token, or only to the listed `userIds`. `input = { title, body, userIds? }`. Customers and staff get `forbidden`. Returns `{ sent, failed, deactivated, errors }`. The sender deactivates tokens that Expo replies with `DeviceNotRegistered`. |

---

## Error model

There is no structured error code system. Errors are returned via the standard GraphQL `errors[]` envelope with a human-readable `message`. Clients distinguish by:

- `unauthorized` — no/invalid bearer when one was required.
- `forbidden` — wrong role.
- `not found` — repository returned `repository.ErrNotFound`.
- `insufficient stock` — a `confirmSale` transaction was rolled back because the inventory `UPDATE ... WHERE quantity >= $` matched zero rows.
- everything else — a wrapped database or validation error.

Adding structured error codes is on the known-limitations list in the README.

---

## Pagination conventions

- `limit` / `offset` is the only paging style. Default `limit` varies by query (50 for catalogs, 25 for inventory, 12 months for the report).
- `inventoryEntries` is the only query that returns a wrapper object (`InventoryPage` with `total`).
- All other list queries return a bare `[T!]!` and have no total count.

---

## Caching / batching

- No request-scoped DataLoader. Sibling resolvers issue independent queries.
- The `ItemsCache` field on `SaleResolver` is the only intra-request memoization.
- This is intentional given the demo scale; it is the first known limitation in the README.
