const canonicalUUIDShape = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const nilUUID = "00000000-0000-0000-0000-000000000000";

/**
 * Returns the one lowercase UUID representation accepted at BFF trust
 * boundaries. Alternate parser spellings and surrounding whitespace are
 * deliberately rejected so cache/session keys cannot fork by text form.
 */
export function canonicalUUID(value: string): string | undefined {
  if (!canonicalUUIDShape.test(value)) return undefined;
  const canonical = value.toLowerCase();
  return canonical === nilUUID ? undefined : canonical;
}

export function isCanonicalUUID(value: string): boolean {
  return canonicalUUID(value) !== undefined;
}

export function canonicalizeUUIDPathSegments(path: string): string {
  return path
    .split("/")
    .map((segment) => {
      if (!canonicalUUIDShape.test(segment)) return segment;
      const canonical = canonicalUUID(segment);
      if (!canonical) throw new Error("invalid UUID path segment");
      return canonical;
    })
    .join("/");
}
