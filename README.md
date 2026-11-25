# Solvun

A small storefront stack: a Go/Fiber API, a React storefront, and a React admin
dashboard for a single operator.

```
main.go            API entry point (config, routing, graceful shutdown)
cmd/hashpw/        prints the bcrypt hash for ADMIN_PASSWORD_HASH
catalog/           product content seeded into Postgres on startup
handler/           HTTP handlers (products, orders, payments, shipping, admin auth)
middleware/        admin session guard
repository/        in-memory and Postgres repositories
payment/           Midtrans client
shipping/          Biteship / flat-rate providers
notification/      email senders (Resend, SMTP, log)
invoice/           PDF invoice rendering
sweeper/           expires unpaid orders and releases reserved stock
session/           in-memory admin session store
db/                schema migration
routes/            route table
web/               customer storefront (Vite + React + TanStack Router)
admin/             admin dashboard (Vite + React + TanStack Router)
deploy/            nginx config and deployment steps
```

## Requirements

- Go 1.26+
- Bun 1.3+ (pinned in both `package.json` files as `packageManager`)
- Postgres — optional in development. Without `DATABASE_URL` the API uses
  in-memory repositories and all data is lost on restart.

## Setup

```bash
cp .env.example .env
# edit .env: DATABASE_URL, ADMIN_EMAIL, ADMIN_PASSWORD_HASH, Midtrans keys,
# shipping, email
```

Create the admin password hash:

```bash
go run ./cmd/hashpw          # prompts for the password
```

Copy the printed bcrypt hash into `ADMIN_PASSWORD_HASH`. Admin sign-in stays
disabled while `ADMIN_EMAIL` or `ADMIN_PASSWORD_HASH` is empty; the API logs a
warning at startup in that case.

## Run locally

| Service | Command | URL |
| --- | --- | --- |
| API | `go run .` | `http://localhost:3000` |
| Storefront | `cd web && bun install && bun run dev` | `http://localhost:5174` |
| Admin | `cd admin && bun install && bun run dev` | `http://localhost:5173` |

When `DATABASE_URL` is set, the schema is created and the catalog is seeded at
startup. Both dev servers proxy `/api` to the Go API, so the browser sees a
single origin and no CORS is involved. The storefront also proxies
`/order`, `/orders`, and `/track`, which the Go app renders as HTML.

Sign in to the dashboard with the `ADMIN_EMAIL` / password pair configured on
the API. The dashboard stores nothing in the browser: the API sets an HttpOnly
session cookie and the router asks `GET /admin/session` whether it is still
valid.

## Checks

```bash
# Backend
gofmt -l .
go vet ./...
go test ./...
go test -race -count=1 ./...

# Storefront (tsc + vite build)
cd web && bun run build

# Admin
cd admin && bun run lint
cd admin && bun run format:check
cd admin && bun run build
cd admin && bun run test:browser:install   # once, downloads Chromium
cd admin && bun run test

# Nginx (needs docker): syntax, then real routing against a mock upstream
bash deploy/nginx/test.sh
```

If Vitest reports a missing browser, re-run `bun run test:browser:install` in
`admin/`: the Playwright cache is shared with other projects on the machine and
an install elsewhere can remove the revision this project pinned.

`deploy/nginx/test.sh` accepts `NGINX_IMAGE` so you can check the exact version
the VPS runs, for example `NGINX_IMAGE=nginx:1.27-alpine`.

## Continuous integration

`.github/workflows/ci.yml` runs on every push and pull request to `main`, and
can be triggered manually from the Actions tab. A push to a branch cancels the
previous run for that branch.

| Job | What it does |
| --- | --- |
| `backend` | `gofmt -l`, `go vet`, `go test -race -count=1 ./...`, `go build ./...` |
| `storefront` | `bun install --frozen-lockfile`, `bun run build`, uploads `web/dist` |
| `admin` | `bun install --frozen-lockfile`, `bun run lint`, `format:check`, build, uploads `admin/dist` |
| `admin-tests` | installs Playwright Chromium, runs the Vitest browser suite |
| `nginx` | `nginx -t` with dummy certificates, then a routing test against a mock upstream |
| `ci` | aggregate gate: fails if any of the jobs above did not succeed |

- The frontend jobs use Bun `1.3.14` (pinned in `env.BUN_VERSION`) and cache
  `~/.bun/install/cache` keyed on the matching `bun.lock`.
- Runners are pinned to `ubuntu-24.04`. `ubuntu-latest` moved to Ubuntu 26.04,
  which Playwright 1.59.1 does not support; bump the image and Playwright
  together.
- `--frozen-lockfile` fails the build when `package.json` and the lockfile
  disagree, which catches a forgotten `bun install`.
