import type {
  ReconciliationRun,
  TransferSummary,
} from "@/features/accounts/types";
import type { LocalOrientation } from "@/lib/api/orientation";

export type WorkspaceStage =
  | "empty"
  | "account_ready"
  | "operational"
  | "attention_required";

export type WorkspaceStageEvidence = Readonly<{
  accountCount: number;
  transfers: readonly TransferSummary[];
  reconciliation: ReconciliationRun | null;
  orientation?: LocalOrientation | null;
  hasCriticalReadError?: boolean;
}>;

export function deriveWorkspaceStage(
  evidence: WorkspaceStageEvidence,
): WorkspaceStage {
  if (
    evidence.hasCriticalReadError ||
    evidence.reconciliation?.status === "failed" ||
    evidence.reconciliation?.status === "mismatch" ||
    (evidence.reconciliation !== null &&
      evidence.reconciliation.mismatch_count !== "0")
  ) {
    return "attention_required";
  }
  if (evidence.accountCount === 0) return "empty";

  const funded = evidence.orientation?.steps.find(
    (step) => step.id === "fund_account",
  );
  const hasPostedFunding = funded?.state === "completed";
  if (evidence.transfers.length > 0 || hasPostedFunding) return "operational";
  return "account_ready";
}
