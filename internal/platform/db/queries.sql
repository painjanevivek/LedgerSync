-- Runtime templates use controlled capabilities. Retired direct writers remain
-- preserved in Git history and the R03 source snapshot. Do not generate them.

-- name: GetOwnedAccount :one
SELECT a.id,a.tenant_id,a.currency,a.status,b.available_minor,b.ledger_minor,b.balance_version,b.updated_at
FROM accounts a
JOIN account_owners o ON o.account_id=a.id AND o.tenant_id=a.tenant_id
JOIN account_balance_projections b ON b.account_id=a.id
WHERE a.id=$1 AND a.tenant_id=$2 AND o.subject_id=$3;

-- name: ControlledSubmitTransfer :one
SELECT * FROM public.controlled_submit_transfer_v1(
 $1::uuid,$2::text,$3::uuid,$4::uuid,$5::bigint,$6::text,
 $7::text,$8::bytea,$9::uuid,$10::text,$11::timestamptz);

-- name: ControlledPostFunding :one
SELECT * FROM public.controlled_post_funding_v1($1::uuid,$2::text,$3::uuid,$4::text,$5::uuid,$6::timestamptz);

-- name: ControlledPostCorrection :one
SELECT * FROM public.controlled_post_transfer_correction_v1($1::uuid,$2::text,$3::uuid,$4::text,$5::uuid,$6::timestamptz,$7::timestamptz);

-- name: ControlledProvisionAccount :one
SELECT * FROM public.controlled_provision_account_v1(
 $1::uuid,$2::text,$3::uuid,$4::text,$5::text,$6::text,$7::text,
 $8::text[],$9::text[],$10::text[],$11::uuid,$12::timestamptz);

-- name: ControlledClaimOutbox :many
SELECT * FROM public.controlled_claim_outbox_v1($1::integer,$2::text,$3::double precision);

-- name: ControlledCompleteOutbox :one
SELECT public.controlled_complete_outbox_v1($1::uuid,$2::text,$3::bigint,$4::timestamptz,$5::text,$6::text);
