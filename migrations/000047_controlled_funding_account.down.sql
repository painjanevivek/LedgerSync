DO $$ BEGIN RAISE EXCEPTION 'Controlled funding account creation requires a reviewed forward migration'; END $$;
