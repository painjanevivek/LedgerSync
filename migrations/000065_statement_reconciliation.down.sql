-- Statement source and decisions are retained evidence. Use a reviewed forward
-- repair; do not remove accepted imports or conceal reconciliation history.
DO $$ BEGIN RAISE EXCEPTION 'statement reconciliation migration is forward-only';END $$;
