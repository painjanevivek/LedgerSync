export type TransferRequestState = "in_progress" | "completed" | "rejected" | "unavailable" | "expired";
export type LiveRoomStatus = "waiting" | "active" | "ended" | "expired";
export type LiveParticipantRole = "owner" | "peer";
export type LiveFindingCode = "confirmed_completed" | "confirmed_rejected" | "still_unresolved" | "escalated";

export type TransferRequestStatus = Readonly<{
  request_reference: string;
  status: TransferRequestState;
  transfer_id?: string;
  checked_at: string;
}>;

export type LiveFinding = Readonly<{
  code: LiveFindingCode;
  authoritative_request_state: TransferRequestState;
  transfer_id?: string;
  recorded_at: string;
}>;

export type LiveRoom = Readonly<{
  room_id: string;
  investigation_id: string;
  request_reference: string;
  status: LiveRoomStatus;
  participant_role: LiveParticipantRole;
  peer_bound: boolean;
  peer_present: boolean;
  version: string;
  created_at: string;
  expires_at: string;
  authoritative_request_state: TransferRequestState;
  transfer_id?: string;
  finding?: LiveFinding;
}>;

export type LiveSignalKind = "offer" | "answer" | "ice" | "end";
export type LiveSignal = Readonly<{
  id: string;
  room_id: string;
  role: LiveParticipantRole;
  sequence: number;
  kind: LiveSignalKind;
  payload: unknown;
  sent_at: string;
}>;

const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/u;
const states = new Set<TransferRequestState>(["in_progress", "completed", "rejected", "unavailable", "expired"]);
const roomStates = new Set<LiveRoomStatus>(["waiting", "active", "ended", "expired"]);
const roles = new Set<LiveParticipantRole>(["owner", "peer"]);
const findings = new Set<LiveFindingCode>(["confirmed_completed", "confirmed_rejected", "still_unresolved", "escalated"]);
const signalKinds = new Set<LiveSignalKind>(["offer", "answer", "ice", "end"]);

export function isLiveUUID(value: unknown): value is string {
  return typeof value === "string" && uuid.test(value);
}

function record(value: unknown): Record<string, unknown> | null {
  return typeof value === "object" && value !== null && !Array.isArray(value) ? value as Record<string, unknown> : null;
}

function timestamp(value: unknown): value is string {
  return typeof value === "string" && value.length <= 64 && !Number.isNaN(Date.parse(value));
}

function version(value: unknown): value is string {
  return typeof value === "string" && /^[1-9][0-9]{0,18}$/u.test(value);
}

export function parseTransferRequestStatus(value: unknown): TransferRequestStatus | null {
  const item = record(value);
  if (!item || !isLiveUUID(item.request_reference) || !states.has(item.status as TransferRequestState) || !timestamp(item.checked_at)) return null;
  if (item.transfer_id !== undefined && !isLiveUUID(item.transfer_id)) return null;
  return { request_reference: item.request_reference, status: item.status as TransferRequestState, checked_at: item.checked_at, ...(item.transfer_id ? { transfer_id: item.transfer_id } : {}) };
}

function parseFinding(value: unknown): LiveFinding | undefined | null {
  if (value === undefined) return undefined;
  const item = record(value);
  if (!item || !findings.has(item.code as LiveFindingCode) || !states.has(item.authoritative_request_state as TransferRequestState) || !timestamp(item.recorded_at)) return null;
  if (item.transfer_id !== undefined && !isLiveUUID(item.transfer_id)) return null;
  return { code: item.code as LiveFindingCode, authoritative_request_state: item.authoritative_request_state as TransferRequestState, recorded_at: item.recorded_at, ...(item.transfer_id ? { transfer_id: item.transfer_id } : {}) };
}

