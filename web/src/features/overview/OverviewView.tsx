"use client";

import { ArrowRight, Bank, ShieldCheck } from "@phosphor-icons/react";
import Link from "next/link";

import type { Account, ReconciliationRun, TransferSummary } from "@/features/accounts/types";
import { EvidenceFreshness, FocusedRetry, PageHeader, RecordLink, StatePanel } from "@/features/console/components";
import { utcDateTime } from "@/features/console/format";
import { LocalOrientationPanel } from "@/features/orientation/LocalOrientationPanel";
import { TransferList } from "@/features/transfers/TransferViews";
import { formatMinorUnits } from "@/lib/money";
import { approvedCurrencyGroups, isAuthoritativelyReconciled } from "@/lib/financial-ui";
import type { LocalOrientation, OperatorPreferenceStepID } from "@/lib/api/orientation";
import { uiDataState } from "@/lib/api/client";

type Props = Readonly<{
  accounts: Account[];
  transfers: TransferSummary[];
  reconciliation: ReconciliationRun | null;
  accountsLoading: boolean;
  transfersLoading: boolean;
  reconciliationLoading: boolean;
  accountsError: string | null;
  transfersError: string | null;
  reconciliationError: string | null;
  accountsVerifiedAt?: string;
  transfersVerifiedAt?: string;
  reconciliationVerifiedAt?: string;
  online: boolean;
  orientation: LocalOrientation | null;
  orientationLoading: boolean;
  orientationError: string | null;
  orientationPreferenceError: string | null;
  orientationPreferenceSaving: boolean;
  canReadOrientation: boolean;
  canWriteOrientation: boolean;
  localWorkspace: boolean;
  forceOrientation?: boolean;
  onRefreshAccounts: () => void;
  onRefreshTransfers: () => void;
  onRefreshReconciliation: () => void;
  onRefreshAll: () => void;
  onRefreshOrientation: () => void;
  onUpdateOrientationPreferences: (change: Readonly<{ dismissed: boolean; completedStepIDs: OperatorPreferenceStepID[] }>) => Promise<boolean>;
}>;

