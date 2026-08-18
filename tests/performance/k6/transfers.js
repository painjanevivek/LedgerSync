import http from "k6/http";
import { check, sleep } from "k6";

const apiBaseURL = __ENV.LEDGERSYNC_PERF_API_URL;
const privateToken = __ENV.LEDGERSYNC_PERF_TOKEN;
const actorAssertion = __ENV.LEDGERSYNC_PERF_ACTOR_ASSERTION;
const sourceAccountId = __ENV.LEDGERSYNC_PERF_SOURCE_ACCOUNT;
const destinationAccountId = __ENV.LEDGERSYNC_PERF_DESTINATION_ACCOUNT;

export const options = {
  scenarios: {
    steady_internal_transfers: { executor: "constant-arrival-rate", rate: Number(__ENV.LEDGERSYNC_PERF_TPS || 10), timeUnit: "1s", duration: "5m", preAllocatedVUs: 25, maxVUs: 100 },
  },
  thresholds: { http_req_failed: ["rate<0.001"], http_req_duration: ["p(95)<500"] },
};

export default function () {
  const key = `${__VU}-${__ITER}-${Date.now()}`;
  const response = http.post(`${apiBaseURL}/api/transfers`, JSON.stringify({ source_account_id: sourceAccountId, destination_account_id: destinationAccountId, amount: "0.01", currency: "USD" }), {
    headers: { Authorization: `Bearer ${privateToken}`, "X-LedgerSync-Actor-Assertion": actorAssertion, "Idempotency-Key": key, "Content-Type": "application/json" },
  });
  check(response, { "transfer reached final outcome": (r) => r.status === 201 || r.status === 409 });
  sleep(0.05);
}
