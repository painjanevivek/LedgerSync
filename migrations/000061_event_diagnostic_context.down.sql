DO $$ BEGIN RAISE EXCEPTION 'Diagnostic origin evidence requires a forward repair; destructive rollback is disabled'; END $$;