- The Go job uses the version in `go.mod` and enables the module cache.
- The backend job starts a disposable `postgres:16` service and exposes it as
  `TEST_DATABASE_URL`, so the settlement transaction and its concurrency run
  against real Postgres on every PR.
- Artifacts are kept for 7 days. They are build outputs only; nothing is
  deployed by this workflow.
- `main` is protected by a repository ruleset (`main: require CI and pull
  requests`): changes must arrive through a pull request and the aggregate `CI`
  check must pass. Only the gate is required, so adding or reordering jobs does
  not break the rule.
- The ruleset has no bypass actors, so even the owner cannot push to `main`
  directly. If Actions is unavailable, disable the ruleset under
  **Settings → Rules**, merge, then re-enable it.

## Daily workflow

`main` is protected: every change goes through a pull request and the aggregate
`CI` check has to pass. Direct pushes and force-pushes to `main` are rejected,
so the loop below is the normal way to ship.

### 1. Start from a clean `main`

```bash
git switch main
git pull --ff-only
git switch -c fix/stock-double-release
```

Name branches `fix/…`, `feat/…`, `docs/…`, or `chore/…`. Committing on `main`
locally is harmless but cannot be pushed; branch first.

### 2. Work, then run what CI runs

```bash
# backend
gofmt -l . && go vet ./... && go test ./...

# storefront
cd web && bun run build

# admin
cd admin && bun run lint && bun run format:check && bun run build && bun run test

# only when deploy/nginx changed
bash deploy/nginx/test.sh
```

Admin `lint` and `format:check` are the two checks that fail most often; running
them before pushing saves a two-minute round trip.

### 3. Push and open the pull request

```bash
git add -A
git commit -m "Short summary" -m "Why this change is needed."
git push -u origin HEAD
gh pr create --fill          # title and body come from the commits
gh pr checks --watch         # live status; Ctrl+C stops watching
gh pr view --web             # open it in the browser
```

The PR reports six checks:

| Check | What it covers |
| --- | --- |
| `Backend (Go)` | gofmt, vet, `go test -race`, build |
| `Storefront (web)` | install + production build |
| `Admin (lint + build)` | eslint, Prettier, tsc + build |
| `Admin (browser tests)` | Vitest in Chromium |
| `Nginx config` | `nginx -t` + 16 routing assertions |
| `CI` | the gate; this is the one that blocks the merge |

### 4. If a check fails

```bash
gh pr checks                  # which check is red
gh run view --log-failed      # only the failing steps
```

Fix it locally with the commands from step 2, commit, and push to the same
branch. The old run is cancelled and a new one starts automatically.

### 5. Merge and clean up

```bash
gh pr merge --rebase --delete-branch
git switch main
git pull --ff-only
```

"Rebase and merge" keeps history linear; merge commits and squash are also
allowed by the ruleset, so pick one and stay consistent. `--delete-branch`
removes the branch locally and on the remote; if the local copy survives, drop it
with `git branch -d <branch>`.

### When Actions is unavailable

Nothing can be merged while GitHub is not creating workflow runs, because the
ruleset has no bypass actors. Unblock with **Settings → Rules → `main: require CI
and pull requests` → Disable**, merge, then **Enable** it again. Changes made
while the ruleset is disabled are not verified by CI, so merge something you
already ran locally.

### Rejected operations

| Message | Meaning | Fix |
| --- | --- | --- |
| `push declined due to repository rule violations` | you pushed to `main` | branch, then open a PR |
| `Required status check "CI" is expected` | same cause, no run exists yet | branch, then open a PR |
| `! [rejected] main -> main (non-fast-forward)` | force-push blocked | rebase your branch onto `main` instead |

## Order lifecycle

1. `POST /orders` validates the cart, recomputes prices and shipping from the
   server, reserves stock, and creates a Midtrans Snap transaction. Carts are
   capped at 50 lines with quantities between 1 and 100, so one request cannot
   fan out into thousands of catalog lookups.
2. Midtrans calls `POST /orders/notification`. The API verifies the
   `signature_key` when `MIDTRANS_SERVER_KEY` is set, re-reads the transaction
   from Midtrans, checks the amount, then claims the order and reserves its
   stock in one transaction. Either the order becomes `paid` with every item
   reserved, or `needs_refund` with no stock touched; a crash cannot leave the
   two halves out of sync.
3. A paid order creates a shipment through the configured shipping provider.
   If the provider call fails, a background job retries it with a delay, up to
   `SHIPMENT_RETRY_MAX_ATTEMPTS`; after that the ops digest reports it and the
   order page offers a manual retry.
