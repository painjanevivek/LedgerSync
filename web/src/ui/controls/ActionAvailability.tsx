import {
  cloneElement,
  isValidElement,
  useId,
  type ButtonHTMLAttributes,
  type ReactElement,
  type ReactNode,
} from "react";

import { UnavailableActionMetric } from "@/ui/controls/UnavailableActionMetric.client";

export type UnavailableActionState =
  | "busy"
  | "offline"
  | "prerequisite"
  | "capability_missing"
  | "step_up"
  | "temporary_unavailable"
  | "unreleased"
  | "terminal";

export type ActionAvailabilityStatus =
  | Readonly<{ state: "available" }>
  | Readonly<{ state: UnavailableActionState; reason: string; recovery?: ReactNode }>;

type ActionElement = ReactElement<ButtonHTMLAttributes<HTMLButtonElement>>;

export function ActionAvailability({
  availability,
  children,
}: Readonly<{ availability: ActionAvailabilityStatus; children: ActionElement }>) {
  const reasonId = useId();
  if (!isValidElement(children)) throw new Error("ActionAvailability requires one button child");
  if (availability.state === "unreleased") return null;
  if (availability.state === "available") return children;

  const describedBy = [children.props["aria-describedby"], reasonId].filter(Boolean).join(" ");
  const control = cloneElement(children, {
    disabled: true,
    "aria-describedby": describedBy,
  });

  return (
    <span className="action-availability" data-availability={availability.state}>
      <UnavailableActionMetric state={availability.state} />
      {control}
      <span id={reasonId} className="action-availability-reason">
        {availability.reason}
        {availability.recovery && <span className="action-availability-recovery">{availability.recovery}</span>}
      </span>
    </span>
  );
}
