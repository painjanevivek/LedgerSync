import { StatementReconciliationController } from "@/features/statement-reconciliation/StatementReconciliationController";
import { statementUUID } from "@/lib/api/statement-reconciliation";
export default async function StatementPage({ searchParams }: Readonly<{
    searchParams: Promise<Record<string, string | string[] | undefined>>;
}>) {
    const query = await searchParams, run = query.run;
    const invalid = Object.keys(query).some(key => key !== "run") || run !== undefined && (typeof run !== "string" || !statementUUID.test(run));
    return <StatementReconciliationController runID={typeof run === "string" && !invalid ? run : undefined} invalid={invalid}/>;
}
