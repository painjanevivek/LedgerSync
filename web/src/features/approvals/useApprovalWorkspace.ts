"use client";

import { useCallback, useRef, useState } from "react";

import {
  beginEvidenceRequest,
  createEvidenceRequestCoordinator,
  finishEvidenceRequest,
  isEvidenceRequestCurrent,
} from "@/features/console/evidenceRequestCoordinator";
import type {
  ApprovalFilters,
  ApprovalItem,
  ApprovalPage,
} from "@/lib/api/approvals";
import { readJSON, unavailableMessage } from "@/lib/api/client";

export function approvalQuery(filters: ApprovalFilters) {
  const query = new URLSearchParams({ limit: "25" });
  if (filters.domain) query.set("domain", filters.domain);
  if (filters.status) query.set("status", filters.status);
  if (filters.requester) query.set("requester", filters.requester);
  if (filters.age) query.set("age", filters.age);
  if (filters.requestedAfter) query.set("requested_after", filters.requestedAfter);
  if (filters.requestedBefore) query.set("requested_before", filters.requestedBefore);
  if (filters.actionableByMe) query.set("actionable_by_me", "true");
  if (filters.cursor) query.set("cursor", filters.cursor);
  return query.toString();
}

export function approvalURL(filters: ApprovalFilters) {
  const query = approvalQuery(filters);
  const visible = new URLSearchParams(query);
  visible.delete("limit");
  return visible.size ? `/approvals?${visible}` : "/approvals";
}

export function useApprovalWorkspace() {
  const [items, setItems] = useState<ApprovalItem[]>([]);
  const [pageCount, setPageCount] = useState(0);
  const [nextCursor, setNextCursor] = useState<string>();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [denied, setDenied] = useState(false);
  const [verifiedAt, setVerifiedAt] = useState<string>();
  const requests = useRef(createEvidenceRequestCoordinator());

  const load = useCallback(async (filters: ApprovalFilters) => {
    const key = approvalQuery(filters);
    const request = beginEvidenceRequest(requests.current, `approvals:${key}`);
    if (!request) return;
    setLoading(true);
    setError(null);
    setDenied(false);
    if (!request.sameResource) {
      setItems([]);
      setPageCount(0);
      setNextCursor(undefined);
      setVerifiedAt(undefined);
    }
    const response = await readJSON<ApprovalPage>(`/api/approvals?${key}`);
    if (!isEvidenceRequestCurrent(requests.current, request.token)) return;
    if (response.ok && Array.isArray(response.data.items)) {
      setItems(response.data.items);
      setPageCount(response.data.page_count);
      setNextCursor(response.data.next_cursor || undefined);
      setVerifiedAt(new Date().toISOString());
    } else if (response.status === 403) {
      setDenied(true);
    } else {
      setError(unavailableMessage(response.status, "approval evidence", response.requestReference));
    }
    if (finishEvidenceRequest(requests.current, request.token)) setLoading(false);
  }, []);

  return { items, pageCount, nextCursor, loading, error, denied, verifiedAt, load };
}
