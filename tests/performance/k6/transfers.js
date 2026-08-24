import http from "k6/http";
import { check, fail, sleep } from "k6";
import { Counter, Trend } from "k6/metrics";

const baseURL = (__ENV.LEDGERSYNC_PERF_BFF_URL || "").replace(/\/$/, "");
const publicOrigin = (__ENV.LEDGERSYNC_PERF_PUBLIC_ORIGIN || baseURL).replace(/\/$/, "");
const sourceAccountId = __ENV.LEDGERSYNC_PERF_SOURCE_ACCOUNT;
const destinationAccountId = __ENV.LEDGERSYNC_PERF_DESTINATION_ACCOUNT;
const workloadShape = (__ENV.LEDGERSYNC_PERF_WORKLOAD_SHAPE || "hot").toLowerCase();
const rate = Number(__ENV.LEDGERSYNC_PERF_TPS || 10);
const duration = __ENV.LEDGERSYNC_PERF_DURATION || "5m";
const replayEvery = Math.max(1, Number(__ENV.LEDGERSYNC_PERF_REPLAY_EVERY || (workloadShape === "retry" ? 1 : 10)));
const lostResponseEvery = workloadShape === "retry"
  ? Math.max(1, Number(__ENV.LEDGERSYNC_PERF_LOST_RESPONSE_EVERY || 2))
  : 0;

function parseAccountPairs() {
  const configured = (__ENV.LEDGERSYNC_PERF_ACCOUNT_PAIRS || "").trim();
  if (!configured) return sourceAccountId && destinationAccountId ? [{ source: sourceAccountId, destination: destinationAccountId }] : [];
  return configured.split(";").map((entry) => {
    const [source, destination, extra] = entry.split(">").map((value) => value?.trim());
    if (!source || !destination || extra || source === destination) fail(`invalid account pair: ${entry}`);
    return { source, destination };
  });
}

const accountPairs = parseAccountPairs();

const validWorkloadShape = workloadShape === "hot" || workloadShape === "mixed" || workloadShape === "retry";
if (!baseURL || accountPairs.length === 0 || !validWorkloadShape) {
  fail("LEDGERSYNC_PERF_BFF_URL, a valid workload shape, and at least one account pair are required");
}
if (workloadShape === "mixed" && accountPairs.length < 2) {
  fail("mixed workload requires at least two LEDGERSYNC_PERF_ACCOUNT_PAIRS entries");
}

const transferLatency = new Trend("ledgersync_transfer_duration", true);
const balanceLatency = new Trend("ledgersync_balance_duration", true);
const replayCount = new Counter("ledgersync_same_key_replays");
const simulatedLostResponseCount = new Counter("ledgersync_simulated_lost_responses");
const unexpectedOutcomeCount = new Counter("ledgersync_unexpected_outcomes");

