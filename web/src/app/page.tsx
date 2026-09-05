import { HomeEntry } from "@/features/landing/HomeEntry";
import { LandingPage } from "@/features/landing/LandingPage";

export default async function Home({ searchParams }: Readonly<{ searchParams: Promise<Record<string, string | string[] | undefined>> }>) {
  const query = await searchParams;
  return <HomeEntry landing={<LandingPage />} showOrientation={query.guide === "1"} />;
}
