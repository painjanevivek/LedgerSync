"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useMemo, useState } from "react";

import { BalanceStatus } from "@/features/accounts/BalanceStatus";
import type { Account, ConsoleSession, Transaction } from "@/features/accounts/types";
import { TransferForm } from "@/features/transfers/TransferForm";
import { formatMinorUnits } from "@/lib/money";

type Section = "overview" | "accounts" | "transfers";
type Props = Readonly<{ initialSection?: Section; initialAccountId?: string }>;

type APIError = { error?: { code?: string } };
type AccountsPayload = { accounts?: Account[] };
type TransactionsPayload = { transactions?: Transaction[] };

async function readJSON<T>(path: string): Promise<{ ok: boolean; status: number; data: T & APIError }> {
  const response = await fetch(path, { cache: "no-store" });
  return { ok: response.ok, status: response.status, data: await response.json().catch(() => ({})) as T & APIError };
}

function accountLabel(account: Account) {
  return `${account.account_id.slice(0, 8)}…`;
}

export function OperatorConsole({ initialSection = "overview", initialAccountId }: Props) {
  const router = useRouter();
  const [section, setSection] = useState<Section>(initialSection);
  const [session, setSession] = useState<ConsoleSession | null>(null);
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [selectedAccountId, setSelectedAccountId] = useState(initialAccountId ?? "");
  const [balance, setBalance] = useState<Account | null>(null);
  const [transactions, setTransactions] = useState<Transaction[]>([]);
  const [loading, setLoading] = useState(true);
  const [balanceLoading, setBalanceLoading] = useState(false);
  const [accountsError, setAccountsError] = useState<string | null>(null);
  const [balanceError, setBalanceError] = useState<string | null>(null);
  const [online, setOnline] = useState(true);

  const selectedAccount = useMemo(
    () => accounts.find((account) => account.account_id === selectedAccountId) ?? accounts[0] ?? null,
    [accounts, selectedAccountId],
  );

  const loadAccounts = useCallback(async (): Promise<Account[]> => {
    const response = await readJSON<AccountsPayload>("/api/me/accounts");
    if (!response.ok) {
      setAccountsError(response.status === 401 ? "Your session has expired. Sign in again to view accounts." : "Accounts are temporarily unavailable. No cached balance is being shown as current.");
      return [];
    }
    const ownedAccounts = Array.isArray(response.data.accounts) ? response.data.accounts : [];
    setAccounts(ownedAccounts);
    setAccountsError(null);
    setSelectedAccountId((current) => current || initialAccountId || ownedAccounts[0]?.account_id || "");
    return ownedAccounts;
  }, [initialAccountId]);

  const loadAccountDetail = useCallback(async (accountId: string) => {
    if (!accountId) return;
    setBalanceLoading(true);
    setBalanceError(null);
    const [balanceResponse, transactionsResponse] = await Promise.all([
      readJSON<Account>(`/api/accounts/${encodeURIComponent(accountId)}/balance`),
      readJSON<TransactionsPayload>(`/api/accounts/${encodeURIComponent(accountId)}/transactions?limit=20`),
    ]);
    if (balanceResponse.ok && balanceResponse.data.account_id) {
      setBalance(balanceResponse.data);
    } else {
      setBalance(null);
      setBalanceError("The authoritative balance could not be confirmed. Try again shortly.");
    }
    setTransactions(transactionsResponse.ok && Array.isArray(transactionsResponse.data.transactions) ? transactionsResponse.data.transactions : []);
    setBalanceLoading(false);
  }, []);

  const refresh = useCallback(async () => {
    const ownedAccounts = await loadAccounts();
    const accountId = selectedAccountId || initialAccountId || ownedAccounts[0]?.account_id;
    if (accountId) await loadAccountDetail(accountId);
  }, [initialAccountId, loadAccountDetail, loadAccounts, selectedAccountId]);

  useEffect(() => {
    let active = true;
    async function bootstrap() {
      const response = await readJSON<ConsoleSession>("/api/session");
      if (active && response.ok && response.data.tenant_id && response.data.csrf_token) setSession(response.data);
      if (active) setLoading(false);
      if (response.ok) {
        const ownedAccounts = await loadAccounts();
        const accountId = initialAccountId || ownedAccounts[0]?.account_id;
        if (accountId) await loadAccountDetail(accountId);
      }
    }
    void bootstrap();
    return () => { active = false; };
  }, [initialAccountId, loadAccountDetail, loadAccounts]);

  useEffect(() => {
    const updateOnline = () => setOnline(navigator.onLine);
    updateOnline();
    window.addEventListener("online", updateOnline);
    window.addEventListener("offline", updateOnline);
    return () => {
      window.removeEventListener("online", updateOnline);
      window.removeEventListener("offline", updateOnline);
    };
  }, []);

  async function signOut() {
    if (!session) return;
    await fetch("/api/auth/sign-out", { method: "POST", headers: { "X-CSRF-Token": session.csrf_token } });
    router.push("/sign-in");
  }

  if (loading) return <main className="boot-screen" aria-busy="true"><p className="eyebrow">LedgerSync</p><h1>Loading your ledger workspace…</h1></main>;
  if (!session) return <main className="sign-in-screen"><section className="sign-in-card"><p className="eyebrow">LedgerSync operator console</p><h1>Every movement, exact and explainable.</h1><p>Use your organization’s identity provider to access the ledger accounts assigned to you.</p><a className="button primary" href="/api/auth/sign-in">Sign in with your organization</a><p className="muted">LedgerSync never displays an unverified cached balance as current.</p></section></main>;

  const activeSection = section;
  const showDetail = activeSection === "overview" || activeSection === "accounts";
  return <div className="app-shell">
    <a className="skip-link" href="#main-content">Skip to main content</a>
    <aside className="side-nav" aria-label="Primary navigation">
      <Link className="brand" href="/">Ledger<span>Sync</span></Link>
      <p className="tenant-label">Operator workspace</p>
      <nav>
        <button className={activeSection === "overview" ? "nav-item active" : "nav-item"} onClick={() => setSection("overview")} aria-current={activeSection === "overview" ? "page" : undefined}>Overview</button>
        <button className={activeSection === "accounts" ? "nav-item active" : "nav-item"} onClick={() => setSection("accounts")} aria-current={activeSection === "accounts" ? "page" : undefined}>Accounts</button>
        <button className={activeSection === "transfers" ? "nav-item active" : "nav-item"} onClick={() => setSection("transfers")} aria-current={activeSection === "transfers" ? "page" : undefined}>Transfers</button>
      </nav>
      <div className="nav-footer"><span className="status-dot" aria-hidden="true" />Secure session<button className="text-button" onClick={signOut}>Sign out</button></div>
    </aside>
    <main id="main-content" className="console-main">
      {!online && <div className="offline-banner" role="status">You are offline. Transfers are disabled until your connection returns; no result is being inferred.</div>}
      <header className="page-header"><div><p className="eyebrow">{activeSection === "transfers" ? "Move funds" : "Ledger visibility"}</p><h1>{activeSection === "transfers" ? "Prepare an internal transfer" : activeSection === "accounts" ? "Owned accounts" : "Balance clarity, without guesswork"}</h1><p>{activeSection === "transfers" ? "Post same-currency internal movement with a retry-safe request key." : "Authoritative balances and ledger events are scoped to your organization."}</p></div><button className="button secondary" onClick={() => void refresh()} disabled={!online}>Refresh data</button></header>
      {accountsError && <div className="inline-alert error" role="alert"><strong>Accounts unavailable</strong><p>{accountsError}</p></div>}
      {!accountsError && accounts.length === 0 && <section className="surface empty-state"><h2>No accounts are assigned yet</h2><p>Your administrator can grant access to a ledger account. This workspace will only show accounts you are authorized to inspect.</p></section>}
      {accounts.length > 0 && <div className="console-grid">
        {(activeSection === "overview" || activeSection === "accounts") && <section className="surface accounts-panel" aria-labelledby="accounts-heading"><div className="panel-heading"><div><p className="eyebrow">Your scope</p><h2 id="accounts-heading">Accounts</h2></div><span className="count-badge">{accounts.length}</span></div><div className="account-list">{accounts.map((account) => <button key={account.account_id} className={selectedAccount?.account_id === account.account_id ? "account-row selected" : "account-row"} onClick={() => { setSelectedAccountId(account.account_id); void loadAccountDetail(account.account_id); }}><span><strong>{accountLabel(account)}</strong><small>{account.currency} · {account.status}</small></span><strong>{formatMinorUnits(account.currency, account.available_minor)}</strong></button>)}</div></section>}
        {showDetail && selectedAccount && <section className="detail-stack"><BalanceStatus currency={balance?.currency ?? selectedAccount.currency} availableMinor={balance?.available_minor} version={balance?.version} asOf={balance?.as_of} loading={balanceLoading} error={balanceError} /><section className="surface transactions" aria-labelledby="transactions-heading"><div className="panel-heading"><div><p className="eyebrow">Immutable history</p><h2 id="transactions-heading">Recent ledger activity</h2></div><span className="muted">{transactions.length} shown</span></div>{transactions.length === 0 ? <p className="empty-copy">No posted or rejected transfer events are available for this account yet.</p> : <div className="table-wrap"><table><thead><tr><th scope="col">Event</th><th scope="col">Direction</th><th scope="col">Amount</th><th scope="col">Status</th><th scope="col">Time</th></tr></thead><tbody>{transactions.map((transaction) => <tr key={`${transaction.transfer_id}-${transaction.direction}`}><td><code>{transaction.transfer_id.slice(0, 12)}…</code></td><td><span className={`direction ${transaction.direction}`}>{transaction.direction}</span></td><td>{formatMinorUnits(transaction.currency, transaction.amount)}</td><td><span className={`status-badge ${transaction.status}`}>{transaction.status}</span></td><td>{new Date(transaction.occurred_at).toLocaleString()}</td></tr>)}</tbody></table></div>}</section></section>}
        {activeSection === "transfers" && <div className="detail-stack"><TransferForm accounts={accounts} tenantId={session.tenant_id} csrfToken={session.csrf_token} disabled={!online || !session.scopes.includes("transfers:write")} onPosted={refresh} />{!session.scopes.includes("transfers:write") && <p className="permission-note">Your role can inspect accounts but cannot post transfers.</p>}</div>}
      </div>}
    </main>
  </div>;
}
