"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useConsoleSession } from "@/features/console/ConsoleSessionBoundary";
import { beginEvidenceRequest, createEvidenceRequestCoordinator, finishEvidenceRequest, invalidateEvidenceRequests, isEvidenceRequestCurrent } from "@/features/console/evidenceRequestCoordinator";
import { readJSON, unavailableMessage } from "@/lib/api/client";
import type { CorrectionPage, CorrectionFilters, CorrectionSubmission, TransferCorrection } from "@/lib/api/corrections";

// Own evidence requests and command state separately from the console view.
export function useCorrectionsWorkspace({ correctionId, filters, invalidQuery }: Readonly<{ correctionId?: string; filters: CorrectionFilters; invalidQuery: boolean }>) {
  const boundary = useConsoleSession();
  const { session, online, hasScope } = boundary;
  const [events, setEvents] = useState<TransferCorrection[]>([]);
  const [selected, setSelected] = useState<TransferCorrection | null>(null);
  const [nextCursor, setNextCursor] = useState<string>();
  const [loading, setLoading] = useState(false);
  const [actionBusy, setActionBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [verifiedAt, setVerifiedAt] = useState<string>();
  const [decisionReason, setDecisionReason] = useState("");
  const [stepUpRequired, setStepUpRequired] = useState(false);
  const listRequests = useRef(createEvidenceRequestCoordinator());
  const detailRequests = useRef(createEvidenceRequestCoordinator());

  const loadList = useCallback(
    async () => {
      const query = new URLSearchParams({ limit: "25" });
      if (filters.status) query.set("status", filters.status);
      if (filters.cursor) query.set("cursor", filters.cursor);
      const request = beginEvidenceRequest(listRequests.current, `corrections:list:${query}`, "replace");
      if (!request) return;
      if (!request.sameResource) {
        setEvents([]);
        setNextCursor(undefined);
        setVerifiedAt(undefined);
      }
      setLoading(true);
      setError(null);
      const response = await readJSON<CorrectionPage>(
        `/api/transfer-corrections?${query}`,
      );
      if (!isEvidenceRequestCurrent(listRequests.current, request.token)) return;
      if (response.ok && Array.isArray(response.data.events)) {
        setEvents(response.data.events);
        setNextCursor(response.data.next_cursor || undefined);
        setVerifiedAt(new Date().toISOString());
      } else
        setError(
          unavailableMessage(
            response.status,
            "transfer correction evidence",
            response.requestReference,
          ),
        );
      if (finishEvidenceRequest(listRequests.current, request.token)) setLoading(false);
    },
    [filters.cursor, filters.status],
  );

  const loadEvent = useCallback(async () => {
    if (!correctionId) return null;
    const request = beginEvidenceRequest(detailRequests.current, `correction:${correctionId}`);
    if (!request) return null;
    if (!request.sameResource) { setSelected(null); setVerifiedAt(undefined); }
    setLoading(true);
    setError(null);
    const response = await readJSON<TransferCorrection>(
      `/api/transfer-corrections/${encodeURIComponent(correctionId)}`,
    );
    if (!isEvidenceRequestCurrent(detailRequests.current, request.token)) return null;
    if (response.ok && response.data.correction_id) {
      setSelected(response.data);
      setVerifiedAt(new Date().toISOString());
      setLoading(false);
      return response.data;
    } else {
      setError(
        response.status === 404
          ? `The correction record was not found in this authorized tenant scope. Request reference: ${response.requestReference}.`
          : unavailableMessage(
              response.status,
              "transfer correction evidence",
              response.requestReference,
            ),
      );
    }
    if (finishEvidenceRequest(detailRequests.current, request.token)) setLoading(false);
    return null;
  }, [correctionId]);

  useEffect(() => {
    if (!session || !online || !hasScope("corrections:read") || invalidQuery)
      return;
    const listCoordinator = listRequests.current;
    const detailCoordinator = detailRequests.current;
    const timer = window.setTimeout(
      () => void (correctionId ? loadEvent() : loadList()),
      0,
    );
    return () => {
      window.clearTimeout(timer);
      invalidateEvidenceRequests(listCoordinator);
      invalidateEvidenceRequests(detailCoordinator);
    };
  }, [correctionId, hasScope, invalidQuery, loadEvent, loadList, online, session]);
  async function act(action: "approve" | "reject" | "cancel") {
    if (!session || !selected) return;
    setActionBusy(true);
    setError(null);
    setStepUpRequired(false);
    try {
      const headers: Record<string, string> = {
        "X-CSRF-Token": session.csrf_token,
        "Content-Type": "application/json",
      };
      const response = await fetch(
        `/api/transfer-corrections/${encodeURIComponent(selected.correction_id)}/${action}`,
        {
          method: "POST",
          headers,
          body: JSON.stringify({ reason: decisionReason }),
        },
      );
      const payload = (await response.json().catch(() => ({}))) as
        | TransferCorrection
        | CorrectionSubmission
        | { error?: { code?: string } };
      if (!response.ok) {
        const code = "error" in payload ? payload.error?.code : undefined;
        if (response.status === 428 || code === "step_up_required")
          setStepUpRequired(true);
        throw new Error(
          response.status === 504
            ? "The command outcome is unknown. Refresh this record before any retry."
            : code === "step_up_required"
              ? "Recent authentication is required before this command can be authorized."
              : `The correction command was not recorded (${code ?? response.status}).`,
        );
      }
      const event =
        "event" in payload ? payload.event : (payload as TransferCorrection);
      if (event.correction_id) setSelected(event);
      else await loadEvent();
      setDecisionReason("");
      setVerifiedAt(new Date().toISOString());
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : "The correction command could not be verified.",
      );
    } finally {
      setActionBusy(false);
    }
  }

  return { ...boundary, events, selected, nextCursor, loading, actionBusy, error, verifiedAt, decisionReason, setDecisionReason, stepUpRequired, loadList, loadEvent, act };
}
