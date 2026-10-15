-- This additive repair introduces no object or data to remove. Keep the safe
-- finalized-transfer attachment and no-update replay behavior on rollback;
-- reverting the former helper would deliberately restore a posting defect.
-- A deeper isolated rollback through 000037 restores its pre-reference guard
-- before removing request_reference. Never run that destructive down path on
-- a preserved runtime database as a restart or repair procedure.
SELECT 1;
