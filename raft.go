package lautta

import "log"

type NodeID int
type TermID int
type LogEntry []byte
type LogIndex int

type Peer struct {
	Address string
}

type Node struct {
	// persisted
	CurrentTerm TermID
	VotedFor    *NodeID
	Log         []LogEntry

	// volatile
	CommitIndex LogIndex
	LastApplied LogIndex

	Leader *LeaderState

	peers []Peer

	appendEntries chan AppendEntriesRequest
	voteRequests  chan RequestVoteRequest

	logger *log.Logger
}

type LeaderState struct {
	NextIndex  map[NodeID]LogIndex
	MatchIndex map[NodeID]LogIndex
}

func NewNode() *Node {
	return &Node{
		CurrentTerm: 0,
		VotedFor:    nil,
		Log:         []LogEntry{},

		CommitIndex: 0,
		LastApplied: 0,

		Leader: nil,

		appendEntries: make(chan AppendEntriesRequest, 10),
		voteRequests:  make(chan RequestVoteRequest, 10),

		logger: log.Default(),
	}
}

func (n *Node) InitializeFromStableStorage() error {
	return nil
}

func (n *Node) StoreState() error {
	return nil
}

func (n *Node) Run() {
	for {
		select {
		case appendEntries := <-n.appendEntries:
			n.logger.Printf("append entries: %v", appendEntries)
		case voteRequest := <-n.voteRequests:
			n.logger.Printf("vote request: %v", voteRequest)
		}
	}
}
