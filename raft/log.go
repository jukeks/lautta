package lautta

func (n *Node) findLog(idx LogIndex) *LogEntry {
	for _, entry := range n.Log {
		if entry.Index == idx {
			return &entry
		}
	}

	return nil
}

func (n *Node) deleteAndAfter(idx LogIndex) {
	for i, entry := range n.Log {
		if entry.Index == idx {
			n.Log = n.Log[:i]
		}
	}
}

func (n *Node) addEntry(entry LogEntry) {
	n.Log = append(n.Log, entry)
}