export function parseLiveRoom(value: unknown): LiveRoom | null {
  const item = record(value);
  if (!item || !isLiveUUID(item.room_id) || !isLiveUUID(item.investigation_id) || !isLiveUUID(item.request_reference)
    || !roomStates.has(item.status as LiveRoomStatus) || !roles.has(item.participant_role as LiveParticipantRole)
    || typeof item.peer_bound !== "boolean" || typeof item.peer_present !== "boolean" || !version(item.version)
    || !timestamp(item.created_at) || !timestamp(item.expires_at) || !states.has(item.authoritative_request_state as TransferRequestState)) return null;
  if (item.transfer_id !== undefined && !isLiveUUID(item.transfer_id)) return null;
  const finding = parseFinding(item.finding);
  if (finding === null) return null;
  return {
    room_id: item.room_id, investigation_id: item.investigation_id, request_reference: item.request_reference,
    status: item.status as LiveRoomStatus, participant_role: item.participant_role as LiveParticipantRole,
    peer_bound: item.peer_bound, peer_present: item.peer_present, version: item.version,
    created_at: item.created_at, expires_at: item.expires_at,
    authoritative_request_state: item.authoritative_request_state as TransferRequestState,
    ...(item.transfer_id ? { transfer_id: item.transfer_id } : {}), ...(finding ? { finding } : {}),
  };
}

export function parseLiveSignalPage(value: unknown): Readonly<{ signals: LiveSignal[]; cursor: string }> | null {
  const item = record(value);
  if (!item || !Array.isArray(item.signals) || item.signals.length > 200 || typeof item.cursor !== "string" || !/^(?:0-0|[1-9][0-9]*-[0-9]+)$/u.test(item.cursor)) return null;
  const parsed: LiveSignal[] = [];
  for (const candidate of item.signals) {
    const signal = record(candidate);
    if (!signal || typeof signal.id !== "string" || !/^[1-9][0-9]*-[0-9]+$/u.test(signal.id) || !isLiveUUID(signal.room_id)
      || !roles.has(signal.role as LiveParticipantRole) || !Number.isSafeInteger(signal.sequence) || Number(signal.sequence) < 1
      || !signalKinds.has(signal.kind as LiveSignalKind) || !timestamp(signal.sent_at)) return null;
    parsed.push({ id: signal.id as string, room_id: signal.room_id as string, role: signal.role as LiveParticipantRole, sequence: signal.sequence as number, kind: signal.kind as LiveSignalKind, payload: signal.payload, sent_at: signal.sent_at as string });
  }
  return { signals: parsed, cursor: item.cursor };
}

export function sanitizeLiveSuccess(path: string, method: "GET" | "POST", value: unknown): unknown | null {
  if (path.includes("/transfer-requests/") && path.endsWith("/status")) return parseTransferRequestStatus(value);
  if (path.endsWith("/signals") && method === "GET") return parseLiveSignalPage(value);
  const item = record(value);
  if (path.endsWith("/invite")) {
    if (!item || !isLiveUUID(item.room_id) || typeof item.invite_token !== "string" || !/^[A-Za-z0-9_-]{32,128}$/u.test(item.invite_token) || !timestamp(item.expires_at)) return null;
    return { room_id: item.room_id, invite_token: item.invite_token, expires_at: item.expires_at };
  }
  if (path.endsWith("/leave") || path.endsWith("/end")) {
    if (!item || !isLiveUUID(item.room_id) || typeof item.outcome !== "string" || !version(item.version) || !timestamp(item.occurred_at)) return null;
    return { room_id: item.room_id, outcome: item.outcome, version: item.version, occurred_at: item.occurred_at };
  }
  return parseLiveRoom(value);
}

export function findingAllowed(code: LiveFindingCode, status: TransferRequestState, transferId?: string): boolean {
  if (code === "confirmed_completed") return status === "completed" && isLiveUUID(transferId);
  if (code === "confirmed_rejected") return status === "rejected" && !transferId;
  return ["in_progress", "unavailable", "expired"].includes(status) && !transferId;
}
