import type { Account } from "@/features/accounts/types";

export function accountLabel(account: Pick<Account, "account_id" | "display_name">) {
  return account.display_name?.trim() || `Account ${account.account_id.slice(0, 8)}`;
}

export function utcDateTime(value?: string) {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.valueOf())) return "Unavailable";
  return new Intl.DateTimeFormat("en-GB", { day: "2-digit", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit", second: "2-digit", hour12: false, timeZone: "UTC", timeZoneName: "short" }).format(date);
}
