type PreferenceStorage = Pick<Storage, "getItem" | "setItem">;

export function disclosurePreferenceKey(
  tenantId: string,
  subjectId: string,
  disclosureKey: string,
): string {
  return `ledgersync:v1:disclosure:${encodeURIComponent(tenantId)}:${encodeURIComponent(subjectId)}:${encodeURIComponent(disclosureKey)}`;
}

export function readDisclosurePreference(
  storage: PreferenceStorage | undefined,
  tenantId: string,
  subjectId: string,
  disclosureKey: string | undefined,
): boolean | undefined {
  if (!storage || !tenantId || !subjectId || !disclosureKey) return undefined;
  try {
    const value = storage.getItem(disclosurePreferenceKey(tenantId, subjectId, disclosureKey));
    return value === "open" ? true : value === "closed" ? false : undefined;
  } catch {
    return undefined;
  }
}

export function writeDisclosurePreference(
  storage: PreferenceStorage | undefined,
  tenantId: string,
  subjectId: string,
  disclosureKey: string | undefined,
  open: boolean,
): boolean {
  if (!storage || !tenantId || !subjectId || !disclosureKey) return false;
  try {
    storage.setItem(
      disclosurePreferenceKey(tenantId, subjectId, disclosureKey),
      open ? "open" : "closed",
    );
    return true;
  } catch {
    return false;
  }
}
