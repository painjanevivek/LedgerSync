"use client";

import { Check, Copy } from "@phosphor-icons/react";
import { useRef, useState } from "react";

import { Identifier } from "@/ui/display/Identifier";
import { IconButton } from "@/ui/controls/IconButton";

export function CopyControl({ value, label = "Copy identifier" }: Readonly<{ value: string; label?: string }>) {
  const [state, setState] = useState<"idle" | "copied" | "failed">("idle");
  const resetTimer = useRef<number | undefined>(undefined);

  async function copy() {
    window.clearTimeout(resetTimer.current);
    try {
      await navigator.clipboard.writeText(value);
      setState("copied");
    } catch {
      setState("failed");
    }
    resetTimer.current = window.setTimeout(() => setState("idle"), 1800);
  }

  return <span className="copy-control"><Identifier value={value} /><IconButton type="button" onClick={() => void copy()} label={label}>{state === "copied" ? <Check aria-hidden="true" /> : <Copy aria-hidden="true" />}</IconButton><span className="sr-only" aria-live="polite">{state === "copied" ? "Copied" : state === "failed" ? "Copy failed" : ""}</span></span>;
}
