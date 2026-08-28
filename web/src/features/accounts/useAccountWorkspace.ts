"use client";

import { useCallback, useMemo, useRef, useState } from "react";

import type { AccountFilters } from "@/features/accounts/AccountViews";
import type { Account, AccountBalance, Transaction } from "@/features/accounts/types";
import { readJSON, unavailableMessage, uiDataState } from "@/lib/api/client";

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
  const [balance, setBalance] = useState<AccountBalance | null>(null);
  const [transactions, setTransactions] = useState<Transaction[]>([]);
  const [historyCursor, setHistoryCursor] = useState<string>();
  const [directoryLoading, setDirectoryLoading] = useState(false);
  const [balanceLoading, setBalanceLoading] = useState(false);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [balanceError, setBalanceError] = useState<string | null>(null);
  const [historyError, setHistoryError] = useState<string | null>(null);
  const [directoryVerifiedAt, setDirectoryVerifiedAt] = useState<string>();
  const [balanceVerifiedAt, setBalanceVerifiedAt] = useState<string>();
  const [historyVerifiedAt, setHistoryVerifiedAt] = useState<string>();
  const directoryKey = useRef<string | undefined>(undefined);
  const detailKey = useRef<string | undefined>(undefined);
  const directoryGeneration = useRef(0);
  const detailGeneration = useRef(0);
  const balanceGeneration = useRef(0);
  const historyGeneration = useRef(0);

  const selected = useMemo(() => accountDetail ?? accounts.find((account) => account.account_id === initialAccountId) ?? null, [accountDetail, accounts, initialAccountId]);

  const load = useCallback(async (requestedFilters: AccountFilters, limit = 25) => {
    const key = accountQuery(requestedFilters, limit);
    const sameQuery = directoryKey.current === key;
    directoryKey.current = key;
    const generation = ++directoryGeneration.current;
    if (!sameQuery) {
      setAccounts([]);
      setNextCursor(undefined);
      setDirectoryVerifiedAt(undefined);
    }
    setDirectoryLoading(true);
    const response = await readJSON<AccountsPayload>(`/api/me/accounts?${key}`);
    if (generation !== directoryGeneration.current) return [];
    if (!response.ok) {
      setError(unavailableMessage(response.status, "accounts", response.requestReference));
      setDirectoryLoading(false);
      return [];
    }
    const items = Array.isArray(response.data.accounts) ? response.data.accounts : [];
    setAccounts(items);
    setNextCursor(response.data.next_cursor || undefined);
    setDirectoryVerifiedAt(new Date().toISOString());
    if (limit === 100) setScopeComplete(!response.data.next_cursor);
    setError(null);
    setDirectoryLoading(false);
    return items;
  }, []);

  const loadBalance = useCallback(async (accountId: string) => {
    const generation = ++balanceGeneration.current;
    setBalanceLoading(true);
    setBalanceError(null);
    const currentBalance = await readJSON<AccountBalance>(`/api/accounts/${encodeURIComponent(accountId)}/balance`);
    if (generation !== balanceGeneration.current) return null;
    if (currentBalance.ok && currentBalance.data.account_id) {
      setBalance(currentBalance.data);
      setBalanceVerifiedAt(currentBalance.data.as_of || new Date().toISOString());
      setBalanceError(null);
      setBalanceLoading(false);
      return currentBalance.data;
    }
    setBalanceError(unavailableMessage(currentBalance.status, "the authoritative balance", currentBalance.requestReference));
    setBalanceLoading(false);
    return null;
  }, []);

  const loadHistory = useCallback(async (accountId: string) => {
    const generation = ++historyGeneration.current;
    setHistoryLoading(true);
    setHistoryError(null);
    const history = await readJSON<TransactionsPayload>(`/api/accounts/${encodeURIComponent(accountId)}/transactions?limit=25`);
    if (generation !== historyGeneration.current) return [];
    if (history.ok && Array.isArray(history.data.transactions)) {
      setTransactions(history.data.transactions);
      setHistoryCursor(history.data.next_cursor || undefined);
      setHistoryVerifiedAt(new Date().toISOString());
      setHistoryError(null);
      setHistoryLoading(false);
      return history.data.transactions;
    }
    setHistoryError(unavailableMessage(history.status, "ledger history", history.requestReference));
    setHistoryLoading(false);
    return [];
  }, []);

  const loadDetail = useCallback(async (accountId: string) => {
    const sameAccount = detailKey.current === accountId;
    detailKey.current = accountId;
    const generation = ++detailGeneration.current;
    if (!sameAccount) {
      setAccountDetail(null);
      setBalance(null);
      setTransactions([]);
      setHistoryCursor(undefined);
      setBalanceVerifiedAt(undefined);
      setHistoryVerifiedAt(undefined);
    }
    let resolvedAccount: Account | null = null;
    const [, resolvedBalance] = await Promise.all([
      (async () => {
        const summary = await readJSON<Account>(`/api/accounts/${encodeURIComponent(accountId)}`);
        if (generation !== detailGeneration.current) return;
        if (summary.ok && summary.data.account_id) { resolvedAccount = summary.data; setAccountDetail(summary.data); setError(null); }
        else setError(unavailableMessage(summary.status, "account details", summary.requestReference));
      })(),
      loadBalance(accountId),
      loadHistory(accountId),
    ]);
    return { account: resolvedAccount, balance: resolvedBalance };
  }, [loadBalance, loadHistory]);

  const loadMoreHistory = useCallback(async () => {
    if (!initialAccountId || !historyCursor) return;
    setHistoryLoading(true);
    const history = await readJSON<TransactionsPayload>(`/api/accounts/${encodeURIComponent(initialAccountId)}/transactions?limit=25&cursor=${encodeURIComponent(historyCursor)}`);
    if (history.ok && Array.isArray(history.data.transactions)) {
      setTransactions((current) => [...current, ...history.data.transactions!]);
      setHistoryCursor(history.data.next_cursor || undefined);
      setHistoryVerifiedAt(new Date().toISOString());
      setHistoryError(null);
    } else setHistoryError(unavailableMessage(history.status, "additional ledger history", history.requestReference));
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

  const directoryState = uiDataState({ loading: directoryLoading, hasData: accounts.length > 0, hasError: Boolean(error) });

  return { accounts, selected, filters, nextCursor, scopeComplete, balance, transactions, historyCursor, directoryLoading, directoryState, balanceLoading, historyLoading, error, balanceError, historyError, directoryVerifiedAt, balanceVerifiedAt, historyVerifiedAt, load, loadDetail, loadBalance, loadHistory, loadMoreHistory, applyFilters, loadNextPage };
}
