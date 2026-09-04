-- This boundary contraction has no routine destructive rollback. Broad table
-- re-grants would recreate the compromised capability that this migration
-- removes. During an incident, issue the narrowest time-bound grant externally,
-- record it, expire it, and repair forward with a reviewed migration.
DO $$
BEGIN
  RAISE EXCEPTION 'direct financial DML cannot be restored by a down migration; repair forward'
    USING ERRCODE='55000';
END;
$$;
