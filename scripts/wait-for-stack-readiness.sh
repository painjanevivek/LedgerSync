#!/usr/bin/env sh

set -eu

base_url="${LEDGERSYNC_SYSTEM_WEB_URL:-http://127.0.0.1:3000}"
max_attempts="${LEDGERSYNC_READINESS_MAX_ATTEMPTS:-60}"
retry_seconds="${LEDGERSYNC_READINESS_RETRY_SECONDS:-2}"
cookie_jar="$(mktemp)"

trap 'rm -f "$cookie_jar"' EXIT HUP INT TERM

attempt=1
while [ "$attempt" -le "$max_attempts" ]; do
  if curl --fail --silent --show-error --connect-timeout 2 --max-time 5 \
    --cookie "$cookie_jar" \
    --cookie-jar "$cookie_jar" \
    "$base_url/api/session" >/dev/null 2>&1 && \
    curl --fail --silent --show-error --connect-timeout 2 --max-time 5 \
      --cookie "$cookie_jar" \
      --cookie-jar "$cookie_jar" \
      "$base_url/api/me/accounts?limit=1&status=active" >/dev/null 2>&1; then
    printf 'LedgerSync BFF, API, and PostgreSQL path ready after %s attempt(s).\n' "$attempt"
    exit 0
  fi

  if [ "$attempt" -lt "$max_attempts" ]; then
    sleep "$retry_seconds"
  fi
  attempt=$((attempt + 1))
done

printf 'LedgerSync stack did not become ready after %s attempt(s).\n' "$max_attempts" >&2
exit 1
