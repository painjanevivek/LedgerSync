"use client";

import { WarningCircle } from "@phosphor-icons/react";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";

import { AccountsView, type AccountFilters } from "@/features/accounts/AccountViews";
import type { ConsoleSession, ReconciliationRun, TransferDetail, TransferSummary } from "@/features/accounts/types";
import { emptyAccountFilters, useAccountWorkspace } from "@/features/accounts/useAccountWorkspace";
import { ConsoleFooter, ConsoleShell, type ConsoleSection } from "@/features/console/ConsoleShell";
import { StatePanel } from "@/features/console/components";
import { OverviewView } from "@/features/overview/OverviewView";
import { ReconciliationView } from "@/features/reconciliation/ReconciliationViews";
import { TransfersView } from "@/features/transfers/TransferViews";
import { readJSON, unavailableMessage } from "@/lib/api/client";

type Props = Readonly<{
  initialSection?: ConsoleSection;
  initialAccountId?: string;
  initialTransferId?: string;
  initialReconciliationRunId?: string;
  initialAccountFilters?: AccountFilters;
  initialAccountFocusId?: string;
}>;
type TransfersPayload = { transfers?: TransferSummary[]; next_cursor?: string };
type RunsPayload = { runs?: ReconciliationRun[]; next_cursor?: string };

