# Solvun

Solvun is a small storefront stack: a Go/Fiber API, a React storefront, and a
React admin dashboard.

| Path       | What it is                                              |
| ---------- | ------------------------------------------------------- |
| `main.go`  | Go API (products, orders, payments, shipping, emails)   |
| `web/`     | Customer storefront (Vite + React + TanStack Router)     |
| `admin/`   | Admin dashboard (Vite + React + TanStack Router)         |
| `catalog/` | Product content seeded into Postgres on startup          |

## Requirements

- Go 1.26+
- Node 20+ and Bun 1.3+
- Postgres (optional in development; without `DATABASE_URL` the API uses
  in-memory repositories and data is lost on restart)

## Setup

```bash
cp .env.example .env
# edit .env: DATABASE_URL, ADMIN_API_KEY, Midtrans keys, shipping, email
```

Generate an admin key with:

```bash
openssl rand -hex 24
```

### Run the API

```bash
go run .
```

The API listens on `PORT` (default `3000`). When `DATABASE_URL` is set, the
schema is created and the catalog is seeded automatically at startup.

### Run the storefront

```bash
cd web
bun install
bun run dev
```

The dev server runs on `http://localhost:5174` and proxies `/api`, `/order`,
`/orders`, and `/track` to the Go API.

### Run the admin dashboard

```bash
cd admin
bun install
VITE_ADMIN_API_KEY=<same value as ADMIN_API_KEY> bun run dev
```

## Tests

```bash
go test ./...
go vet ./...
```

```bash
cd web && bun run build
cd admin && bun run test
```

## Continuous integration

`.github/workflows/ci.yml` runs on every push and pull request to `main`, and
can be triggered manually from the Actions tab. A push to a branch cancels the
previous run for that branch.

| Job            | What it does                                                                 |
| -------------- | ---------------------------------------------------------------------------- |
| `backend`      | `gofmt -l`, `go vet`, `go test -race -count=1 ./...`, `go build ./...`        |
| `storefront`   | `bun install --frozen-lockfile`, `bun run build`, uploads `web/dist`          |
| `admin`        | `bun install --frozen-lockfile`, `bun run lint`, `format:check`, build, uploads `admin/dist` |
| `admin-tests`  | installs Playwright Chromium, runs the Vitest browser suite                   |
| `ci`           | aggregate gate: fails if any of the jobs above did not succeed                |

Notes:

- The frontend jobs use Bun `1.3.14` (pinned in `env.BUN_VERSION`) and cache
  `~/.bun/install/cache` keyed on the matching `bun.lock`, so installs are fast
  and reproducible.
- `--frozen-lockfile` fails the build when `package.json` and the lockfile
  disagree, which catches forgotten `bun install` runs.
- The Go job uses the version in `go.mod` and enables the module cache.
- Artifacts are kept for 7 days. They are build outputs only; nothing is
  deployed by this workflow.
- Keep the `ci` job as the single required status check in branch protection so
  adding or reordering jobs does not break the rule.

## Order lifecycle

1. `POST /orders` validates the cart, recomputes prices and shipping from the
   server, reserves stock, and creates a Midtrans Snap transaction.
2. Midtrans calls `POST /orders/notification`; the API re-reads the transaction
   status from Midtrans and marks the order `paid`.
3. A paid order creates a shipment through the configured shipping provider.
4. Unpaid orders are expired by the sweeper after `ORDER_PENDING_TTL_MINUTES`
   and their stock is released.

Order statuses: `pending`, `paid`, `expired`, `cancelled`, `needs_refund`.

## Configuration

All variables are documented in `.env.example`. The most important ones:

| Variable                      | Purpose                                              |
| ----------------------------- | ---------------------------------------------------- |
| `DATABASE_URL`                | Postgres DSN; empty uses in-memory repositories       |
| `ADMIN_API_KEY`               | `X-API-Key` for `/admin/*`; empty rejects all calls   |
| `MIDTRANS_*`                  | Snap client/server keys and environment               |
| `ORDER_PENDING_TTL_MINUTES`   | How long unpaid orders keep stock reserved (default 20) |
| `ORDER_SWEEP_INTERVAL_SECONDS`| Sweeper interval (default 60)                         |
| `SHIPPING_PROVIDER`           | `biteship`, `flat`, or `disabled`                     |
| `STORE_BASE_URL`              | Public store URL used in payment redirects and emails |
| `CORS_ORIGINS`                | Comma-separated allowed browser origins               |

## Deployment notes

- The storefront and admin call the API through `/api`; put a reverse proxy in
  front of both and rewrite `/api/*` to the Go service.
- Set `CORS_ORIGINS` to the real store/admin origins; the default is `*`.
- Point Midtrans and Biteship webhooks at the public API:
  `/orders/notification` and `/shipping/notification`.
- Back up the database before deploying: migrations run at startup and are not
  rolled back automatically.

## Known gaps

- Admin sign-in is still a mock; the admin API key is compiled into the
  frontend bundle. Replace it with server-side auth before exposing the admin
  publicly.
- No rate limiting on public endpoints yet.
- Admin lists load every order/product (no pagination).
