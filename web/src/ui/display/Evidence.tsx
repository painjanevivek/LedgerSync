import { ArrowClockwise, CheckCircle, Clock, WarningCircle } from "@phosphor-icons/react";

type EvidenceFreshnessProps = Readonly<{ state: "current" | "refreshing" | "historical"; verifiedAt?: string; label?: string; reason?: string }>;

/** Historical evidence remains explicitly timestamped and is never presented as a current zero. */
export function EvidenceFreshness(props: EvidenceFreshnessProps) {
  const label = props.label ?? "Evidence";
  const timestamp = props.verifiedAt ? new Date(props.verifiedAt).toLocaleString("en-GB", { timeZone: "UTC", hour12: false }) : undefined;
  const icon = props.state === "current" ? <CheckCircle weight="fill" aria-hidden="true" /> : props.state === "refreshing" ? <ArrowClockwise aria-hidden="true" /> : <Clock weight="fill" aria-hidden="true" />;
  const copy = props.state === "current"
    ? `${label} verified ${timestamp} UTC`
    : props.state === "refreshing"
      ? timestamp ? `Refreshing; prior ${label.toLowerCase()} verified ${timestamp} UTC` : `Loading ${label.toLowerCase()}`
      : `${label} not refreshed; last verified ${timestamp} UTC${props.reason ? `. ${props.reason}` : ""}`;
  return <p className={`evidence-freshness ${props.state}`} role="status">{icon}<span>{copy}</span></p>;
}

export function EvidenceStepMarker({ sequence, state }: Readonly<{ sequence: number; state: "available" | "bounded" | "missing" | "unavailable" }>) {
  return <div className={`evidence-stage-marker evidence-step-marker ${state}`} aria-hidden="true"><span>{sequence}</span>{state === "available" ? <CheckCircle weight="fill" /> : state === "unavailable" ? <WarningCircle weight="fill" /> : <Clock weight="fill" />}</div>;
}
