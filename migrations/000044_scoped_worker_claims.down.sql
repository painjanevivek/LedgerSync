-- Forward repair only: old binaries require an explicit compatibility review.
DO $$ BEGIN RAISE EXCEPTION 'Scoped worker claims require a reviewed forward migration'; END $$;
