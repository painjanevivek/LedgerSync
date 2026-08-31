export function Identifier({ value, className }: Readonly<{ value: string; className?: string }>) {
  return <code className={className} title={value}>{value}</code>;
}
