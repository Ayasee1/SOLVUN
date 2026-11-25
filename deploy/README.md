# Deploy notes

Shape of the deployment:

- **API**: Go binary as a `systemd --user` service on `127.0.0.1:3001`
  (see `.monocode/deploy-recipe.sh`).
- **Storefront**: static build in `/var/www/solvun/web`.
- **Admin**: static build in `/var/www/solvun/admin`.
- **nginx**: one host, two subdomains, `/api` proxied to the Go service.

Two subdomains are used instead of one domain with paths because the admin
bundle is built with `base: '/'`. Both hosts proxy `/api`, so the browser sees a
single origin per host and the `HttpOnly` admin session cookie is always sent.

## Files

```
deploy/nginx/solvun.conf                      # the site config
deploy/nginx/snippets/proxy-api.conf          # shared proxy headers/timeouts
deploy/nginx/snippets/security-shop.conf      # headers + CSP for the storefront
deploy/nginx/snippets/security-admin.conf     # headers + CSP for the dashboard
```

`add_header` does not merge: a location that sets its own `add_header` must
include the security snippet again. The config already does this for
`/index.html`; keep that in mind when adding locations.

## Validate before deploying

```bash
bash deploy/nginx/test.sh
```

The script needs docker and does two things:

1. `nginx -t` on the untouched config with dummy certificates, so TLS
   directives and every `include` are checked.
2. Starts a plain-HTTP copy with a mock upstream and asserts the routing: `/api`
   prefix stripping, `/order` `/orders` `/track` passthrough, the SPA fallback,
   cache headers, and that the storefront cannot reach `/api/admin/*`.

CI runs the same script, so a config change cannot land untested. To check the
version the VPS actually runs:

```bash
NGINX_IMAGE=nginx:1.27-alpine bash deploy/nginx/test.sh
```

## 1. DNS

Point both names at the server before requesting certificates:

```
shop.example.com    A/AAAA  <server ip>
admin.example.com   A/AAAA  <server ip>
```

## 2. Build and upload the frontends

```bash
cd web && bun install --frozen-lockfile && bun run build
cd ../admin && bun install --frozen-lockfile && bun run build
```

```bash
rsync -a --delete web/dist/   user@host:/var/www/solvun/web/
rsync -a --delete admin/dist/ user@host:/var/www/solvun/admin/
```

`--delete` matters: old hashed assets would otherwise pile up forever.

## 3. Certificates (first run only)

nginx will not start with the final config until the certificate files exist.
Bootstrap with an HTTP-only block, then request the certificates:

```nginx
# /etc/nginx/sites-available/solvun-bootstrap.conf
server {
    listen 80;
    server_name shop.example.com admin.example.com;
    location /.well-known/acme-challenge/ { root /var/www/certbot; }
    location / { return 404; }
}
```

```bash
sudo mkdir -p /var/www/certbot
sudo ln -sf /etc/nginx/sites-available/solvun-bootstrap.conf /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx

sudo certbot certonly --webroot -w /var/www/certbot \
  -d shop.example.com -d admin.example.com
```

Remove the bootstrap symlink afterwards. Renewal keeps working because the final
config still serves `/.well-known/acme-challenge/` over HTTP.

## 4. Install the nginx config

```bash
sudo cp deploy/nginx/solvun.conf /etc/nginx/sites-available/solvun.conf
sudo mkdir -p /etc/nginx/snippets
sudo cp deploy/nginx/snippets/*.conf /etc/nginx/snippets/

sudo ln -sf /etc/nginx/sites-available/solvun.conf /etc/nginx/sites-enabled/
sudo rm -f /etc/nginx/sites-enabled/default

sudo nginx -t && sudo systemctl reload nginx
```

Replace `shop.example.com`, `admin.example.com`, and the `/var/www/solvun/*`
paths first.

`http2 on;` needs nginx 1.25.1 or newer. On older builds use
`listen 443 ssl http2;` instead.

## 5. Backend environment

Add these to the service environment (see `.env.example`):

```env
# nginx runs on the same host, so trust its X-Forwarded-For header.
TRUST_PROXY=true

# Public URL used in payment redirects, emails, and order links.
STORE_BASE_URL=https://shop.example.com

# Browsers now call the API through nginx on these origins.
CORS_ORIGINS=https://shop.example.com,https://admin.example.com

ADMIN_EMAIL=owner@example.com
ADMIN_PASSWORD_HASH=<output of: go run ./cmd/hashpw>
ADMIN_COOKIE_SECURE=true

# Shared secret for the Biteship webhook (see step 6).
BITESHIP_WEBHOOK_TOKEN=<random string>

# Retry failed shipments and email a digest of orders that need a human.
SHIPMENT_RETRY_ENABLED=true
OPS_EMAIL=owner@example.com
```

`TRUST_PROXY=true` is safe only because the API listens on `127.0.0.1` and
nginx is the sole client. If the API is ever exposed directly, set it back to
`false`; otherwise callers can spoof `X-Forwarded-For` and dodge the sign-in
rate limit.

