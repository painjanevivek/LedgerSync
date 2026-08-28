"use client";

import { ArrowClockwise, Database, LockKey, ShieldCheck } from "@phosphor-icons/react";
import Link from "next/link";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import type { ReconciliationRun } from "@/features/accounts/types";
import { CopyControl, StatePanel, StatusBadge } from "@/features/console/components";
import {
  newReconciliationIdempotencyKey,
  parseReconciliationCommandIntent,
  reconciliationCommandStorageKey,
  type ReconciliationCommandIntent,
} from "@/features/reconciliation/reconciliationCommandIntent";
import { isReconciliationRun, useReconciliationCommand, type ReconciliationCommandOutcome } from "@/features/reconciliation/useReconciliationCommand";
import { utcDateTime } from "@/features/console/format";

const maximumAutomaticPolls = 8;
const automaticPollIntervalMilliseconds = 1_000;
const automaticPollDeadlineMilliseconds = 15_000;

type Props = Readonly<{
  tenantId: string;
  csrfToken: string;
  online: boolean;
  canWrite: boolean;
  latestRun: ReconciliationRun | null;
  onObserved: (run: ReconciliationRun) => void;
  onRefreshHistory: () => Promise<void>;
}>;

export function ReconciliationCommand({ tenantId, csrfToken, online, canWrite, latestRun, onObserved, onRefreshHistory }: Props) {
  const storageKey = useMemo(() => reconciliationCommandStorageKey(tenantId), [tenantId]);
  const [intent, setIntent] = useState<ReconciliationCommandIntent | null>(null);
  const [outcome, setOutcome] = useState<ReconciliationCommandOutcome | null>(null);
  const [result, setResult] = useState<Extract<ReconciliationCommandOutcome, { kind: "run" }> | null>(null);
  const [pollAttempts, setPollAttempts] = useState(0);
  const [pollingStopped, setPollingStopped] = useState(false);
  const [checking, setChecking] = useState(false);
  const [statusMessage, setStatusMessage] = useState<string | null>(null);
  const { pending, send } = useReconciliationCommand(csrfToken);
  const reviewHeading = useRef<HTMLHeadingElement>(null);
  const outcomeHeading = useRef<HTMLHeadingElement>(null);
  const pollInFlight = useRef(false);
  const pollController = useRef<AbortController | null>(null);
  const mounted = useRef(true);

  const persist = useCallback((next: ReconciliationCommandIntent | null) => {
    setIntent(next);
    if (next) sessionStorage.setItem(storageKey, JSON.stringify(next));
    else sessionStorage.removeItem(storageKey);
  }, [storageKey]);

  useEffect(() => {
    const frame = requestAnimationFrame(() => {
      const restored = parseReconciliationCommandIntent(sessionStorage.getItem(storageKey), tenantId);
      if (!restored) return;
      setIntent(restored);
      if (restored.state === "unknown") setOutcome({ kind: "unknown", code: "retained_unknown", message: "A previous reconciliation submission has no confirmed result. Its exact retry key remains locked for safe recovery." });
    });
    return () => cancelAnimationFrame(frame);
  }, [storageKey, tenantId]);

  useEffect(() => () => {
    mounted.current = false;
    pollController.current?.abort();
  }, []);

  useEffect(() => {
    if (intent?.state === "review") reviewHeading.current?.focus();
    else if (intent?.state === "running") outcomeHeading.current?.focus();
  }, [intent?.state]);

  useEffect(() => {
    if (outcome || result) outcomeHeading.current?.focus();
  }, [outcome, result]);

  const observe = useCallback((run: ReconciliationRun, commandResult?: Extract<ReconciliationCommandOutcome, { kind: "run" }>) => {
    onObserved(run);
    setStatusMessage(null);
    if (run.status === "running") {
      const sameRunningIntent = intent?.state === "running" && intent.runId === run.run_id;
      persist({
        version: 1,
        tenantId,
        idempotencyKey: intent?.idempotencyKey ?? newReconciliationIdempotencyKey(),
        state: "running",
        runId: run.run_id,
        submittedAt: intent?.submittedAt ?? new Date().toISOString(),
      });
      if (!sameRunningIntent) {
        setPollAttempts(0);
        setPollingStopped(false);
      }
      setOutcome(null);
      setResult(null);
      return;
    }
    persist(null);
    setOutcome(null);
    setResult(commandResult ?? { kind: "run", run, replayed: false });
    void onRefreshHistory();
  }, [intent, onObserved, onRefreshHistory, persist, tenantId]);

  const refreshRun = useCallback(async (runId: string, manual = false) => {
    if (!online || pollInFlight.current) return false;
    const controller = new AbortController();
    pollController.current = controller;
    pollInFlight.current = true;
    setChecking(true);
    try {
      const response = await fetch(`/api/reconciliation/runs/${encodeURIComponent(runId)}`, { cache: "no-store", signal: controller.signal });
      const value: unknown = await response.json().catch(() => ({}));
      if (response.ok && isReconciliationRun(value)) {
        observe(value);
        if (manual && value.status === "running") setStatusMessage("The authoritative run is still in progress. No pass or mismatch result has been inferred.");
        return value.status !== "running";
      }
      setStatusMessage("Current run evidence is unavailable. The last verified history remains visible; no result is inferred.");
      return false;
    } catch (error) {
      if (!(error instanceof DOMException && error.name === "AbortError")) setStatusMessage("Current run evidence is unavailable. The last verified history remains visible; no result is inferred.");
      return false;
    } finally {
      if (pollController.current === controller) pollController.current = null;
      pollInFlight.current = false;
      if (mounted.current) setChecking(false);
    }
  }, [observe, online]);

  useEffect(() => {
    if (intent?.state !== "running" || !intent.runId || !online || pollingStopped) return;
    if (pollAttempts >= maximumAutomaticPolls) return;
    const submittedAt = intent.submittedAt ? Date.parse(intent.submittedAt) : Date.now();
    const remaining = submittedAt + automaticPollDeadlineMilliseconds - Date.now();
    if (remaining <= 0) {
      const expired = window.setTimeout(() => {
        setPollingStopped(true);
        setStatusMessage("Automatic checking reached its fixed deadline. Use manual refresh when authoritative evidence is available.");
      }, 0);
      return () => window.clearTimeout(expired);
    }
    const timer = window.setTimeout(() => {
      void refreshRun(intent.runId!).finally(() => {
        const nextAttempt = pollAttempts + 1;
        setPollAttempts(nextAttempt);
        if (nextAttempt >= maximumAutomaticPolls || Date.now() >= submittedAt + automaticPollDeadlineMilliseconds) {
          setPollingStopped(true);
          setStatusMessage("Automatic checking reached its fixed deadline. Use manual refresh when authoritative evidence is available.");
        }
      });
    }, Math.min(automaticPollIntervalMilliseconds, remaining));
    return () => window.clearTimeout(timer);
  }, [intent, online, pollAttempts, pollingStopped, refreshRun]);

  function beginReview() {
    setOutcome(null);
    setResult(null);
    setStatusMessage(null);
    persist({ version: 1, tenantId, idempotencyKey: newReconciliationIdempotencyKey(), state: "review" });
  }

  async function submit(retained: ReconciliationCommandIntent) {
    const submitted: ReconciliationCommandIntent = {
      ...retained,
      state: "unknown",
      submittedAt: retained.submittedAt ?? new Date().toISOString(),
    };
    persist(submitted);
    setOutcome(null);
    setResult(null);
    const next = await send(submitted.idempotencyKey);
    if (next.kind === "run") {
      observe(next.run, next);
      return;
    }
    if (next.kind === "already_running") {
      persist(null);
      setOutcome(next);
      void onRefreshHistory();
      return;
    }
    setOutcome(next);
    if (next.kind === "denied" || next.kind === "error") persist(null);
  }

  function abandonUnknown() {
    if (!window.confirm("Stop retaining this retry key? This does not cancel a reconciliation that may already have started. Refresh history before starting another run.")) return;
    persist(null);
    setOutcome(null);
    setStatusMessage("The retry key is no longer retained. This did not cancel any reconciliation that may already be running.");
  }

  const running = intent?.state === "running" && intent.runId;
  const unknown = intent?.state === "unknown";
  const commandLocked = Boolean(intent && intent.state !== "review");
  const watermark = latestRun?.ledger_watermark || "No retained watermark";

  return <section className="reconciliation-control" aria-labelledby="reconciliation-control-heading">
    <div className="section-heading reconciliation-control-heading">
      <div><p className="eyebrow">Serialized operator control</p><h2 id="reconciliation-control-heading">Run reconciliation</h2><p>Start one tenant-wide comparison between projected balances and immutable postings. Existing run evidence remains available below.</p></div>
      <button className="button primary guarded-control" type="button" disabled={!online || !canWrite || pending || commandLocked} onClick={beginReview}><ShieldCheck aria-hidden="true" />Run reconciliation</button>
    </div>
    {!canWrite && <p className="permission-note">Read-only role: reconciliation can be inspected, but your role cannot start a run.</p>}
    {!online && <StatePanel kind="offline" title="Reconciliation controls are offline" message="Known run history remains visible. Starting and checking a run are disabled until the browser reconnects." />}

    {intent?.state === "review" && <section className="surface reconciliation-command-document" aria-labelledby="reconciliation-review-heading">
      <p className="eyebrow">Command review</p>
      <h3 ref={reviewHeading} tabIndex={-1} id="reconciliation-review-heading">Review authoritative reconciliation scope</h3>
      <div className="reconciliation-boundary-proof">
        <div><Database aria-hidden="true" /><span>Tenant scope</span><strong>All authorized INR accounts</strong></div>
        <div><LockKey aria-hidden="true" /><span>Current recorded ledger watermark</span><strong>{watermark}</strong></div>
        <p><span>Comparison</span><strong>Projected balances against immutable postings</strong></p>
        <p><span>Financial mutation</span><strong>None — no balance, transfer, posting, or journal entry is changed</strong></p>
      </div>
      <p className="guardrail-copy">The private service serializes runs per tenant. Completion creates new immutable evidence; it does not replace earlier successful or mismatch history.</p>
      <div className="action-row reconciliation-command-actions"><button className="button secondary guarded-control" type="button" disabled={pending} onClick={() => persist(null)}>Cancel</button><button className="button primary guarded-control" type="button" disabled={pending || !online || !canWrite} onClick={() => void submit(intent)}>{pending ? "Starting reconciliation…" : "Start reconciliation"}</button></div>
    </section>}

    {running && <section className="surface reconciliation-command-document running" role="status" aria-labelledby="reconciliation-running-heading" aria-live="polite">
      <p className="eyebrow">Authoritative run in progress</p>
      <h3 ref={outcomeHeading} tabIndex={-1} id="reconciliation-running-heading">Reconciliation running</h3>
      <p>LedgerSync has a stable run ID. No passing or mismatch result is inferred yet. Automatic checking is bounded to {maximumAutomaticPolls} attempts and a fixed deadline.</p>
      <dl className="review-grid reconciliation-run-grid"><div><dt>Run ID</dt><dd><CopyControl value={intent.runId!} /></dd></div><div><dt>Status</dt><dd><StatusBadge tone="warning">Running</StatusBadge></dd></div><div><dt>Request submitted</dt><dd>{intent.submittedAt ? utcDateTime(intent.submittedAt) : "Submission time unavailable"}</dd></div></dl>
      {statusMessage && <StatePanel kind={pollingStopped ? "unknown" : "empty"} title={pollingStopped ? "Automatic checking stopped" : "Run status checked"} message={statusMessage} />}
      <div className="action-row"><button className="button secondary guarded-control" type="button" disabled={!online||checking} onClick={() => void refreshRun(intent.runId!, true)}><ArrowClockwise aria-hidden="true" />{checking?"Checking run status…":"Refresh run status"}</button><Link className="button secondary guarded-control" href={`/reconciliation/${encodeURIComponent(intent.runId!)}`}>Open run evidence</Link></div>
    </section>}

    {unknown && outcome && outcome.kind !== "run" && <section className="account-command-recovery reconciliation-command-recovery" aria-labelledby="reconciliation-unknown-heading" aria-live="polite">
      <h3 ref={outcomeHeading} tabIndex={-1} id="reconciliation-unknown-heading">Reconciliation outcome not confirmed</h3>
      <StatePanel kind="unknown" title="Exact request key retained" message={outcome.message} />
      <dl className="review-grid reconciliation-run-grid"><div><dt>Idempotency key</dt><dd><CopyControl value={intent.idempotencyKey} label="Copy retained reconciliation key" /></dd></div><div><dt>Submitted</dt><dd>{intent.submittedAt ? utcDateTime(intent.submittedAt) : "Submission time unavailable"}</dd></div></dl>
      <div className="action-row reconciliation-command-actions"><button className="button secondary guarded-control" type="button" disabled={!online} onClick={() => void onRefreshHistory()}>Refresh run history</button><button className="button secondary guarded-control" type="button" disabled={pending} onClick={abandonUnknown}>Stop retaining request</button><button className="button primary guarded-control" type="button" disabled={pending || !online || !canWrite} onClick={() => void submit(intent)}>{pending ? "Retrying retained request…" : "Retry same reconciliation request"}</button></div>
    </section>}

    {outcome?.kind === "already_running" && <section className="account-command-recovery reconciliation-command-recovery" aria-labelledby="reconciliation-conflict-heading">
      <h3 ref={outcomeHeading} tabIndex={-1} id="reconciliation-conflict-heading">Reconciliation already running</h3>
      <StatePanel kind="unknown" title="Parallel run prevented" message={outcome.message} />
      <div className="action-row"><button className="button secondary guarded-control" type="button" onClick={() => void onRefreshHistory()}>Refresh run history</button>{outcome.runId && <Link className="button primary guarded-control" href={`/reconciliation/${encodeURIComponent(outcome.runId)}`}>Open running reconciliation</Link>}</div>
    </section>}

    {outcome && outcome.kind !== "run" && outcome.kind !== "already_running" && !unknown && <section className="account-command-recovery reconciliation-command-recovery" aria-labelledby="reconciliation-error-heading">
      <h3 ref={outcomeHeading} tabIndex={-1} id="reconciliation-error-heading">Reconciliation did not start</h3>
      <StatePanel kind={outcome.kind === "denied" ? "denied" : "error"} title={outcome.kind === "denied" ? "Run not authorized" : "No result produced"} message={outcome.message} />
    </section>}

    {result && <section className={`surface reconciliation-command-result ${result.run.status === "mismatch" || result.run.status === "failed" ? "mismatch" : ""}`} role="region" aria-labelledby="reconciliation-result-heading" aria-live="polite">
      <p className="eyebrow">Authoritative command result</p>
      <h3 ref={outcomeHeading} tabIndex={-1} id="reconciliation-result-heading">{result.run.status === "matched" ? "Reconciliation passed" : result.run.status === "mismatch" ? "Mismatch detected" : "Reconciliation failed"}</h3>
      <p>{result.run.status === "matched" ? "Projected balances agree with immutable postings for the recorded scope and watermark." : "No all-clear conclusion is shown. Investigate the retained run evidence before normal processing continues."}</p>
      <dl className="review-grid reconciliation-run-grid"><div><dt>Run ID</dt><dd><CopyControl value={result.run.run_id} /></dd></div><div><dt>Request handling</dt><dd>{result.replayed ? "Original run returned from the retained key" : "New run response"}</dd></div>{result.requestReference && <div><dt>Request reference</dt><dd><CopyControl value={result.requestReference} label="Copy request reference" /></dd></div>}</dl>
      <Link className="button primary guarded-control" href={`/reconciliation/${encodeURIComponent(result.run.run_id)}`}>Open authoritative run evidence</Link>
    </section>}

    {!intent && !outcome && !result && statusMessage && <StatePanel kind="unknown" title="Retained request released" message={statusMessage} />}
  </section>;
}
