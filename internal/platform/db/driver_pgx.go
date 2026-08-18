package db

// Register pgx with database/sql for the API, worker, migration, and
// reconciliation commands. No domain or application package depends on it.
import _ "github.com/jackc/pgx/v5/stdlib"