4. Unpaid orders are expired by the sweeper after `ORDER_PENDING_TTL_MINUTES`
   and their stock is released. Before expiring, the sweeper asks Midtrans for
   the real status: a payment that settled while the webhook was missing is
   confirmed instead of expired. If Midtrans is unreachable the order keeps its
   stock and is retried on the next tick, for at most 24 hours.

Statuses: `pending`, `paid`, `expired`, `cancelled`, `needs_refund`, `refunded`.

## API surface

Public, consumed by the storefront:

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/health` | liveness/readiness probe |
| `GET` | `/store/config` | Midtrans client key and environment |
| `GET` | `/products`, `/products/:id` | catalog |
| `POST` | `/cart` | price and availability check |
| `POST` | `/shipping/rates` | courier quotes |
| `POST` | `/orders` | create order and Snap transaction |
| `GET` | `/orders/:id` | public order status |
| `GET` | `/orders/:id/invoice.pdf` | invoice PDF |
| `POST` | `/orders/:id/cancel` | cancel a pending order |
| `GET`/`POST` | `/order/:id`, `/track` | HTML order and tracking pages |
| `POST` | `/orders/notification` | Midtrans webhook |
| `POST` | `/shipping/notification` | Biteship webhook |

Admin, all requiring a session:

| Method | Path | Purpose |
| --- | --- | --- |
| `POST` | `/admin/session` | sign in |
| `GET` | `/admin/session` | session status |
| `DELETE` | `/admin/session` | sign out |
| `GET` | `/admin/orders`, `/admin/orders/:id` | order list and detail |
| `POST` | `/admin/orders/:id/shipment` | create the shipment for a paid order whose provider call failed |
| `POST` | `/admin/orders/:id/refund` | full refund of a `needs_refund` order through Midtrans |
| `GET` | `/admin/products` | product list, including hidden ones |
| `PATCH` | `/admin/products/:id` | price, stock, visibility |
| `DELETE` | `/admin/products/:id` | remove a product |

### Rate limits

Public endpoints are limited per client address with a sliding one-minute
window. Exceeding a limit returns `429` with `Retry-After` and
`{"error":"Too many requests, please slow down"}`. Buckets are independent per
route, so exhausting the order budget still lets the storefront load products.

| Endpoint | Requests per minute |
| --- | --- |
| `POST /orders` | 5 |
| `POST /orders/:id/cancel` | 5 |
| `GET /orders/:id/invoice.pdf` | 10 |
| `POST /track` | 10 |
| `POST /shipping/rates` | 20 |
| `GET /orders/:id`, `GET /track` | 30 |
| `GET /order/:id`, `GET /store/config`, `POST /cart` | 60 |
| `GET /products`, `GET /products/:id` | 120 |
| `POST /orders/notification`, `POST /shipping/notification` | 120 |

- The client key is `c.IP()`. Behind nginx that is the real address only when
  `TRUST_PROXY=true`; otherwise every request looks like `127.0.0.1`.
- Buckets live in process memory: they reset on restart and, with more than one
  instance, the limit applies per instance.
- `RATE_LIMIT_ENABLED=false` disables all of them, which is handy for load
  tests. Leave it on in production.
- `GET /health` and the admin session endpoints are not limited; sign-in has its
  own limiter in the handler and a second one in nginx.

## Admin sign-in

The admin is a single-operator dashboard, so credentials come from the
environment instead of a user table.

1. `POST /admin/session` with `{email, password}` verifies the bcrypt hash and
   sets `solvun_admin_session`: `HttpOnly`, `SameSite=Lax`, `Secure` by default.
2. `GET /admin/session` tells the dashboard whether the cookie is still valid.
3. `DELETE /admin/session` revokes the token and clears the cookie.
4. Every `/admin/*` route requires a live session. The token is never readable
   from JavaScript, and no API key is compiled into the frontend bundle.

Sessions are stored in memory, so restarting the API signs the operator out.
Failed sign-ins are limited to 5 per client per 15 minutes; nginx adds a second
limit on `POST /api/admin/session` in production.

Because the cookie is `SameSite=Lax`, the dashboard and the API must be served
from the same site. Serving both subdomains behind one nginx host (as `deploy/`
does) satisfies this.

## Operations alerts

Set `OPS_EMAIL` and the API emails a digest of orders that need a human:

- `needs_refund`: the payment was captured but the stock was gone.
- paid orders that have a courier but no shipment, older than
  `OPS_ALERT_GRACE_MINUTES` (the settlement path normally creates it right away).

The digest goes out one minute after startup and then every
`OPS_ALERT_INTERVAL_HOURS`, and only when at least one order is flagged. An empty
inbox therefore means nothing is waiting. It uses the same sender as customer
mail, so with no provider configured it is only logged.

Shipment creation is also retried automatically before an order reaches the
digest: the retrier looks at paid orders without a shipment every
`SHIPMENT_RETRY_INTERVAL_MINUTES`, counts the attempt before calling the provider,
and stops after `SHIPMENT_RETRY_MAX_ATTEMPTS`. All three paths (settlement, the
admin button, and the retrier) share one idempotent create, so a recovering
provider never gets two shipments for the same order.

## Configuration

All variables are documented in `.env.example`. The most important ones:

| Variable | Purpose |
| --- | --- |
| `DATABASE_URL` | Postgres DSN; empty uses in-memory repositories |
| `ADMIN_EMAIL` | operator email for `/admin`; empty disables sign-in |
| `ADMIN_PASSWORD_HASH` | bcrypt hash from `go run ./cmd/hashpw` |
| `ADMIN_SESSION_TTL_MINUTES` | admin session lifetime (default 720) |
| `ADMIN_COOKIE_SECURE` | mark the session cookie `Secure` (default `true`) |
| `MIDTRANS_*` | Snap client/server keys and environment |
| `ORDER_PENDING_TTL_MINUTES` | how long unpaid orders keep stock reserved (default 20; keep above `MIDTRANS_EXPIRY_MINUTES`) |
| `ORDER_SWEEP_INTERVAL_SECONDS` | sweeper interval (default 60) |
| `SHIPPING_PROVIDER` | `biteship`, `flat`, or `disabled` |
| `STORE_BASE_URL` | public store URL used in payment redirects and emails |
| `CORS_ORIGINS` | comma-separated allowed browser origins |
| `TRUST_PROXY` | read client IP from `X-Forwarded-For` (nginx on host) |
| `RATE_LIMIT_ENABLED` | per-client limits on public endpoints (default `true`) |
| `OPS_EMAIL` | recipient for the refund/shipment digest; empty disables it |
| `OPS_ALERT_INTERVAL_HOURS` | digest interval (default 24) |
| `OPS_ALERT_GRACE_MINUTES` | ignore paid orders younger than this (default 10) |
| `SHIPMENT_RETRY_ENABLED` | retry failed shipment creation in the background (default `true`) |
| `SHIPMENT_RETRY_MAX_ATTEMPTS` | attempts per order before giving up (default 5) |
| `SHIPMENT_RETRY_INTERVAL_MINUTES` | delay between attempts and the job interval (default 10) |
| `BITESHIP_WEBHOOK_TOKEN` | shared secret required on the shipping webhook |

## Deployment

A ready-to-use nginx setup lives in `deploy/`: one host, `shop.` and `admin.`
subdomains, `/api` proxied to the Go service on `127.0.0.1:3001`. See
`deploy/README.md` for DNS, certificates, upload commands, and verification.

- The proxy strips `/api` and forwards `/order`, `/orders`, and `/track`
  untouched, because the Go app renders those pages.
- Set `CORS_ORIGINS` to the real store/admin origins; the default is `*`.
- Set `TRUST_PROXY=true` when nginx runs on the same host, so logs and the
  sign-in limiter see the real client IP.
- Point Midtrans and Biteship webhooks at the public API:
  `/orders/notification` and
  `/shipping/notification?token=<BITESHIP_WEBHOOK_TOKEN>`.
- Back up the database before deploying: migrations run at startup and are not
  rolled back automatically.
- The deployment recipe in `.monocode/` is gitignored, so it lives only on the
  machine that deploys.

## Known gaps

- Admin sessions live in memory: a restart signs the operator out, and running
  more than one API instance would need a shared session store.
- Biteship does not sign its webhooks. With `BITESHIP_WEBHOOK_TOKEN` set, the
  URL carries a shared secret and the payload is still re-read from the provider:
  anyone who learns the URL can trigger a status refresh, but cannot forge a
  state.
- Settlement claims the order and reserves stock in one Postgres transaction.
  The in-memory repository used for dev runs the two steps in sequence, so it is
  not crash-atomic; production uses the transactional path.
- Refunds are issued from the order page and only cover the full amount; partial
  refunds and automatic refund reconciliation are not implemented.
- Shipment creation is retried automatically a bounded number of times; after
  that the digest lists the order and the order page offers a manual retry.
  The retry delay is fixed, not exponential.
- Admin lists load every row. Orders accept a `?status=` filter, but there is no
  pagination anywhere and the product list is filtered in the browser.
- `GET /orders/:id` is a capability URL: anyone holding the link can read the
  order, including customer details. UUIDs make guessing impractical, but the
  link should still be treated as a secret.
- Nothing is deployed by CI. The workflow validates a commit; shipping it is
  still a manual `rsync` plus service restart.
