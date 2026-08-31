"use client";

import { useEffect, useState } from "react";

import { emptyAccountFilters, useAccountWorkspace } from "@/features/accounts/useAccountWorkspace";
import { ConsoleRouteFrame } from "@/features/console/ConsoleRouteFrame";
import { useConsoleSession } from "@/features/console/ConsoleSessionBoundary";
import { PageHeader, StatePanel } from "@/features/console/components";
import { TransfersView } from "@/features/transfers/TransferViews";
import { useTransferWorkspace } from "@/features/transfers/useTransferWorkspace";

type Props = Readonly<{
  transferId?: string;
  preferredDestinationId?: string;
  returnTo?: string;
  filters?: { query: string; status: string };
}>;

export function TransfersController({ transferId, preferredDestinationId, returnTo, filters }: Props) {
  const { session, online, hasScope } = useConsoleSession();
  const accounts = useAccountWorkspace(undefined, emptyAccountFilters);
  const workspace = useTransferWorkspace(filters);
  const loadAccounts = accounts.load;
  const { loadDetail, loadExplainability, loadList } = workspace;
  const [initialEvidenceSettled, setInitialEvidenceSettled] = useState(false);

  useEffect(() => {
    if (!session || !online) return;
    if (!hasScope("transfers:read")) return;
    let active = true;
    const canExplain = ["explainability:read", "transfers:read", "events:read", "reconciliation:read"].every(hasScope);
    const timer = window.setTimeout(() => {
      void Promise.all([
        hasScope("accounts:read") ? loadAccounts(emptyAccountFilters, 100) : Promise.resolve(),
        transferId ? loadDetail(transferId) : loadList(),
        transferId && canExplain ? loadExplainability(transferId) : Promise.resolve(),
      ]).finally(() => {
        if (active) setInitialEvidenceSettled(true);
      });
    }, 0);
    return () => {
      active = false;
      window.clearTimeout(timer);
    };
  }, [hasScope, loadAccounts, loadDetail, loadExplainability, loadList, online, session, transferId]);

  return (
    <ConsoleRouteFrame section="transfers" loadingLabel="Transfers" pending={hasScope("transfers:read") && !initialEvidenceSettled}>
      {session && !hasScope("transfers:read") ? (
        <><PageHeader eyebrow="Work / Transfers" title="Transfers" description="Move an exact amount between authorized accounts, then check the immutable result."/><StatePanel kind="denied" title="Transfer read authority required" message="Your server-issued session does not include transfers:read. No protected transfer or account-picker request was made."/></>
      ) : session && (
        <TransfersView
          accounts={accounts.accounts}
          accountsLoading={accounts.directoryLoading}
          accountsError={accounts.error}
          accountsVerifiedAt={accounts.directoryVerifiedAt}
          transfers={workspace.transfers}
          transfersVerifiedAt={workspace.verifiedAt}
          detail={workspace.detail}
          detailRequested={Boolean(transferId)}
          explainability={workspace.explainability}
          explainabilityLoading={workspace.explainabilityLoading}
          explainabilityError={workspace.explainabilityError}
          error={workspace.error}
          loading={transferId ? workspace.detailLoading : workspace.listLoading}
          nextCursor={workspace.nextCursor}
          online={online}
          canWrite={hasScope("transfers:write") && accounts.scopeComplete}
          canExport={hasScope("exports:read") && hasScope("transfers:read")}
          canReadExplainability={["explainability:read", "transfers:read", "events:read", "reconciliation:read"].every(hasScope)}
          canReadCorrections={hasScope("corrections:read")}
          canWriteCorrections={hasScope("corrections:write")}
          writeUnavailableReason={!accounts.scopeComplete ? "Transfer creation is disabled because the authorized account picker is larger than one bounded page. Use the API until server-backed account selection is configured." : undefined}
          tenantId={session.tenant_id}
          csrfToken={session.csrf_token}
          preferredDestinationId={preferredDestinationId}
          returnTo={returnTo}
          initialFilters={filters}
          onRefreshAccounts={async () => {
            await accounts.load(emptyAccountFilters, 100);
          }}
          onRefresh={async () => {
            if (transferId) await workspace.loadDetail(transferId);
            else await workspace.loadList();
          }}
          onRefreshExplainability={() => {
            if (transferId) void workspace.loadExplainability(transferId);
          }}
          onMore={() => {
            if (workspace.nextCursor) void workspace.loadList(workspace.nextCursor, true);
          }}
        />
      )}
    </ConsoleRouteFrame>
  );
}
