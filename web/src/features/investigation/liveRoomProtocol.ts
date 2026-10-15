export const liveChannelName = "ledgersync-investigation-v1";
export const maximumLiveMessageBytes = 2 * 1024;
export const maximumLiveMessages = 200;

const containsURL = /(?:https?:\/\/|www\.)/iu;
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/u;

export function signalNegotiationID(payload: unknown): string {
  if (typeof payload !== "object" || payload === null || Array.isArray(payload)) return "";
  const value = (payload as Record<string, unknown>).negotiation_id;
  return typeof value === "string" && uuid.test(value) ? value : "";
}

export type SharedIntent = Readonly<{
  sourceAccountId: string;
  destinationAccountId: string;
  currency: string;
  amountMinor: string;
}>;

export type PeerEnvelope =
  | Readonly<{ type: "message"; id: string; text: string }>
  | Readonly<{ type: "intent"; intent: SharedIntent }>;

export function validLiveMessage(text: string): boolean {
  return text.trim().length > 0
    && !containsURL.test(text)
    && new TextEncoder().encode(text).byteLength <= maximumLiveMessageBytes;
}

export function parsePeerEnvelope(raw: unknown, receiverRole: "owner" | "peer"): PeerEnvelope | null {
  if (typeof raw !== "string" || new TextEncoder().encode(raw).byteLength > maximumLiveMessageBytes + 1024) return null;
  try {
    const value = JSON.parse(raw) as Record<string, unknown>;
    if (value.version !== 1) return null;
    if (value.type === "message" && typeof value.id === "string" && uuid.test(value.id)
      && typeof value.text === "string" && validLiveMessage(value.text)) {
      return { type: "message", id: value.id, text: value.text };
    }
    if (value.type !== "intent" || receiverRole !== "peer" || typeof value.intent !== "object" || value.intent === null || Array.isArray(value.intent)) return null;
    const intent = value.intent as Record<string, unknown>;
    if (!uuid.test(String(intent.sourceAccountId ?? "")) || !uuid.test(String(intent.destinationAccountId ?? ""))
      || intent.sourceAccountId === intent.destinationAccountId || !/^[A-Z]{3}$/u.test(String(intent.currency ?? ""))
      || !/^[1-9][0-9]*$/u.test(String(intent.amountMinor ?? ""))) return null;
    return {
      type: "intent",
      intent: {
        sourceAccountId: intent.sourceAccountId as string,
        destinationAccountId: intent.destinationAccountId as string,
        currency: intent.currency as string,
        amountMinor: intent.amountMinor as string,
      },
    };
  } catch {
    return null;
  }
}
