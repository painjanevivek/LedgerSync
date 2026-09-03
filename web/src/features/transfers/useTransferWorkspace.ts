"use client";

import { useCallback, useRef, useState } from "react";

import type { TransferDetail, TransferSummary } from "@/features/accounts/types";
import {
  beginEvidenceRequest,
  createEvidenceRequestCoordinator,
  finishEvidenceRequest,
  isEvidenceRequestCurrent,
} from "@/features/console/evidenceRequestCoordinator";
import { readJSON, unavailableMessage } from "@/lib/api/client";
import type { TransferExplainability } from "@/lib/api/orientation";
import { emptyTransferFilters, type TransferFilters } from "@/lib/page-query/transfers";

type TransfersPayload = Readonly<{
  transfers?: TransferSummary[];
  next_cursor?: string;
}>;

export function useTransferWorkspace(filters: TransferFilters = emptyTransferFilters) {
  const [transfers, setTransfers] = useState<TransferSummary[]>([]);
  const [detail, setDetail] = useState<TransferDetail | null>(null);
  const [explainability, setExplainability] = useState<TransferExplainability | null>(null);
  const [listLoading, setListLoading] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [explainabilityLoading, setExplainabilityLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [explainabilityError, setExplainabilityError] = useState<string | null>(null);
  const [nextCursor, setNextCursor] = useState<string>();
  const [verifiedAt, setVerifiedAt] = useState<string>();
  const listRequests = useRef(createEvidenceRequestCoordinator());
  const detailRequests = useRef(createEvidenceRequestCoordinator());
  const explainabilityRequests = useRef(createEvidenceRequestCoordinator());
  const loadList = useCallback(async () => {
    const parameters = new URLSearchParams({ limit: "25" });
    if (filters.cursor) parameters.set("cursor", filters.cursor);
    if (filters.query) parameters.set("q", filters.query);
    if (filters.accountId) parameters.set("accountId", filters.accountId);
    if (filters.status) parameters.set("status", filters.status);
    if (filters.from) parameters.set("from", filters.from);
    if (filters.to) parameters.set("to", filters.to);
    const request = beginEvidenceRequest(listRequests.current, `transfers:${parameters}`, "replace");
    if (!request) return;
    if (!request.sameResource) {
      setTransfers([]);
      setNextCursor(undefined);
      setVerifiedAt(undefined);
    }
    setListLoading(true);
    const response = await readJSON<TransfersPayload>(`/api/transfers?${parameters}`);
    if (!isEvidenceRequestCurrent(listRequests.current, request.token)) return;
    if (!response.ok) {
      setError(unavailableMessage(response.status, "transfer records", response.requestReference));
    } else {
      const items = Array.isArray(response.data.transfers) ? response.data.transfers : [];
      setTransfers(items);
      setNextCursor(response.data.next_cursor || undefined);
      setVerifiedAt(new Date().toISOString());
      setError(null);
    }
    if (finishEvidenceRequest(listRequests.current, request.token)) setListLoading(false);
  }, [filters.accountId, filters.cursor, filters.from, filters.query, filters.status, filters.to]);

  const loadDetail = useCallback(async (transferId: string) => {
    const request = beginEvidenceRequest(detailRequests.current, `transfer:${transferId}`);
    if (!request) return;
    if (!request.sameResource) setDetail(null);
    setDetailLoading(true);
    const response = await readJSON<TransferDetail>(`/api/transfers/${encodeURIComponent(transferId)}`);
    if (!isEvidenceRequestCurrent(detailRequests.current, request.token)) return;
    if (response.ok && response.data.transfer_id) {
      setDetail(response.data);
      setError(null);
    } else {
      setError(unavailableMessage(response.status, "transfer details", response.requestReference));
    }
    if (finishEvidenceRequest(detailRequests.current, request.token)) setDetailLoading(false);
  }, []);

  const loadExplainability = useCallback(async (transferId: string) => {
    const request = beginEvidenceRequest(explainabilityRequests.current, `explainability:${transferId}`);
    if (!request) return;
    if (!request.sameResource) setExplainability(null);
    setExplainabilityLoading(true);
    const response = await readJSON<TransferExplainability>(`/api/transfers/${encodeURIComponent(transferId)}/explainability`);
    if (!isEvidenceRequestCurrent(explainabilityRequests.current, request.token)) return;
    if (response.ok && Array.isArray(response.data.stages)) {
      setExplainability(response.data);
      setExplainabilityError(null);
    } else {
      setExplainabilityError(
        response.status === 404
          ? `The linked timeline was not found in this authorized tenant scope. Request reference: ${response.requestReference}.`
          : unavailableMessage(response.status, "stored evidence timeline", response.requestReference),
      );
    }
    if (finishEvidenceRequest(explainabilityRequests.current, request.token)) setExplainabilityLoading(false);
  }, []);

  return {
    transfers,
    detail,
    explainability,
    listLoading,
    detailLoading,
    explainabilityLoading,
    error,
    explainabilityError,
    nextCursor,
    verifiedAt,
    loadList,
    loadDetail,
    loadExplainability,
  } as const;
}
