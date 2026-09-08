"use client";

import { useEffect, useState } from "react";
import { useConsoleSession } from "@/features/console/ConsoleSessionBoundary";
import { LiveInvestigationRoom } from "@/features/investigation/LiveInvestigationRoom";
import { PageHeader } from "@/ui/display/PageHeader";
import { StatePanel } from "@/ui/display/StatePanel";

export function LiveInvestigationJoin({ roomId }: Readonly<{ roomId?: string }>) {
  const { session } = useConsoleSession(); const [invite, setInvite] = useState<string | null>(null);
  useEffect(() => { const parameters = new URLSearchParams(window.location.hash.slice(1)); const token = parameters.get("invite"); window.history.replaceState(null, "", `${window.location.pathname}${window.location.search}`); const timer = window.setTimeout(() => setInvite(token && /^[A-Za-z0-9_-]{32,128}$/u.test(token) ? token : ""), 0); return () => window.clearTimeout(timer); }, []);
  return <><PageHeader eyebrow="Investigation / Live room" title="Join transfer investigation" description="Compare an uncertain transfer request with one other authorized operator. Financial commands remain outside this room." />{!session || invite === null ? <StatePanel title="Preparing secure room entry" message="The invitation is kept only in memory and removed from the address bar before redemption." /> : invite === "" && !roomId ? <StatePanel kind="error" title="Invitation missing or invalid" message="Ask the initiating operator for a new single-use invitation. No transfer status changed." /> : <LiveInvestigationRoom csrfToken={session.csrf_token} inviteToken={invite || undefined} existingRoomId={roomId} />}</>;
}
