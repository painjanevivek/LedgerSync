"use client";

import { WarningCircle } from "@phosphor-icons/react";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";

import { AccountsView, type AccountFilters } from "@/features/accounts/AccountViews";
import { AccountCreateFlow } from "@/features/accounts/AccountCreateFlow";
import { hasPositiveMinorUnits } from "@/features/accounts/accountCommandIntent";
import type { ConsoleSession, ReconciliationRun, TransferDetail, TransferSummary } from "@/features/accounts/types";
import { emptyAccountFilters, useAccountWorkspace } from "@/features/accounts/useAccountWorkspace";
import { ConsoleFooter, ConsoleShell, type ConsoleSection } from "@/features/console/ConsoleShell";
import { PageHeader, StatePanel } from "@/features/console/components";
import { OverviewView } from "@/features/overview/OverviewView";
import { ReconciliationView } from "@/features/reconciliation/ReconciliationViews";
import { TransfersView } from "@/features/transfers/TransferViews";
import { readJSON, unavailableMessage } from "@/lib/api/client";
import type { LocalOrientation, TransferExplainability } from "@/lib/api/orientation";

type Props = Readonly<{
  initialSection?: ConsoleSection;
  initialAccountId?: string;
  initialTransferId?: string;
  initialReconciliationRunId?: string;
  initialReconciliationReturnTo?: string;
  initialAccountFilters?: AccountFilters;
  initialAccountFocusId?: string;
  initialAccountCreate?: boolean;
  initialAccountReturnTo?: string;
  initialTransferDestinationId?: string;
  initialTransferReturnTo?: string;
  initialTransferFilters?: { query: string; status: string };
  initialShowOrientation?: boolean;
}>;
type TransfersPayload = { transfers?: TransferSummary[]; next_cursor?: string };
type RunsPayload = { runs?: ReconciliationRun[]; next_cursor?: string };