export function OverviewView({ accounts, transfers, reconciliation, accountsLoading, transfersLoading, reconciliationLoading, accountsError, transfersError, reconciliationError, accountsVerifiedAt, transfersVerifiedAt, reconciliationVerifiedAt, online, orientation, orientationLoading, orientationError, orientationPreferenceError, orientationPreferenceSaving, canReadOrientation, canWriteOrientation, localWorkspace, forceOrientation, onRefreshAccounts, onRefreshTransfers, onRefreshReconciliation, onRefreshAll, onRefreshOrientation, onUpdateOrientationPreferences }: Props) {
  const { currency, mixedCurrency, operatingMinor: operating, customerFundsMinor: customerFunds }=approvedCurrencyGroups(accounts);
  const asOf=accounts.map((account)=>account.as_of).filter(Boolean).sort().at(0);
  const busy = accountsLoading || transfersLoading || reconciliationLoading;
  const accountState = uiDataState({ loading: accountsLoading, hasData: accounts.length > 0, hasError: Boolean(accountsError), online, partial: Boolean(accountsError?.includes("partial")) });
  const transferState = uiDataState({ loading: transfersLoading, hasData: transfers.length > 0, hasError: Boolean(transfersError), online });
  const reconciliationState = uiDataState({ loading: reconciliationLoading, hasData: Boolean(reconciliation), hasError: Boolean(reconciliationError), online });
  const newWorkspace =
    !accountsLoading &&
    !transfersLoading &&
    !reconciliationLoading &&
    !accountsError &&
    !transfersError &&
    !reconciliationError &&
    accounts.length === 0 &&
    transfers.length === 0 &&
    !reconciliation;
  return <>
    <PageHeader eyebrow="Operations / Authoritative ledger" title="Overview" description={newWorkspace ? "Your workspace is ready. Begin with one real account and let every later balance come from posted ledger records." : "Current balances, transfers, reconciliation results, and exceptions for this workspace."}><button className="button secondary" type="button" onClick={onRefreshAll} disabled={!online || busy}>{busy ? "Refreshing dashboard…" : "Refresh dashboard"}</button></PageHeader>
    {localWorkspace&&forceOrientation&&<LocalOrientationPanel evidence={orientation} loading={orientationLoading} error={orientationError} preferenceError={orientationPreferenceError} preferenceSaving={orientationPreferenceSaving} online={online} canRead={canReadOrientation} canWrite={canWriteOrientation} onRefresh={onRefreshOrientation} onUpdatePreferences={onUpdateOrientationPreferences}/>}
    {newWorkspace&&<section className="new-workspace" aria-labelledby="new-workspace-title">
      <div className="new-workspace-mark" aria-hidden="true"><Bank weight="fill" /></div>
      <div className="new-workspace-copy">
        <p className="eyebrow">New workspace</p>
        <h2 id="new-workspace-title">Start with four simple steps</h2>
        <p>Create an account, add a funding record, review it, then make a transfer.</p>
        <div className="new-workspace-actions">
          <Link className="button primary" href="/accounts/new">Create your first account <ArrowRight aria-hidden="true" /></Link>
          <Link className="button secondary" href="/guide">Follow the guide</Link>
        </div>
      </div>
      <ol className="new-workspace-path" aria-label="First ledger steps">
        <li><Link href="/accounts/new"><span>01</span><strong>Create an account</strong><small>Set up where the money belongs.</small></Link></li>
        <li><Link href="/funding"><span>02</span><strong>Add a funding record</strong><small>Add the payment reference and supporting document.</small></Link></li>
        <li><Link href="/funding"><span>03</span><strong>Review the record</strong><small>Check it before it can change a balance.</small></Link></li>
        <li><Link href="/transfers"><span>04</span><strong>Make a transfer</strong><small>Move an exact amount between your accounts.</small></Link></li>
      </ol>
    </section>}
    <section className="overview-data-state overview-account-state" data-data-state={accountState} aria-label="Account details state">
      {accountsError && <StatePanel
        kind="error"
        title="Account details unavailable"
        message={accountsError}
        action={<FocusedRetry label="Retry accounts only" onRetry={onRefreshAccounts} disabled={!online} busy={accountsLoading} />}
      />}
      {accountsVerifiedAt&&accounts.length>0&&<EvidenceFreshness state={accountsError||!online?"historical":accountsLoading?"refreshing":"current"} verifiedAt={accountsVerifiedAt} label="Account totals" reason={accountsError??(!online?"Reconnect before relying on totals.":undefined)}/>}
      {accountsLoading&&accounts.length===0&&<StatePanel title="Loading account details" message="Authoritative account balances are loading. No zero balance or empty tenant is inferred."/>}
      {!newWorkspace&&!accountsError&&!accountsLoading&&accounts.length===0&&<StatePanel title="No authorized accounts" message="The verified authorized scope is empty. Create an account or ask an administrator to grant account access."/>}
      {accounts.length>0&&mixedCurrency&&<StatePanel kind="error" title="Mixed-currency pilot data blocked" message="Loaded accounts are preserved, but LedgerSync will not combine balances across currencies. Investigate tenant provisioning before relying on overview totals."/>}
      {accounts.length>0&&!mixedCurrency&&currency&&<div className="overview-metrics"><section className="balance-document"><div className="document-topline"><p>Operating-controlled balances</p><span>As of {utcDateTime(asOf)}</span></div><strong className="hero-amount">{formatMinorUnits(currency,operating)}</strong><p className="metric-definition">Excludes customer-funds category. Amounts with different ownership semantics are never combined silently.</p><Link className="text-link" href="/accounts">View account details →</Link></section><section className="balance-document secondary-balance"><div className="document-topline"><p>Customer funds</p><span>Separately classified</span></div><strong className="hero-amount">{formatMinorUnits(currency,customerFunds)}</strong><p className="metric-definition">Presented separately to avoid implying these funds are operating capital.</p></section></div>}
    </section>
    {!newWorkspace&&<section className="overview-data-state overview-reconciliation-state" data-data-state={reconciliationState} aria-label="Reconciliation results state">
      {reconciliationError && <StatePanel
        kind="error"
        title="Reconciliation results unavailable"
        message={reconciliationError}
        action={<FocusedRetry label="Retry reconciliation only" onRetry={onRefreshReconciliation} disabled={!online} busy={reconciliationLoading} />}
      />}
      {reconciliationVerifiedAt&&reconciliation&&<EvidenceFreshness state={reconciliationError||!online?"historical":reconciliationLoading?"refreshing":"current"} verifiedAt={reconciliationVerifiedAt} label="Reconciliation" reason={reconciliationError??(!online?"Reconnect before treating the run as current.":undefined)}/>}
      {reconciliationLoading&&!reconciliation&&<StatePanel title="Loading reconciliation results" message="No passing result or mismatch count is inferred while authoritative records load."/>}
      {reconciliation ? (
        <section className={`evidence-strip ${isAuthoritativelyReconciled(reconciliation) ? "" : "caution"}`}>
          <ShieldCheck weight="fill" aria-hidden="true" />
          <div>
            <strong>{isAuthoritativelyReconciled(reconciliation) ? "Latest reconciliation passed" : "Reconciliation requires attention"}</strong>
            <span>Run {reconciliation.run_id} · {reconciliation.mismatch_count} mismatches · {utcDateTime(reconciliation.completed_at)}</span>
          </div>
          <RecordLink href={`/reconciliation/${reconciliation.run_id}`} label="Open result" />
        </section>
      ) : !reconciliationError && !reconciliationLoading ? (
        <StatePanel kind="unknown" title="No reconciliation results" message="The verified history contains no authoritative run. No passing result is inferred." action={<Link className="text-link" href="/reconciliation">View reconciliation</Link>} />
      ) : null}
    </section>}
    {!newWorkspace&&<section className="overview-data-state overview-transfer-state" data-data-state={transferState} aria-label="Transfer history state">
      {transfersError && <StatePanel
        kind="error"
        title="Transfer history unavailable"
        message={transfersError}
        action={<FocusedRetry label="Retry transfers only" onRetry={onRefreshTransfers} disabled={!online} busy={transfersLoading} />}
      />}
      {transfersVerifiedAt&&transfers.length>0&&<EvidenceFreshness state={transfersError||!online?"historical":transfersLoading?"refreshing":"current"} verifiedAt={transfersVerifiedAt} label="Transfer history" reason={transfersError??(!online?"Reconnect before treating history as current.":undefined)}/>}
      {transfersLoading&&transfers.length===0&&<StatePanel title="Loading transfer history" message="No empty transfer history is inferred while immutable records load."/>}
      {!transfersError&&!transfersLoading&&transfers.length===0&&<StatePanel title="No transfer records" message="The verified authorized history is empty."/>}
      {transfers.length>0&&<TransferList variant="recent" transfers={transfers.slice(0,5)} returnTo="/"/>}
    </section>}
  </>;
}
