DO $$ BEGIN RAISE EXCEPTION 'Controlled verification requires a reviewed forward migration'; END $$;
