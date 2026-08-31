"use client";

import { useCallback, useRef, useState } from "react";

import type { TransferDetail, TransferSummary } from "@/features/accounts/types";
import {
  appendUniqueBy,
  beginEvidenceRequest,
  createEvidenceRequestCoordinator,
  finishEvidenceRequest,
  isEvidenceRequestCurrent,
} from "@/features/console/evidenceRequestCoordinator";
import { readJSON, unavailableMessage } from "@/lib/api/client";
import type { TransferExplainability } from "@/lib/api/orientation";

type TransfersPayload = Readonly<{
  transfers?: TransferSummary[];
  next_cursor?: string;
}>;

export function useTransferWorkspace(filters?: Readonly<{ query: string; status: string }>) {
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
  const query = filters?.query ?? "";
  const status = filters?.status ?? "all";

  const loadList = useCallback(async (cursor?: string, append = false) => {
    const resourceKey = `transfers:q=${encodeURIComponent(query)}&status=${encodeURIComponent(status)}`;
    const request = beginEvidenceRequest(listRequests.current, resourceKey, append ? "append" : "replace");
    if (!request) return;
    if (!request.sameResource) {
      setTransfers([]);
      setNextCursor(undefined);
      setVerifiedAt(undefined);
    }
    setListLoading(true);
    const parameters = new URLSearchParams({ limit: "25" });
    if (cursor) parameters.set("cursor", cursor);
    if (query) parameters.set("q", query);
    if (status !== "all") parameters.set("status", status);
    const response = await readJSON<TransfersPayload>(`/api/transfers?${parameters}`);
    if (!isEvidenceRequestCurrent(listRequests.current, request.token)) return;
    if (!response.ok) {
      setError(unavailableMessage(response.status, "transfer records", response.requestReference));
    } else {
      const items = Array.isArray(response.data.transfers) ? response.data.transfers : [];
      setTransfers((current) => append ? appendUniqueBy(current, items, (transfer) => transfer.transfer_id) : items);
      setNextCursor(response.data.next_cursor || undefined);
      setVerifiedAt(new Date().toISOString());
      setError(null);
    }
    if (finishEvidenceRequest(listRequests.current, request.token)) setListLoading(false);
  }, [query, status]);

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
