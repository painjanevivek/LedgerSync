import { OverviewController } from "@/features/overview/OverviewController";

export default async function Home({ searchParams }: Readonly<{ searchParams: Promise<Record<string, string | string[] | undefined>> }>) {
  const query = await searchParams;
  return <OverviewController showOrientation={query.guide === "1"} />;
}
