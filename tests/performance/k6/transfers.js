import http from "k6/http";
import { check, fail } from "k6";
import { Counter, Trend } from "k6/metrics";

const baseURL = (__ENV.LEDGERSYNC_PERF_BFF_URL || "").replace(/\/$/, "");
const publicOrigin = (__ENV.LEDGERSYNC_PERF_PUBLIC_ORIGIN || baseURL).replace(/\/$/, "");
const sourceAccountId = __ENV.LEDGERSYNC_PERF_SOURCE_ACCOUNT;
const destinationAccountId = __ENV.LEDGERSYNC_PERF_DESTINATION_ACCOUNT;
const rate = Number(__ENV.LEDGERSYNC_PERF_TPS || 10);
const duration = __ENV.LEDGERSYNC_PERF_DURATION || "5m";

if (!baseURL || !sourceAccountId || !destinationAccountId) {
  fail("LEDGERSYNC_PERF_BFF_URL, LEDGERSYNC_PERF_SOURCE_ACCOUNT, and LEDGERSYNC_PERF_DESTINATION_ACCOUNT are required");
}

const transferLatency = new Trend("ledgersync_transfer_duration", true);
const balanceLatency = new Trend("ledgersync_balance_duration", true);
const replayCount = new Counter("ledgersync_same_key_replays");
const unexpectedOutcomeCount = new Counter("ledgersync_unexpected_outcomes");

export const options = {
  noCookiesReset: true,
  scenarios: {
    steady_internal_transfers: {
      executor: "constant-arrival-rate",
      rate,
      timeUnit: "1s",
      duration,
      preAllocatedVUs: Math.max(25, rate * 2),
      maxVUs: Math.max(100, rate * 5),
      gracefulStop: "15s",
    },
  },
  thresholds: {
    http_req_failed: ["rate<0.001"],
    ledgersync_transfer_duration: ["p(95)<500"],
    ledgersync_balance_duration: ["p(95)<200"],
    ledgersync_unexpected_outcomes: ["count==0"],
  },
};

let csrfToken = "";
let diagnosticsRemaining = 3;

function recordUnexpected(response, journey, payload = undefined) {
  unexpectedOutcomeCount.add(1, { journey, status: String(response.status) });
  if (diagnosticsRemaining > 0) {
    const code = payload?.error?.code || "missing_error_code";
    console.error(`unexpected_${journey}_status=${response.status} code=${code}`);
    diagnosticsRemaining -= 1;
  }
}

function ensureSession() {
  if (csrfToken) return;
  const response = http.get(`${baseURL}/api/session`, { tags: { journey: "session" } });
  const payload = response.json();
  const established = check(response, {
    "BFF session established": (result) => result.status === 200 && typeof payload?.csrf_token === "string" && payload.csrf_token.length > 0,
  });
  if (!established) fail(`BFF session unavailable (${response.status})`);
  csrfToken = payload.csrf_token;
}

function transfer(key) {
  const response = http.post(
    `${baseURL}/api/transfers`,
    JSON.stringify({ sourceAccountId, destinationAccountId, amount: { currency: "USD", minorUnits: "1" } }),
    {
      headers: {
        "Content-Type": "application/json",
        "Idempotency-Key": key,
        "X-CSRF-Token": csrfToken,
        Origin: publicOrigin,
      },
      tags: { journey: "transfer" },
    },
  );
  transferLatency.add(response.timings.duration);
  const payload = response.json();
  const valid = check(response, {
    "transfer posted exactly": (result) => result.status === 201 && payload?.status === "posted" && typeof payload?.transfer_id === "string",
  });
  if (!valid) recordUnexpected(response, "transfer", payload);
  return { response, payload };
}

export default function () {
  ensureSession();
  const key = `k6-${__VU}-${__ITER}-${Date.now()}`;
  const first = transfer(key);

  // Sample explicit lost-response recovery without turning every iteration
  // into twice the write traffic. The same key and payload must replay.
  if (__ITER % 10 === 0 && first.response.status === 201) {
    const replay = transfer(key);
    replayCount.add(1);
    const safe = check(replay.response, {
      "same-key retry replays one transfer": (result) => result.headers["Idempotent-Replay"] === "true" && replay.payload?.transfer_id === first.payload?.transfer_id,
    });
    if (!safe) recordUnexpected(replay.response, "replay", replay.payload);
  }

  const balance = http.get(`${baseURL}/api/accounts/${sourceAccountId}/balance`, { tags: { journey: "balance" } });
  balanceLatency.add(balance.timings.duration);
  if (!check(balance, { "current exact balance returned": (result) => result.status === 200 && typeof result.json()?.available_minor === "string" })) {
    recordUnexpected(balance, "balance", balance.json());
  }

  if (__ITER % 5 === 0) {
    const accounts = http.get(`${baseURL}/api/me/accounts?limit=25&status=active`, { tags: { journey: "accounts" } });
    if (!check(accounts, { "account page available": (result) => result.status === 200 })) recordUnexpected(accounts, "accounts", accounts.json());
    const history = http.get(`${baseURL}/api/accounts/${sourceAccountId}/transactions?limit=25`, { tags: { journey: "history" } });
    if (!check(history, { "history page available": (result) => result.status === 200 })) recordUnexpected(history, "history", history.json());
  }
  if (__ITER % 20 === 0) {
    check(http.get(`${baseURL}/api/reconciliation/runs?limit=1`, { tags: { journey: "reconciliation" } }), { "reconciliation evidence available": (result) => result.status === 200 });
  }
}
