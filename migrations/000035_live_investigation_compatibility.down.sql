-- Deliberately retain the converged column names. Reversing them would make a
-- fresh 000034 database incompatible and could strand rooms created after the
-- compatibility migration. Only the additive operation ledger is removed.
DROP TABLE IF EXISTS investigation_live_room_operations;
ALTER TABLE investigation_live_room_findings DROP CONSTRAINT IF EXISTS investigation_live_room_findings_authoritative_check;
