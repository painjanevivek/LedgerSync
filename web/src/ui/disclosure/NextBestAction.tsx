import { useId, type ReactNode } from "react";

export function NextBestAction({
  eyebrow = "Recommended next action",
  title,
  message,
  action,
  attention = false,
}: Readonly<{
  eyebrow?: string;
  title: string;
  message: string;
  action?: ReactNode;
  attention?: boolean;
}>) {
  const titleId = useId();
  return (
    <section
      className={`next-best-action${attention ? " requires-attention" : ""}`}
      aria-labelledby={titleId}
    >
      <div>
        <p className="eyebrow">{eyebrow}</p>
        <h2 id={titleId}>{title}</h2>
        <p>{message}</p>
      </div>
      {action ? <div className="action-row">{action}</div> : null}
    </section>
  );
}
