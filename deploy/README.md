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
| Biteship | `https://shop.example.com/api/shipping/notification` |

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

## Rollback

- **Frontends**: re-run the `rsync` from the previous commit's build.
- **API**: `git checkout --detach <previous-sha>` and restart the service.
- **Database**: migrations run at startup and are not rolled back
  automatically, so take a backup before deploying.
