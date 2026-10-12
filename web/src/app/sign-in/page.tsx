import { LoginScreen } from "@/features/auth/LoginScreen";
import { safeInternalReturnPath } from "@/lib/navigation";

export default async function SignInPage({ searchParams }: Readonly<{ searchParams: Promise<Record<string, string | string[] | undefined>> }>) {
  const query = await searchParams;
  const returnTo = typeof query.return_to === "string" ? safeInternalReturnPath(query.return_to) : undefined;
  return <LoginScreen returnTo={returnTo} />;
}
