"use client";

import { Check, Copy } from "@phosphor-icons/react";
import { useEffect, useRef, useState } from "react";

export function DeveloperCopyCode({ value, label }: Readonly<{ value: string; label: string }>) {
  const [status, setStatus] = useState<"idle" | "copied" | "failed">("idle");
  const resetTimer = useRef<number | null>(null);

  useEffect(() => () => {
    if (resetTimer.current !== null) window.clearTimeout(resetTimer.current);
  }, []);

  async function copy() {
    if (resetTimer.current !== null) window.clearTimeout(resetTimer.current);
    try {
      await navigator.clipboard.writeText(value);
      setStatus("copied");
    } catch {
      setStatus("failed");
    }
    resetTimer.current = window.setTimeout(() => setStatus("idle"), 1800);
  }

  return (
    <div className="developer-code">
      <div>
        <span>{label}</span>
        <button type="button" className="code-copy-button" onClick={() => void copy()} aria-label={`Copy ${label}`}>
          {status === "copied" ? <Check aria-hidden="true" /> : <Copy aria-hidden="true" />}
          {status === "copied" ? "Copied" : "Copy"}
        </button>
      </div>
      <pre tabIndex={0} aria-label={label}><code>{value}</code></pre>
      <span className="sr-only" role="status" aria-live="polite">
        {status === "copied" ? `${label} copied` : status === "failed" ? `${label} copy failed` : ""}
      </span>
    </div>
  );
}
