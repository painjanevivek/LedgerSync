import contract from "@/lib/generated/developer-retry-contract.json";

const idempotent = new Set(contract.idempotent_operations);

export function operationRetryGuidance(operation: Readonly<{ operation_id: string; method: string }>): string {
  if (operation.method === "GET") return "Repeat only after checking freshness and authorization.";
  if (operation.operation_id === "runReconciliation") return "Record the run intent and key. Inspect run state after an unknown result before a deliberate new run.";
  if (idempotent.has(operation.operation_id)) return "Same key + same intent replays; a changed intent conflicts. Persist the complete intent and original credential binding before send. An unknown response keeps that command.";
  return "Record the decision intent. Inspect current resource state after a lost response; this operation does not declare a retry key.";
}
