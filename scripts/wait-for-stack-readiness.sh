#!/usr/bin/env sh

set -eu

base_url="${LEDGERSYNC_SYSTEM_WEB_URL:-http://127.0.0.1:3000}"
max_attempts="${LEDGERSYNC_READINESS_MAX_ATTEMPTS:-60}"
retry_seconds="${LEDGERSYNC_READINESS_RETRY_SECONDS:-2}"
cookie_jar="$(mktemp)"

trap 'rm -f "$cookie_jar"' EXIT HUP INT TERM

attempt=1
while [ "$attempt" -le "$max_attempts" ]; do
  session_status="$(curl --silent --show-error --connect-timeout 2 --max-time 5 \
    --noproxy '127.0.0.1,localhost,::1' \
    --cookie "$cookie_jar" \
    --cookie-jar "$cookie_jar" \
    --output /dev/null \
    --write-out '%{http_code}' \
    "$base_url/api/session" 2>/dev/null || true)"
  [ -n "$session_status" ] || session_status="000"
  login_status="not-attempted"

  # The session endpoint is intentionally read-only. A fresh local stack must
  # establish its loopback-only development session through the same explicit
  # sign-in route as a browser before protected readiness can be asserted.
  if [ "$session_status" = "401" ]; then
    login_status="$(curl --silent --show-error --connect-timeout 2 --max-time 5 \
      --noproxy '127.0.0.1,localhost,::1' \
      --location \
      --cookie "$cookie_jar" \
      --cookie-jar "$cookie_jar" \
      --output /dev/null \
      --write-out '%{http_code}' \
      "$base_url/api/auth/sign-in?return_to=%2F" 2>/dev/null || true)"
    [ -n "$login_status" ] || login_status="000"
    session_status="$(curl --silent --show-error --connect-timeout 2 --max-time 5 \
      --noproxy '127.0.0.1,localhost,::1' \
      --cookie "$cookie_jar" \
      --cookie-jar "$cookie_jar" \
      --output /dev/null \
      --write-out '%{http_code}' \
      "$base_url/api/session" 2>/dev/null || true)"
    [ -n "$session_status" ] || session_status="000"
  fi

  accounts_status="not-attempted"
  case "$session_status" in
    2??)
      accounts_status="$(curl --silent --show-error --connect-timeout 2 --max-time 5 \
        --noproxy '127.0.0.1,localhost,::1' \
        --cookie "$cookie_jar" \
        --cookie-jar "$cookie_jar" \
        --output /dev/null \
        --write-out '%{http_code}' \
        "$base_url/api/me/accounts?limit=1&status=active" 2>/dev/null || true)"
      [ -n "$accounts_status" ] || accounts_status="000"
      ;;
  esac

  if [ "${session_status#2}" != "$session_status" ] && [ "${accounts_status#2}" != "$accounts_status" ]; then
    printf 'LedgerSync BFF, API, and PostgreSQL path ready after %s attempt(s).\n' "$attempt"
    exit 0
  fi

  if [ "$attempt" -eq 1 ] || [ "$attempt" -eq "$max_attempts" ] || [ $((attempt % 10)) -eq 0 ]; then
    printf 'LedgerSync readiness pending at attempt %s/%s (login=%s session=%s accounts=%s).\n' \
      "$attempt" "$max_attempts" "$login_status" "$session_status" "$accounts_status" >&2
  fi

  if [ "$attempt" -lt "$max_attempts" ]; then
    sleep "$retry_seconds"
  fi
  attempt=$((attempt + 1))
done

printf 'LedgerSync stack did not become ready after %s attempt(s).\n' "$max_attempts" >&2
exit 1
