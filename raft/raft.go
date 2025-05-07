package lautta

import (
	"log"
	"os"
	"time"
)

type NodeID int
type TermID int
type LogIndex int

type LogEntry struct {
	Term    TermID
	Index   LogIndex
	Payload []byte
}

type Peer struct {
	ID      NodeID
	Address string
}

type Config struct {
	ID      NodeID
	Address string
	Peers   []Peer
}

type NodeState int

const (
	Follower NodeState = iota
	Candidate
	Leader
)

type LeaderState struct {
	NextIndex  map[NodeID]LogIndex
	MatchIndex map[NodeID]LogIndex
}

const (
	Tick             = 1000 * time.Millisecond
	HeartbeatTimeout = 5 * time.Second
	ElectionTimeout  = 1 * time.Second
)

type Node struct {
	config Config

	// persisted
	CurrentTerm TermID
	VotedFor    *NodeID
	Log         []LogEntry

	// volatile
	CommitIndex LogIndex
	LastApplied LogIndex

	LastHeartbeat         time.Time
	LastElection          time.Time
	votes                 int
	voteResponsesReceived int

	State NodeState

	Leader *LeaderState

	Quit chan bool
	Done chan bool

	logger *log.Logger

	comms Comms
}

func NewNode(config Config, comms Comms) *Node {
	return &Node{
		config: config,
		comms:  comms,

		CurrentTerm: 0,
		VotedFor:    nil,
		Log:         []LogEntry{},

		CommitIndex: 0,
		LastApplied: 0,

		State:  Follower,
		Leader: nil,

		LastHeartbeat: time.Now(),

		Quit: make(chan bool, 1),
		Done: make(chan bool, 1),

		logger: log.New(os.Stderr, "[raft] ", log.Lmicroseconds),
	}
}

func (n *Node) getLastLog() LogEntry {
	lastLog := LogEntry{}
	if len(n.Log) > 0 {
		lastLog = n.Log[len(n.Log)-1]
	}

	return lastLog
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
	n.logger.Printf("starting node %d", n.config.ID)

loop:
	for {
		select {
		case <-time.After(Tick):
			n.handleTick()

		case appendEntryReq := <-n.comms.AppendEntriesRequestsIn:
			n.handleAppendEntriesRequest(appendEntryReq)

		case appendEntryResp := <-n.comms.AppendEntriesResponsesIn:
			n.handleAppendEntriesResponse(appendEntryResp)

		case voteRequest := <-n.comms.RequestVoteRequestsIn:
			n.handleVoteRequest(voteRequest)

		case voteResponse := <-n.comms.RequestVoteResponsesIn:
			n.handleVoteResponse(voteResponse)

		case proposeReq := <-n.comms.ProposeRequestsIn:
			n.handleProposeRequest(proposeReq)

		case <-n.Quit:
			n.logger.Println("quitting")
			break loop
		}
	}

	n.Done <- true
}
