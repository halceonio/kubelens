#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<USAGE
Usage:
  KEYCLOAK_URL=... REALM=... CLIENT_ID=... CLIENT_SECRET=... USERNAME=... PASSWORD=... \
    $0 [--json]

Notes:
- Requires "Direct Access Grants" enabled for the client.
- This uses the Resource Owner Password flow (grant_type=password).
- Output defaults to raw access token; use --json for full response.

Optional:
  SCOPE="openid email profile groups" (default)
USAGE
}

OUTPUT_JSON=false
if [[ ${1:-} == "--json" ]]; then
  OUTPUT_JSON=true
elif [[ ${1:-} != "" ]]; then
  usage
  exit 1
fi

: "${KEYCLOAK_URL:?KEYCLOAK_URL is required}"
: "${REALM:?REALM is required}"
: "${CLIENT_ID:?CLIENT_ID is required}"
: "${CLIENT_SECRET:?CLIENT_SECRET is required}"
: "${USERNAME:?USERNAME is required}"
: "${PASSWORD:?PASSWORD is required}"

SCOPE="${SCOPE:-openid email profile groups}"
TOKEN_ENDPOINT="${KEYCLOAK_URL%/}/realms/${REALM}/protocol/openid-connect/token"

response=$(curl -sS -X POST "$TOKEN_ENDPOINT" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  --data-urlencode "grant_type=password" \
  --data-urlencode "client_id=${CLIENT_ID}" \
  --data-urlencode "client_secret=${CLIENT_SECRET}" \
  --data-urlencode "username=${USERNAME}" \
  --data-urlencode "password=${PASSWORD}" \
  --data-urlencode "scope=${SCOPE}")

KC_RESPONSE="$response" KC_OUTPUT_JSON="$OUTPUT_JSON" python3 - <<'PY'
import json
import os
import sys

raw = os.environ.get("KC_RESPONSE")
if not raw:
    print("Missing response", file=sys.stderr)
    sys.exit(1)

try:
    data = json.loads(raw)
except json.JSONDecodeError as exc:
    print(f"Invalid JSON response: {exc}", file=sys.stderr)
    print(raw)
    sys.exit(1)

if "error" in data:
    print(data.get("error_description") or data.get("error"), file=sys.stderr)
    sys.exit(1)

if os.environ.get("KC_OUTPUT_JSON") == "true":
    print(json.dumps(data, indent=2))
    sys.exit(0)

access_token = data.get("access_token")
if not access_token:
    print("No access_token returned", file=sys.stderr)
    sys.exit(1)

print(access_token)
PY
