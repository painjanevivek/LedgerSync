-- Reservation evidence and obligations require a compatible forward repair.
DO $$ BEGIN RAISE EXCEPTION 'reservation migration is forward only; use a compatible image and forward repair'; END $$;
