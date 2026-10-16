import { ArrowRight } from "@phosphor-icons/react";
import Link from "next/link";

export function RecordLink({ href, label, id }: Readonly<{ href: string; label: string; id?: string }>) {
  // Load financial record destinations on selection, not for every visible row.
  return <Link prefetch={false} className="record-link" href={href} id={id}>{label}<ArrowRight aria-hidden="true" /></Link>;
}
