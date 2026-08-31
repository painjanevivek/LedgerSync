"use client";

import { useFinancialPostCommand } from "@/features/console/useFinancialPostCommand";
import type { TransferCorrection } from "@/lib/api/corrections";

function parseCorrectionPost(value: unknown): TransferCorrection | null {
  if (!value || Array.isArray(value) || typeof value !== "object") return null;
  const candidate = (value as { event?: unknown }).event;
  if (!candidate || Array.isArray(candidate) || typeof candidate !== "object") return null;
  const event = candidate as Partial<TransferCorrection>;
  return typeof event.correction_id === "string"
    && event.status === "posted"
    && typeof event.amount_minor === "string"
    && typeof event.currency === "string"
    ? event as TransferCorrection
    : null;
}

export function useCorrectionCommand(tenantId: string, correctionId: string, csrfToken: string) {
  return useFinancialPostCommand<TransferCorrection>({
    domain: "correction",
    tenantId,
    recordId: correctionId,
    csrfToken,
    endpoint: `/api/transfer-corrections/${encodeURIComponent(correctionId)}/post`,
    parseSuccess: parseCorrectionPost,
  });
}
