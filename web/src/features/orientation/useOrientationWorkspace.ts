"use client";

import { useCallback, useRef, useState } from "react";

import type { ConsoleSession } from "@/features/accounts/types";
import {
  beginEvidenceRequest,
  createEvidenceRequestCoordinator,
  finishEvidenceRequest,
  isEvidenceRequestCurrent,
} from "@/features/console/evidenceRequestCoordinator";
import { readJSON, unavailableMessage, writeJSON } from "@/lib/api/client";
import type { LocalOrientation, OperatorPreferenceStepID } from "@/lib/api/orientation";

export function useOrientationWorkspace(session: ConsoleSession | null) {
  const [evidence, setEvidence] = useState<LocalOrientation | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [preferenceSaving, setPreferenceSaving] = useState(false);
  const [preferenceError, setPreferenceError] = useState<string | null>(null);
  const requests = useRef(createEvidenceRequestCoordinator());
  const preferenceInFlight = useRef(false);

  const load = useCallback(async () => {
    const request = beginEvidenceRequest(requests.current, "local-orientation");
    if (!request) return;
    setLoading(true);
    const response = await readJSON<LocalOrientation>("/api/local/orientation");
    if (!isEvidenceRequestCurrent(requests.current, request.token)) return;
    if (response.ok && Array.isArray(response.data.steps)) {
      setEvidence(response.data);
      setError(null);
    } else {
      setError(unavailableMessage(response.status, "local orientation evidence", response.requestReference));
    }
    if (finishEvidenceRequest(requests.current, request.token)) setLoading(false);
  }, []);

  const updatePreferences = useCallback(async (change: Readonly<{
    dismissed: boolean;
    completedStepIDs: OperatorPreferenceStepID[];
  }>) => {
    if (!session || !evidence || preferenceInFlight.current) return false;
    preferenceInFlight.current = true;
    setPreferenceSaving(true);
    setPreferenceError(null);
    const response = await writeJSON<LocalOrientation>(
      "/api/local/orientation/preferences",
      "PUT",
      session.csrf_token,
      {
        expected_version: evidence.preference_version,
        dismissed: change.dismissed,
        completed_step_ids: change.completedStepIDs,
      },
    );
    if (response.ok && Array.isArray(response.data.steps)) {
      setEvidence(response.data);
      setError(null);
    } else {
      const responseUnknown = response.status === 0 || response.errorCode === "upstream_timeout" || response.errorCode === "temporary_unavailable";
      if (responseUnknown || response.status === 409) await load();
      const action = response.status === 409
        ? "The preference changed in another session, so the latest server state was loaded."
        : responseUnknown
          ? "The response was unknown, so current server state was refreshed without assuming the change succeeded."
          : "The previous server-owned preference was preserved.";
      setPreferenceError(`${action} Request reference: ${response.requestReference}.`);
    }
    preferenceInFlight.current = false;
    setPreferenceSaving(false);
    return response.ok;
  }, [evidence, load, session]);

  return { evidence, loading, error, preferenceSaving, preferenceError, load, updatePreferences } as const;
}
