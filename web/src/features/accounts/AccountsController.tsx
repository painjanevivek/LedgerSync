"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";

import { AccountCreateFlow } from "@/features/accounts/AccountCreateFlow";
import { AccountsView, type AccountFilters } from "@/features/accounts/AccountViews";
import { hasPositiveMinorUnits } from "@/features/accounts/accountCommandIntent";
import { emptyAccountFilters, useAccountWorkspace } from "@/features/accounts/useAccountWorkspace";
import { ConsoleRouteFrame } from "@/features/console/ConsoleRouteFrame";
import { useConsoleSession } from "@/features/console/ConsoleSessionBoundary";
import { PageHeader, StatePanel } from "@/features/console/components";

type Props = Readonly<{
  accountId?: string;
  filters?: AccountFilters;
  focusAccountId?: string;
  create?: boolean;
  returnTo?: string;
  invalidQuery?: boolean;
}>;

export function AccountsController({
  accountId,
  filters = emptyAccountFilters,
  focusAccountId,
  create = false,
  returnTo = "/accounts",
  invalidQuery = false,
}: Props) {
  const { session, online, hasScope } = useConsoleSession();
  const workspace = useAccountWorkspace(accountId, filters);
  const { load, loadDetail, filters: activeFilters } = workspace;
  const [initialEvidenceSettled, setInitialEvidenceSettled] = useState(false);
  const directory = !accountId && !create;

  const refresh = useCallback(async () => {
    await Promise.all([
      load(directory ? activeFilters : emptyAccountFilters, directory ? 25 : 100),
      accountId ? loadDetail(accountId) : Promise.resolve(),
    ]);
  }, [accountId, activeFilters, directory, load, loadDetail]);

  useEffect(() => {
    if (!session || !online || invalidQuery) return;
    if (!hasScope("accounts:read")) return;
    let active = true;
    const timer = window.setTimeout(() => {
      void refresh().finally(() => {
        if (active) setInitialEvidenceSettled(true);
      });
    }, 0);
    return () => {
      active = false;
      window.clearTimeout(timer);
    };
  }, [hasScope, invalidQuery, online, refresh, session]);

  return (
    <ConsoleRouteFrame section="accounts" loadingLabel="Accounts" pending={hasScope("accounts:read") && !invalidQuery && !initialEvidenceSettled}>
      {session && !create && !hasScope("accounts:read") ? (
        <><PageHeader eyebrow="Work / Accounts" title="Accounts" description="Find and manage the accounts you are allowed to use."/><StatePanel kind="denied" title="Account read authority required" message="Your server-issued session does not include accounts:read. No protected account request was made."/></>
      ) : session && !create && invalidQuery ? (
        <><PageHeader eyebrow="Ledger / Accounts" title="Accounts" description="Find and manage the accounts you are allowed to use."/><StatePanel kind="error" title="Invalid account investigation URL" message="The shared URL contains an unknown, repeated, empty, oversized, or malformed filter. No protected account request was made." action={<Link className="button secondary" href="/accounts">Clear invalid filters</Link>}/></>
      ) : session && (create ? (
        <AccountCreateFlow
          tenantId={session.tenant_id}
          tenantLabel={session.tenant_label ?? "Ledger tenant"}
          environmentLabel={session.environment === "local" ? "Local workspace" : "Verified production"}
          csrfToken={session.csrf_token}
          online={online}
          canWrite={hasScope("accounts:write")}
          canTransfer={hasScope("transfers:write")}
          fundingScopeComplete={workspace.scopeComplete}
          fundedSourceAvailable={workspace.accounts.some((candidate) => candidate.status === "active" && candidate.currency === "INR" && hasPositiveMinorUnits(candidate.available_minor))}
          returnTo={returnTo}
          onCreated={async () => {
            await workspace.load(workspace.filters, 100);
          }}
        />
      ) : (
        <AccountsView
          accounts={workspace.accounts}
          selected={workspace.selected}
          detailRequested={Boolean(accountId)}
          balance={workspace.balance}
          transactions={workspace.transactions}
          balanceLoading={workspace.balanceLoading}
          historyLoading={workspace.historyLoading}
          directoryLoading={workspace.directoryLoading}
          balanceError={workspace.balanceError}
          historyError={workspace.historyError}
          balanceVerifiedAt={workspace.balanceVerifiedAt}
          historyVerifiedAt={workspace.historyVerifiedAt}
          directoryVerifiedAt={workspace.directoryVerifiedAt}
          error={workspace.error}
          online={online}
          filters={workspace.filters}
          nextCursor={workspace.nextCursor}
          historyNextCursor={workspace.historyCursor}
          focusAccountId={focusAccountId}
          tenantId={session.tenant_id}
          csrfToken={session.csrf_token}
          canWrite={hasScope("accounts:write")}
          canTransfer={hasScope("transfers:write")}
          canExport={hasScope("exports:read") && hasScope("transactions:read")}
          fundingScopeComplete={workspace.scopeComplete}
          detailReturnTo={accountId ? returnTo : undefined}
          onRefresh={() => void refresh()}
          onHistoryNext={() => void workspace.loadMoreHistory()}
          onRefreshBalance={() => {
            if (accountId) void workspace.loadBalance(accountId);
          }}
          onRefreshHistory={() => {
            if (accountId) void workspace.loadHistory(accountId);
          }}
          onAccountChanged={async () => {
            if (accountId) await workspace.loadDetail(accountId);
            await workspace.load(workspace.filters, 100);
          }}
          onRefreshLifecycleEvidence={async () => accountId ? workspace.loadDetail(accountId) : { account: null, balance: null }}
        />
      ))}
    </ConsoleRouteFrame>
  );
}
