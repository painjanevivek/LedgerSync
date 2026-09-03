"use client";

import { Button } from "@/ui/controls/Button";

export function Pagination({ nextCursor, onNext, busy, label = "Load more" }: Readonly<{ nextCursor?: string; onNext: () => void; busy?: boolean; label?: string }>) {
  return <div className="pagination"><span>{nextCursor ? "More records are available" : "End of available records"}</span>{nextCursor && <Button variant="secondary" type="button" busy={busy} busyLabel="Loading…" onClick={onNext}>{label}</Button>}</div>;
}
