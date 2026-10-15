export type PresentationTone = "neutral" | "positive" | "warning" | "danger" | "unknown";

export type PresentationStatus = Readonly<{
  title: string;
  explanation: string;
  attention: boolean;
  tone: PresentationTone;
  nextAction?: Readonly<{ label: string; href: string }>;
  evidence?: ReadonlyArray<Readonly<{ label: string; value: string }>>;
}>;

export function transferStatusPresentation(status: "posted" | "rejected" | "pending"): PresentationStatus {
  switch (status) {
    case "posted":
      return { title: "Transfer completed", explanation: "The exact amount was recorded in both accounts.", attention: false, tone: "positive" };
    case "rejected":
      return { title: "Transfer did not complete", explanation: "No money moved. Open the transfer to see what needs to change.", attention: true, tone: "danger" };
    case "pending":
      return { title: "We are still confirming this transfer", explanation: "Do not create another transfer. Check this transfer's status first.", attention: true, tone: "unknown" };
  }
}

export function reconciliationPresentation(status: "matched" | "mismatch" | "failed" | "running", mismatchCount: string): PresentationStatus {
  if (status === "matched" && mismatchCount === "0") {
    return { title: "Balances checked", explanation: "The latest completed balance check found no differences.", attention: false, tone: "positive" };
  }
  if (status === "running") {
    return { title: "Balance check in progress", explanation: "Keep using the last verified balances until this check finishes.", attention: false, tone: "neutral" };
  }
  if (status === "mismatch") {
    return { title: "A balance needs review", explanation: `${mismatchCount} ${mismatchCount === "1" ? "item needs" : "items need"} review before relying on the latest check.`, attention: true, tone: "warning" };
  }
  return { title: "Balance check unavailable", explanation: "Keep using the last verified balances and try the check again.", attention: true, tone: "unknown" };
}
