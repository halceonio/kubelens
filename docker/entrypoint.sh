#!/bin/sh
set -euo pipefail

truthy() {
  case "${1:-}" in
    1|true|TRUE|yes|YES|on|ON) return 0 ;;
    *) return 1 ;;
  esac
}

log() {
  printf "[kubelens-entrypoint] %s\n" "$*" >&2
}

CONFIG_PATH="${KUBELENS_CONFIG:-/etc/kubelens/config.yaml}"
RUNTIME_CONFIG="${KUBELENS_RUNTIME_CONFIG:-/var/lib/kubelens/config.runtime.yaml}"
SUPERVISOR_DIR="/etc/supervisord.d"

ensure_writable_config() {
  if [ -f "$CONFIG_PATH" ] && [ ! -w "$CONFIG_PATH" ]; then
    mkdir -p "$(dirname "$RUNTIME_CONFIG")"
    cp "$CONFIG_PATH" "$RUNTIME_CONFIG"
    CONFIG_PATH="$RUNTIME_CONFIG"
    export KUBELENS_CONFIG="$CONFIG_PATH"
    log "using runtime config at $CONFIG_PATH"
  fi
}

extract_database_url() {
  if [ ! -f "$CONFIG_PATH" ]; then
    return
  fi
  awk -F': *' '/database_url:/{print $2; exit}' "$CONFIG_PATH" | tr -d '"'"'"
}

prepare_sqlite() {
  db_url="$1"
  case "$db_url" in
    sqlite://*|sqlite3://*|file:*)
      ;;
    *)
      return
      ;;
  esac

  db_path="$db_url"
  db_path="${db_path#sqlite://}"
  db_path="${db_path#sqlite3://}"
  db_path="${db_path#file:}"
  db_path="${db_path%%\?*}"
  if [ -z "$db_path" ]; then
    return
  fi

  db_dir="$(dirname "$db_path")"
  mkdir -p "$db_dir"
  chmod 777 "$db_dir" || true

  if command -v sqlite3 >/dev/null 2>&1; then
    sqlite3 "$db_path" <<'SQL'
PRAGMA journal_mode=WAL;
PRAGMA synchronous=NORMAL;
PRAGMA temp_store=MEMORY;
PRAGMA cache_size=-20000;
PRAGMA wal_autocheckpoint=1000;
PRAGMA busy_timeout=5000;
SQL
    log "sqlite pragmas applied to $db_path"
  else
    log "sqlite3 not installed; skipping pragma tuning"
  fi
}

maybe_enable_local_cache() {
  use_local_cache=false
  if truthy "${START_LOCAL_VALKEY:-}"; then
    use_local_cache=true
  elif truthy "${START_LOCAL_REDIS:-}"; then
    use_local_cache=true
  fi

  if [ "$use_local_cache" != "true" ]; then
    return
  fi

  data_dir="${LOCAL_VALKEY_DATA_DIR:-${LOCAL_REDIS_DATA_DIR:-/data/cache}}"
  mkdir -p "$data_dir"
  chmod 777 "$data_dir" || true

  ensure_writable_config
  if [ -f "$CONFIG_PATH" ]; then
    cat >> "$CONFIG_PATH" <<EOF

cache:
  enabled: true
  redis_url: "redis://localhost:6379/0"
EOF
  fi

  mkdir -p "$SUPERVISOR_DIR"
  cat > "$SUPERVISOR_DIR/valkey.conf" <<EOF
[program:valkey]
command=/usr/bin/valkey-server /etc/valkey/valkey.conf --dir $data_dir
autostart=true
autorestart=true
startretries=3
stdout_logfile=/dev/fd/1
stdout_logfile_maxbytes=0
stderr_logfile=/dev/fd/2
stderr_logfile_maxbytes=0
EOF

  log "local valkey enabled (dir=$data_dir)"
}

if [ -f "$CONFIG_PATH" ]; then
  db_url="$(extract_database_url)"
  if [ -n "$db_url" ]; then
    prepare_sqlite "$db_url"
  fi
else
  log "config not found at $CONFIG_PATH"
fi

maybe_enable_local_cache

exec /usr/bin/supervisord -c /etc/supervisord.conf