## 6. Webhooks

Point the providers at the public host, not at the internal port:

| Provider | URL |
| --- | --- |
| Midtrans | `https://shop.example.com/api/orders/notification` |
| Biteship | `https://shop.example.com/api/shipping/notification?token=<BITESHIP_WEBHOOK_TOKEN>` |

## 7. Verify

```bash
# HSTS and CSP present
curl -sI https://shop.example.com/ | grep -iE 'strict-transport|content-security'

# API reachable through the proxy
curl -s https://shop.example.com/api/products | head -c 120; echo

# Order pages still come from the Go app
curl -sI https://shop.example.com/track | head -1

# Admin API is closed without a session
curl -s -o /dev/null -w '%{http_code}\n' https://admin.example.com/api/admin/orders   # 401

# Sign-in sets the cookie
curl -s -D - -o /dev/null -X POST https://admin.example.com/api/admin/session \
  -H 'Content-Type: application/json' \
  -d '{"email":"owner@example.com","password":"<password>"}' | grep -i set-cookie

# The storefront must not expose admin endpoints
curl -s -o /dev/null -w '%{http_code}\n' https://shop.example.com/api/admin/orders   # 404
```

## Optional: lock the dashboard to known networks

The admin is a single-operator dashboard. If its hostname is public, add an
address allowlist to the admin `server` block; everyone else is rejected by
nginx before the app is reached:

```nginx
allow 203.0.113.10;
allow 10.0.0.0/8;
deny all;
```

Keep the app-level session auth regardless: the allowlist is a second layer,
not a replacement.

## Continuous deployment

`.github/workflows/deploy.yml` deploys every commit that passes CI on `main`.

1. The CI workflow finishes successfully on a push to `main`.
2. The deploy workflow checks out that exact commit and downloads the
   `storefront-dist` and `admin-dist` artifacts from the CI run.
3. It runs `deploy/remote-deploy.sh <sha> <path>` over SSH. The script fetches
   the commit, checks `gofmt` and `go vet`, builds, restarts the user service and
   waits for `/health`.
4. If health fails, the script restores the previous commit **and** the previous
   binary, restarts and exits non-zero. The frontends are then left untouched.
5. Only after a healthy API does it rsync `web/dist` and `admin/dist` to the
   nginx roots.
6. Deploys are serialized (`concurrency: deploy-production`,
   `cancel-in-progress: false`): a new push queues behind a running deploy
   instead of interrupting it.

The job only runs when `DEPLOY_HOST` is set, so it stays skipped (green) until
the repository is configured.

### One-time setup on the VPS

```bash
# backend-only clone; do not run bun install here
git clone https://github.com/Ayasee1/SOLVUN.git ~/developer/golearn-staging

# the user service must exist and start at boot without a login session
systemctl --user enable golearn-staging
loginctl enable-linger "$USER"

# frontend roots writable by the deploy user (no sudo in the workflow)
sudo mkdir -p /var/www/solvun/web /var/www/solvun/admin
sudo chown -R "$USER": /var/www/solvun

# key used by GitHub Actions
ssh-keygen -t ed25519 -C github-actions-deploy -f ~/.ssh/deploy_key -N ""
cat ~/.ssh/deploy_key.pub >> ~/.ssh/authorized_keys

# pinned host key
ssh-keyscan -H <server-host>
```

The service unit's `ExecStart` must point at the `golearn` binary inside the
clone (the script rebuilds it in place).

### GitHub settings

Create the environment **production** (Settings → Environments). Set the
**repository** variables (Settings → Secrets and variables → Actions →
Variables) — the workflow's `if` reads `DEPLOY_HOST` from there, so it must not
live only in the environment:

| Variable | Example |
| --- | --- |
| `DEPLOY_HOST` | `vps.example.com` |
| `DEPLOY_USER` | `ikhsan` |
| `DEPLOY_PATH` | `/home/ikhsan/developer/golearn-staging` |
| `DEPLOY_WEB_ROOT` | `/var/www/solvun/web` |
| `DEPLOY_ADMIN_ROOT` | `/var/www/solvun/admin` |

Secrets (repository or environment level):

| Secret | Contents |
| --- | --- |
| `DEPLOY_SSH_KEY` | the private key from `~/.ssh/deploy_key` |
| `DEPLOY_KNOWN_HOSTS` | output of `ssh-keyscan -H <server-host>` |

Adding required reviewers to the environment turns the automatic deploy into a
manual approval gate without changing the workflow.

## Rollback

- **API during a deploy**: automatic. The script restores the previous commit
  and binary when `/health` fails, then exits non-zero.
- **API after the fact**: `bash deploy/remote-deploy.sh <previous-sha> <path>`
  on the server, or run the same command locally over SSH.
- **Frontends**: redeploy the previous commit (revert it on `main` and let CI
  deploy, or rsync that build manually).
- **Database**: migrations run at startup and are not rolled back
  automatically, so take a backup before deploying.
