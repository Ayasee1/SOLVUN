#!/usr/bin/env bash
#
# Validates deploy/nginx with a real nginx binary.
#
#   1. Runs `nginx -t` on the untouched config using dummy certificates, so the
#      TLS block and every `include` are checked.
#   2. Rewrites a throwaway copy to plain HTTP with an in-container mock
#      upstream, starts it, and asserts the routing rules that matter:
#      /api prefix stripping, /order and /orders passthrough, the SPA fallback,
#      cache headers, and the shop/admin split.
#
# Usage:
#   bash deploy/nginx/test.sh
#   NGINX_IMAGE=nginx:1.27-alpine bash deploy/nginx/test.sh   # match the server
#
# Requires docker. Nothing outside a temp directory is touched.

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
NGINX_DIR="$SCRIPT_DIR"
IMAGE="${NGINX_IMAGE:-nginx:alpine}"
HOSTS=(shop.example.com admin.example.com)

info() { printf '\n== %s\n' "$*"; }
fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }

command -v docker >/dev/null 2>&1 || fail "docker is required"
docker info >/dev/null 2>&1 || fail "the docker daemon is not running"
command -v openssl >/dev/null 2>&1 || fail "openssl is required"

TMP=$(mktemp -d)
CONTAINER="solvun-nginx-test-$$"
PORT=$(( (RANDOM % 20000) + 20000 ))

cleanup() {
    docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
    rm -rf "$TMP"
}
trap cleanup EXIT

mkdir -p "$TMP/conf.d" "$TMP/testconf" "$TMP/web/assets" "$TMP/admin/assets"

for host in "${HOSTS[@]}"; do
    mkdir -p "$TMP/live/$host"
    openssl req -x509 -newkey rsa:2048 -nodes -days 1 \
        -subj "/CN=$host" \
        -keyout "$TMP/live/$host/privkey.pem" \
        -out "$TMP/live/$host/fullchain.pem" >/dev/null 2>&1
done

# --------------------------------------------------------------- 1. nginx -t

info "nginx -t with dummy certificates ($IMAGE)"

cp "$NGINX_DIR/solvun.conf" "$TMP/conf.d/solvun.conf"

docker run --rm \
    -v "$TMP/conf.d:/etc/nginx/conf.d:ro" \
    -v "$NGINX_DIR/snippets:/etc/nginx/snippets:ro" \
    -v "$TMP/live:/etc/letsencrypt/live:ro" \
    "$IMAGE" nginx -t

# ------------------------------------------------- 2. routing, plain HTTP

info "building an HTTP-only copy for the routing checks"

python3 - "$NGINX_DIR/solvun.conf" "$TMP/testconf/solvun.conf" <<'PY'
import re
import sys

src_path, out_path = sys.argv[1], sys.argv[2]
src = open(src_path, encoding="utf-8").read()


def cut(text, start_marker, end_marker):
    try:
        start = text.index(start_marker)
        end = text.index(end_marker)
    except ValueError:
        sys.exit(
            "test harness is out of date: the markers around the "
            "http->https block moved; update deploy/nginx/test.sh"
        )
    return text[:start] + text[end:]


# Drop the redirect server so both names can listen on :80 in the test.
src = cut(
    src,
    "# ------------------------------------------------------- http -> https",
    "# ------------------------------------------------------------ storefront",
)

if "listen 443 ssl;" not in src:
    sys.exit("test harness is out of date: no 'listen 443 ssl;' found")

src = src.replace("listen 443 ssl;", "listen 80;")
src = re.sub(r"^ *listen \[::\]:443 ssl;\n", "", src, flags=re.M)
src = re.sub(r"^ *http2 on;\n", "", src, flags=re.M)
src = re.sub(r"^ *ssl_[a-z_]+ .*;\n", "", src, flags=re.M)

if re.search(r"^\s*ssl_\w+", src, flags=re.M):
    sys.exit("test harness is out of date: an ssl_ directive survived")

# Mock upstream. The real config proxies to 127.0.0.1:3001, which lives in the
# same container, so this block is the only backend the test needs.
src += """
# --- test only: mock upstream standing in for the Go API ---
server {
    listen 127.0.0.1:3001;

    location / {
        add_header X-Mock-Path $uri always;
        add_header X-Mock-Method $request_method always;
        return 200 "upstream";
    }
}
"""

open(out_path, "w", encoding="utf-8").write(src)
PY

printf '<!doctype html><title>storefront-fixture</title>' > "$TMP/web/index.html"
printf '<!doctype html><title>admin-fixture</title>' > "$TMP/admin/index.html"
mkdir -p "$TMP/media/products"
printf 'png' > "$TMP/media/products/test.png"
printf 'console.log("fixture")' > "$TMP/web/assets/app-abc123.js"
printf 'console.log("fixture")' > "$TMP/admin/assets/app-abc123.js"

info "starting nginx on 127.0.0.1:$PORT"

docker run -d --name "$CONTAINER" \
    -p "127.0.0.1:$PORT:80" \
    -v "$TMP/testconf:/etc/nginx/conf.d:ro" \
    -v "$NGINX_DIR/snippets:/etc/nginx/snippets:ro" \
    -v "$TMP/web:/var/www/solvun/web:ro" \
    -v "$TMP/admin:/var/www/solvun/admin:ro" \
    -v "$TMP/media:/var/www/solvun/media:ro" \
    "$IMAGE" >/dev/null

