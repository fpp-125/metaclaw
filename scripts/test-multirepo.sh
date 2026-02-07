#!/usr/bin/env bash
set -euo pipefail

DEV_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
ENGINE_ROOT="$DEV_ROOT/MetaClaw"
SKILLS_ROOT="$DEV_ROOT/metaclaw-skills"
EXAMPLES_ROOT="$DEV_ROOT/metaclaw-examples"
REGISTRY_ROOT="$DEV_ROOT/metaclaw-registry"

for d in "$SKILLS_ROOT" "$EXAMPLES_ROOT" "$REGISTRY_ROOT"; do
  if [ ! -d "$d" ]; then
    echo "missing repo: $d" >&2
    exit 1
  fi
done

echo "[1/5] testing metaclaw-skills"
(
  cd "$SKILLS_ROOT"
  go test ./...
  go run ./cmd/skilllint ./skills
)

echo "[2/5] testing metaclaw-registry"
(
  cd "$REGISTRY_ROOT"
  go test ./...
)

echo "[3/5] validating metaclaw-examples"
(
  cd "$EXAMPLES_ROOT"
  METACLAW_BIN="$ENGINE_ROOT/metaclaw" ./scripts/validate_examples.sh
)

echo "[4/5] starting metaclaw-registry for API E2E"
TMP_DIR="$(mktemp -d /tmp/metaclaw-multirepo-XXXX)"
REGISTRY_BIN="$TMP_DIR/metaclaw-registry"
REGISTRY_DB="$TMP_DIR/registry.json"
PORT=18088
TOKEN="dev-token"
(
  cd "$REGISTRY_ROOT"
  go build -o "$REGISTRY_BIN" ./cmd/metaclaw-registry
)
"$REGISTRY_BIN" --addr ":$PORT" --data "$REGISTRY_DB" --admin-token "$TOKEN" >/tmp/metaclaw-registry-e2e.log 2>&1 &
REG_PID=$!
trap 'kill "$REG_PID" >/dev/null 2>&1 || true' EXIT
sleep 1

echo "[5/5] publishing and querying artifacts"
SKILL_PAYLOAD='{"kind":"skill","name":"obsidian.search","version":"v1.0.0","digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","ociRef":"ghcr.io/metaclaw/skills/obsidian.search:v1.0.0"}'
CAPSULE_PAYLOAD='{"kind":"capsule","name":"obsidian-bot","version":"v0.1.0","digest":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","ociRef":"ghcr.io/metaclaw/capsules/obsidian-bot:v0.1.0"}'

curl -sS -X POST "http://127.0.0.1:$PORT/v1/artifacts" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "$SKILL_PAYLOAD" >/tmp/metaclaw-reg-post-skill.json

curl -sS -X POST "http://127.0.0.1:$PORT/v1/artifacts" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "$CAPSULE_PAYLOAD" >/tmp/metaclaw-reg-post-capsule.json

LIST_JSON="$(curl -sS "http://127.0.0.1:$PORT/v1/artifacts?kind=skill")"
GET_JSON="$(curl -sS "http://127.0.0.1:$PORT/v1/artifacts/capsule/obsidian-bot/v0.1.0")"

echo "$LIST_JSON" | rg -q 'obsidian.search'
echo "$GET_JSON" | rg -q 'obsidian-bot'

echo "multi-repo e2e: OK"
