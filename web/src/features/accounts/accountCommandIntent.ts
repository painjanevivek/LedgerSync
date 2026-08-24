import type { Account } from "@/features/accounts/types";

export const accountCategories = ["operating", "customer_funds", "payroll", "payables", "expenses", "reserve"] as const;
export type AccountCategory = (typeof accountCategories)[number];
export type AccountTargetStatus = Account["status"];

export type CreateAccountFields = Readonly<{
  display_name: string;
  external_reference: string;
  category: AccountCategory;
  currency: "INR";
}>;

export type CreateAccountIntent = Readonly<{
  version: 1;
  kind: "create";
  tenantId: string;
  idempotencyKey: string;
  stage: "identity" | "boundary" | "review" | "unknown";
  request: CreateAccountFields;
}>;

export type LifecycleAccountIntent = Readonly<{
  version: 1;
  kind: "lifecycle";
  tenantId: string;
  accountId: string;
  idempotencyKey: string;
  state: "unknown";
  request: Readonly<{
    expected_version: string;
    target_status: AccountTargetStatus;
    reason: string;
  }>;
}>;

const printableKey = /^[\x21-\x7e]{16,255}$/;
const externalReference = /^[A-Za-z0-9][A-Za-z0-9._-]{2,63}$/;
const positiveVersion = /^[1-9][0-9]{0,18}$/;

function record(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function cleanText(value: unknown, maximum: number): value is string {
  return typeof value === "string" && [...value].length >= 1 && [...value].length <= maximum
    && ![...value].some((character) => {
      const point = character.codePointAt(0) ?? 0;
      return point <= 0x1f || point >= 0x7f && point <= 0x9f;
    });
}

export function createAccountStorageKey(tenantId: string) {
  return `ledgersync:account-create:v1:${tenantId}`;
}

export function lifecycleAccountStorageKey(tenantId: string, accountId: string) {
  return `ledgersync:account-lifecycle:v1:${tenantId}:${accountId}`;
}

export function newAccountIdempotencyKey(): string {
  return `account-${crypto.randomUUID()}`;
}

export function validCreateAccountFields(value: CreateAccountFields): boolean {
  return validAccountDisplayName(value.display_name)
    && validAccountExternalReference(value.external_reference)
    && accountCategories.includes(value.category)
    && value.currency === "INR";
}

export function validAccountDisplayName(value: string): boolean { return cleanText(value, 120); }
export function validAccountExternalReference(value: string): boolean { return externalReference.test(value); }

export function normalizeCreateAccountFields(value: CreateAccountFields): CreateAccountFields {
  return {
    display_name: value.display_name.trim(),
    external_reference: value.external_reference.trim().toLowerCase(),
    category: value.category,
    currency: "INR",
  };
}

export function validLifecycleReason(value: string): boolean {
  return value.trim().length > 0 && cleanText(value, 256);
}

export function hasPositiveMinorUnits(value: string): boolean {
  return /^[1-9][0-9]*$/.test(value);
}

export function parseCreateAccountIntent(raw: string | null, tenantId: string): CreateAccountIntent | null {
  if (!raw) return null;
  try {
    const value: unknown = JSON.parse(raw);
    if (!record(value) || value.version !== 1 || value.kind !== "create" || value.tenantId !== tenantId
      || typeof value.idempotencyKey !== "string" || !printableKey.test(value.idempotencyKey)
      || !["identity", "boundary", "review", "unknown"].includes(String(value.stage)) || !record(value.request)) return null;
    const request = value.request;
    const draftIsBounded = typeof request.display_name === "string" && [...request.display_name].length <= 120
      && typeof request.external_reference === "string" && request.external_reference.length <= 64
      && typeof request.category === "string" && accountCategories.includes(request.category as AccountCategory)
      && request.currency === "INR";
    return draftIsBounded && (value.stage !== "unknown" || validCreateAccountFields(request as unknown as CreateAccountFields))
      ? value as unknown as CreateAccountIntent : null;
  } catch { return null; }
}

export function parseLifecycleAccountIntent(raw: string | null, tenantId: string, accountId: string): LifecycleAccountIntent | null {
  if (!raw) return null;
  try {
    const value: unknown = JSON.parse(raw);
    if (!record(value) || value.version !== 1 || value.kind !== "lifecycle" || value.state !== "unknown"
      || value.tenantId !== tenantId || value.accountId !== accountId
      || typeof value.idempotencyKey !== "string" || !printableKey.test(value.idempotencyKey) || !record(value.request)) return null;
    const request = value.request;
    return typeof request.expected_version === "string" && positiveVersion.test(request.expected_version)
      && ["active", "frozen", "closed"].includes(String(request.target_status))
      && typeof request.reason === "string" && validLifecycleReason(request.reason)
      ? value as unknown as LifecycleAccountIntent : null;
  } catch { return null; }
}
