import type { Account } from "@/features/accounts/types";

export type PreparedTransfer = Readonly<{
  source: Account;
  destination: Account;
  amountMinor: string;
}>;

export type StoredTransferIntent = Readonly<{
  version: 1;
  idempotencyKey: string;
  sourceAccountId: string;
  destinationAccountId: string;
  currency: string;
  amountMinor: string;
}>;

export function transferIntentStorageKey(tenantId: string): string {
  return `ledgersync.transfer.intent.${tenantId}`;
}

export function createStoredTransferIntent(idempotencyKey: string, prepared: PreparedTransfer): StoredTransferIntent {
  return {
    version: 1,
    idempotencyKey,
    sourceAccountId: prepared.source.account_id,
    destinationAccountId: prepared.destination.account_id,
    currency: prepared.source.currency,
    amountMinor: prepared.amountMinor,
  };
}

export function storedIntentMatches(intent: StoredTransferIntent, prepared: PreparedTransfer): boolean {
  return intent.sourceAccountId === prepared.source.account_id
    && intent.destinationAccountId === prepared.destination.account_id
    && intent.currency === prepared.source.currency
    && intent.amountMinor === prepared.amountMinor;
}

export function parseStoredTransferIntent(raw: string | null): StoredTransferIntent | null {
  if (!raw) return null;
  try {
    const value = JSON.parse(raw) as Partial<StoredTransferIntent>;
    if (
      value.version !== 1
      || typeof value.idempotencyKey !== "string"
      || value.idempotencyKey.length < 16
      || value.idempotencyKey.length > 255
      || typeof value.sourceAccountId !== "string"
      || value.sourceAccountId.length === 0
      || value.sourceAccountId.length > 128
      || typeof value.destinationAccountId !== "string"
      || value.destinationAccountId.length === 0
      || value.destinationAccountId.length > 128
      || value.sourceAccountId === value.destinationAccountId
      || typeof value.currency !== "string"
      || !/^[A-Z]{3}$/.test(value.currency)
      || typeof value.amountMinor !== "string"
      || !/^[1-9][0-9]*$/.test(value.amountMinor)
    ) {
      return null;
    }
    return value as StoredTransferIntent;
  } catch {
    return null;
  }
}
