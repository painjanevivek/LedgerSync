import type { Metadata, Viewport } from "next";
import { RouteProviders } from "@/features/console/WorkspaceProviders";
import "./globals.css";

// A per-request CSP nonce requires dynamic rendering so Next can attach it to
// its hydration scripts instead of reusing static HTML with an old nonce.
export const dynamic = "force-dynamic";

export const metadata: Metadata = {
  title: "LedgerSync | Clear money operations",
  description: "Understand balances, move money, and resolve work with confidence.",
};

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
  viewportFit: "cover",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return <html lang="en"><body><RouteProviders>{children}</RouteProviders></body></html>;
}
