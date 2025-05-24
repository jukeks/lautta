package lautta

// Proxy log operations to maintain lastlog in memory
type LastLogCache struct {
	logStore LogStore
	lastLog  *LogEntry
}

func NewLastLogCache(logStore LogStore) LogStore {
	return &LastLogCache{
		logStore: logStore,
	}
}

func (c *LastLogCache) Add(log LogEntry) error {
	if err := c.logStore.Add(log); err != nil {
		return err
	}
	// latest add is always the last log
	c.lastLog = &log
	return nil
}

func (c *LastLogCache) GetLastLog() (LogEntry, error) {
	if c.lastLog != nil {
		return *c.lastLog, nil
	}
	lastLog, err := c.logStore.GetLastLog()
	if err != nil {
		return LogEntry{}, err
	}
	c.lastLog = &lastLog
	return lastLog, nil
}

func (c *LastLogCache) GetFrom(idx LogIndex) ([]LogEntry, error) {
	return c.logStore.GetFrom(idx)
}

func (c *LastLogCache) Get(idx LogIndex) (LogEntry, error) {
	return c.logStore.Get(idx)
}

func (c *LastLogCache) DeleteFrom(idx LogIndex) error {
	if err := c.logStore.DeleteFrom(idx); err != nil {
		return err
	}
	c.lastLog = nil
	return nil
}

func (c *LastLogCache) GetBetween(start, end LogIndex) ([]LogEntry, error) {
	return c.logStore.GetBetween(start, end)
}