export const options = {
  noCookiesReset: true,
  scenarios: {
    steady_internal_transfers: {
      executor: "constant-arrival-rate",
      rate,
      timeUnit: "1s",
      duration,
      // The journey includes balance, account, history, and reconciliation
      // reads after posting. Keep the injector above the longest permitted
      // end-to-end iteration so generator exhaustion cannot look like service
      // saturation in a constant-arrival-rate qualification.
      preAllocatedVUs: Math.max(50, rate * 3),
      maxVUs: Math.max(200, rate * 10),
      gracefulStop: "15s",
    },
  },
  thresholds: {
    "http_req_failed{journey:transfer}": ["rate<0.001"],
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

function decodePayload(response) {
  try {
    return response.json();
  } catch {
    return undefined;
  }
}

function sendTransfer(key, pair, journey = "transfer", timeout = undefined) {
  const requestOptions = {
    headers: {
      "Content-Type": "application/json",
      "Idempotency-Key": key,
      "X-CSRF-Token": csrfToken,
      Origin: publicOrigin,
    },
    tags: { journey, workload_shape: workloadShape },
  };
  if (timeout) requestOptions.timeout = timeout;
  return http.post(
    `${baseURL}/api/transfers`,
    JSON.stringify({ sourceAccountId: pair.source, destinationAccountId: pair.destination, amount: { currency: "INR", minorUnits: "100" } }),
    requestOptions,
  );
}

function transfer(key, pair) {
  const response = sendTransfer(key, pair);
  transferLatency.add(response.timings.duration, { workload_shape: workloadShape });
  const payload = decodePayload(response);
  const valid = check(response, {
    "transfer posted exactly": (result) => result.status === 201 && payload?.status === "posted" && typeof payload?.transfer_id === "string",
  });
  if (!valid) recordUnexpected(response, "transfer", payload);
  return { response, payload };
}

function transferAfterSimulatedLostResponse(key, pair) {
  const first = sendTransfer(key, pair, "simulated_lost_response", "1ms");
  simulatedLostResponseCount.add(1);
  const firstPayload = decodePayload(first);
  const bounded = check(first, {
    "simulated lost response is timeout or committed response": (result) => result.status === 0 || result.status === 201,
  });
  if (!bounded) recordUnexpected(first, "simulated_lost_response", firstPayload);
  sleep(0.05);
  const replay = transfer(key, pair);
  replayCount.add(1);
  const safe = check(replay.response, {
    "lost-response retry resolves one transfer": (result) => result.status === 201 &&
      (first.status === 0 || (result.headers["Idempotent-Replay"] === "true" && replay.payload?.transfer_id === firstPayload?.transfer_id)),
  });
  if (!safe) recordUnexpected(replay.response, "lost_response_retry", replay.payload);
  return replay;
}

function accountPair() {
  return accountPairs[(__ITER + __VU) % accountPairs.length];
}

export default function () {
  ensureSession();
  const pair = accountPair();
  const key = `k6-${workloadShape}-${__VU}-${__ITER}-${Date.now()}`;
  let first;
  if (lostResponseEvery > 0 && (__ITER + __VU) % lostResponseEvery === 0) {
    first = transferAfterSimulatedLostResponse(key, pair);
  } else {
    first = transfer(key, pair);
  }

  // Sample explicit same-key recovery for normal/hot traffic. Retry-heavy
  // traffic uses the simulated-lost-response path above on its configured rate.
  if (lostResponseEvery === 0 && (__ITER + __VU) % replayEvery === 0 && first.response.status === 201) {
    const replay = transfer(key, pair);
    replayCount.add(1);
    const safe = check(replay.response, {
      "same-key retry replays one transfer": (result) => result.headers["Idempotent-Replay"] === "true" && replay.payload?.transfer_id === first.payload?.transfer_id,
    });
    if (!safe) recordUnexpected(replay.response, "replay", replay.payload);
  }

  const balance = http.get(`${baseURL}/api/accounts/${pair.source}/balance`, { tags: { journey: "balance", workload_shape: workloadShape } });
  balanceLatency.add(balance.timings.duration, { workload_shape: workloadShape });
  if (!check(balance, { "current exact balance returned": (result) => result.status === 200 && typeof result.json()?.available_minor === "string" })) {
    recordUnexpected(balance, "balance", decodePayload(balance));
  }

  if (__ITER % 5 === 0) {
    const accounts = http.get(`${baseURL}/api/me/accounts?limit=25&status=active`, { tags: { journey: "accounts", workload_shape: workloadShape } });
    if (!check(accounts, { "account page available": (result) => result.status === 200 })) recordUnexpected(accounts, "accounts", decodePayload(accounts));
    const history = http.get(`${baseURL}/api/accounts/${pair.source}/transactions?limit=25`, { tags: { journey: "history", workload_shape: workloadShape } });
    if (!check(history, { "history page available": (result) => result.status === 200 })) recordUnexpected(history, "history", decodePayload(history));
  }
  if (__ITER % 20 === 0) {
    check(http.get(`${baseURL}/api/reconciliation/runs?limit=1`, { tags: { journey: "reconciliation", workload_shape: workloadShape } }), { "reconciliation evidence available": (result) => result.status === 200 });
  }
}
