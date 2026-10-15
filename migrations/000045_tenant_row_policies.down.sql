DO $$ BEGIN RAISE EXCEPTION 'Tenant isolation requires a reviewed forward migration'; END $$;
