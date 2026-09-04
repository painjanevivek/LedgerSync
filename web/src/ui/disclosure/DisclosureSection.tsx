"use client";

import {
  useEffect,
  useId,
  useRef,
  useState,
  type ReactNode,
} from "react";

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

function storedPreference(key: string | undefined): boolean | undefined {
  if (!key || typeof window === "undefined") return undefined;
  const value = window.localStorage.getItem(`ledgersync:disclosure:${key}`);
  return value === "open" ? true : value === "closed" ? false : undefined;
}

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
    const remembered = storedPreference(rememberKey);
    if (!attention && remembered !== undefined && details.current) {
      details.current.open = remembered;
    }
    revealTarget();
    window.addEventListener("hashchange", revealTarget);
    return () => window.removeEventListener("hashchange", revealTarget);
  }, [attention, fragment, rememberKey]);

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
        if (rememberKey) {
          window.localStorage.setItem(
            `ledgersync:disclosure:${rememberKey}`,
            nextOpen ? "open" : "closed",
          );
        }
      }}
    >
      <summary aria-describedby={summary ? summaryId : undefined}>
        <span>
          <strong>{title}</strong>
          {summary ? <small id={summaryId}>{summary}</small> : null}
        </span>
        {attention ? <b>Needs attention</b> : null}
      </summary>
      {open || !lazy ? (
        <div className="disclosure-content">{children}</div>
      ) : null}
    </details>
  );
}
