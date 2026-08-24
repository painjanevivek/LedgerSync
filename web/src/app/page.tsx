import { OperatorConsole } from "@/features/accounts/OperatorConsole";

export default async function Home({ searchParams }: Readonly<{ searchParams: Promise<Record<string, string | string[] | undefined>> }>) {
  const query = await searchParams;
  return <OperatorConsole initialShowOrientation={query.guide === "1"} />;
}
