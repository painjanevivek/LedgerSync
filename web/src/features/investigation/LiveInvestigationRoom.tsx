"use client";

import Link from "next/link";
import { type FormEvent, useCallback, useEffect, useRef, useState } from "react";

import { findingAllowed, parseLiveRoom, parseLiveSignalPage, parseTransferRequestStatus, type LiveFindingCode, type LiveRoom, type TransferRequestStatus } from "@/lib/api/live-investigation";
import { liveChannelName, maximumLiveMessages, parsePeerEnvelope, signalNegotiationID, validLiveMessage, type SharedIntent } from "@/features/investigation/liveRoomProtocol";
import { CopyControl } from "@/ui/controls/CopyControl.client";
import { Money } from "@/ui/display/Money";
import { StatePanel } from "@/ui/display/StatePanel";
import { Timestamp } from "@/ui/display/Timestamp";

type ChatMessage = Readonly<{ id: string; role: "owner" | "peer"; text: string; receivedAt: string }>;
type Props = Readonly<{ csrfToken: string; requestReference?: string; intent?: SharedIntent; inviteToken?: string; existingRoomId?: string }>;

function commandHeaders(csrfToken: string, idempotency = false): HeadersInit {
  return { "Content-Type": "application/json", "X-CSRF-Token": csrfToken, ...(idempotency ? { "Idempotency-Key": crypto.randomUUID() } : {}) };
}

