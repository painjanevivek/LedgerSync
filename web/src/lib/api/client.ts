export type APIError = { error?: { code?: string } };

export async function readJSON<T>(path: string) {
  const response = await fetch(path, { cache: "no-store" });
  return {
    ok: response.ok,
    status: response.status,
    data: await response.json().catch(() => ({})) as T & APIError,
  };
}

export function unavailableMessage(status: number, subject: string) {
  if (status === 401) return `Your session expired. Re-authenticate before viewing ${subject}.`;
  if (status === 403) return `Your role is not authorized to view ${subject}.`;
  return `${subject[0].toUpperCase()}${subject.slice(1)} are temporarily unavailable. No result is being inferred.`;
}
