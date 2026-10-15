"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { readJSON, unavailableMessage } from "@/lib/api/client";
import { parseLiveRoom, type LiveRoom } from "@/lib/api/live-investigation";
import { StatePanel } from "@/ui/display/StatePanel";
import { Timestamp } from "@/ui/display/Timestamp";

export function LiveInvestigationHistory({ roomId }: Readonly<{ roomId: string }>) {
  const [room, setRoom] = useState<LiveRoom | null>(null);
  const [error, setError] = useState<string | null>(null);
  useEffect(() => {
    let active = true;
    void readJSON<unknown>(`/api/investigation/live/rooms/${encodeURIComponent(roomId)}`).then(result => {
      if (!active) return;
      const value = result.ok ? parseLiveRoom(result.data) : null;
      if (value) setRoom(value);
      else setError(result.ok ? "The retained room record is unavailable." : unavailableMessage(result.status, "room history", result.requestReference));
    });
    return () => { active = false; };
  }, [roomId]);
  return <section aria-labelledby="retained-room-heading">
    <h2 id="retained-room-heading">Retained room history</h2>
    <p>Live collaboration is disabled. This view reads stored evidence only. It cannot join a room, send signals, change presence, or record a new finding.</p>
    {error ? <StatePanel kind="error" title="Room history unavailable" message={error} /> : !room ? <StatePanel title="Loading authorized room history" message="Checking access to the retained record." /> : <dl className="evidence-list">
      <div><dt>Stored room status</dt><dd>{room.status}</dd></div>
      <div><dt>Recorded expiry</dt><dd><Timestamp value={room.expires_at} /></dd></div>
      <div><dt>Request state at this read</dt><dd>{room.authoritative_request_state.replaceAll("_", " ")}</dd></div>
      <div><dt>Stored finding</dt><dd>{room.finding ? <>{room.finding.code.replaceAll("_", " ")} · <Timestamp value={room.finding.recorded_at} /></> : "No finding was recorded."}</dd></div>
    </dl>}
    <Link href="/transfers">Open transfer recovery</Link>
  </section>;
}
