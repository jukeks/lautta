package lautta

import (
	"fmt"
	"log"
	"math/rand"
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
	Tick             = 250 * time.Millisecond
	HeartbeatTimeout = 500 * time.Millisecond
	ElectionTimeout  = 1 * time.Second
)

type Node struct {
	config Config

	// persisted
	CurrentTerm TermID
	VotedFor    *NodeID
	logStore    LogStore
	stableStore StableStore

	// volatile
	CommitIndex LogIndex
	LastApplied LogIndex

	LastHeartbeat         time.Time
	LastElection          time.Time
	votes                 int
	voteResponsesReceived int

	State NodeState

	Leader *LeaderState

	ongoingOperations map[LogIndex]ProposeRequest

	Quit chan bool
	Done chan bool

	logger *log.Logger

	comms Comms

	fsm FSM
}

func NewNode(config Config, comms Comms, fsm FSM, logStore LogStore, stableStore StableStore) *Node {
	prefix := fmt.Sprintf("[node %d] ", config.ID)
	return &Node{
		config: config,
		comms:  comms,

		CurrentTerm: 0,
		VotedFor:    nil,
		logStore:    NewLastLogCache(logStore),
		stableStore: stableStore,

		CommitIndex: 0,
		LastApplied: 0,

		State:  Follower,
		Leader: nil,

		LastHeartbeat: time.Now(),

		ongoingOperations: make(map[LogIndex]ProposeRequest),

		Quit: make(chan bool, 1),
		Done: make(chan bool, 1),

		logger: log.New(os.Stderr, prefix, 0),

		fsm: fsm,
	}
}

func (n *Node) Stop() {
	n.Quit <- true
	<-n.Done
}

func (n *Node) Run() {
	n.logger.Printf("starting node %d", n.config.ID)
	var err error
	n.CurrentTerm, n.VotedFor, err = n.stableStore.Restore()
	if err != nil {
		n.logger.Fatalf("failed to restore from stable store: %v", err)
	}

	// jitter for randomizing startup election
	<-time.After(time.Duration(rand.Int63n(int64(HeartbeatTimeout))))
	ticker := time.NewTicker(Tick)

loop:
	for {
		select {
		case <-ticker.C:
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
