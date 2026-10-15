DO $$ BEGIN
 RAISE EXCEPTION 'Session storage boundaries are forward-only. Keep the scoped session adapter and repair forward; preserve all session and preference records.';
END $$;
