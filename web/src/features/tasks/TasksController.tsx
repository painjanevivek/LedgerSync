"use client";

import { useCallback, useEffect, useMemo, useSyncExternalStore } from "react";

import { useApprovalWorkspace } from "@/features/approvals/useApprovalWorkspace";
import { ConsoleRouteFrame } from "@/features/console/ConsoleRouteFrame";
import { useConsoleSession } from "@/features/console/ConsoleSessionBoundary";
import { canOpenApprovalInbox, deriveConsoleCapabilities } from "@/features/console/capabilities";
import { useReconciliationWorkspace } from "@/features/reconciliation/useReconciliationWorkspace";
import { TasksView } from "@/features/tasks/TasksView";
import { useTransferWorkspace } from "@/features/transfers/useTransferWorkspace";
import { emptyApprovalFilters } from "@/lib/api/approvals";
import { useSupplementalTasks } from "./useSupplementalTasks";
import { transferIntentStorageKey } from "@/features/transfers/transferIntent";
import type { TaskCoverage } from "./taskPresentation";

export function TasksController() {
  const { session, online } = useConsoleSession();
  const capabilities = useMemo(() => deriveConsoleCapabilities(session), [session]);
  const approvals = useApprovalWorkspace();
  const transfers = useTransferWorkspace();
  const reconciliation = useReconciliationWorkspace();
  const supplemental = useSupplementalTasks(session, online);
  const subscribe = useCallback((notify: () => void) => { window.addEventListener("ledgersync-transfer-intent", notify); return () => window.removeEventListener("ledgersync-transfer-intent", notify); }, []);
  const getSnapshot = useCallback(() => { try { return session ? sessionStorage.getItem(transferIntentStorageKey(session.tenant_id)) : null; } catch { return "storage-unavailable"; } }, [session]);
  const retainedTransfer = useSyncExternalStore(subscribe, getSnapshot, () => null);
  const loadApprovals = approvals.load;
  const loadTransfers = transfers.loadList;
  const loadReconciliation = reconciliation.loadList;

  useEffect(() => {
    if (!session || !online) return;
    const timer = window.setTimeout(() => {
      void Promise.all([
        canOpenApprovalInbox(capabilities) ? loadApprovals(emptyApprovalFilters) : Promise.resolve(),
        capabilities.transfersRead ? loadTransfers() : Promise.resolve(),
        capabilities.reconciliationRead ? loadReconciliation() : Promise.resolve(),
      ]);
    }, 0);
    return () => window.clearTimeout(timer);
  }, [capabilities, loadApprovals, loadReconciliation, loadTransfers, online, session]);

  const loading = approvals.loading || transfers.listLoading || reconciliation.listLoading || supplemental.coverage.some(source => source.state === "loading");
  const verified = (!canOpenApprovalInbox(capabilities) || Boolean(approvals.verifiedAt))
    && (!capabilities.transfersRead || Boolean(transfers.verifiedAt))
    && (!capabilities.reconciliationRead || Boolean(reconciliation.verifiedAt));
  const partial = Boolean(approvals.nextCursor || transfers.nextCursor || reconciliation.nextCursor);
  const coverage: TaskCoverage[] = [
    { id: "approvals", label: "Review decisions", href: "/approvals", state: !canOpenApprovalInbox(capabilities) ? "not-authorized" : approvals.error || approvals.denied ? "unavailable" : approvals.loading || !approvals.verifiedAt ? "loading" : approvals.nextCursor ? "partial" : "verified" },
    { id: "transfers", label: "Transfers", href: "/transfers", state: !capabilities.transfersRead ? "not-authorized" : transfers.error ? "unavailable" : transfers.listLoading || !transfers.verifiedAt ? "loading" : transfers.nextCursor ? "partial" : "verified" },
    { id: "balance", label: "Latest balance check", href: "/reconciliation", state: !capabilities.reconciliationRead ? "not-authorized" : reconciliation.error ? "unavailable" : reconciliation.listLoading || !reconciliation.verifiedAt ? "loading" : "verified" },
    ...supplemental.coverage,
  ];
  return <ConsoleRouteFrame section="tasks" loadingLabel="Tasks" pending={loading}>
    {session && <TasksView
      approvals={approvals.items}
      supplemental={supplemental.tasks}
      coverage={coverage}
      retainedTransfer={capabilities.transfersRead && retainedTransfer !== "storage-unavailable" && Boolean(retainedTransfer)}
      storageUnavailable={capabilities.transfersRead && retainedTransfer === "storage-unavailable"}
      transfers={transfers.transfers}
      reconciliation={reconciliation.runs[0] ?? null}
      loading={loading}
      verified={verified}
      partial={partial}
      online={online}
      errors={[approvals.error, transfers.error, reconciliation.error, approvals.denied ? "Review access is unavailable." : null].filter((value): value is string => Boolean(value))}
      onRefresh={() => void Promise.all([
        supplemental.refresh(),
        canOpenApprovalInbox(capabilities) ? loadApprovals(emptyApprovalFilters) : Promise.resolve(),
        capabilities.transfersRead ? loadTransfers() : Promise.resolve(),
        capabilities.reconciliationRead ? loadReconciliation() : Promise.resolve(),
      ])}
    />}
  </ConsoleRouteFrame>;
}
