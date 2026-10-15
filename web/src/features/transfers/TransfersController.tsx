"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

import { emptyAccountFilters, useAccountWorkspace } from "@/features/accounts/useAccountWorkspace";
import { ConsoleRouteFrame } from "@/features/console/ConsoleRouteFrame";
import { useConsoleSession } from "@/features/console/ConsoleSessionBoundary";
import { useReconciliationWorkspace } from "@/features/reconciliation/useReconciliationWorkspace";
import { PageHeader } from "@/ui/display/PageHeader";
import { StatePanel } from "@/ui/display/StatePanel";
import { TransfersView } from "@/features/transfers/TransferViews";
import { useTransferWorkspace } from "@/features/transfers/useTransferWorkspace";
import { emptyTransferFilters, transferURL, type TransferFilters } from "@/lib/page-query/transfers";

type Props = Readonly<{
  transferId?: string;
  preferredDestinationId?: string;
  returnTo?: string;
  filters?: TransferFilters;
  invalidQuery?: boolean;
}>;

export function TransfersController({ transferId, preferredDestinationId, returnTo, filters = emptyTransferFilters, invalidQuery = false }: Props) {
  const router = useRouter();
  const { session, online, hasScope } = useConsoleSession();
  const accounts = useAccountWorkspace(undefined, emptyAccountFilters);
  const workspace = useTransferWorkspace(filters);
  const reconciliation = useReconciliationWorkspace();
  const loadAccounts = accounts.load;
  const { loadDetail, loadExplainability, loadList } = workspace;
  const loadReconciliation = reconciliation.loadList;
  const [initialEvidenceSettled, setInitialEvidenceSettled] = useState(false);

  useEffect(() => {
    if (!session || !online) return;
    if (!hasScope("transfers:read") || invalidQuery) return;
    let active = true;
    const canExplain = ["explainability:read", "transfers:read", "events:read", "reconciliation:read"].every(hasScope);
    const timer = window.setTimeout(() => {
      void Promise.all([
        hasScope("accounts:read") ? loadAccounts(emptyAccountFilters, 100) : Promise.resolve(),
        loadList(),
        transferId ? loadDetail(transferId) : Promise.resolve(),
        transferId && canExplain ? loadExplainability(transferId) : Promise.resolve(),
        hasScope("reconciliation:read") ? loadReconciliation() : Promise.resolve(),
      ]).finally(() => {
        if (active) setInitialEvidenceSettled(true);
      });
    }, 0);
    return () => {
      active = false;
      window.clearTimeout(timer);
    };
  }, [hasScope, invalidQuery, loadAccounts, loadDetail, loadExplainability, loadList, loadReconciliation, online, session, transferId]);

  return (
    <ConsoleRouteFrame section="transfers" loadingLabel="Transfers" pending={hasScope("transfers:read") && !initialEvidenceSettled}>
      {session && !hasScope("transfers:read") ? (
        <><PageHeader eyebrow="Work / Transfers" title="Transfers" description="Move an exact amount between authorized accounts, then check the immutable result."/><StatePanel kind="denied" title="Transfer read authority required" message="Your server-issued session does not include transfers:read. No protected transfer or account-picker request was made."/></>
      ) : session && invalidQuery && !transferId ? (
        <><PageHeader eyebrow="Ledger / Transfers" title="Transfers" description="Move an exact amount between your accounts, then check the result."/><StatePanel kind="error" title="Invalid transfer investigation URL" message="The shared URL contains an unknown, repeated, empty, oversized, malformed, or reversed filter. No protected transfer or account-picker request was made." action={<button className="button secondary" type="button" onClick={() => router.replace("/transfers")}>Clear invalid filters</button>}/></>
      ) : session && (
        <TransfersView
          key={`${transferId ?? transferURL(filters)}:${preferredDestinationId ?? "register"}`}
          accounts={accounts.accounts}
          accountsLoading={accounts.directoryLoading}
          accountsError={accounts.error}
          accountsVerifiedAt={accounts.directoryVerifiedAt}
          accountsScopeComplete={accounts.scopeComplete}
          canReadAccounts={hasScope("accounts:read")}
          transfers={workspace.transfers}
          transfersVerifiedAt={workspace.verifiedAt}
          detail={workspace.detail}
          detailError={workspace.detailError}
          detailLoading={workspace.detailLoading}
          detailVerifiedAt={workspace.detailVerifiedAt}
          detailRequested={Boolean(transferId)}
          selectedTransferId={transferId}
          reconciliation={reconciliation.runs[0] ?? null}
          reconciliationLoading={reconciliation.listLoading}
          reconciliationError={reconciliation.error}
          reconciliationVerifiedAt={reconciliation.verifiedAt}
          canReadReconciliation={hasScope("reconciliation:read")}
          explainability={workspace.explainability}
          explainabilityLoading={workspace.explainabilityLoading}
          explainabilityError={workspace.explainabilityError}
          error={workspace.error}
          loading={workspace.listLoading}
          nextCursor={workspace.nextCursor}
          online={online}
          canWrite={hasScope("transfers:write") && accounts.scopeComplete}
          canCreateAccounts={hasScope("accounts:write")}
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
          onApplyFilters={(next) => router.push(transferURL(next))}
          onClearFilters={() => router.push("/transfers")}
          onRefreshAccounts={async () => {
            if (hasScope("accounts:read")) await accounts.load(emptyAccountFilters, 100);
          }}
          onRefreshHistory={workspace.loadList}
          onRefreshDetail={async () => {
            if (transferId && hasScope("transfers:read")) await workspace.loadDetail(transferId);
          }}
          onRefresh={async () => {
            await Promise.all([
              workspace.loadList(),
              transferId ? workspace.loadDetail(transferId) : Promise.resolve(),
              hasScope("accounts:read") ? accounts.load(emptyAccountFilters, 100) : Promise.resolve(),
              hasScope("reconciliation:read") ? reconciliation.loadList() : Promise.resolve(),
            ]);
          }}
          onRefreshExplainability={() => {
            if (transferId) void workspace.loadExplainability(transferId);
          }}
        />
      )}
    </ConsoleRouteFrame>
  );
}
