import type { Metadata, Viewport } from "next";
import "../styles/tokens.css";
import "./globals.css";
import "../styles/responsive.css";

// A per-request CSP nonce requires dynamic rendering so Next can attach it to
// its hydration scripts instead of reusing static HTML with an old nonce.
export const dynamic = "force-dynamic";

export const metadata: Metadata = {
  title: "LedgerSync | Operator Console",
  description: "Exact, explainable internal ledger transfers and balances.",
};

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
  viewportFit: "cover",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return <html lang="en"><body>{children}</body></html>;
}
