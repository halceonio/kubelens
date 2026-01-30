#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<USAGE
Usage:
  KEYCLOAK_URL=... REALM=... CLIENT_ID=... CLIENT_SECRET=... \
    $0 [--json]

Optional:
  SCOPE="openid email profile groups" (default)

Notes:
- Requires "OAuth 2.0 Device Authorization Grant" enabled for the client.
- Prints a verification URL + user code, then polls until token is issued.
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

SCOPE="${SCOPE:-openid email profile groups}"

DEVICE_ENDPOINT="${KEYCLOAK_URL%/}/realms/${REALM}/protocol/openid-connect/auth/device"
TOKEN_ENDPOINT="${KEYCLOAK_URL%/}/realms/${REALM}/protocol/openid-connect/token"

init_response=$(curl -sS -X POST "$DEVICE_ENDPOINT" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  --data-urlencode "client_id=${CLIENT_ID}" \
  --data-urlencode "client_secret=${CLIENT_SECRET}" \
  --data-urlencode "scope=${SCOPE}")

KC_RESPONSE="$init_response" python3 - <<'PY'
import json
import os
import sys

raw = os.environ.get("KC_RESPONSE")
try:
    data = json.loads(raw)
except json.JSONDecodeError as exc:
    print(f"Invalid JSON response: {exc}", file=sys.stderr)
    print(raw)
    sys.exit(1)

if "error" in data:
    print(data.get("error_description") or data.get("error"), file=sys.stderr)
    sys.exit(1)

device_code = data.get("device_code")
user_code = data.get("user_code")
verification_uri = data.get("verification_uri") or data.get("verification_uri_complete")
interval = data.get("interval", 5)
expires_in = data.get("expires_in", 600)

if not device_code or not user_code or not verification_uri:
    print("Missing device_code/user_code/verification_uri in response", file=sys.stderr)
    print(raw)
    sys.exit(1)

print(device_code)
print(user_code)
print(verification_uri)
print(interval)
print(expires_in)
PY

read -r DEVICE_CODE
read -r USER_CODE
read -r VERIFY_URI
read -r INTERVAL
read -r EXPIRES_IN

cat <<EOF_INFO

Open the following URL and enter the code:
  ${VERIFY_URI}
Code:
  ${USER_CODE}

Polling for token (expires in ${EXPIRES_IN}s)...
EOF_INFO

start=$(date +%s)
while true; do
  now=$(date +%s)
  elapsed=$((now - start))
  if [[ $elapsed -ge $EXPIRES_IN ]]; then
    echo "Device code expired. Please run the script again." >&2
    exit 1
  fi

  resp=$(curl -sS -X POST "$TOKEN_ENDPOINT" \
    -H "Content-Type: application/x-www-form-urlencoded" \
    --data-urlencode "grant_type=urn:ietf:params:oauth:grant-type:device_code" \
    --data-urlencode "device_code=${DEVICE_CODE}" \
    --data-urlencode "client_id=${CLIENT_ID}" \
    --data-urlencode "client_secret=${CLIENT_SECRET}")

  KC_RESPONSE="$resp" KC_OUTPUT_JSON="$OUTPUT_JSON" python3 - <<'PY'
import json
import os
import sys

raw = os.environ.get("KC_RESPONSE")
try:
    data = json.loads(raw)
except json.JSONDecodeError:
    print(raw)
    sys.exit(0)

if "error" in data:
    err = data.get("error")
    if err in ("authorization_pending", "slow_down"):
        print(err, file=sys.stderr)
        sys.exit(2)
    print(data.get("error_description") or err, file=sys.stderr)
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

  status=$?
  if [[ $status -eq 0 ]]; then
    exit 0
  elif [[ $status -eq 2 ]]; then
    echo -n "." >&2
    sleep "$INTERVAL"
  else
    exit 1
  fi
done
