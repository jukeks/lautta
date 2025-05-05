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
	ID NodeID

	// persisted
	CurrentTerm TermID
	VotedFor    *NodeID
	Log         []LogEntry

	// volatile
	CommitIndex LogIndex
	LastApplied LogIndex

	Leader *LeaderState

	peers []Peer

	AppendEntries chan AppendEntriesRequest
	VoteRequests  chan RequestVoteRequest
	Heartbeats    chan HeartbeatRequest
	Quit          chan bool
	Done          chan bool

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

		AppendEntries: make(chan AppendEntriesRequest, 10),
		VoteRequests:  make(chan RequestVoteRequest, 10),
		Heartbeats:    make(chan HeartbeatRequest, 10),
		Quit:          make(chan bool, 1),
		Done:          make(chan bool, 1),

		logger: log.Default(),
	}
}

func (n *Node) InitializeFromStableStorage() error {
	return nil
}

func (n *Node) StoreState() error {
	return nil
}

func (n *Node) Stop() {
	n.Quit <- true
	<-n.Done
}

func (n *Node) Run() {
loop:
	for {
		select {
		case appendEntries := <-n.AppendEntries:
			n.logger.Printf("append entries: %+v", appendEntries)
			appendEntries.Ret <- AppendEntriesResponse{}

		case voteRequest := <-n.VoteRequests:
			n.logger.Printf("vote request: %+v", voteRequest)
			voteRequest.Ret <- RequestVoteResponse{}

		case heartbeat := <-n.Heartbeats:
			n.logger.Printf("heartbeat: %+v", heartbeat)
			heartbeat.Ret <- HeartbeatResponse{NodeID: n.ID}

		case <-n.Quit:
			n.logger.Println("quitting")
			break loop
		}
	}

	n.Done <- true
}
