import { formatMinorUnits } from "@/lib/money";

export function Money({ currency, minorUnits, className }: Readonly<{ currency: string; minorUnits: string; className?: string }>) {
  const formatted = formatMinorUnits(currency, minorUnits);
  return <span className={className} data-currency={currency.trim().toUpperCase()} data-minor-units={minorUnits}>{formatted}</span>;
}
