"use client";

import { useCallback, useEffect, useState } from "react";

import { emptyAccountFilters, useAccountWorkspace } from "@/features/accounts/useAccountWorkspace";
import { ConsoleRouteFrame } from "@/features/console/ConsoleRouteFrame";
import { useConsoleSession } from "@/features/console/ConsoleSessionBoundary";
import { deriveConsoleCapabilities } from "@/features/console/capabilities";
import { useOrientationWorkspace } from "@/features/orientation/useOrientationWorkspace";
import { OverviewView } from "@/features/overview/OverviewView";
import { useReconciliationWorkspace } from "@/features/reconciliation/useReconciliationWorkspace";
import { useTransferWorkspace } from "@/features/transfers/useTransferWorkspace";

export function OverviewController({ showOrientation = false }: Readonly<{ showOrientation?: boolean }>) {
  const { session, online } = useConsoleSession();
  const accounts = useAccountWorkspace(undefined, emptyAccountFilters);
  const transfers = useTransferWorkspace();
  const reconciliation = useReconciliationWorkspace();
  const orientation = useOrientationWorkspace(session);
  const capabilities = deriveConsoleCapabilities(session);
  const loadAccounts = accounts.load;
  const loadTransfers = transfers.loadList;
  const loadReconciliation = reconciliation.loadList;
  const loadOrientation = orientation.load;
  const [initialEvidenceSettled, setInitialEvidenceSettled] = useState(false);

  const refreshAll = useCallback(async () => {
    await Promise.all([
      capabilities.accountsRead ? loadAccounts(emptyAccountFilters, 100) : Promise.resolve(),
      capabilities.transfersRead ? loadTransfers() : Promise.resolve(),
      capabilities.reconciliationRead ? loadReconciliation() : Promise.resolve(),
    ]);
  }, [capabilities.accountsRead, capabilities.reconciliationRead, capabilities.transfersRead, loadAccounts, loadReconciliation, loadTransfers]);

  useEffect(() => {
    if (!session || !online) return;
    let active = true;
    const timer = window.setTimeout(() => {
      void Promise.all([
        refreshAll(),
        capabilities.localDiagnosticsRead ? loadOrientation() : Promise.resolve(),
      ]).finally(() => {
        if (active) setInitialEvidenceSettled(true);
      });
    }, 0);
    return () => {
      active = false;
      window.clearTimeout(timer);
    };
  }, [capabilities.localDiagnosticsRead, loadOrientation, online, refreshAll, session]);

  return (
    <ConsoleRouteFrame section="overview" loadingLabel="Overview" pending={!initialEvidenceSettled}>
      {session && (
        <OverviewView
          accounts={accounts.accounts}
          transfers={transfers.transfers}
          reconciliation={reconciliation.runs[0] ?? null}
          accountsLoading={accounts.directoryLoading}
          transfersLoading={transfers.listLoading}
          reconciliationLoading={reconciliation.listLoading}
          accountsError={accounts.error ?? (!accounts.scopeComplete ? "The authorized account scope exceeds one bounded page. Previously verified accounts remain visible, but totals are partial and cannot be treated as tenant-wide." : null)}
          transfersError={transfers.error}
          reconciliationError={reconciliation.error}
          accountsVerifiedAt={accounts.directoryVerifiedAt}
          transfersVerifiedAt={transfers.verifiedAt}
          reconciliationVerifiedAt={reconciliation.verifiedAt}
          online={online}
          capabilities={capabilities}
          orientation={orientation.evidence}
          orientationLoading={orientation.loading}
          orientationError={orientation.error}
          orientationPreferenceError={orientation.preferenceError}
          orientationPreferenceSaving={orientation.preferenceSaving}
          canReadOrientation={capabilities.localDiagnosticsRead}
          canWriteOrientation={capabilities.localOrientationWrite}
          localWorkspace={session.environment === "local"}
          forceOrientation={showOrientation}
          onRefreshAccounts={() => void accounts.load(emptyAccountFilters, 100)}
          onRefreshTransfers={() => void transfers.loadList()}
          onRefreshReconciliation={() => void reconciliation.loadList()}
          onRefreshAll={() => void refreshAll()}
          onRefreshOrientation={() => void orientation.load()}
          onUpdateOrientationPreferences={orientation.updatePreferences}
        />
      )}
    </ConsoleRouteFrame>
  );
}
