export type EvidenceRequestMode = "replace" | "append";

export type EvidenceRequestCoordinator = {
  generation: number;
  resourceKey?: string;
  appendInFlight: boolean;
};

export type EvidenceRequestToken = Readonly<{
  generation: number;
  resourceKey: string;
  mode: EvidenceRequestMode;
}>;

export function createEvidenceRequestCoordinator(): EvidenceRequestCoordinator {
  return { generation: 0, appendInFlight: false };
}

export function beginEvidenceRequest(
  coordinator: EvidenceRequestCoordinator,
  resourceKey: string,
  mode: EvidenceRequestMode = "replace",
): Readonly<{ token: EvidenceRequestToken; sameResource: boolean }> | null {
  const sameResource = coordinator.resourceKey === resourceKey;
  if (mode === "append" && (!sameResource || coordinator.appendInFlight)) return null;
  coordinator.resourceKey = resourceKey;
  coordinator.generation += 1;
  coordinator.appendInFlight = mode === "append";
  return {
    token: { generation: coordinator.generation, resourceKey, mode },
    sameResource,
  };
}

export function isEvidenceRequestCurrent(
  coordinator: EvidenceRequestCoordinator,
  token: EvidenceRequestToken,
): boolean {
  return coordinator.generation === token.generation
    && coordinator.resourceKey === token.resourceKey;
}

export function finishEvidenceRequest(
  coordinator: EvidenceRequestCoordinator,
  token: EvidenceRequestToken,
): boolean {
  const current = isEvidenceRequestCurrent(coordinator, token);
  if (current && token.mode === "append") coordinator.appendInFlight = false;
  return current;
}

export function invalidateEvidenceRequests(coordinator: EvidenceRequestCoordinator): void {
  coordinator.generation += 1;
  coordinator.appendInFlight = false;
}

export function appendUniqueBy<T>(
  current: readonly T[],
  incoming: readonly T[],
  identity: (item: T) => string,
): T[] {
  const seen = new Set(current.map(identity));
  return [...current, ...incoming.filter((item) => {
    const key = identity(item);
    if (!key || seen.has(key)) return false;
    seen.add(key);
    return true;
  })];
}
