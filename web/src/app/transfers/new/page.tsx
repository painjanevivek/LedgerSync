import { TransfersController } from "@/features/transfers/TransfersController";
import { safeInternalReturnPath } from "@/lib/navigation";
import { parseStrictListQuery } from "@/lib/strict-list-query";

export default async function NewTransferPage({ searchParams }: Readonly<{ searchParams: Promise<Record<string, string | string[] | undefined>> }>) {
  const parsed = parseStrictListQuery(await searchParams, {
    return_to: { maximumLength: 2_048, validate: value => {
      const safe = safeInternalReturnPath(value);
      if (!safe) return false;
      const path = safe.split("?")[0];
      return path === "/transfers" || path === "/accounts" || /^\/accounts\/[0-9a-f-]{36}$/i.test(path);
    } },
  });
  return <TransfersController creating invalidQuery={!parsed.ok} returnTo={parsed.ok ? parsed.values.return_to : undefined} />;
}
