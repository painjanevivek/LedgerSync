export type ExactMoneyInput = Readonly<{
  currency: string;
  minorUnits: string;
}>;

export type CreateTransferInput = Readonly<{
  sourceAccountId: string;
  destinationAccountId: string;
  amount: ExactMoneyInput;
}>;

export type TransferResult = Readonly<{
  transfer_id: string;
  status: "posted" | "rejected";
  currency: string;
  amount_minor: string;
  occurred_at: string;
  minimum_balance_versions: Record<string, string>;
  balances?: Record<string, TransferBalance>;
  rejection_code?: string;
  metadata_status?: "complete" | "partial" | "unavailable";
  warnings?: readonly (
    | "destination_consistency_unavailable"
    | "consistency_requirement_unavailable"
    | "consistency_header_unavailable"
  )[];
}>;

export type TransferBalance = Readonly<{
  account_id: string;
  currency: string;
  posted_minor: string;
  version: string;
  as_of: string;
}>;

type PrivateTransferRequest = Readonly<{
  source_account_id: string;
  destination_account_id: string;
  amount: string;
  currency: string;
}>;

const currencyExponents: Readonly<Record<string, number>> = {
  EUR: 2,
  GBP: 2,
  INR: 2,
  JPY: 0,
  KWD: 3,
  USD: 2,
};

// decimalFromMinorUnits deliberately uses string operations only. Browser
// Number values cannot safely represent all signed-64-bit financial values.
export function decimalFromMinorUnits(currency: string, minorUnits: string): string {
  const normalizedCurrency = currency.trim().toUpperCase();
  const exponent = currencyExponents[normalizedCurrency];
  const normalizedMinor = minorUnits.trim();
  if (exponent === undefined || !/^[1-9][0-9]*$/.test(normalizedMinor)) {
    throw new Error("invalid exact money input");
  }
  if (exponent === 0) return normalizedMinor;
  const padded = normalizedMinor.padStart(exponent + 1, "0");
  const whole = padded.slice(0, -exponent);
  const fraction = padded.slice(-exponent);
  return `${whole}.${fraction}`;
}

export function toPrivateTransferRequest(input: CreateTransferInput): PrivateTransferRequest {
  const sourceAccountId = input.sourceAccountId.trim();
  const destinationAccountId = input.destinationAccountId.trim();
  const currency = input.amount.currency.trim().toUpperCase();
  if (!sourceAccountId || !destinationAccountId || sourceAccountId === destinationAccountId) {
    throw new Error("invalid transfer account input");
  }
  return {
    source_account_id: sourceAccountId,
    destination_account_id: destinationAccountId,
    amount: decimalFromMinorUnits(currency, input.amount.minorUnits),
    currency,
  };
}
