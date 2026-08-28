export type APIError = { error?: { code?: string } };

export type UIDataState =
  | "loading"
  | "ready-empty"
  | "ready-populated"
  | "partial"
  | "stale"
  | "unavailable"
  | "forbidden"
  | "offline"
  | "unknown-after-submit";

export type ReadJSONResult<T> = Readonly<{
  ok: boolean;
  status: number;
  data: T & APIError;
  requestReference: string;
  errorCode?: string;
}>;

function requestReference(): string {
  return globalThis.crypto?.randomUUID?.() ?? `browser-${Date.now().toString(36)}`;
}

export async function readJSON<T>(path: string): Promise<ReadJSONResult<T>> {
  const localReference = requestReference();
  try {
    const response = await fetch(path, {
      cache: "no-store",
      headers: { "X-Request-ID": localReference },
      signal: AbortSignal.timeout(8_000),
    });
    const data = await response.json().catch(() => ({})) as T & APIError;
    return {
      ok: response.ok,
      status: response.status,
      data,
      requestReference: response.headers.get("X-Request-ID") ?? localReference,
      errorCode: data.error?.code,
    };
  } catch {
    return {
      ok: false,
      status: 0,
      data: {} as T & APIError,
      requestReference: localReference,
      errorCode: "connection_unavailable",
    };
  }
}

export function unavailableMessage(status: number, subject: string, reference?: string) {
  const evidence = "Previously verified evidence, if shown, remains historical; no empty or successful result is being inferred.";
  const next = status === 401
    ? `Your session expired while requesting ${subject}. ${evidence} Sign in again, then retry only this request.`
    : status === 403
      ? `Your role is not authorized to view ${subject}. ${evidence} Ask an administrator for the required scope.`
      : status === 0
        ? `${subject[0].toUpperCase()}${subject.slice(1)} could not be reached. ${evidence} Check the connection, then retry only this request.`
        : `${subject[0].toUpperCase()}${subject.slice(1)} could not be refreshed. ${evidence} Retry only this evidence request.`;
  return reference ? `${next} Request reference: ${reference}.` : next;
}

export function uiDataState({ loading, hasData, hasError, online = true, forbidden = false, partial = false, unknownAfterSubmit = false }: Readonly<{ loading: boolean; hasData: boolean; hasError: boolean; online?: boolean; forbidden?: boolean; partial?: boolean; unknownAfterSubmit?: boolean }>): UIDataState {
  if (unknownAfterSubmit) return "unknown-after-submit";
  if (forbidden) return "forbidden";
  if (!online) return "offline";
  if (loading && !hasData) return "loading";
  if (partial) return "partial";
  if (hasError && hasData) return "stale";
  if (hasError) return "unavailable";
  return hasData ? "ready-populated" : "ready-empty";
}
