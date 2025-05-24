package lautta

type StableStore interface {
	Store(currentTerm TermID, votedFor *NodeID) error
	Restore() (TermID, *NodeID, error)
}

type InMemStableStore struct {
	currentTerm TermID
	votedFor    *NodeID
}

func NewInMemStableStore() StableStore {
	return &InMemStableStore{}
}

func (s *InMemStableStore) Store(currentTerm TermID, votedFor *NodeID) error {
	s.currentTerm = currentTerm
	s.votedFor = votedFor
	return nil
}

func (s *InMemStableStore) Restore() (TermID, *NodeID, error) {
	return s.currentTerm, s.votedFor, nil
}