READY=0
for _ in $(seq 1 40); do
    if curl -sf -o /dev/null -H "Host: ${HOSTS[0]}" "http://127.0.0.1:$PORT/"; then
        READY=1
        break
    fi
    sleep 0.5
done

if [ "$READY" -ne 1 ]; then
    docker logs "$CONTAINER" >&2 || true
    fail "nginx did not become ready"
fi

# ------------------------------------------------------------- assertions

FAILURES=0

ok() { printf '  ok   %s\n' "$1"; }
no() { printf '  FAIL %s\n' "$1"; printf '       %s\n' "$2"; FAILURES=$((FAILURES + 1)); }

status_of() { curl -s -o /dev/null -w '%{http_code}' "$@"; }

header_of() {
    local name=$1
    shift
    curl -s -o /dev/null -D - "$@" \
        | tr -d '\r' \
        | awk -v want="$(printf '%s' "$name" | tr 'A-Z' 'a-z')" \
            'tolower($1) == want":" { print $2; exit }'
}

body_of() { curl -s "$@"; }

check_status() {
    local want=$1 label=$2
    shift 2
    local got
    got=$(status_of "$@" || true)
    if [ "$got" = "$want" ]; then ok "$label"; else no "$label" "status $got, want $want"; fi
}

check_header() {
    local name=$1 want=$2 label=$3
    shift 3
    local got
    got=$(header_of "$name" "$@" || true)
    if [ "$got" = "$want" ]; then ok "$label"; else no "$label" "$name: '$got', want '$want'"; fi
}

check_body() {
    local want=$1 label=$2
    shift 2
    local got
    got=$(body_of "$@" || true)
    case "$got" in
        *"$want"*) ok "$label" ;;
        *) no "$label" "body '$got' does not contain '$want'" ;;
    esac
}

SHOP=(-H "Host: ${HOSTS[0]}")
ADMIN=(-H "Host: ${HOSTS[1]}")
BASE="http://127.0.0.1:$PORT"
UUID=11111111-1111-1111-1111-111111111111

info "shop(${HOSTS[0]})"

check_header X-Mock-Path /orders/123 \
    "/api/orders/123 strips the /api prefix" \
    "${SHOP[@]}" "$BASE/api/orders/123"

check_header X-Mock-Path /shipping/notification \
    "the shipping webhook reaches the API with its token" \
    "${SHOP[@]}" "$BASE/api/shipping/notification?token=secret-token"

if docker exec "$CONTAINER" grep -q "secret-token" /var/log/nginx/solvun-shop.access.log 2>/dev/null; then
    no "the webhook token stays out of the access log" "token found in the access log"
else
    ok "the webhook token stays out of the access log"
fi

check_status 404 \
    "/api/admin/orders is not exposed on the storefront" \
    "${SHOP[@]}" "$BASE/api/admin/orders"

check_header X-Mock-Path "/order/$UUID" \
    "/order/<id> is passed through untouched" \
    "${SHOP[@]}" "$BASE/order/$UUID"

check_header X-Mock-Path "/orders/$UUID/invoice.pdf" \
    "/orders/<id>/invoice.pdf is passed through untouched" \
    "${SHOP[@]}" "$BASE/orders/$UUID/invoice.pdf"

check_header X-Mock-Method POST \
    "POST /track reaches the API" \
    -X POST "${SHOP[@]}" "$BASE/track"

check_body storefront-fixture \
    "/checkout falls back to the SPA" \
    "${SHOP[@]}" "$BASE/checkout"

check_header Cache-Control "max-age=31536000" \
    "hashed assets are cached for a year" \
    "${SHOP[@]}" "$BASE/assets/app-abc123.js"

check_status 200 \
    "uploaded product images are served" \
    "${SHOP[@]}" "$BASE/media/products/test.png"

check_header Cache-Control "max-age=2592000" \
    "uploaded product images are cached for 30 days" \
    "${SHOP[@]}" "$BASE/media/products/test.png"

check_status 404 \
    "a missing asset is a 404, not the SPA" \
    "${SHOP[@]}" "$BASE/assets/missing.js"

check_header Cache-Control "no-cache" \
    "index.html is revalidated" \
    "${SHOP[@]}" "$BASE/"

HEADERS=$(curl -s -o /dev/null -D - "${SHOP[@]}" "$BASE/")
for name in Content-Security-Policy Strict-Transport-Security X-Content-Type-Options; do
    if printf '%s' "$HEADERS" | tr -d '\r' | grep -qi "^$name:"; then
        ok "index.html sends $name"
    else
        no "index.html sends $name" "header missing"
    fi
done

info "admin(${HOSTS[1]})"

check_header X-Mock-Path /admin/orders \
    "/api/admin/orders reaches the API" \
    "${ADMIN[@]}" "$BASE/api/admin/orders"

check_header X-Mock-Path /admin/session \
    "POST /api/admin/session reaches the API" \
    -X POST "${ADMIN[@]}" "$BASE/api/admin/session"

check_body admin-fixture \
    "/orders/abc falls back to the admin SPA" \
    "${ADMIN[@]}" "$BASE/orders/abc"

check_header Cache-Control "max-age=31536000" \
    "admin assets are cached for a year" \
    "${ADMIN[@]}" "$BASE/assets/app-abc123.js"

check_status 200 \
    "uploaded images resolve on the admin host too" \
    "${ADMIN[@]}" "$BASE/media/products/test.png"

printf '\n'
if [ "$FAILURES" -ne 0 ]; then
    docker logs "$CONTAINER" >&2 || true
    fail "$FAILURES routing check(s) failed"
fi

info "all nginx checks passed"
