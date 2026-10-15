import type { ReactNode } from "react";

export function PageHeader({ title, description, children }: Readonly<{
  eyebrow?: string;
  title: string;
  description: string;
  children?: ReactNode;
}>) {
  return <header className="page-header"><div><h1>{title}</h1><p>{description}</p></div>{children}</header>;
}
