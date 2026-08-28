"use client";

import { ArrowsCounterClockwise, LockKey } from "@phosphor-icons/react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useMemo, useState } from "react";

import type { TransferDetail } from "@/features/accounts/types";
import {
  CopyControl,
  StatePanel,
  StatusBadge,
} from "@/features/console/components";
import type {
  CorrectionReasonCode,
  CorrectionSubmission,
} from "@/lib/api/corrections";

const reasons: ReadonlyArray<
  Readonly<{ value: CorrectionReasonCode; label: string }>
> = [
  { value: "duplicate", label: "Duplicate transfer" },
  { value: "wrong_destination", label: "Wrong destination" },
  { value: "wrong_amount", label: "Wrong amount" },
  { value: "customer_request", label: "Customer request" },
  { value: "operational_error", label: "Operational error" },
  { value: "compliance_reversal", label: "Compliance reversal" },
];

export function TransferCorrectionPanel({
  transfer,
  csrfToken,
  online,
  canRead,
  canWrite,
}: Readonly<{
  transfer: TransferDetail;
  csrfToken: string;
  online: boolean;
  canRead: boolean;
  canWrite: boolean;
}>) {
  const router = useRouter();
  const [reasonCode, setReasonCode] =
    useState<CorrectionReasonCode>("operational_error");
  const [operatorNote, setOperatorNote] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [stepUpRequired, setStepUpRequired] = useState(false);
  const idempotencyKey = useMemo(() => `correction-${crypto.randomUUID()}`, []);
  const returnTo = `/transfers/${encodeURIComponent(transfer.transfer_id)}`;

  async function requestCorrection(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setError(null);
    setStepUpRequired(false);
    try {
      const response = await fetch(
        `/api/transfers/${encodeURIComponent(transfer.transfer_id)}/corrections`,
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            "X-CSRF-Token": csrfToken,
            "Idempotency-Key": idempotencyKey,
          },
          body: JSON.stringify({ reasonCode, operatorNote }),
        },
      );
      const payload = (await response.json().catch(() => ({}))) as
        CorrectionSubmission | { error?: { code?: string } };
      if (!response.ok) {
        const code = "error" in payload ? payload.error?.code : undefined;
        if (response.status === 428 || code === "step_up_required")
          setStepUpRequired(true);
        throw new Error(
          response.status === 504
            ? "The request outcome is unknown. Refresh this transfer before retrying with the same intent."
            : code === "step_up_required"
              ? "Recent authentication is required before requesting a correction."
              : `The correction request was not recorded (${code ?? response.status}).`,
        );
      }
      if ("event" in payload && payload.event.correction_id)
        router.push(
          `/corrections/${encodeURIComponent(payload.event.correction_id)}`,
        );
      else
        throw new Error(
          "The correction response did not include its immutable record identifier.",
        );
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : "The correction request could not be verified.",
      );
    } finally {
      setBusy(false);
    }
  }

  if (!canRead) return null;
  if (transfer.correction_id)
    return (
      <section className="correction-transfer-link">
        <ArrowsCounterClockwise aria-hidden="true" />
        <div>
          <p className="eyebrow">Additive correction linked</p>
          <h2>
            {transfer.correction_role === "compensation"
              ? "This is the compensating transfer"
              : "This transfer has a correction record"}
          </h2>
          <p>
            Original and reverse journals remain separately visible under the
            policy-versioned control record.
          </p>
          <CopyControl value={transfer.correction_id} />
        </div>
        <Link
          className="button secondary"
          href={`/corrections/${encodeURIComponent(transfer.correction_id)}`}
        >
          Open correction evidence
        </Link>
      </section>
    );
  if (
    transfer.financial_status !== "posted" ||
    transfer.correction_role === "compensation"
  )
    return null;
  return (
    <section className="correction-request">
      <header>
        <LockKey aria-hidden="true" />
        <div>
          <p className="eyebrow">Controlled correction</p>
          <h2>Request an exact additive reversal</h2>
          <p>
            The original transfer and journal never change. A different
            authorized operator must approve before one exact reverse transfer
            can post.
          </p>
        </div>
        <StatusBadge tone="warning">dual control</StatusBadge>
      </header>
      {error && (
        <StatePanel
          kind="error"
          title="Correction request not verified"
          message={error}
        />
      )}
      {stepUpRequired && (
        <StatePanel
          kind="unknown"
          title="Recent authentication required"
          message="Reauthenticate, then return to this transfer. No correction request was assumed to succeed."
          action={
            <Link
              className="button primary"
              href={`/api/auth/sign-in?prompt=login&return_to=${encodeURIComponent(returnTo)}`}
            >
              Reauthenticate
            </Link>
          }
        />
      )}
      {canWrite ? (
        <details>
          <summary>Start correction request</summary>
          <form onSubmit={requestCorrection}>
            <label>
              Reason code
              <select
                value={reasonCode}
                onChange={(event) =>
                  setReasonCode(event.target.value as CorrectionReasonCode)
                }
              >
                {reasons.map((reason) => (
                  <option key={reason.value} value={reason.value}>
                    {reason.label}
                  </option>
                ))}
              </select>
            </label>
            <label>
              Verified operator note
              <textarea
                required
                rows={4}
                maxLength={500}
                value={operatorNote}
                onChange={(event) => setOperatorNote(event.target.value)}
                placeholder="Describe the evidence that justifies an exact reversal"
              />
            </label>
            <div className="correction-request-proof">
              <strong>Before you submit</strong>
              <ul>
                <li>
                  The request is policy-versioned and expires if not completed
                  in time.
                </li>
                <li>You cannot approve or post your own request.</li>
                <li>
                  Posting creates a new balanced journal; it never edits this
                  one.
                </li>
              </ul>
            </div>
            <button
              className="button danger guarded-control"
              type="submit"
              disabled={!online || busy || !operatorNote.trim()}
            >
              {busy ? "Recording request…" : "Record correction request"}
            </button>
          </form>
        </details>
      ) : (
        <p className="permission-note">
          Your role can inspect correction evidence but cannot request a
          correction.
        </p>
      )}
    </section>
  );
}
