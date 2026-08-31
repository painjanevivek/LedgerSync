"use client";

import { useCallback, useRef, useState } from "react";

import type { ReconciliationRun } from "@/features/accounts/types";
import {
  beginEvidenceRequest,
  createEvidenceRequestCoordinator,
  finishEvidenceRequest,
  isEvidenceRequestCurrent,
} from "@/features/console/evidenceRequestCoordinator";
import { readJSON, unavailableMessage } from "@/lib/api/client";
import type { ReconciliationFilters } from "@/lib/page-query/reconciliation";

type RunsPayload = Readonly<{ runs?: ReconciliationRun[]; next_cursor?: string }>;

export function useReconciliationWorkspace(detailRunId?: string, filters: ReconciliationFilters = {}) {
  const [runs, setRuns] = useState<ReconciliationRun[]>([]);
  const [detail, setDetail] = useState<ReconciliationRun | null>(null);
  const [listLoading, setListLoading] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [nextCursor, setNextCursor] = useState<string>();
  const [verifiedAt, setVerifiedAt] = useState<string>();
  const listRequests = useRef(createEvidenceRequestCoordinator());
  const detailRequests = useRef(createEvidenceRequestCoordinator());

  const loadList = useCallback(async () => {
    const query = new URLSearchParams({ limit: "25" });
    if (filters.cursor) query.set("cursor", filters.cursor);
    const request = beginEvidenceRequest(listRequests.current, `reconciliation-runs:${query}`, "replace");
    if (!request) return;
    setListLoading(true);
    const response = await readJSON<RunsPayload>(`/api/reconciliation/runs?${query}`);
    if (!isEvidenceRequestCurrent(listRequests.current, request.token)) return;
    if (!response.ok) {
      setError(unavailableMessage(response.status, "authoritative reconciliation results", response.requestReference));
    } else {
      const items = Array.isArray(response.data.runs) ? response.data.runs : [];
      setRuns(items);
      setNextCursor(response.data.next_cursor || undefined);
      setVerifiedAt(new Date().toISOString());
      setError(null);
    }
    if (finishEvidenceRequest(listRequests.current, request.token)) setListLoading(false);
  }, [filters.cursor]);

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
