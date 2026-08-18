-- LedgerSync's financial schema is owned by versioned migrations only.
-- This migration intentionally establishes only database capabilities shared
-- by later financial tables; it contains no runtime seed data.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL
);
