"use client";

import { useCallback, useMemo, useState } from "react";

import type { AccountFilters } from "@/features/accounts/AccountViews";
import type { Account, Transaction } from "@/features/accounts/types";
import { readJSON, unavailableMessage } from "@/lib/api/client";

type AccountsPayload = { accounts?: Account[]; next_cursor?: string };
type TransactionsPayload = { transactions?: Transaction[]; next_cursor?: string };

export const emptyAccountFilters: AccountFilters = { query: "", status: "", category: "" };

function accountQuery(filters: AccountFilters, limit: number): string {
  const params = new URLSearchParams({ limit: String(limit) });
  if (filters.query) params.set("q", filters.query);
  if (filters.status) params.set("status", filters.status);
  if (filters.category) params.set("category", filters.category);
  if (filters.cursor) params.set("cursor", filters.cursor);
  return params.toString();
}

function accountDirectoryURL(filters: AccountFilters): string {
  const params = new URLSearchParams();
  if (filters.query) params.set("q", filters.query);
  if (filters.status) params.set("status", filters.status);
  if (filters.category) params.set("category", filters.category);
  if (filters.cursor) params.set("cursor", filters.cursor);
  const query = params.toString();
  return query ? `/accounts?${query}` : "/accounts";
}

export function useAccountWorkspace(initialAccountId: string | undefined, initialFilters: AccountFilters) {
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [accountDetail, setAccountDetail] = useState<Account | null>(null);
  const [filters, setFilters] = useState<AccountFilters>(initialFilters);
  const [nextCursor, setNextCursor] = useState<string>();
  const [scopeComplete, setScopeComplete] = useState(true);
  const [balance, setBalance] = useState<Account | null>(null);
  const [transactions, setTransactions] = useState<Transaction[]>([]);
  const [historyCursor, setHistoryCursor] = useState<string>();
  const [directoryLoading, setDirectoryLoading] = useState(false);
  const [balanceLoading, setBalanceLoading] = useState(false);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [balanceError, setBalanceError] = useState<string | null>(null);
  const [historyError, setHistoryError] = useState<string | null>(null);

  const selected = useMemo(() => accountDetail ?? accounts.find((account) => account.account_id === initialAccountId) ?? null, [accountDetail, accounts, initialAccountId]);

  const load = useCallback(async (requestedFilters: AccountFilters, limit = 25) => {
    setDirectoryLoading(true);
    const response = await readJSON<AccountsPayload>(`/api/me/accounts?${accountQuery(requestedFilters, limit)}`);
    if (!response.ok) {
      setError(unavailableMessage(response.status, "accounts"));
      setDirectoryLoading(false);
      return [];
    }
    const items = Array.isArray(response.data.accounts) ? response.data.accounts : [];
    setAccounts(items);
    setNextCursor(response.data.next_cursor || undefined);
    if (limit === 100) setScopeComplete(!response.data.next_cursor);
    setError(null);
    setDirectoryLoading(false);
    return items;
  }, []);

  const loadDetail = useCallback(async (accountId: string) => {
    setBalanceLoading(true);
    setHistoryLoading(true);
    setBalanceError(null);
    setHistoryError(null);
    const [summary, currentBalance, history] = await Promise.all([
      readJSON<Account>(`/api/accounts/${encodeURIComponent(accountId)}`),
      readJSON<Account>(`/api/accounts/${encodeURIComponent(accountId)}/balance`),
      readJSON<TransactionsPayload>(`/api/accounts/${encodeURIComponent(accountId)}/transactions?limit=25`),
    ]);
    setAccountDetail(summary.ok && summary.data.account_id ? summary.data : null);
    if (!summary.ok) setError(unavailableMessage(summary.status, "account evidence"));
    setBalance(currentBalance.ok && currentBalance.data.account_id ? currentBalance.data : null);
    if (!currentBalance.ok) setBalanceError("The authoritative balance could not be verified. An older value is not shown as current.");
    setBalanceLoading(false);
    if (history.ok && Array.isArray(history.data.transactions)) {
      setTransactions(history.data.transactions);
      setHistoryCursor(history.data.next_cursor || undefined);
      setHistoryError(null);
    } else {
      setTransactions([]);
      setHistoryError(history.status === 401 ? "Your session expired. Re-authenticate before viewing ledger history." : history.status === 403 ? "Your role is not authorized to view ledger history." : "Ledger history is temporarily unavailable. No empty result is being inferred.");
    }
    setHistoryLoading(false);
  }, []);

  const loadMoreHistory = useCallback(async () => {
    if (!initialAccountId || !historyCursor) return;
    setHistoryLoading(true);
    const history = await readJSON<TransactionsPayload>(`/api/accounts/${encodeURIComponent(initialAccountId)}/transactions?limit=25&cursor=${encodeURIComponent(historyCursor)}`);
    if (history.ok && Array.isArray(history.data.transactions)) {
      setTransactions((current) => [...current, ...history.data.transactions!]);
      setHistoryCursor(history.data.next_cursor || undefined);
      setHistoryError(null);
    } else setHistoryError("Additional ledger history is temporarily unavailable. Existing entries remain verified.");
    setHistoryLoading(false);
  }, [historyCursor, initialAccountId]);

  function applyFilters(requested: Omit<AccountFilters, "cursor">) {
    const next = { ...requested, cursor: undefined };
    setFilters(next);
    window.history.replaceState(null, "", accountDirectoryURL(next));
    void load(next);
  }

  function loadNextPage() {
    if (!nextCursor) return;
    const next = { ...filters, cursor: nextCursor };
    setFilters(next);
    window.history.replaceState(null, "", accountDirectoryURL(next));
    void load(next);
  }

  return { accounts, selected, filters, nextCursor, scopeComplete, balance, transactions, historyCursor, directoryLoading, balanceLoading, historyLoading, error, balanceError, historyError, load, loadDetail, loadMoreHistory, applyFilters, loadNextPage };
}
