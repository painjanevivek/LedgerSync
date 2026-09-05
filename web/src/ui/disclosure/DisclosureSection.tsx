"use client";

import {
  useEffect,
  useId,
  useRef,
  useState,
  type ReactNode,
} from "react";

import { useConsoleSession } from "@/features/console/ConsoleSessionBoundary";
import {
  readDisclosurePreference,
  writeDisclosurePreference,
} from "@/ui/disclosure/disclosure-preference";

export type DisclosurePriority = "primary" | "secondary" | "advanced";

export type DisclosureContract = Readonly<{
  id: string;
  title: string;
  summary?: string;
  defaultOpen?: boolean;
  attention?: boolean;
  fragment?: string;
  priority?: DisclosurePriority;
}>;

type Props = DisclosureContract &
  Readonly<{
    children: ReactNode;
    className?: string;
    lazy?: boolean;
    rememberKey?: string;
  }>;

export function DisclosureSection({
  id,
  title,
  summary,
  defaultOpen = false,
  attention = false,
  fragment = id,
  priority = "secondary",
  children,
  className,
  lazy = false,
  rememberKey,
}: Props) {
  const { session } = useConsoleSession();
  const summaryId = useId();
  const details = useRef<HTMLDetailsElement>(null);
  const [open, setOpen] = useState(defaultOpen);

  useEffect(() => {
    const targetHash = `#${fragment}`;
    const revealTarget = () => {
      if (window.location.hash === targetHash && details.current) {
        details.current.open = true;
      }
    };
    const remembered = readDisclosurePreference(
      window.localStorage,
      session?.tenant_id ?? "",
      session?.subject_id ?? "",
      rememberKey,
    );
    if (!attention && remembered !== undefined && details.current) {
      details.current.open = remembered;
    }
    revealTarget();
    window.addEventListener("hashchange", revealTarget);
    return () => window.removeEventListener("hashchange", revealTarget);
  }, [attention, fragment, rememberKey, session?.subject_id, session?.tenant_id]);

  return (
    <details
      ref={details}
      id={id}
      className={[
        "disclosure-section",
        `priority-${priority}`,
        attention ? "requires-attention" : "",
        className ?? "",
      ]
        .filter(Boolean)
        .join(" ")}
      open={attention || open}
      onToggle={(event) => {
        const nextOpen = event.currentTarget.open;
        setOpen(nextOpen);
        writeDisclosurePreference(
          window.localStorage,
          session?.tenant_id ?? "",
          session?.subject_id ?? "",
          rememberKey,
          nextOpen,
        );
      }}
    >
      <summary aria-describedby={summary ? summaryId : undefined}>
        <span>
          <strong>{title}</strong>
          {summary ? <small id={summaryId}>{summary}</small> : null}
        </span>
        {attention ? <b>Needs attention</b> : null}
      </summary>
      {attention || open || !lazy ? (
        <div className="disclosure-content">{children}</div>
      ) : null}
    </details>
  );
}
