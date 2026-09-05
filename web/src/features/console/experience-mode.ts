export type ExperienceMode = "simple" | "expert";

export type OperatorUIPreferences = Readonly<{
  experienceMode: ExperienceMode;
}>;

type PreferenceStorage = Pick<Storage, "getItem" | "setItem">;

export function experiencePreferenceKey(tenantId: string, subjectId: string): string {
  return `ledgersync:v1:experience:${encodeURIComponent(tenantId)}:${encodeURIComponent(subjectId)}`;
}

export function readExperienceMode(
  storage: PreferenceStorage | undefined,
  tenantId: string,
  subjectId: string,
): ExperienceMode {
  if (!storage || !tenantId || !subjectId) return "simple";
  try {
    return storage.getItem(experiencePreferenceKey(tenantId, subjectId)) === "expert" ? "expert" : "simple";
  } catch {
    return "simple";
  }
}

export function writeExperienceMode(
  storage: PreferenceStorage | undefined,
  tenantId: string,
  subjectId: string,
  mode: ExperienceMode,
): boolean {
  if (!storage || !tenantId || !subjectId) return false;
  try {
    storage.setItem(experiencePreferenceKey(tenantId, subjectId), mode);
    return true;
  } catch {
    return false;
  }
}
