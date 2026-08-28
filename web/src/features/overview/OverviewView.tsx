"use client";

import { ShieldCheck } from "@phosphor-icons/react";
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
  localDemo: boolean;
  forceOrientation?: boolean;
  onRefreshAccounts: () => void;
  onRefreshTransfers: () => void;
  onRefreshReconciliation: () => void;
  onRefreshAll: () => void;
  onRefreshOrientation: () => void;
  onUpdateOrientationPreferences: (change: Readonly<{ dismissed: boolean; completedStepIDs: OperatorPreferenceStepID[] }>) => Promise<boolean>;
}>;

export function OverviewView({ accounts, transfers, reconciliation, accountsLoading, transfersLoading, reconciliationLoading, accountsError, transfersError, reconciliationError, accountsVerifiedAt, transfersVerifiedAt, reconciliationVerifiedAt, online, orientation, orientationLoading, orientationError, orientationPreferenceError, orientationPreferenceSaving, canReadOrientation, canWriteOrientation, localDemo, forceOrientation, onRefreshAccounts, onRefreshTransfers, onRefreshReconciliation, onRefreshAll, onRefreshOrientation, onUpdateOrientationPreferences }: Props) {
  const { currency, mixedCurrency, operatingMinor: operating, customerFundsMinor: customerFunds }=approvedCurrencyGroups(accounts);
  const asOf=accounts.map((account)=>account.as_of).filter(Boolean).sort().at(0);
  const busy = accountsLoading || transfersLoading || reconciliationLoading;
  const accountState = uiDataState({ loading: accountsLoading, hasData: accounts.length > 0, hasError: Boolean(accountsError), online, partial: Boolean(accountsError?.includes("partial")) });
  const transferState = uiDataState({ loading: transfersLoading, hasData: transfers.length > 0, hasError: Boolean(transfersError), online });
  const reconciliationState = uiDataState({ loading: reconciliationLoading, hasData: Boolean(reconciliation), hasError: Boolean(reconciliationError), online });
  return <>
    <PageHeader eyebrow="Operations / Authoritative ledger" title="Overview" description="Fresh financial evidence and exceptions for the current tenant."><button className="button secondary" type="button" onClick={onRefreshAll} disabled={!online || busy}>{busy ? "Refreshing evidence…" : "Refresh evidence"}</button></PageHeader>
    {localDemo&&<LocalOrientationPanel evidence={orientation} loading={orientationLoading} error={orientationError} preferenceError={orientationPreferenceError} preferenceSaving={orientationPreferenceSaving} online={online} canRead={canReadOrientation} canWrite={canWriteOrientation} forceOpen={forceOrientation} onRefresh={onRefreshOrientation} onUpdatePreferences={onUpdateOrientationPreferences}/>}
    <section className="overview-data-state overview-account-state" data-data-state={accountState} aria-label="Account evidence state">
      {accountsError && <StatePanel
        kind="error"
        title="Account evidence unavailable"
        message={accountsError}
        action={<FocusedRetry label="Retry accounts only" onRetry={onRefreshAccounts} disabled={!online} busy={accountsLoading} />}
      />}
      {accountsVerifiedAt&&accounts.length>0&&<EvidenceFreshness state={accountsError||!online?"historical":accountsLoading?"refreshing":"current"} verifiedAt={accountsVerifiedAt} label="Account totals" reason={accountsError??(!online?"Reconnect before relying on totals.":undefined)}/>}
      {accountsLoading&&accounts.length===0&&<StatePanel title="Loading account evidence" message="Authoritative account balances are loading. No zero balance or empty tenant is inferred."/>}
      {!accountsError&&!accountsLoading&&accounts.length===0&&<StatePanel title="No authorized accounts" message="The verified authorized scope is empty. An administrator must grant account access before balances can be inspected."/>}
      {accounts.length>0&&mixedCurrency&&<StatePanel kind="error" title="Mixed-currency pilot data blocked" message="Loaded accounts are preserved, but LedgerSync will not combine balances across currencies. Investigate tenant provisioning before relying on overview totals."/>}
      {accounts.length>0&&!mixedCurrency&&currency&&<div className="overview-metrics"><section className="balance-document"><div className="document-topline"><p>Operating-controlled balances</p><span>As of {utcDateTime(asOf)}</span></div><strong className="hero-amount">{formatMinorUnits(currency,operating)}</strong><p className="metric-definition">Excludes customer-funds category. Amounts with different ownership semantics are never combined silently.</p><Link className="text-link" href="/accounts">View account evidence →</Link></section><section className="balance-document secondary-balance"><div className="document-topline"><p>Customer funds</p><span>Separately classified</span></div><strong className="hero-amount">{formatMinorUnits(currency,customerFunds)}</strong><p className="metric-definition">Presented separately to avoid implying these funds are operating capital.</p></section></div>}
    </section>
    <section className="overview-data-state overview-reconciliation-state" data-data-state={reconciliationState} aria-label="Reconciliation evidence state">
      {reconciliationError && <StatePanel
        kind="error"
        title="Reconciliation evidence unavailable"
        message={reconciliationError}
        action={<FocusedRetry label="Retry reconciliation only" onRetry={onRefreshReconciliation} disabled={!online} busy={reconciliationLoading} />}
      />}
      {reconciliationVerifiedAt&&reconciliation&&<EvidenceFreshness state={reconciliationError||!online?"historical":reconciliationLoading?"refreshing":"current"} verifiedAt={reconciliationVerifiedAt} label="Reconciliation" reason={reconciliationError??(!online?"Reconnect before treating the run as current.":undefined)}/>}
      {reconciliationLoading&&!reconciliation&&<StatePanel title="Loading reconciliation evidence" message="No passing result or mismatch count is inferred while authoritative evidence loads."/>}
      {reconciliation ? (
        <section className={`evidence-strip ${isAuthoritativelyReconciled(reconciliation) ? "" : "caution"}`}>
          <ShieldCheck weight="fill" aria-hidden="true" />
          <div>
            <strong>{isAuthoritativelyReconciled(reconciliation) ? "Latest reconciliation passed" : "Reconciliation requires attention"}</strong>
            <span>Run {reconciliation.run_id} · {reconciliation.mismatch_count} mismatches · {utcDateTime(reconciliation.completed_at)}</span>
          </div>
          <RecordLink href={`/reconciliation/${reconciliation.run_id}`} label="Open evidence" />
        </section>
      ) : !reconciliationError && !reconciliationLoading ? (
        <StatePanel kind="unknown" title="No reconciliation evidence" message="The verified history contains no authoritative run. No passing result is inferred." action={<Link className="text-link" href="/reconciliation">Inspect evidence</Link>} />
      ) : null}
    </section>
    <section className="overview-data-state overview-transfer-state" data-data-state={transferState} aria-label="Transfer evidence state">
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
    </section>
  </>;
}
