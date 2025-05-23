package lautta

type FSM interface {
	Apply(LogEntry) error
	// Snapshot
	// Restore
}
