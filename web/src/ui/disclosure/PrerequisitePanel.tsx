import { useId, type ReactNode } from "react";

export function PrerequisitePanel({
  title,
  message,
  action,
}: Readonly<{ title: string; message: string; action?: ReactNode }>) {
  const titleId = useId();
  return (
    <section className="prerequisite-panel" aria-labelledby={titleId}>
      <p className="eyebrow">Required before continuing</p>
      <h2 id={titleId}>{title}</h2>
      <p>{message}</p>
      {action ? <div className="action-row">{action}</div> : null}
    </section>
  );
}
