import type { ButtonHTMLAttributes, ReactNode } from "react";

export function IconButton({ label, children, className = "", ...props }: Readonly<Omit<ButtonHTMLAttributes<HTMLButtonElement>, "aria-label"> & { label: string; children: ReactNode }>) {
  return <button {...props} className={className} aria-label={label}>{children}</button>;
}
