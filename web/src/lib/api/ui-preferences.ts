import type { ExperienceMode } from "@/features/console/experience-mode";

export type UIPreferenceResponse = Readonly<{
  experience_mode: ExperienceMode;
  version: string;
  updated_at?: string;
}>;

const versionPattern = /^(?:0|[1-9][0-9]{0,18})$/;

export function sanitizeUIPreference(value: unknown): UIPreferenceResponse | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) return null;
  const record = value as Record<string, unknown>;
  const allowed = new Set(["experience_mode", "version", "updated_at"]);
  if (Object.keys(record).some((key) => !allowed.has(key))) return null;
  if ((record.experience_mode !== "simple" && record.experience_mode !== "expert") || typeof record.version !== "string" || !versionPattern.test(record.version)) return null;
  if (record.updated_at !== undefined && (typeof record.updated_at !== "string" || record.updated_at.length > 64 || Number.isNaN(Date.parse(record.updated_at)))) return null;
  return {
    experience_mode: record.experience_mode,
    version: record.version,
    ...(typeof record.updated_at === "string" ? { updated_at: record.updated_at } : {}),
  };
}
