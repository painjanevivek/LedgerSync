import type { Metadata } from "next";
import { LandingPage } from "@/features/landing/LandingPage";

export const metadata: Metadata = { title: "LedgerSync | One clear step at a time", description: "Understand balances, review money movements, and resolve work needing your attention." };
export default function WelcomePage() { return <LandingPage />; }
