import { CopyControl } from "@/ui/controls/CopyControl.client";

export function RecordIdentity({ value, label = "Record reference" }: Readonly<{ value: string; label?: string }>) {
  const short = value.length > 12 ? `…${value.slice(-8)}` : value;
  return <span className="record-identity"><span><small>{label}</small><code title={value}>{short}</code><span className="record-identity-full">{value}</span></span><CopyControl value={value} label={`Copy ${label.toLowerCase()}`} /></span>;
}
