import { formatCurrencyMinorUnits, formatMinorUnits } from "@/lib/money";

export function Money({ currency, minorUnits, className, localized = false }: Readonly<{ currency: string; minorUnits: string; className?: string; localized?: boolean }>) {
  const formatted = localized ? formatCurrencyMinorUnits(currency, minorUnits) : formatMinorUnits(currency, minorUnits);
  return <span className={className} data-currency={currency.trim().toUpperCase()} data-minor-units={minorUnits}>{formatted}</span>;
}