export function OperatorConsole({ initialSection = "overview", initialAccountId, initialTransferId, initialReconciliationRunId, initialReconciliationReturnTo, initialAccountFilters = emptyAccountFilters, initialAccountFocusId, initialAccountCreate = false, initialAccountReturnTo = "/accounts", initialTransferDestinationId, initialTransferReturnTo, initialTransferFilters, initialShowOrientation = false }: Props) {
  const router = useRouter();
  const [session, setSession] = useState<ConsoleSession | null>(null);
  const accountWorkspace = useAccountWorkspace(initialAccountId, initialAccountFilters);
  const { load: loadAccounts, loadDetail: loadAccountDetail } = accountWorkspace;
  const [transfers, setTransfers] = useState<TransferSummary[]>([]);
  const [transferDetail, setTransferDetail] = useState<TransferDetail | null>(null);
  const [runs, setRuns] = useState<ReconciliationRun[]>([]);
  const [runDetail, setRunDetail] = useState<ReconciliationRun | null>(null);
  const [loading, setLoading] = useState(true);
  const [transfersLoading, setTransfersLoading] = useState(false);
  const [transferDetailLoading, setTransferDetailLoading] = useState(false);
  const [runsLoading, setRunsLoading] = useState(false);
  const [runDetailLoading, setRunDetailLoading] = useState(false);
  const [transferError, setTransferError] = useState<string | null>(null);
  const [reconciliationError, setReconciliationError] = useState<string | null>(null);
  const [transferCursor, setTransferCursor] = useState<string>();
  const [runCursor, setRunCursor] = useState<string>();
  const [online, setOnline] = useState(true);
  const [orientation, setOrientation] = useState<LocalOrientation | null>(null);
  const [orientationLoading, setOrientationLoading] = useState(false);
  const [orientationError, setOrientationError] = useState<string | null>(null);
  const [explainability, setExplainability] = useState<TransferExplainability | null>(null);
  const [explainabilityLoading, setExplainabilityLoading] = useState(false);
  const [explainabilityError, setExplainabilityError] = useState<string | null>(null);
  const transferFilterQuery = initialTransferFilters?.query ?? "";
  const transferFilterStatus = initialTransferFilters?.status ?? "all";


  const loadTransfers = useCallback(async (cursor?: string, append = false) => {
    setTransfersLoading(true);
    const query = new URLSearchParams({ limit: "25" });
    if (cursor) query.set("cursor", cursor);
    if (transferFilterQuery) query.set("q", transferFilterQuery);
    if (transferFilterStatus !== "all") query.set("status", transferFilterStatus);
    const response = await readJSON<TransfersPayload>(`/api/transfers?${query}`);
    if (!response.ok) setTransferError(unavailableMessage(response.status, "transfer records"));
    else {
      const items = Array.isArray(response.data.transfers) ? response.data.transfers : [];
      setTransfers((current) => append ? [...current, ...items] : items);
      setTransferCursor(response.data.next_cursor || undefined);
      setTransferError(null);
    }
    setTransfersLoading(false);
  }, [transferFilterQuery, transferFilterStatus]);

  const loadTransferDetail = useCallback(async (id: string) => {
    setTransferDetailLoading(true);
    const response = await readJSON<TransferDetail>(`/api/transfers/${encodeURIComponent(id)}`);
    if (response.ok && response.data.transfer_id) { setTransferDetail(response.data); setTransferError(null); }
    else setTransferError(unavailableMessage(response.status, "transfer evidence"));
    setTransferDetailLoading(false);
  }, []);

  const loadOrientation = useCallback(async () => {
    setOrientationLoading(true);
    const response = await readJSON<LocalOrientation>("/api/local/orientation");
    if (response.ok && Array.isArray(response.data.steps)) { setOrientation(response.data); setOrientationError(null); }
    else setOrientationError(unavailableMessage(response.status, "local orientation evidence"));
    setOrientationLoading(false);
  }, []);

  const loadExplainability = useCallback(async (id: string) => {
    setExplainabilityLoading(true);
    const response = await readJSON<TransferExplainability>(`/api/transfers/${encodeURIComponent(id)}/explainability`);
    if (response.ok && Array.isArray(response.data.stages)) { setExplainability(response.data); setExplainabilityError(null); }
    else setExplainabilityError(response.status === 404 ? "The linked timeline was not found in this authorized tenant scope." : unavailableMessage(response.status, "stored evidence timeline"));
    setExplainabilityLoading(false);
  }, []);

  const loadRuns = useCallback(async (cursor?: string, append = false) => {
    setRunsLoading(true);
    const response = await readJSON<RunsPayload>(`/api/reconciliation/runs?limit=25${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ""}`);
    if (!response.ok) setReconciliationError("Authoritative reconciliation evidence is unavailable. LedgerSync will not infer a passing result.");
    else {
      const items = Array.isArray(response.data.runs) ? response.data.runs : [];
      setRuns((current) => append ? [...current, ...items] : items);
      setRunCursor(response.data.next_cursor || undefined);
      setReconciliationError(null);
    }
    setRunsLoading(false);
  }, []);

  const loadRunDetail = useCallback(async (id: string) => {
    setRunDetailLoading(true);
    const response = await readJSON<ReconciliationRun>(`/api/reconciliation/runs/${encodeURIComponent(id)}`);
    if (response.ok && response.data.run_id) { setRunDetail(response.data); setReconciliationError(null); }
    else setReconciliationError("The selected reconciliation evidence is unavailable or outside this tenant.");
    setRunDetailLoading(false);
  }, []);

  const observeRun = useCallback((run: ReconciliationRun) => {
    setRuns((current) => [run, ...current.filter((candidate) => candidate.run_id !== run.run_id)]);
    if (initialReconciliationRunId === run.run_id) setRunDetail(run);
    setReconciliationError(null);
  }, [initialReconciliationRunId]);

  const refresh = useCallback(async () => {
    const directory = initialSection === "accounts" && !initialAccountId && !initialAccountCreate;
    if (initialSection === "accounts") {
      await Promise.all([
        loadAccounts(directory ? accountWorkspace.filters : emptyAccountFilters, directory ? 25 : 100),
        initialAccountId ? loadAccountDetail(initialAccountId) : Promise.resolve(),
      ]);
      return;
    }
    if (initialSection === "overview") await Promise.all([loadAccounts(emptyAccountFilters, 100), loadTransfers(), loadRuns()]);
  }, [accountWorkspace.filters, initialAccountCreate, initialAccountId, initialSection, loadAccountDetail, loadAccounts, loadRuns, loadTransfers]);

  useEffect(() => {
    let active = true;
    (async () => {
      const response = await readJSON<ConsoleSession>("/api/session");
      if (active && response.ok && response.data.tenant_id) {
        setSession(response.data);
        setLoading(false);
        const directory = initialSection === "accounts" && !initialAccountId && !initialAccountCreate;
        const canExplain = ["explainability:read", "transfers:read", "events:read", "reconciliation:read"].every((scope) => response.data.scopes.includes(scope));
        if (initialSection === "overview") await Promise.all([
          loadAccounts(emptyAccountFilters, 100), loadTransfers(), loadRuns(),
          response.data.environment === "demo" && response.data.scopes.includes("local:read") ? loadOrientation() : Promise.resolve(),
        ]);
        if (initialSection === "accounts") await Promise.all([
          loadAccounts(directory ? initialAccountFilters : emptyAccountFilters, directory ? 25 : 100),
          initialAccountId ? loadAccountDetail(initialAccountId) : Promise.resolve(),
        ]);
        if (initialSection === "transfers") await Promise.all([
          loadAccounts(emptyAccountFilters, 100),
          initialTransferId ? loadTransferDetail(initialTransferId) : loadTransfers(),
          initialTransferId && canExplain ? loadExplainability(initialTransferId) : Promise.resolve(),
        ]);
        if (initialSection === "reconciliation") await (initialReconciliationRunId ? loadRunDetail(initialReconciliationRunId) : loadRuns());
      }
      if (active) setLoading(false);
    })();
    return () => { active = false; };
  }, [initialAccountCreate, initialAccountFilters, initialAccountId, initialReconciliationRunId, initialSection, initialTransferId, loadAccountDetail, loadAccounts, loadExplainability, loadOrientation, loadRunDetail, loadRuns, loadTransferDetail, loadTransfers]);

  useEffect(() => {
    const update = () => setOnline(navigator.onLine);
    update();
    window.addEventListener("online", update);
    window.addEventListener("offline", update);
    return () => { window.removeEventListener("online", update); window.removeEventListener("offline", update); };
  }, []);

  async function signOut() {
    if (!session) return;
    await fetch("/api/auth/sign-out", { method: "POST", headers: { "X-CSRF-Token": session.csrf_token } });
    router.refresh();
  }

  if (loading) {
    const title = initialSection === "accounts" ? "Accounts" : initialSection === "transfers" ? "Transfers" : initialSection === "reconciliation" ? "Reconciliation" : "Overview";
    return <ConsoleShell section={initialSection} tenantLabel="Verifying tenant" tenantMeta="Secure session" environmentLabel="Checking environment" operatorLabel="Verifying operator" operatorMeta="Authorization pending">
      <div className="session-loading" aria-busy="true">
        <PageHeader eyebrow={`${title} · LedgerSync operator workspace`} title="Verifying access" description="Verifying the authorized tenant scope before financial evidence is displayed." />
        <StatePanel title="Loading verified evidence" message="Balances, transfer history, and reconciliation evidence are loading from their authoritative sources." />
      </div>
      <ConsoleFooter />
    </ConsoleShell>;
  }
  if (!session) return <main className="boot-screen"><p className="eyebrow">Authentication required</p><h1>Operator workspace unavailable</h1><StatePanel kind="denied" title="No authorized session" message="Configure the approved OIDC provider, or explicitly enable the isolated local demo environment. No financial data is displayed." /></main>;

  return <ConsoleShell section={initialSection} tenantLabel={session.tenant_label ?? "Ledger tenant"} tenantMeta={session.tenant_id} environmentLabel={session.environment === "demo" ? "Isolated demo" : "Verified production"} operatorLabel={session.operator_label ?? session.subject_id} operatorMeta={session.environment === "demo" ? "Non-production data" : "Authorized operator"} preview={session.environment === "demo"} onSignOut={() => void signOut()}>
    {!online && <div className="offline-banner" role="status"><WarningCircle weight="fill" aria-hidden="true" /><span><strong>You are offline.</strong> Writes are disabled and no unverified result is shown.</span></div>}
    {initialSection === "overview" && <OverviewView accounts={accountWorkspace.accounts} transfers={transfers} reconciliation={runs[0] ?? null} loading={accountWorkspace.directoryLoading || transfersLoading || runsLoading} error={accountWorkspace.error ?? (!accountWorkspace.scopeComplete ? "The authorized account scope exceeds one bounded page. LedgerSync will not present a partial page as a complete balance aggregate." : null)} online={online} tenantId={session.tenant_id} orientation={orientation} orientationLoading={orientationLoading} orientationError={orientationError} canReadOrientation={session.environment === "demo" && session.scopes.includes("local:read")} localDemo={session.environment === "demo"} forceOrientation={initialShowOrientation} onRefresh={() => void refresh()} onRefreshOrientation={() => void loadOrientation()} />}
    {initialSection === "accounts" && (initialAccountCreate ? <AccountCreateFlow tenantId={session.tenant_id} tenantLabel={session.tenant_label ?? "Ledger tenant"} environmentLabel={session.environment === "demo" ? "Isolated demo" : "Verified production"} csrfToken={session.csrf_token} online={online} canWrite={session.scopes.includes("accounts:write")} canTransfer={session.scopes.includes("transfers:write")} fundingScopeComplete={accountWorkspace.scopeComplete} fundedSourceAvailable={accountWorkspace.accounts.some((candidate) => candidate.status === "active" && candidate.currency === "INR" && hasPositiveMinorUnits(candidate.available_minor))} returnTo={initialAccountReturnTo} onCreated={async () => { await loadAccounts(accountWorkspace.filters, 100); }} /> : <AccountsView accounts={accountWorkspace.accounts} selected={accountWorkspace.selected} detailRequested={Boolean(initialAccountId)} balance={accountWorkspace.balance} transactions={accountWorkspace.transactions} balanceLoading={accountWorkspace.balanceLoading} historyLoading={accountWorkspace.historyLoading} directoryLoading={accountWorkspace.directoryLoading} balanceError={accountWorkspace.balanceError} historyError={accountWorkspace.historyError} balanceVerifiedAt={accountWorkspace.balanceVerifiedAt} historyVerifiedAt={accountWorkspace.historyVerifiedAt} directoryVerifiedAt={accountWorkspace.directoryVerifiedAt} error={accountWorkspace.error} online={online} filters={accountWorkspace.filters} nextCursor={accountWorkspace.nextCursor} historyNextCursor={accountWorkspace.historyCursor} focusAccountId={initialAccountFocusId} tenantId={session.tenant_id} csrfToken={session.csrf_token} canWrite={session.scopes.includes("accounts:write")} canTransfer={session.scopes.includes("transfers:write")} canExport={session.scopes.includes("exports:read")&&session.scopes.includes("transactions:read")} fundingScopeComplete={accountWorkspace.scopeComplete} detailReturnTo={initialAccountId ? initialAccountReturnTo : undefined} onRefresh={() => void refresh()} onApplyFilters={accountWorkspace.applyFilters} onNext={accountWorkspace.loadNextPage} onHistoryNext={() => void accountWorkspace.loadMoreHistory()} onAccountChanged={async () => { if (initialAccountId) await loadAccountDetail(initialAccountId); await loadAccounts(accountWorkspace.filters, 100); }} onRefreshLifecycleEvidence={async () => initialAccountId ? loadAccountDetail(initialAccountId) : { account: null, balance: null }} />)}
    {initialSection === "transfers" && <TransfersView accounts={accountWorkspace.accounts} transfers={transfers} detail={transferDetail} detailRequested={Boolean(initialTransferId)} explainability={explainability} explainabilityLoading={explainabilityLoading} explainabilityError={explainabilityError} error={transferError} loading={initialTransferId ? transferDetailLoading : transfersLoading} nextCursor={transferCursor} online={online} canWrite={session.scopes.includes("transfers:write") && accountWorkspace.scopeComplete} canExport={session.scopes.includes("exports:read")&&session.scopes.includes("transfers:read")} canReadExplainability={["explainability:read", "transfers:read", "events:read", "reconciliation:read"].every((scope) => session.scopes.includes(scope))} writeUnavailableReason={!accountWorkspace.scopeComplete ? "Transfer creation is disabled because the authorized account picker is larger than one bounded page. Use the API until server-backed account selection is configured." : undefined} tenantId={session.tenant_id} csrfToken={session.csrf_token} preferredDestinationId={initialTransferDestinationId} returnTo={initialTransferReturnTo} initialFilters={initialTransferFilters} onRefresh={async () => { if (initialTransferId) await loadTransferDetail(initialTransferId); else await loadTransfers(); }} onRefreshExplainability={() => { if (initialTransferId) void loadExplainability(initialTransferId); }} onMore={() => { if (transferCursor) void loadTransfers(transferCursor, true); }} />}
    {initialSection === "reconciliation" && <ReconciliationView runs={runs} detail={runDetail} detailRequested={Boolean(initialReconciliationRunId)} error={reconciliationError} loading={initialReconciliationRunId ? runDetailLoading : runsLoading} nextCursor={runCursor} tenantId={session.tenant_id} csrfToken={session.csrf_token} online={online} canWrite={session.scopes.includes("reconciliation:write")} canExport={session.scopes.includes("exports:read")&&session.scopes.includes("reconciliation:read")} returnTo={initialReconciliationReturnTo} onObserved={observeRun} onRefresh={() => initialReconciliationRunId ? loadRunDetail(initialReconciliationRunId) : loadRuns()} onMore={() => { if (runCursor) void loadRuns(runCursor, true); }} />}
    <ConsoleFooter />
  </ConsoleShell>;
}
