import { utcDateTime } from "@/features/console/format";

export function Timestamp({ value, fallback = "Unavailable", className, inheritTypography = true }: Readonly<{ value?: string; fallback?: string; className?: string; inheritTypography?: boolean }>) {
  const classes = ["timestamp-value", inheritTypography ? "inherit-typography" : "", className].filter(Boolean).join(" ");
  if (!value) return <span className={classes}>{fallback}</span>;
  const date = new Date(value);
  if (Number.isNaN(date.valueOf())) return <span className={classes}>{fallback}</span>;
  return <time className={classes} dateTime={date.toISOString()}>{utcDateTime(value)}</time>;
}
