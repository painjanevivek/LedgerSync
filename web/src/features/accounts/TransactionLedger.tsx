"use client";

import { CheckCircle, WarningCircle } from "@phosphor-icons/react";
import type { ReactNode } from "react";

import type { Account, Transaction } from "@/features/accounts/types";
import { EvidenceFreshness, Pagination, RecordLink, StatePanel, StatusBadge } from "@/features/console/components";
import { accountLabel, utcDateTime } from "@/features/console/format";
import { formatMinorUnits } from "@/lib/money";

type Props = Readonly<{
  transactions: Transaction[];
  account?: Account | null;
  loading: boolean;
  error: string | null;
  nextCursor?: string;
  onNext: () => void;
  exportAction?: ReactNode;
  verifiedAt?: string;
}>;

export function TransactionLedger({ transactions, account, loading, error, nextCursor, onNext, exportAction, verifiedAt }: Props) {
  return <section className="ledger-section" aria-labelledby="transactions-heading" aria-busy={loading}>
    <div className="section-heading"><div><p className="eyebrow">Immutable activity</p><h2 id="transactions-heading">Ledger entries</h2><p>{account ? `${accountLabel(account)} · ${account.currency}` : "Select an account to inspect its postings."}</p></div>{exportAction}</div>
    {loading && transactions.length === 0 && <StatePanel title="Loading ledger history" message="Reading immutable postings from the authorized account scope." />}
    {error && <StatePanel kind="error" title="Ledger history unavailable" message={error} />}
    {verifiedAt && transactions.length > 0 && <EvidenceFreshness state={error ? "historical" : loading ? "refreshing" : "current"} verifiedAt={verifiedAt} label="Ledger history" reason={error ?? undefined} />}
    {!loading && !error && transactions.length === 0 && <StatePanel title="No ledger entries" message="This account has no posted transfer history in the available window." />}
    {transactions.length > 0 && <><div className="ledger-list">{transactions.map((transaction) => <article className="ledger-row" key={`${transaction.transfer_id}-${transaction.direction}`}>
      <span className={`status-icon ${transaction.status === "posted" ? "success" : "danger"}`} aria-hidden="true">{transaction.status === "posted" ? <CheckCircle weight="fill" /> : <WarningCircle weight="fill" />}</span>
      <div className="ledger-identity"><strong>{transaction.direction === "debit" ? "Transfer sent" : "Transfer received"}</strong><span><code>{transaction.transfer_id}</code> · {utcDateTime(transaction.occurred_at)}</span></div>
      <strong className="ledger-amount">{transaction.direction === "debit" ? "−" : "+"}{formatMinorUnits(transaction.currency, transaction.amount)}</strong><StatusBadge tone={transaction.status === "posted" ? "success" : "danger"}>{transaction.status}</StatusBadge><RecordLink href={`/transfers/${transaction.transfer_id}`} label={`Open transfer ${transaction.transfer_id}`} />
    </article>)}</div><Pagination nextCursor={nextCursor} busy={loading} onNext={onNext} /></>}
  </section>;
}
