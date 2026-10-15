-- Refuse to erase retained recovery identity after first use.
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM transfer_recovery_intents) THEN
   RAISE EXCEPTION 'retained transfer recovery intents prevent rollback';
 END IF;
END $$;
DROP FUNCTION controlled_prepare_transfer_recovery_v1(UUID,TEXT,UUID,TEXT,UUID,UUID,TEXT,BIGINT,BYTEA);
DROP TABLE transfer_recovery_intents;
