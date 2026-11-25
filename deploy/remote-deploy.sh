#!/usr/bin/env bash
#
# Deploys one commit on the VPS.
#
#   bash remote-deploy.sh <sha> <project-dir>
#
# The project directory is a clone of this repository that only exists to run
# the API. Do not build the frontends there and do not edit files by hand:
# `git checkout` must be able to switch commits cleanly.
#
# What it does:
#   1. copies the running binary aside,
#   2. fetches and checks out the requested commit,
#   3. verifies formatting and vet, then builds,
#   4. restarts the user service and waits for /health,
#   5. restores the previous commit and binary if the health check fails.
#
# Environment overrides: SERVICE, HEALTH_URL, HEALTH_RETRIES.

set -euo pipefail

SHA=${1:?usage: remote-deploy.sh <sha> <project-dir>}
PROJECT_DIR=${2:?usage: remote-deploy.sh <sha> <project-dir>}

SERVICE=${SERVICE:-golearn-staging}
HEALTH_URL=${HEALTH_URL:-http://127.0.0.1:3001/health}
HEALTH_RETRIES=${HEALTH_RETRIES:-30}

log() { printf '\n==> %s\n' "$*"; }

cd "$PROJECT_DIR"

PREVIOUS=$(git rev-parse HEAD)

if [ -x golearn ]; then
    cp -f golearn golearn.previous
fi

log "fetching and checking out $SHA"
git fetch --prune origin
git checkout --detach "$SHA"

log "checking formatting"
unformatted=$(gofmt -l . | grep -v node_modules || true)
if [ -n "$unformatted" ]; then
    printf 'these files need gofmt:\n%s\n' "$unformatted"
    exit 1
fi

log "vetting and building"
# node_modules is excluded in case someone ever ran bun install here.
packages=$(go list ./... | grep -v node_modules)
go vet $packages
go build -o golearn .

# systemd --user needs the runtime dir when called over SSH.
export XDG_RUNTIME_DIR=${XDG_RUNTIME_DIR:-/run/user/$(id -u)}

healthy() {
    local attempt
    for ((attempt = 1; attempt <= HEALTH_RETRIES; attempt++)); do
        if curl -fsS --max-time 2 "$HEALTH_URL" >/dev/null 2>&1; then
            return 0
        fi
        sleep 1
    done
    return 1
}

log "restarting $SERVICE"
systemctl --user restart "$SERVICE"

if healthy; then
    log "deployed $SHA"
    exit 0
fi

log "health check failed, rolling back to $PREVIOUS"
systemctl --user status "$SERVICE" --no-pager || true

git checkout --detach "$PREVIOUS"
if [ -x golearn.previous ]; then
    mv -f golearn.previous golearn
fi
systemctl --user restart "$SERVICE"

if healthy; then
    log "rolled back to $PREVIOUS, service is healthy again"
else
    log "rollback is unhealthy too, manual attention required"
fi

exit 1
