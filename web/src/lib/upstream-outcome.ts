export const privateReadTimeoutMilliseconds = 5_000;
export const privateWriteTimeoutMilliseconds = 8_000;

export function isPrivateAPITimeout(error: unknown): boolean {
  return error instanceof DOMException && error.name === "TimeoutError";
}
