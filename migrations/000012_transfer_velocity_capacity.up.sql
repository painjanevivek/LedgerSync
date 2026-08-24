-- Exact, bounded rolling-velocity state for the internal-transfer hot path.
-- The immutable transfers table remains the financial source of truth. These
-- rows are transactionally maintained policy state and can be rebuilt from
-- posted transfers if an operational repair is ever required.

CREATE TABLE transfer_velocity_events (
  transfer_id UUID PRIMARY KEY REFERENCES transfers(id),
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  actor_subject_id TEXT NOT NULL,
  source_account_id UUID NOT NULL REFERENCES accounts(id),
  amount_minor BIGINT NOT NULL CHECK (amount_minor > 0),
  occurred_at TIMESTAMPTZ NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  CHECK (expires_at = occurred_at + INTERVAL '24 hours'),
  FOREIGN KEY (source_account_id, tenant_id) REFERENCES accounts(id, tenant_id)
);

CREATE INDEX transfer_velocity_events_expiry_idx
  ON transfer_velocity_events (tenant_id, expires_at, transfer_id);

CREATE TABLE transfer_velocity_totals (
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  dimension_type TEXT NOT NULL CHECK (dimension_type IN ('tenant','actor','source')),
  dimension_reference TEXT NOT NULL,
  total_minor BIGINT NOT NULL DEFAULT 0 CHECK (total_minor >= 0),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, dimension_type, dimension_reference)
);

-- Preserve the exact active rolling window when upgrading an existing pilot
-- database. Older transfers remain immutable history and require no policy row.
INSERT INTO transfer_velocity_events (
  transfer_id, tenant_id, actor_subject_id, source_account_id,
  amount_minor, occurred_at, expires_at
)
SELECT id, tenant_id, actor_subject_id, debit_account_id,
       amount_minor, completed_at, completed_at + INTERVAL '24 hours'
FROM transfers
WHERE status = 'posted'
  AND completed_at > now() - INTERVAL '24 hours'
ON CONFLICT (transfer_id) DO NOTHING;

INSERT INTO transfer_velocity_totals (
  tenant_id, dimension_type, dimension_reference, total_minor
)
SELECT tenant_id, dimension_type, dimension_reference, SUM(amount_minor)
FROM (
  SELECT tenant_id, 'tenant'::text AS dimension_type,
         tenant_id::text AS dimension_reference, amount_minor
  FROM transfer_velocity_events
  UNION ALL
  SELECT tenant_id, 'actor', actor_subject_id, amount_minor
  FROM transfer_velocity_events
  UNION ALL
  SELECT tenant_id, 'source', source_account_id::text, amount_minor
  FROM transfer_velocity_events
) AS dimensions
GROUP BY tenant_id, dimension_type, dimension_reference
ON CONFLICT (tenant_id, dimension_type, dimension_reference)
DO UPDATE SET total_minor = EXCLUDED.total_minor, updated_at = now();
