import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "LedgerSync",
  description: "Secure, consistent balance visibility",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return <html lang="en"><body>{children}</body></html>;
}