export function LiveInvestigationRoom({ csrfToken, requestReference, intent, inviteToken, existingRoomId }: Props) {
  const [requestStatus, setRequestStatus] = useState<TransferRequestStatus | null>(null);
  const [room, setRoom] = useState<LiveRoom | null>(null);
  const [inviteURL, setInviteURL] = useState("");
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [sharedIntent, setSharedIntent] = useState<SharedIntent | null>(intent ?? null);
  const [connectionState, setConnectionState] = useState<"not-connected" | "connecting" | "connected" | "disconnected">("not-connected");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [draft, setDraft] = useState("");
  const [finding, setFinding] = useState<LiveFindingCode>("still_unresolved");
  const channelRef = useRef<RTCDataChannel | null>(null);
  const signalSequence = useRef(0);
  const signalCursor = useRef("0-0");
  const negotiationID = useRef("");
  const processedOfferID = useRef("");
  const sentIntent = useRef(false);
  const activeRoomID = room?.room_id;
  const activeRoomRole = room?.participant_role;
  const activeRoomStatus = room?.status;

  const checkStatus = useCallback(async () => {
    if (!requestReference) return;
    setBusy(true); setError(null);
    try {
      const response = await fetch(`/api/investigation/live/status/${encodeURIComponent(requestReference)}`, { cache: "no-store" });
      const parsed = parseTransferRequestStatus(await response.json().catch(() => null));
      if (!response.ok || !parsed) throw new Error("status_failed");
      setRequestStatus(parsed);
    } catch { setError("The transfer status could not be checked. This does not mean the transfer failed; use the normal status check before taking another action."); }
    finally { setBusy(false); }
  }, [requestReference]);

  const startRoom = useCallback(async () => {
    if (!requestReference || requestStatus?.status !== "in_progress") return;
    setBusy(true); setError(null);
    try {
      const response = await fetch("/api/investigation/live/rooms", { method: "POST", headers: commandHeaders(csrfToken, true), body: JSON.stringify({ request_reference: requestReference }) });
      const parsed = parseLiveRoom(await response.json().catch(() => null));
      if (!response.ok || !parsed) throw new Error("room_start_failed");
      setRoom(parsed); setConnectionState("connecting");
    } catch { setError("Live collaboration could not start. The transfer remains unchanged; continue with the normal status and safe-retry controls."); }
    finally { setBusy(false); }
  }, [csrfToken, requestReference, requestStatus?.status]);

  useEffect(() => {
    if (!inviteToken || room) return;
    let cancelled = false;
    void (async () => {
      setBusy(true); setError(null);
      try {
        const response = await fetch("/api/investigation/live/rooms/join", { method: "POST", headers: commandHeaders(csrfToken, true), body: JSON.stringify({ invite_token: inviteToken }) });
        const parsed = parseLiveRoom(await response.json().catch(() => null));
        if (!response.ok || !parsed) throw new Error("room_join_failed");
        if (!cancelled) { setRoom(parsed); setConnectionState("connecting"); }
      } catch { if (!cancelled) setError("This invitation is unavailable, expired, already used, or not authorized for this workspace."); }
      finally { if (!cancelled) setBusy(false); }
    })();
    return () => { cancelled = true; };
  }, [csrfToken, inviteToken, room]);

  useEffect(() => {
    if (!existingRoomId || inviteToken || room) return;
    let cancelled = false;
    void (async () => {
      setBusy(true); setError(null);
      try {
        const response = await fetch(`/api/investigation/live/rooms/${encodeURIComponent(existingRoomId)}`, { cache: "no-store" });
        const parsed = parseLiveRoom(await response.json().catch(() => null));
        if (!response.ok || !parsed) throw new Error("room_read_failed");
        if (!cancelled) { setRoom(parsed); setConnectionState("connecting"); }
      } catch { if (!cancelled) setError("This room is unavailable, expired, ended, or no longer authorized for this operator."); }
      finally { if (!cancelled) setBusy(false); }
    })();
    return () => { cancelled = true; };
  }, [existingRoomId, inviteToken, room]);

  const sendSignal = useCallback(async (roomId: string, kind: "offer" | "answer" | "ice" | "end", payload: unknown) => {
    // A reconnect starts a fresh browser process but Redis still retains the
    // previous per-participant sequence. Initialize lazily during this event,
    // not during render, and stay within Number.MAX_SAFE_INTEGER.
    if (signalSequence.current === 0) {
      signalSequence.current = Date.now() * 1_000 + crypto.getRandomValues(new Uint16Array(1))[0];
    }
    signalSequence.current += 1;
    const response = await fetch(`/api/investigation/live/rooms/${encodeURIComponent(roomId)}/signals`, { method: "POST", headers: commandHeaders(csrfToken), body: JSON.stringify({ sequence: signalSequence.current, kind, payload }) });
    if (!response.ok) throw new Error("signal_failed");
  }, [csrfToken]);

  const configureChannel = useCallback((channel: RTCDataChannel, role: "owner" | "peer") => {
    channelRef.current = channel;
    channel.onopen = () => {
      setConnectionState("connected");
      if (role === "owner" && intent && !sentIntent.current) { channel.send(JSON.stringify({ version: 1, type: "intent", intent })); sentIntent.current = true; }
    };
    channel.onclose = () => setConnectionState("disconnected");
    channel.onerror = () => setConnectionState("disconnected");
    channel.onmessage = (event) => {
      const value = parsePeerEnvelope(event.data, role);
      if (!value) return;
      if (value.type === "message") {
        const peerRole: "owner" | "peer" = role === "owner" ? "peer" : "owner";
        setMessages((current) => [...current, { id: value.id, role: peerRole, text: value.text, receivedAt: new Date().toISOString() }].slice(-maximumLiveMessages));
      } else {
        setSharedIntent(value.intent);
      }
    };
  }, [intent]);

  useEffect(() => {
    if (!activeRoomID || activeRoomStatus === "ended" || activeRoomStatus === "expired") return;
    const roomId = activeRoomID;
    let disposed = false;
    const refresh = async () => {
      try {
        const response = await fetch(`/api/investigation/live/rooms/${encodeURIComponent(roomId)}`, { cache: "no-store" });
        const current = parseLiveRoom(await response.json().catch(() => null));
        if (!response.ok || !current) throw new Error("room_refresh_failed");
        if (!disposed) setRoom(current);
      } catch {
        if (!disposed) setError("Live room evidence could not be refreshed. This does not change or conceal the transfer result.");
      }
    };
    void refresh();
    const refreshTimer = window.setInterval(() => void refresh(), 5_000);
    return () => { disposed = true; window.clearInterval(refreshTimer); };
  }, [activeRoomID, activeRoomStatus]);

  useEffect(() => {
    if (!activeRoomID || !activeRoomRole || activeRoomStatus === "ended" || activeRoomStatus === "expired") return;
    const roomId = activeRoomID; const role = activeRoomRole;
    let disposed = false;
    const peer = new RTCPeerConnection({ iceServers: [] });
    negotiationID.current = role === "owner" ? crypto.randomUUID() : "";
    processedOfferID.current = "";
    peer.onconnectionstatechange = () => {
      if (["failed", "disconnected", "closed"].includes(peer.connectionState)) setConnectionState("disconnected");
      else if (peer.connectionState === "connected") setConnectionState("connected");
    };
    peer.onicecandidate = (event) => {
      if (!event.candidate || !negotiationID.current) return;
      void sendSignal(roomId, "ice", { ...event.candidate.toJSON(), negotiation_id: negotiationID.current }).catch(() => setError("Live signalling was interrupted. The transfer status is unchanged."));
    };
    if (role === "peer") peer.ondatachannel = (event) => event.channel.label === liveChannelName && configureChannel(event.channel, role);
    let ownerChannel: RTCDataChannel | null = null;
    void (async () => {
      if (role !== "owner") return;
      ownerChannel = peer.createDataChannel(liveChannelName, { ordered: true }); configureChannel(ownerChannel, role);
      const offer = await peer.createOffer(); await peer.setLocalDescription(offer); await sendSignal(roomId, "offer", { type: offer.type, sdp: offer.sdp, negotiation_id: negotiationID.current });
    })().catch(() => setError("The encrypted text channel could not be prepared. The transfer status is unchanged."));
    const poll = async () => {
      try {
        const response = await fetch(`/api/investigation/live/rooms/${encodeURIComponent(roomId)}/signals?cursor=${encodeURIComponent(signalCursor.current)}`, { cache: "no-store" });
        const page = parseLiveSignalPage(await response.json().catch(() => null));
        if (!response.ok || !page) throw new Error("signal_read_failed");
        signalCursor.current = page.cursor;
        const incoming = page.signals.filter((signal) => !disposed && signal.role !== role);
        if (role === "peer") {
          const latestOffer = incoming.filter((signal) => signal.kind === "offer" && signalNegotiationID(signal.payload)).at(-1);
          if (latestOffer && latestOffer.id !== processedOfferID.current) {
            const nextNegotiationID = signalNegotiationID(latestOffer.payload);
            await peer.setRemoteDescription(latestOffer.payload as RTCSessionDescriptionInit);
            negotiationID.current = nextNegotiationID;
            processedOfferID.current = latestOffer.id;
            const answer = await peer.createAnswer();
            await peer.setLocalDescription(answer);
            await sendSignal(roomId, "answer", { type: answer.type, sdp: answer.sdp, negotiation_id: nextNegotiationID });
          }
        } else if (!peer.remoteDescription) {
          const latestAnswer = incoming.filter((signal) => signal.kind === "answer" && signalNegotiationID(signal.payload) === negotiationID.current).at(-1);
          if (latestAnswer) await peer.setRemoteDescription(latestAnswer.payload as RTCSessionDescriptionInit);
        }
        for (const signal of incoming) {
          if (signal.kind === "ice" && peer.remoteDescription && signalNegotiationID(signal.payload) === negotiationID.current) {
            await peer.addIceCandidate(signal.payload as RTCIceCandidateInit);
          } else if (signal.kind === "end") {
            peer.close(); setConnectionState("disconnected");
          }
        }
      } catch { if (!disposed) setError("Live signalling is unavailable. This does not change or conceal the transfer result."); }
    };
    void poll(); const signalTimer = window.setInterval(() => void poll(), 1000);
    const presence = () => void fetch(`/api/investigation/live/rooms/${encodeURIComponent(roomId)}/presence`, { method: "POST", headers: commandHeaders(csrfToken), body: "{}" }).catch(() => undefined);
    presence(); const presenceTimer = window.setInterval(presence, 10_000);
    return () => { disposed = true; window.clearInterval(signalTimer); window.clearInterval(presenceTimer); ownerChannel?.close(); peer.close(); channelRef.current = null; };
  }, [configureChannel, csrfToken, activeRoomID, activeRoomRole, activeRoomStatus, sendSignal]);

  useEffect(() => {
    if (!activeRoomID) return;
    const timer = window.setInterval(() => void (async () => { const response = await fetch(`/api/investigation/live/rooms/${encodeURIComponent(activeRoomID)}`, { cache: "no-store" }).catch(() => null); if (!response?.ok) return; const current = parseLiveRoom(await response.json().catch(() => null)); if (current) setRoom(current); })(), 5000);
    return () => window.clearInterval(timer);
  }, [activeRoomID]);

  async function issueInvite() {
    if (!room) return; setBusy(true); setError(null);
    try { const response = await fetch(`/api/investigation/live/rooms/${room.room_id}/invite`, { method: "POST", headers: commandHeaders(csrfToken), body: "{}" }); const value = await response.json() as { invite_token?: string }; if (!response.ok || typeof value.invite_token !== "string") throw new Error("invite_failed"); setInviteURL(`${window.location.origin}/investigations/live/${room.room_id}#invite=${encodeURIComponent(value.invite_token)}`); }
    catch { setError("A single-use invitation could not be created. No room membership changed."); } finally { setBusy(false); }
  }

  function sendMessage(event: FormEvent) {
    event.preventDefault(); const text = draft.trim(); const channel = channelRef.current;
    if (!text || !channel || channel.readyState !== "open" || !room) return;
    if (!validLiveMessage(text)) { setError("Messages must be plain text under 2 KiB and cannot contain URLs."); return; }
    const id = crypto.randomUUID(); channel.send(JSON.stringify({ version: 1, type: "message", id, text }));
    setMessages((current) => [...current, { id, role: room.participant_role, text, receivedAt: new Date().toISOString() }].slice(-maximumLiveMessages)); setDraft("");
  }

  async function endOrLeave(end: boolean) {
    if (!room) return; setBusy(true); setError(null);
    try { if (end) await sendSignal(room.room_id, "end", {}).catch(() => undefined); const response = await fetch(`/api/investigation/live/rooms/${room.room_id}/${end ? "end" : "leave"}`, { method: "POST", headers: commandHeaders(csrfToken, end), body: JSON.stringify({ expected_version: room.version }) }); if (!response.ok) throw new Error("room_mutation_failed"); setConnectionState("disconnected"); if (end) setRoom((current) => current ? { ...current, status: "ended", peer_present: false } : current); }
    catch { setError(`The room could not be ${end ? "ended" : "left"}. Refresh its current state before retrying.`); } finally { setBusy(false); }
  }

  async function recordFinding() {
    if (!room || room.participant_role !== "owner" || !findingAllowed(finding, room.authoritative_request_state, room.transfer_id)) return;
    setBusy(true); setError(null);
    try { const response = await fetch(`/api/investigation/live/rooms/${room.room_id}/finding`, { method: "POST", headers: commandHeaders(csrfToken, true), body: JSON.stringify({ expected_version: room.version, code: finding }) }); const parsed = parseLiveRoom(await response.json().catch(() => null)); if (!response.ok || !parsed) throw new Error("finding_failed"); setRoom(parsed); }
    catch { setError("The finding was not recorded because it no longer matches the authoritative request state, or the room changed."); } finally { setBusy(false); }
  }

  if (!room) return <section className="live-investigation-entry" aria-labelledby="live-investigation-entry-heading"><h3 id="live-investigation-entry-heading">Investigate with another operator</h3><p>First check the authoritative request status. If it is still unresolved, two authorized operators can compare evidence in an ephemeral text room.</p><div className="action-row">{requestReference && <button className="button secondary" type="button" disabled={busy} onClick={() => void checkStatus()}>{busy ? "Checking…" : "Check status"}</button>}{requestStatus?.status === "in_progress" && <button className="button secondary" type="button" disabled={busy} onClick={() => void startRoom()}>Start investigation</button>}</div>{requestStatus && <p role="status"><strong>Authoritative request state:</strong> {requestStatus.status.replaceAll("_", " ")}{requestStatus.transfer_id && <> · <Link href={`/transfers/${requestStatus.transfer_id}`}>Open transfer</Link></>}</p>}{inviteToken && busy && <p role="status">Redeeming the single-use invitation…</p>}{error && <StatePanel kind="error" title="Collaboration unavailable" message={error} />}</section>;

  const lifecycleClosed = room.status === "ended" || room.status === "expired";
  const displayedConnection = lifecycleClosed ? room.status : connectionState.replace("-", " ");
  return <section className="live-investigation-room" aria-labelledby="live-room-heading"><header><div><p className="eyebrow">Read-only investigation aid</p><h2 id="live-room-heading">Uncertain transfer room</h2></div><span className={`live-connection ${lifecycleClosed ? "disconnected" : connectionState}`} role="status">{displayedConnection}</span></header><StatePanel kind="unknown" title="Financial result remains server-controlled" message="Messages in this room cannot approve, retry, correct, or move money. Use only the normal LedgerSync controls for financial actions." /><div className="live-room-grid"><section aria-labelledby="participants-heading"><h3 id="participants-heading">Participants</h3><ul className="live-participants"><li><strong>Initiating operator</strong><span>{room.participant_role === "owner" || room.peer_bound ? "Joined" : "Waiting"}</span></li><li><strong>Invited operator</strong><span>{room.peer_present ? "Connected" : room.peer_bound ? "Joined · offline" : "Waiting for invite"}</span></li></ul>{room.participant_role === "owner" && !room.peer_bound && !lifecycleClosed && <button className="button secondary" type="button" disabled={busy} onClick={() => void issueInvite()}>Create single-use invite</button>}{inviteURL && !lifecycleClosed && <div className="live-invite"><p>Expires in 10 minutes and can be redeemed once.</p><CopyControl value={inviteURL} label="Copy single-use invitation" /></div>}</section><section aria-labelledby="request-evidence-heading"><h3 id="request-evidence-heading">Request evidence</h3><dl className="evidence-list"><div><dt>Request reference</dt><dd><CopyControl value={room.request_reference} /></dd></div><div><dt>Authoritative state</dt><dd>{room.authoritative_request_state.replaceAll("_", " ")}</dd></div><div><dt>Room expires</dt><dd><Timestamp value={room.expires_at} /></dd></div>{room.transfer_id && <div><dt>Confirmed transfer</dt><dd><Link href={`/transfers/${room.transfer_id}`}>{room.transfer_id}</Link></dd></div>}</dl></section></div>{sharedIntent && <section className="live-intent" aria-labelledby="submitted-intent-heading"><h3 id="submitted-intent-heading">Submitted intent</h3><p>Shared by the initiating operator—not a confirmed ledger result.</p><dl className="review-grid"><div><dt>From</dt><dd>{sharedIntent.sourceAccountId}</dd></div><div><dt>To</dt><dd>{sharedIntent.destinationAccountId}</dd></div><div><dt>Amount</dt><dd><Money currency={sharedIntent.currency} minorUnits={sharedIntent.amountMinor} /></dd></div></dl></section>}<section className="live-chat" aria-labelledby="live-chat-heading"><h3 id="live-chat-heading">Ephemeral messages</h3><p className="muted">Messages disappear on reload and are not stored by LedgerSync. Do not share secrets, credentials, or idempotency keys.</p><ol aria-live="polite">{messages.length === 0 ? <li className="live-chat-empty">No messages in this browser session.</li> : messages.map((message) => <li key={message.id} className={message.role === room.participant_role ? "mine" : "theirs"}><strong>{message.role === room.participant_role ? "You" : "Other operator"}</strong><p>{message.text}</p></li>)}</ol><form onSubmit={sendMessage}><label htmlFor="live-message">Message</label><textarea id="live-message" value={draft} onChange={(event) => setDraft(event.target.value)} maxLength={2048} disabled={connectionState !== "connected"} /><button className="button primary" type="submit" disabled={connectionState !== "connected" || !draft.trim()}>Send message</button></form></section>{room.participant_role === "owner" && !room.finding && !lifecycleClosed && <section className="live-finding" aria-labelledby="finding-heading"><h3 id="finding-heading">Final finding</h3><p>The available codes are constrained by the latest authoritative request state.</p><label htmlFor="live-finding-code">Finding</label><select id="live-finding-code" value={finding} onChange={(event) => setFinding(event.target.value as LiveFindingCode)}><option value="still_unresolved">Still unresolved</option><option value="escalated">Escalated</option><option value="confirmed_completed">Confirmed completed</option><option value="confirmed_rejected">Confirmed rejected</option></select><button className="button secondary" type="button" disabled={busy || !findingAllowed(finding, room.authoritative_request_state, room.transfer_id)} onClick={() => void recordFinding()}>Record audited finding</button></section>}{room.finding && <StatePanel title="Finding recorded" message={`${room.finding.code.replaceAll("_", " ")} · authoritative state ${room.finding.authoritative_request_state.replaceAll("_", " ")}.`} />}{error && <StatePanel kind="error" title="Live collaboration interrupted" message={error} />}{lifecycleClosed ? <StatePanel title={`Room ${room.status}`} message="The collaboration channel is closed. This did not change the transfer result or the recorded finding." /> : <div className="action-row"><button className="button secondary" type="button" disabled={busy} onClick={() => void endOrLeave(room.participant_role === "owner")}>{room.participant_role === "owner" ? "End room" : "Leave room"}</button></div>}</section>;
}
