"use client";

import { useEffect } from "react";

import { emitGuardrailMetric } from "@/lib/guardrail-metrics";
import type { UnavailableActionState } from "@/ui/controls/ActionAvailability";

export function UnavailableActionMetric({ state }: Readonly<{ state: UnavailableActionState }>) {
  useEffect(() => {
    emitGuardrailMetric("unavailable_action", state);
  }, [state]);
  return null;
}
