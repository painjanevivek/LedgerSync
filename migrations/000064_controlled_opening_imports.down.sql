-- Financial opening evidence is permanent. Recover by a reviewed forward repair.
DO $$ BEGIN RAISE EXCEPTION 'controlled opening imports require forward repair; financial evidence cannot be removed'; END $$;
