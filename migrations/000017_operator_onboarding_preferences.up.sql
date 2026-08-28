-- Operator guidance preferences are server-owned convenience state. They are
-- tenant- and subject-scoped, versioned for lost-update protection, and kept
-- separate from immutable financial and audit evidence.
CREATE TABLE operator_onboarding_preferences (
  tenant_id UUID NOT NULL REFERENCES tenants(id),
  subject_id TEXT NOT NULL CHECK (char_length(subject_id) BETWEEN 1 AND 256),
  dismissed BOOLEAN NOT NULL DEFAULT false,
  completed_step_ids JSONB NOT NULL DEFAULT '[]'::jsonb
    CHECK (
      jsonb_typeof(completed_step_ids) = 'array'
      AND jsonb_array_length(completed_step_ids) <= 7
    ),
  version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id,subject_id)
);

DO $$
BEGIN
  IF to_regrole('ledgersync_api') IS NOT NULL THEN
    EXECUTE 'GRANT SELECT,INSERT,UPDATE ON operator_onboarding_preferences TO ledgersync_api';
  END IF;
END $$;
