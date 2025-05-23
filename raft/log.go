package lautta

type Log interface {
	Add(LogEntry) error
	Get(idx LogIndex) (LogEntry, error)
	GetFrom(idx LogIndex) ([]LogEntry, error)
	GetLastLog() (LogEntry, error)
	DeleteFrom(idx LogIndex) error
}

type InMemLog struct {
	entries []LogEntry
}

func NewInMemLog() Log {
	return &InMemLog{entries: []LogEntry{}}
}

func (l *InMemLog) Get(idx LogIndex) (LogEntry, error) {
	for _, entry := range l.entries {
		if entry.Index == idx {
			return entry, nil
		}
	}

	return LogEntry{}, nil
}

func (l *InMemLog) DeleteFrom(idx LogIndex) error {
	for i, entry := range l.entries {
		if entry.Index == idx {
			l.entries = l.entries[:i]
			break
		}
	}

	return nil
}

func (l *InMemLog) Add(entry LogEntry) error {
	l.entries = append(l.entries, entry)
	return nil
}

func (l *InMemLog) GetFrom(idx LogIndex) ([]LogEntry, error) {
	slice := make([]LogEntry, 0)
	appending := true
	for _, entry := range l.entries {
		if entry.Index == idx {
			appending = true
		}

		if !appending {
			continue
		}
		slice = append(slice, entry)
	}

	return slice, nil
}

func (l *InMemLog) GetLastLog() (LogEntry, error) {
	if len(l.entries) == 0 {
		return LogEntry{}, nil
	}

	return l.entries[len(l.entries)-1], nil
}
