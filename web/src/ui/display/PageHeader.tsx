import type { ReactNode } from "react";

export function PageHeader({ eyebrow, title, description, children }: Readonly<{
  eyebrow: string;
  title: string;
  description: string;
  children?: ReactNode;
}>) {
  return <header className="page-header"><div><p className="eyebrow">{eyebrow}</p><h1>{title}</h1><p>{description}</p></div>{children}</header>;
}
