DO $$ BEGIN RAISE EXCEPTION 'Controlled lifecycle cutover requires forward repair. Restore only into an isolated target.'; END $$;
