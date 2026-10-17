// Read only queries against the owned synthetic database.
export const sampleSQL = `SELECT json_build_object('pending',count(*) FILTER(WHERE published_at IS NULL AND dead_at IS NULL),
  'oldest_seconds',COALESCE(extract(epoch
  FROM clock_timestamp()-min(occurred_at) FILTER(WHERE published_at IS NULL AND dead_at IS NULL)),0),
  'dead',count(*) FILTER(WHERE dead_at IS NOT NULL),
  'connections',(SELECT count(*)
  FROM pg_stat_activity
  WHERE datname=current_database()),
  'lock_waiters',(SELECT count(*)
  FROM pg_stat_activity
  WHERE datname=current_database() AND wait_event_type='Lock'),
  'database_bytes',pg_database_size(current_database()))
  FROM outbox_events`;

export const financialSQL = `WITH totals AS (SELECT a.id,a.tenant_id,p.ledger_minor,p.available_minor,p.balance_version,o.opening_ledger_minor::numeric+COALESCE(sum(CASE WHEN l.direction='credit' THEN l.amount_minor::numeric ELSE -l.amount_minor::numeric END),0) expected
  FROM accounts a
  JOIN account_balance_projections p ON p.account_id=a.id
  JOIN account_opening_balances o ON o.account_id=a.id
  LEFT JOIN ledger_postings l ON l.account_id=a.id
  GROUP BY a.id,a.tenant_id,p.ledger_minor,p.available_minor,p.balance_version,o.opening_ledger_minor), bad_journals AS (SELECT j.id
  FROM journal_transactions j
  LEFT JOIN ledger_postings p ON p.journal_transaction_id=j.id
  GROUP BY j.id
  HAVING count(p.id)<>2 OR count(DISTINCT p.account_id)<>2 OR count(DISTINCT p.currency)<>1 OR sum(CASE WHEN p.direction='debit' THEN -p.amount_minor::numeric ELSE p.amount_minor::numeric END)<>0)
  SELECT json_build_object('balances',(SELECT json_agg(json_build_object('account_id',id::text,
  'ledger_minor',ledger_minor::text,
  'available_minor',available_minor::text,'balance_version',balance_version::text))
  FROM totals),
  'accounts',(SELECT count(*)
  FROM accounts),
  'balances_checked',(SELECT count(*)
  FROM totals),
  'projection_mismatches',(SELECT count(*)
  FROM totals
  WHERE expected<>ledger_minor OR ledger_minor<>available_minor),
  'unbalanced_journals',(SELECT count(*)
  FROM bad_journals),
  'unresolved_keys',(SELECT count(*)
  FROM idempotency_requests
  WHERE state NOT IN ('completed',
  'failed')),
  'duplicate_keys',(SELECT count(*)
  FROM (SELECT tenant_id,actor_subject_id,operation,idempotency_key
  FROM idempotency_requests
  GROUP BY 1,2,3,4
  HAVING count(*)>1) d),
  'tenant_leaks',(SELECT count(*)
  FROM transfers t
  JOIN accounts a ON a.id=t.debit_account_id
  JOIN accounts b ON b.id=t.credit_account_id
  WHERE t.tenant_id<>a.tenant_id OR t.tenant_id<>b.tenant_id),
  'active_holds',(SELECT count(*)
  FROM account_holds
  WHERE state='active'),
  'posted',(SELECT count(*)
  FROM transfers
  WHERE status='posted'),
  'rejected',(SELECT count(*)
  FROM transfers
  WHERE status='rejected'),
  'journals',(SELECT count(*)
  FROM journal_transactions),
  'postings',(SELECT count(*)
  FROM ledger_postings),
  'outbox_pending',(SELECT count(*)
  FROM outbox_events
  WHERE published_at IS NULL AND dead_at IS NULL),
  'outbox_dead',(SELECT count(*)
  FROM outbox_events
  WHERE dead_at IS NOT NULL))`;
