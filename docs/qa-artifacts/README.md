# QA artifacts

This folder contains Playwright/MCP-browser screenshots that were collected during manual end-to-end verification. They are kept here (rather than in the repo root, where they used to sprawl) as visual evidence that specific screens render correctly.

These are **demo evidence**, not an automated test suite. There is no Playwright suite in this repo; the `.playwright-mcp/` folder is gitignored and ephemeral.

## Folders

```
admin-ui/        Angular admin desktop captures at 1440x900
customer-ui/    React Native customer web at 390x844 (iPhone 12-ish viewport)
inventory/      Inventory grid (admin)
responsive/     Same admin screens at 390x844 (phone) and 768x1024 (tablet)
```

## Files

| File | Captured from | Notes |
|------|---------------|-------|
| `admin-ui/admin-dashboard-1440x900.png` | Angular admin, `/admin/dashboard` | Logged in as `admin@ficct.local`. |
| `admin-ui/admin-products-1440x900.png` | Angular admin, `/admin/products` | Product list. |
| `admin-ui/admin-products-with-actions-1440x900.png` | Same screen with row actions visible. | |
| `inventory/admin-inventory-1440x900.png` | Angular admin, `/admin/inventory` | Per-branch grid with the search/filter inputs. |
| `customer-ui/customer-catalog-after-login-390x844.png` | React Native web at `localhost:4300`, signed in as `cliente@ficct.local`. | |
| `customer-ui/customer-catalog-with-images-390x844.png` | Same screen showing the seeded `/static/products/*.svg` placeholders. | |
| `responsive/admin-dashboard-390x844.png` | Admin dashboard at phone width. | |
| `responsive/admin-products-390x844.png` | Admin products at phone width. | |
| `responsive/admin-products-768x1024.png` | Admin products at tablet width. | |

## How to refresh

Run the full system (`docker compose -f docker-compose.full.yml up -d --build`), open the relevant URL in a Chromium/MCP browser session, and `Save image as ...` into the appropriate folder using the same naming convention: `<surface>-<viewport>.png`. Do **not** drop screenshots in the repo root — `.gitignore` blocks `/*.png` to prevent that regression.
