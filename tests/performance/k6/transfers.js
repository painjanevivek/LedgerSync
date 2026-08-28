import http from "k6/http";
import { check, fail, sleep } from "k6";
import { Counter, Trend } from "k6/metrics";

const baseURL = (__ENV.LEDGERSYNC_PERF_BFF_URL || "").replace(/\/$/, "");
const publicOrigin = (__ENV.LEDGERSYNC_PERF_PUBLIC_ORIGIN || baseURL).replace(/\/$/, "");
const publicHost = publicOrigin.replace(/^https?:\/\//, "").split("/")[0];
const sourceAccountId = __ENV.LEDGERSYNC_PERF_SOURCE_ACCOUNT;
const destinationAccountId = __ENV.LEDGERSYNC_PERF_DESTINATION_ACCOUNT;
const workloadShape = (__ENV.LEDGERSYNC_PERF_WORKLOAD_SHAPE || "hot").toLowerCase();
const rate = Number(__ENV.LEDGERSYNC_PERF_TPS || 10);
const duration = __ENV.LEDGERSYNC_PERF_DURATION || "5m";
const durationMatch = duration.match(/^([1-9][0-9]*)(s|m)$/);
const durationSeconds = durationMatch ? Number(durationMatch[1]) * (durationMatch[2] === "m" ? 60 : 1) : 0;
const controlIterations = Math.ceil(durationSeconds / 60);
const controlIntervalSeconds = durationSeconds / Math.max(1, controlIterations);
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
if (!baseURL || accountPairs.length === 0 || !validWorkloadShape || durationSeconds === 0) {
  fail("LEDGERSYNC_PERF_BFF_URL, a valid workload shape, and at least one account pair are required");
}
if (workloadShape === "mixed" && accountPairs.length < 2) {
  fail("mixed workload requires at least two LEDGERSYNC_PERF_ACCOUNT_PAIRS entries");
}

const transferLatency = new Trend("ledgersync_transfer_duration", true);
const balanceLatency = new Trend("ledgersync_balance_duration", true);
const diagnosticsLatency = new Trend("ledgersync_diagnostics_duration", true);
const eventsLatency = new Trend("ledgersync_events_duration", true);
const accountCommandLatency = new Trend("ledgersync_account_command_duration", true);
const reconciliationCommandLatency = new Trend("ledgersync_reconciliation_command_duration", true);
const replayCount = new Counter("ledgersync_same_key_replays");
const transferIterationCount = new Counter("ledgersync_transfer_iterations");
const controlIterationCount = new Counter("ledgersync_control_iterations");
const controlAccountCount = new Counter("ledgersync_control_accounts_created");
const controlReconciliationCount = new Counter("ledgersync_control_reconciliations_completed");
const simulatedLostResponseCount = new Counter("ledgersync_simulated_lost_responses");
const unexpectedOutcomeCount = new Counter("ledgersync_unexpected_outcomes");

const scenarios = {
  steady_internal_transfers: {
    executor: "per-vu-iterations",
    exec: "transferTraffic",
    vus: rate,
    iterations: durationSeconds,
    maxDuration: `${durationSeconds + 60}s`,
  },
};

if (workloadShape === "mixed") {
  scenarios.low_rate_controls = {
    executor: "per-vu-iterations",
    exec: "lowRateControls",
    vus: 1,
    iterations: controlIterations,
    maxDuration: `${durationSeconds + 60}s`,
  };
}

export const options = {
  noCookiesReset: true,
  scenarios,
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
  const payload = decodePayload(response);
  const established = check(response, {
    "BFF session established": (result) => result.status === 200 && typeof payload?.csrf_token === "string" && payload.csrf_token.length > 0,
  });
  if (!established) {
    recordUnexpected(response, "session", payload);
    fail(`BFF session unavailable (${response.status})`);
  }
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
      // The host-networked load container reaches the exact configured public
      // origin. This header is an assertion of that contract, not an override
      // for a different transport host.
      Host: publicHost,
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

function mutationOptions(key, journey) {
  return {
    headers: {
      "Content-Type": "application/json",
      "Idempotency-Key": key,
      "X-CSRF-Token": csrfToken,
      Origin: publicOrigin,
      Host: publicHost,
    },
    tags: { journey, workload_shape: workloadShape },
  };
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

export function transferTraffic() {
  const iterationStartedAt = Date.now();
  ensureSession();
  transferIterationCount.add(1);
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

  if (__ITER < durationSeconds - 1) {
    sleep(Math.max(0, (iterationStartedAt + 1000 - Date.now()) / 1000));
  }
}

export function lowRateControls() {
  const iterationStartedAt = Date.now();
  ensureSession();
  controlIterationCount.add(1);

  const suffix = `${__VU}-${__ITER}-${Date.now()}`;
  const accountKey = `k6-account-${suffix}`;
  const accountBody = JSON.stringify({
    display_name: `Capacity control ${suffix}`,
    external_reference: `capacity-control-${suffix}`,
    category: "operating",
    currency: "INR",
  });
  const account = http.post(`${baseURL}/api/me/accounts`, accountBody, mutationOptions(accountKey, "account_command"));
  accountCommandLatency.add(account.timings.duration);
  const accountPayload = decodePayload(account);
  const accountCreated = check(account, {
    "zero account control created": (result) => result.status === 201 &&
      typeof accountPayload?.account_id === "string" &&
      accountPayload?.available_minor === "0" &&
      accountPayload?.ledger_minor === "0",
  });
  if (!accountCreated) {
    recordUnexpected(account, "account_command", accountPayload);
  } else {
    controlAccountCount.add(1);
    const replay = http.post(`${baseURL}/api/me/accounts`, accountBody, mutationOptions(accountKey, "account_command_replay"));
    accountCommandLatency.add(replay.timings.duration, { replay: "true" });
    const replayPayload = decodePayload(replay);
    const replayed = check(replay, {
      "account control same-key retry replays": (result) => result.status === 201 &&
        result.headers["Idempotent-Replay"] === "true" &&
        replayPayload?.account_id === accountPayload?.account_id,
    });
    if (!replayed) recordUnexpected(replay, "account_command_replay", replayPayload);
  }

  const reconciliationKey = `k6-reconciliation-${suffix}`;
  const reconciliationOptions = mutationOptions(reconciliationKey, "reconciliation_command");
  reconciliationOptions.responseCallback = http.expectedStatuses(201, 409);
  const reconciliation = http.post(
    `${baseURL}/api/reconciliation/runs`,
    "{}",
    reconciliationOptions,
  );
  reconciliationCommandLatency.add(reconciliation.timings.duration);
  const reconciliationPayload = decodePayload(reconciliation);
  const reconciliationCompleted = check(reconciliation, {
    "reconciliation control is completed or safely excluded": (result) =>
      (result.status === 201 && (reconciliationPayload?.status === "matched" || reconciliationPayload?.status === "mismatch")) ||
      (result.status === 409 && reconciliationPayload?.error?.code === "reconciliation_already_running" && typeof reconciliationPayload?.run_id === "string"),
  });
  if (!reconciliationCompleted) {
    recordUnexpected(reconciliation, "reconciliation_command", reconciliationPayload);
  } else if (reconciliation.status === 201) {
    controlReconciliationCount.add(1);
  }

  const diagnostics = http.get(`${baseURL}/api/local/diagnostics`, { tags: { journey: "diagnostics", workload_shape: workloadShape } });
  diagnosticsLatency.add(diagnostics.timings.duration);
  const diagnosticsPayload = decodePayload(diagnostics);
  if (!check(diagnostics, {
    "bounded diagnostics evidence available": (result) => result.status === 200 &&
      (diagnosticsPayload?.overall_state === "ready" || diagnosticsPayload?.overall_state === "degraded"),
  })) recordUnexpected(diagnostics, "diagnostics", diagnosticsPayload);

  const events = http.get(`${baseURL}/api/events?limit=10`, { tags: { journey: "events", workload_shape: workloadShape } });
  eventsLatency.add(events.timings.duration);
  const eventsPayload = decodePayload(events);
  if (!check(events, {
    "bounded event evidence page available": (result) => result.status === 200 && Array.isArray(eventsPayload?.events) && eventsPayload.events.length <= 10,
  })) recordUnexpected(events, "events", eventsPayload);

  const transfers = http.get(`${baseURL}/api/transfers?limit=10&status=posted`, { tags: { journey: "transfer_list", workload_shape: workloadShape } });
  const transfersPayload = decodePayload(transfers);
  if (!check(transfers, {
    "bounded filtered transfer page available": (result) => result.status === 200 && Array.isArray(transfersPayload?.transfers) && transfersPayload.transfers.length <= 10,
  })) recordUnexpected(transfers, "transfer_list", transfersPayload);

  if (__ITER < controlIterations - 1) {
    sleep(Math.max(0, (iterationStartedAt + controlIntervalSeconds * 1000 - Date.now()) / 1000));
  }
}
