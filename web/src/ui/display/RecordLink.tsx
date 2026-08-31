import { ArrowRight } from "@phosphor-icons/react";
import Link from "next/link";

export function RecordLink({ href, label, id }: Readonly<{ href: string; label: string; id?: string }>) {
  return <Link className="record-link" href={href} id={id}>{label}<ArrowRight aria-hidden="true" /></Link>;
}
