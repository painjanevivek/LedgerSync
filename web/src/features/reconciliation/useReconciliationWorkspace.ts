"use client";

import { useCallback, useRef, useState } from "react";

import type { ReconciliationRun } from "@/features/accounts/types";
import {
  appendUniqueBy,
  beginEvidenceRequest,
  createEvidenceRequestCoordinator,
  finishEvidenceRequest,
  isEvidenceRequestCurrent,
} from "@/features/console/evidenceRequestCoordinator";
import { readJSON, unavailableMessage } from "@/lib/api/client";

type RunsPayload = Readonly<{ runs?: ReconciliationRun[]; next_cursor?: string }>;

export function useReconciliationWorkspace(detailRunId?: string) {
  const [runs, setRuns] = useState<ReconciliationRun[]>([]);
  const [detail, setDetail] = useState<ReconciliationRun | null>(null);
  const [listLoading, setListLoading] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [nextCursor, setNextCursor] = useState<string>();
  const [verifiedAt, setVerifiedAt] = useState<string>();
  const listRequests = useRef(createEvidenceRequestCoordinator());
  const detailRequests = useRef(createEvidenceRequestCoordinator());

  const loadList = useCallback(async (cursor?: string, append = false) => {
    const request = beginEvidenceRequest(listRequests.current, "reconciliation-runs", append ? "append" : "replace");
    if (!request) return;
    setListLoading(true);
    const response = await readJSON<RunsPayload>(`/api/reconciliation/runs?limit=25${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ""}`);
    if (!isEvidenceRequestCurrent(listRequests.current, request.token)) return;
    if (!response.ok) {
      setError(unavailableMessage(response.status, "authoritative reconciliation results", response.requestReference));
    } else {
      const items = Array.isArray(response.data.runs) ? response.data.runs : [];
      setRuns((current) => append ? appendUniqueBy(current, items, (run) => run.run_id) : items);
      setNextCursor(response.data.next_cursor || undefined);
      setVerifiedAt(new Date().toISOString());
      setError(null);
    }
    if (finishEvidenceRequest(listRequests.current, request.token)) setListLoading(false);
  }, []);

  const loadDetail = useCallback(async (runId: string) => {
    const request = beginEvidenceRequest(detailRequests.current, `reconciliation:${runId}`);
    if (!request) return;
    if (!request.sameResource) setDetail(null);
    setDetailLoading(true);
    const response = await readJSON<ReconciliationRun>(`/api/reconciliation/runs/${encodeURIComponent(runId)}`);
    if (!isEvidenceRequestCurrent(detailRequests.current, request.token)) return;
    if (response.ok && response.data.run_id) {
      setDetail(response.data);
      setError(null);
    } else {
      setError(unavailableMessage(response.status, "the selected reconciliation result", response.requestReference));
    }
    if (finishEvidenceRequest(detailRequests.current, request.token)) setDetailLoading(false);
  }, []);

  const observe = useCallback((run: ReconciliationRun) => {
    setRuns((current) => [run, ...current.filter((candidate) => candidate.run_id !== run.run_id)]);
    if (detailRunId === run.run_id) setDetail(run);
    setError(null);
  }, [detailRunId]);

  return { runs, detail, listLoading, detailLoading, error, nextCursor, verifiedAt, loadList, loadDetail, observe } as const;
}
