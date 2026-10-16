#!/usr/bin/env bash
# Container smoke test (ADR 0021/0022): runs the image with the same
# hardening as the NAS project file and walks through first-run setup.
#
#   scripts/test/container-smoke.sh [image]      (default: hallido:dev, see make image)
set -euo pipefail

IMAGE="${1:-hallido:dev}"
NAME="hallido-smoke-$$"
VOL="hallido-smoke-$$"
PORT="${SMOKE_PORT:-18799}"
BASE="http://127.0.0.1:${PORT}"

command -v docker >/dev/null 2>&1 || { echo "Docker missing — test skipped." >&2; exit 0; }

cleanup() {
  docker rm -f "$NAME" >/dev/null 2>&1 || true
  docker volume rm "$VOL" >/dev/null 2>&1 || true
}
trap cleanup EXIT

fail() { echo "FAIL $*" >&2; docker logs "$NAME" 2>&1 | tail -20 >&2 || true; exit 1; }

start() {
  docker run -d --name "$NAME" \
    --read-only --cap-drop ALL --security-opt no-new-privileges:true \
    --memory 128m --pids-limit 64 \
    -e SRA_DRIVER=mock -p "127.0.0.1:${PORT}:8790" \
    -v "${VOL}:/data" "$IMAGE" >/dev/null
}

wait_healthy() {
  for _ in $(seq 1 30); do
    [[ "$(docker inspect -f '{{.State.Health.Status}}' "$NAME" 2>/dev/null)" == healthy ]] && return 0
    sleep 2
  done
  fail "container not healthy"
}

start
wait_healthy
echo "ok   healthy (read-only, no capabilities)"

[[ "$(docker inspect -f '{{.Config.User}}' "$NAME")" == "65532:65532" ]] || fail "image user is not 65532"
echo "ok   runs as UID 65532"

curl -fsS "$BASE/api/v1/setup" | grep -q '"open":true' || fail "setup not open on first start"
code=$(curl -s -o /dev/null -w '%{http_code}' "$BASE/api/v1/scenes")
[[ "$code" == 401 ]] || fail "scenes without token: HTTP $code"
code=$(curl -s -o /dev/null -w '%{http_code}' -X POST -H 'Content-Type: application/json' -d '{}' "$BASE/api/v1/setup/token")
[[ "$code" == 400 ]] || fail "token request without setup header: HTTP $code"
echo "ok   setup open, API closed"

token=$(curl -fsS -X POST -H 'X-Hallido-Setup: 1' -H 'Content-Type: application/json' \
  -d '{"timezone":"Europe/Berlin"}' "$BASE/api/v1/setup/token" | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')
[[ "$token" =~ ^[0-9A-Z]{4}(-[0-9A-Z]{4}){7}$ ]] || fail "no token from setup"
code=$(curl -s -o /dev/null -w '%{http_code}' -X POST -H 'X-Hallido-Setup: 1' -H 'Content-Type: application/json' -d '{}' "$BASE/api/v1/setup/token")
[[ "$code" == 409 ]] || fail "second token request: HTTP $code"
curl -fsS -H "Authorization: Bearer ${token}" "$BASE/api/v1/scenes" >/dev/null || fail "scenes with token"
echo "ok   access key created once, API works with it"

logs=$(curl -fsS -H "Authorization: Bearer ${token}" "$BASE/api/v1/logs")
grep -q '"file":true' <<<"$logs" || fail "log file not enabled"
grep -q "access key created" <<<"$logs" || fail "setup not in the log"
grep -q "$token" <<<"$logs" && fail "token appears in the log"
docker logs "$NAME" 2>&1 | grep -q "$token" && fail "token appears in the container log"
echo "ok   log in /data/logs, no secrets"

docker restart "$NAME" >/dev/null
wait_healthy
curl -fsS "$BASE/api/v1/setup" | grep -q '"setup_required":false' || fail "setup open again after restart"
curl -fsS -H "Authorization: Bearer ${token}" "$BASE/api/v1/schedules" | grep -q 'Europe/Berlin' || fail "time zone not kept"
echo "ok   restart keeps access key and time zone"
echo "container-smoke: all checks passed"
