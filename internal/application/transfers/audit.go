package transfers

// Audit event names are stable vocabulary shared by the database adapter and
// operational evidence. They intentionally include identifiers/outcomes, not
// raw account balances or credentials.
const (
	AuditTransferPosted   = "transfer.posted"
	AuditTransferRejected = "transfer.rejected"
)

// OutcomeObserver is a narrow post-command observability port. Observers are
// called only after a repository returns, and must never influence a financial
// result or expose sensitive request data.
type OutcomeObserver interface {
	ObserveSubmission(Result, bool)
	ObserveFailure(error)
}