export function OperatorConsole({ initialSection = "overview", initialAccountId, initialTransferId, initialReconciliationRunId, initialAccountFilters = emptyAccountFilters, initialAccountFocusId }: Props) {
  const router = useRouter();
  const [session, setSession] = useState<ConsoleSession | null>(null);
  const accountWorkspace = useAccountWorkspace(initialAccountId, initialAccountFilters);
  const { load: loadAccounts, loadDetail: loadAccountDetail } = accountWorkspace;
  const [transfers, setTransfers] = useState<TransferSummary[]>([]);
  const [transferDetail, setTransferDetail] = useState<TransferDetail | null>(null);
  const [runs, setRuns] = useState<ReconciliationRun[]>([]);
  const [runDetail, setRunDetail] = useState<ReconciliationRun | null>(null);
  const [loading, setLoading] = useState(true);
  const [detailLoading, setDetailLoading] = useState(false);
  const [transferError, setTransferError] = useState<string | null>(null);
  const [reconciliationError, setReconciliationError] = useState<string | null>(null);
  const [transferCursor, setTransferCursor] = useState<string>();
  const [runCursor, setRunCursor] = useState<string>();
  const [online, setOnline] = useState(true);


  const loadTransfers = useCallback(async (cursor?: string, append = false) => {
    setDetailLoading(true);
    const response = await readJSON<TransfersPayload>(`/api/transfers?limit=25${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ""}`);
    if (!response.ok) setTransferError(unavailableMessage(response.status, "transfer records"));
    else {
      const items = Array.isArray(response.data.transfers) ? response.data.transfers : [];
      setTransfers((current) => append ? [...current, ...items] : items);
      setTransferCursor(response.data.next_cursor || undefined);
      setTransferError(null);
    }
    setDetailLoading(false);
  }, []);

  const loadTransferDetail = useCallback(async (id: string) => {
    setDetailLoading(true);
    const response = await readJSON<TransferDetail>(`/api/transfers/${encodeURIComponent(id)}`);
    if (response.ok && response.data.transfer_id) { setTransferDetail(response.data); setTransferError(null); }
    else setTransferError(unavailableMessage(response.status, "transfer evidence"));
    setDetailLoading(false);
  }, []);

  const loadRuns = useCallback(async (cursor?: string, append = false) => {
    setDetailLoading(true);
    const response = await readJSON<RunsPayload>(`/api/reconciliation/runs?limit=25${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ""}`);
    if (!response.ok) setReconciliationError("Authoritative reconciliation evidence is unavailable. LedgerSync will not infer a passing result.");
    else {
      const items = Array.isArray(response.data.runs) ? response.data.runs : [];
      setRuns((current) => append ? [...current, ...items] : items);
      setRunCursor(response.data.next_cursor || undefined);
      setReconciliationError(null);
    }
    setDetailLoading(false);
  }, []);

  const loadRunDetail = useCallback(async (id: string) => {
    setDetailLoading(true);
    const response = await readJSON<ReconciliationRun>(`/api/reconciliation/runs/${encodeURIComponent(id)}`);
    if (response.ok && response.data.run_id) { setRunDetail(response.data); setReconciliationError(null); }
    else setReconciliationError("The selected reconciliation evidence is unavailable or outside this tenant.");
    setDetailLoading(false);
  }, []);

  const refresh = useCallback(async () => {
    const directory = initialSection === "accounts";
    const owned = await loadAccounts(directory ? accountWorkspace.filters : emptyAccountFilters, directory ? 25 : 100);
    await Promise.all([
      initialAccountId ? loadAccountDetail(initialAccountId) : owned[0] && !directory ? loadAccountDetail(owned[0].account_id) : Promise.resolve(),
      loadTransfers(),
      loadRuns(),
    ]);
  }, [accountWorkspace.filters, initialAccountId, initialSection, loadAccountDetail, loadAccounts, loadRuns, loadTransfers]);

  useEffect(() => {
    let active = true;
    (async () => {
      const response = await readJSON<ConsoleSession>("/api/session");
      if (active && response.ok && response.data.tenant_id) {
        setSession(response.data);
        const directory = initialSection === "accounts";
        const owned = await loadAccounts(directory ? initialAccountFilters : emptyAccountFilters, directory ? 25 : 100);
        await Promise.all([
          initialAccountId ? loadAccountDetail(initialAccountId) : owned[0] && !directory ? loadAccountDetail(owned[0].account_id) : Promise.resolve(),
          initialTransferId ? loadTransferDetail(initialTransferId) : loadTransfers(),
          initialReconciliationRunId ? loadRunDetail(initialReconciliationRunId) : loadRuns(),
        ]);
      }
      if (active) setLoading(false);
    })();
    return () => { active = false; };
  }, [initialAccountFilters, initialAccountId, initialReconciliationRunId, initialSection, initialTransferId, loadAccountDetail, loadAccounts, loadRunDetail, loadRuns, loadTransferDetail, loadTransfers]);

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

  if (loading) return <main className="boot-screen" aria-busy="true"><p className="eyebrow">LedgerSync operator workspace</p><h1>Verifying the ledger session…</h1><div className="loading-rule" aria-hidden="true" /></main>;
  if (!session) return <main className="boot-screen"><p className="eyebrow">Authentication required</p><h1>Operator workspace unavailable</h1><StatePanel kind="denied" title="No authorized session" message="Configure the approved OIDC provider, or explicitly enable the isolated local demo environment. No financial data is displayed." /></main>;

  return <ConsoleShell section={initialSection} tenantLabel={session.tenant_label ?? "Ledger tenant"} tenantMeta={session.tenant_id} environmentLabel={session.environment === "demo" ? "Isolated demo" : "Verified production"} operatorLabel={session.operator_label ?? session.subject_id} operatorMeta={session.environment === "demo" ? "Non-production data" : "Authorized operator"} preview={session.environment === "demo"} onSignOut={() => void signOut()}>
    {!online && <div className="offline-banner" role="status"><WarningCircle weight="fill" aria-hidden="true" /><span><strong>You are offline.</strong> Writes are disabled and no unverified result is shown.</span></div>}
    {initialSection === "overview" && <OverviewView accounts={accountWorkspace.accounts} transfers={transfers} reconciliation={runs[0] ?? null} error={accountWorkspace.error ?? (!accountWorkspace.scopeComplete ? "The authorized account scope exceeds one bounded page. LedgerSync will not present a partial page as a complete balance aggregate." : null)} online={online} onRefresh={() => void refresh()} />}
    {initialSection === "accounts" && <AccountsView accounts={accountWorkspace.accounts} selected={accountWorkspace.selected} balance={accountWorkspace.balance} transactions={accountWorkspace.transactions} balanceLoading={accountWorkspace.balanceLoading} historyLoading={accountWorkspace.historyLoading} directoryLoading={accountWorkspace.directoryLoading} balanceError={accountWorkspace.balanceError} historyError={accountWorkspace.historyError} error={accountWorkspace.error} online={online} filters={accountWorkspace.filters} nextCursor={accountWorkspace.nextCursor} historyNextCursor={accountWorkspace.historyCursor} focusAccountId={initialAccountFocusId} onRefresh={() => void refresh()} onApplyFilters={accountWorkspace.applyFilters} onNext={accountWorkspace.loadNextPage} onHistoryNext={() => void accountWorkspace.loadMoreHistory()} />}
    {initialSection === "transfers" && <TransfersView accounts={accountWorkspace.accounts} transfers={transfers} detail={transferDetail} error={transferError} loading={detailLoading} nextCursor={transferCursor} online={online} canWrite={session.scopes.includes("transfers:write") && accountWorkspace.scopeComplete} writeUnavailableReason={!accountWorkspace.scopeComplete ? "Transfer creation is disabled because the authorized account picker is larger than one bounded page. Use the API until server-backed account selection is configured." : undefined} tenantId={session.tenant_id} csrfToken={session.csrf_token} onRefresh={async () => { await refresh(); }} onMore={() => { if (transferCursor) void loadTransfers(transferCursor, true); }} />}
    {initialSection === "reconciliation" && <ReconciliationView runs={runs} detail={runDetail} error={reconciliationError} loading={detailLoading} nextCursor={runCursor} onRefresh={() => void loadRuns()} onMore={() => { if (runCursor) void loadRuns(runCursor, true); }} />}
    <ConsoleFooter />
  </ConsoleShell>;
}
