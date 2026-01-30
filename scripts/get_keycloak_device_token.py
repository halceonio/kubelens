#!/usr/bin/env python3
import argparse
import json
import os
import sys
import time
import urllib.parse
import urllib.request


def require_env(name: str) -> str:
    value = os.environ.get(name)
    if not value:
        print(f"{name} is required", file=sys.stderr)
        sys.exit(1)
    return value


def post_form(url: str, data: dict) -> dict:
    body = urllib.parse.urlencode(data).encode("utf-8")
    req = urllib.request.Request(
        url,
        data=body,
        headers={"Content-Type": "application/x-www-form-urlencoded"},
        method="POST",
    )
    with urllib.request.urlopen(req) as resp:
        payload = resp.read().decode("utf-8")
    try:
        return json.loads(payload)
    except json.JSONDecodeError:
        print("Non-JSON response:")
        print(payload)
        sys.exit(1)


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Interactive Keycloak device authorization flow"
    )
    parser.add_argument("--json", action="store_true", help="Output full JSON")
    args = parser.parse_args()

    keycloak_url = require_env("KEYCLOAK_URL")
    realm = require_env("REALM")
    client_id = require_env("CLIENT_ID")
    client_secret = require_env("CLIENT_SECRET")
    scope = os.environ.get("SCOPE", "openid email profile groups")

    device_endpoint = f"{keycloak_url.rstrip('/')}/realms/{realm}/protocol/openid-connect/auth/device"
    token_endpoint = f"{keycloak_url.rstrip('/')}/realms/{realm}/protocol/openid-connect/token"

    init_resp = post_form(
        device_endpoint,
        {
            "client_id": client_id,
            "client_secret": client_secret,
            "scope": scope,
        },
    )

    if "error" in init_resp:
        print(init_resp.get("error_description") or init_resp.get("error"), file=sys.stderr)
        return 1

    device_code = init_resp.get("device_code")
    user_code = init_resp.get("user_code")
    verification_uri = init_resp.get("verification_uri") or init_resp.get(
        "verification_uri_complete"
    )
    interval = int(init_resp.get("interval", 5))
    expires_in = int(init_resp.get("expires_in", 600))

    if not device_code or not user_code or not verification_uri:
        print("Missing device_code/user_code/verification_uri", file=sys.stderr)
        print(json.dumps(init_resp, indent=2))
        return 1

    print("\nOpen the following URL and enter the code:")
    print(f"  {verification_uri}")
    print("Code:")
    print(f"  {user_code}")
    print(f"\nDevice code expires in {expires_in}s")

    input("\nPress ENTER after completing login in the browser...")

    start = time.time()
    while True:
        if time.time() - start >= expires_in:
            print("Device code expired. Run the script again.", file=sys.stderr)
            return 1

        token_resp = post_form(
            token_endpoint,
            {
                "grant_type": "urn:ietf:params:oauth:grant-type:device_code",
                "device_code": device_code,
                "client_id": client_id,
                "client_secret": client_secret,
            },
        )

        if "error" in token_resp:
            err = token_resp.get("error")
            if err in ("authorization_pending", "slow_down"):
                time.sleep(interval)
                continue
            print(token_resp.get("error_description") or err, file=sys.stderr)
            return 1

        if args.json:
            print(json.dumps(token_resp, indent=2))
        else:
            access_token = token_resp.get("access_token")
            if not access_token:
                print("No access_token returned", file=sys.stderr)
                return 1
            print(access_token)
        return 0


if __name__ == "__main__":
    raise SystemExit(main())
