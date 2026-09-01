# Investigation evidence bundle threat model

## Boundary and assets

The browser is untrusted. The BFF accepts only a same-origin, CSRF-protected POST containing the current positive workspace version. The private API reauthenticates the actor assertion and requires tenant operator/admin role, `investigation:read`, `exports:read`, a released root-domain read scope, current ownership, and current object access. PostgreSQL remains authoritative; the ZIP is a historical derivative.

Protected assets include financial values, tenant/operator identity, event payloads, credentials, free-form investigation text, cross-tenant existence, current workspace state, and the integrity of downloaded evidence.

## Abuse cases and controls

| Threat | Released control |
| --- | --- |
| Cross-tenant or previous-owner download | Workspace lookup repeats tenant, owner, root-domain and object authorization; absent and unauthorized workspaces share the non-disclosing not-found boundary. |
| Scope reviewed against stale workspace state | Browser submits the reviewed version; private API rejects a changed version with a conflict before generation. |
| CSRF or drive-by download | BFF requires exact Host, same-origin CSRF, POST, strict JSON and a signed session. |
| Bulk extraction | One owned workspace per request, at most 21 historical references and 21 current rows, 10 requests/minute, 512 KiB archive limit and bounded upstream timeout. |
| Payload, secret, money or free-text leakage | Generator uses fixed columns and never serializes titles, safe labels, money/currency, payloads, bodies, headers, credentials, tenant/operator labels or notes. |
| Spreadsheet formula execution | Exported fields are server-controlled enums, UUIDs, timestamps and bounded request references; arbitrary free text is excluded rather than escaped. |
| Archive substitution or truncation | Manifest contains SHA-256 for each CSV; API returns complete-ZIP SHA-256; BFF validates media type, filename, size, schema, expiry and digest before browser delivery. |
| Unaudited generation | API writes a successful immutable generation event before attachment headers or bytes; audit failure is fail-closed. The event does not falsely claim the client completed its download. |
| Stale bundle treated as current | Manifest and UI state that the ZIP is historical only, includes generated/expiry UTC, and directs the operator back to the authorized workspace for current state. |
| Server-side evidence accumulation | ZIP is generated in memory and is not stored after the response. No signed URL, object-storage key, background job or archive table exists. |

## Residual custody

After the browser receives the archive, endpoint/device controls govern copying, retention and deletion. LedgerSync cannot revoke an already downloaded file. Production enablement therefore depends on the organization’s endpoint protection, export-access review and incident process; the application-level expiry is a handling warning, not cryptographic destruction.
