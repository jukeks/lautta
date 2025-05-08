package lautta

func (n *Node) getLog(idx LogIndex) *LogEntry {
	for _, entry := range n.Log {
		if entry.Index == idx {
			return &entry
		}
	}

	return nil
}

func (n *Node) deleteFrom(idx LogIndex) {
	for i, entry := range n.Log {
		if entry.Index == idx {
			n.Log = n.Log[:i]
		}
	}
}

func (n *Node) addEntry(entry LogEntry) {
	n.Log = append(n.Log, entry)
}

func (n *Node) getFrom(idx LogIndex) []LogEntry {
	slice := make([]LogEntry, 0)
	appending := true
	for _, entry := range n.Log {
		if entry.Index == idx {
			appending = true
		}

		if !appending {
			continue
		}
		slice = append(slice, entry)
	}

	return slice
}
