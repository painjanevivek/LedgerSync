"use client";

import { WarningCircle } from "@phosphor-icons/react";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useRef, useState } from "react";

import type { Account } from "@/features/accounts/types";
import { ConsoleFooter, ConsoleShell, OperatorWorkspace } from "@/features/console/ConsoleShell";
import { useConsoleSession } from "@/features/console/ConsoleSessionBoundary";
import { PageHeader, StatePanel } from "@/features/console/components";
import { appendUniqueBy, beginEvidenceRequest, createEvidenceRequestCoordinator, finishEvidenceRequest, invalidateEvidenceRequests, isEvidenceRequestCurrent } from "@/features/console/evidenceRequestCoordinator";
import { FundingRequestFlow } from "@/features/funding/FundingRequestFlow";
import { FundingDetailView, FundingListView, FundingWorkspaceRail } from "@/features/funding/FundingViews";
import type { FundingEvent, FundingPage, FundingReconciliation, FundingSubmission } from "@/lib/api/funding";
import { readJSON, unavailableMessage } from "@/lib/api/client";

type AccountPage = Readonly<{ accounts: Account[]; next_cursor?: string }>;

export function FundingConsole({ fundingEventId }: Readonly<{ fundingEventId?: string }>) {
  const router = useRouter();
  const { session, sessionLoading, sessionError, online, hasScope } = useConsoleSession();
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [accountsLoading, setAccountsLoading] = useState(false);
  const [accountsError, setAccountsError] = useState<string | null>(null);
  const [accountsScopeComplete, setAccountsScopeComplete] = useState(false);
  const [events, setEvents] = useState<FundingEvent[]>([]);
  const [selected, setSelected] = useState<FundingEvent | null>(null);
  const [reconciliation, setReconciliation] = useState<FundingReconciliation | null>(null);
  const [nextCursor, setNextCursor] = useState<string>();
  const [loading, setLoading] = useState(false);
  const [actionBusy, setActionBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [verifiedAt, setVerifiedAt] = useState<string>();
  const [requestOpen, setRequestOpen] = useState(false);
  const listRequests = useRef(createEvidenceRequestCoordinator());
  const detailRequests = useRef(createEvidenceRequestCoordinator());
  const reconciliationRequests = useRef(createEvidenceRequestCoordinator());
  const accountGeneration = useRef(0);

  const loadAccounts = useCallback(async () => {
    const current = ++accountGeneration.current;
    setAccountsLoading(true);
    setAccountsError(null);
    const response = await readJSON<AccountPage>("/api/me/accounts?limit=100&status=active");
    if (current !== accountGeneration.current) return;
    if (response.ok && Array.isArray(response.data.accounts)) {
      setAccounts(response.data.accounts);
      setAccountsScopeComplete(!response.data.next_cursor);
    } else {
      setAccountsError(unavailableMessage(response.status, "eligible funding accounts", response.requestReference));
      setAccountsScopeComplete(false);
    }
    setAccountsLoading(false);
  }, []);

  const loadList = useCallback(async (cursor?: string) => {
    const request = beginEvidenceRequest(listRequests.current, "funding:list", cursor ? "append" : "replace");
    if (!request) return;
    setLoading(true); setError(null);
    const suffix = cursor ? `&cursor=${encodeURIComponent(cursor)}` : "";
    const response = await readJSON<FundingPage>(`/api/funding-events?limit=25${suffix}`);
    if (!isEvidenceRequestCurrent(listRequests.current, request.token)) return;
    if (response.ok && Array.isArray(response.data.events)) {
      setEvents((existing) => cursor ? appendUniqueBy(existing, response.data.events, (event) => event.funding_event_id) : response.data.events);
      setNextCursor(response.data.next_cursor || undefined);
      setVerifiedAt(new Date().toISOString());
    } else setError(unavailableMessage(response.status, "funding records", response.requestReference));
    if (finishEvidenceRequest(listRequests.current, request.token)) setLoading(false);
  }, []);

  const loadEvent = useCallback(async () => {
    if (!fundingEventId) return null;
    const request = beginEvidenceRequest(detailRequests.current, `funding:${fundingEventId}`);
    if (!request) return null;
    setLoading(true); setError(null); setReconciliation(null);
    if (!request.sameResource) { setSelected(null); setVerifiedAt(undefined); }
    const response = await readJSON<FundingEvent>(`/api/funding-events/${encodeURIComponent(fundingEventId)}`);
    if (!isEvidenceRequestCurrent(detailRequests.current, request.token)) return null;
    if (response.ok && response.data.funding_event_id) {
      setSelected(response.data);
      setVerifiedAt(new Date().toISOString());
      setLoading(false);
      return response.data;
    } else {
      setError(response.status === 404 ? `The selected funding record was not found in this authorized tenant scope. Request reference: ${response.requestReference}.` : unavailableMessage(response.status, "funding records", response.requestReference));
    }
    if (finishEvidenceRequest(detailRequests.current, request.token)) setLoading(false);
    return null;
  }, [fundingEventId]);

  useEffect(() => {
    if (!session || !online || !hasScope("funding:read")) return;
    const listCoordinator = listRequests.current;
    const detailCoordinator = detailRequests.current;
    const timer = window.setTimeout(() => { void loadAccounts(); void (fundingEventId ? loadEvent() : loadList()); }, 0);
    return () => { window.clearTimeout(timer); invalidateEvidenceRequests(listCoordinator); invalidateEvidenceRequests(detailCoordinator); accountGeneration.current += 1; };
  }, [fundingEventId, hasScope, loadAccounts, loadEvent, loadList, online, session]);

  async function act(path: string, body?: Record<string, string>, idempotencyKey?: string) {
    if (!session || !selected) return false;
    setActionBusy(true); setError(null);
    try {
      const headers: Record<string, string> = { "X-CSRF-Token": session.csrf_token };
      if (body) headers["Content-Type"] = "application/json";
      if (idempotencyKey) headers["Idempotency-Key"] = idempotencyKey;
      const response = await fetch(`/api/funding-events/${encodeURIComponent(selected.funding_event_id)}/${path}`, { method: "POST", headers, body: body ? JSON.stringify(body) : undefined });
      const payload = await response.json() as FundingEvent | FundingSubmission | { error?: string };
      if (!response.ok) throw new Error(response.status === 504 ? "Funding outcome unknown. Refresh this evidence before retrying the identical action." : `Action was not recorded (${"error" in payload ? payload.error ?? response.status : response.status}).`);
      const resultingEvent = "event" in payload ? payload.event : "funding_event_id" in payload ? payload : null;
      if (path === "compensations" && resultingEvent?.funding_event_id) router.push(`/funding/${encodeURIComponent(resultingEvent.funding_event_id)}`);
      else await loadEvent();
      return true;
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "The funding action could not be verified.");
      return false;
    } finally {
      setActionBusy(false);
    }
  }

  async function reconcile() {
    if (!selected) return;
    const eventId = selected.funding_event_id;
    const request = beginEvidenceRequest(reconciliationRequests.current, `funding-reconciliation:${eventId}`);
    if (!request) return;
    setActionBusy(true); setError(null);
    const response = await readJSON<FundingReconciliation>(`/api/funding-events/${encodeURIComponent(eventId)}/reconciliation`);
    if (!isEvidenceRequestCurrent(reconciliationRequests.current, request.token)) return;
    if (response.ok && response.data.funding_event_id) setReconciliation(response.data);
    else setError(unavailableMessage(response.status, "funding reconciliation", response.requestReference));
    if (finishEvidenceRequest(reconciliationRequests.current, request.token)) setActionBusy(false);
  }

  if (sessionLoading) return <ConsoleShell section="funding" tenantLabel="Verifying tenant" tenantMeta="Secure session" environmentLabel="Checking environment" operatorLabel="Verifying operator" operatorMeta="Authorization pending"><OperatorWorkspace className="funding-workspace" footer={<ConsoleFooter />} rail={<FundingWorkspaceRail events={[]} loading error={null} online />} railLabel="Funding workflow context"><PageHeader eyebrow="Funding records · LedgerSync" title="Verifying access" description="Checking finance access before any external reference or journal state is shown." /><StatePanel title="Loading authorized records" message="No funding state is inferred while the session boundary is verified." /></OperatorWorkspace></ConsoleShell>;
  if (!session) return <main className="boot-screen"><p className="eyebrow">Access not verified</p><h1>Funding workspace unavailable</h1><StatePanel kind={sessionError ? "error" : "denied"} title={sessionError ? "Session unavailable" : "No authorized session"} message={sessionError ?? "Sign in with an approved finance operator identity. No funding records are displayed."} /></main>;

  const canRead = hasScope("funding:read");
  const canWrite = hasScope("funding:write");
  const canApprove = hasScope("funding:approve");
  const selectedAccount = accounts.find((account) => account.account_id === selected?.destination_account_id);
  return <ConsoleShell section="funding" tenantLabel={session.tenant_label ?? "Ledger tenant"} tenantMeta={session.tenant_id} environmentLabel={session.environment === "local" ? "Local workspace" : "Verified production"} operatorLabel={session.operator_label ?? session.subject_id} operatorMeta={session.environment === "local" ? "This workstation" : "Authorized finance operator"}>
    <OperatorWorkspace className="funding-workspace" footer={<ConsoleFooter />} rail={<FundingWorkspaceRail events={events} event={selected} loading={loading} error={error} online={online} />} railLabel="Funding workflow context">
      {!online && <div className="offline-banner" role="status"><WarningCircle weight="fill" aria-hidden="true" /><span><strong>You are offline.</strong> Funding actions are disabled; retained evidence is historical until refreshed.</span></div>}
      {!canRead ? <><PageHeader eyebrow="Ledger / Funding records" title="Funding records" description="Controlled external value references and their balanced journals." /><StatePanel kind="denied" title="Funding read scope required" message="Ask a tenant administrator for funding:read. LedgerSync does not broaden funding record visibility." /></> : fundingEventId ? <FundingDetailView event={selected} account={selectedAccount} session={session} reconciliation={reconciliation} verifiedAt={verifiedAt} loading={loading} actionBusy={actionBusy} error={error} online={online} canWrite={canWrite} canApprove={canApprove} onRefresh={loadEvent} onAction={act} onReconcile={() => void reconcile()} /> : <><FundingListView events={events} accounts={accounts} nextCursor={nextCursor} verifiedAt={verifiedAt} loading={loading} error={error} online={online} canWrite={canWrite} onOpenRequest={() => setRequestOpen(true)} onRefresh={() => void loadList()} onNext={() => nextCursor && void loadList(nextCursor)} /><FundingRequestFlow accounts={accounts} accountsLoading={accountsLoading} accountsError={accountsError} accountsScopeComplete={accountsScopeComplete} onRetryAccounts={() => void loadAccounts()} csrfToken={session.csrf_token} online={online} canWrite={canWrite} open={requestOpen} onClose={() => setRequestOpen(false)} onCreated={async (created) => { setEvents((current) => [created, ...current]); router.push(`/funding/${encodeURIComponent(created.funding_event_id)}`); }} /></>}
    </OperatorWorkspace>
  </ConsoleShell>;
}
