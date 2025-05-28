package lautta

import (
	"fmt"
	"log/slog"
	"math/rand"
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

func (s NodeState) String() string {
	switch s {
	case Follower:
		return "Follower"
	case Candidate:
		return "Candidate"
	case Leader:
		return "Leader"
	default:
		return fmt.Sprintf("State(%d)", s)
	}
}

type LeaderState struct {
	NextIndex  map[NodeID]LogIndex
	MatchIndex map[NodeID]LogIndex
}

const (
	TickDuration     = 50 * time.Millisecond
	HeartbeatTimeout = 500 * time.Millisecond
	ElectionTimeout  = 250 * time.Millisecond
)

type Node struct {
	config Config

	// persisted
	currentTerm TermID
	votedFor    *NodeID
	logStore    LogStore
	stableStore StableStore

	// volatile
	commitIndex LogIndex
	lastApplied LogIndex

	lastHeartbeat         time.Time
	lastElection          time.Time
	votes                 int
	voteResponsesReceived int

	state NodeState

	leader *LeaderState

	ongoingOperations map[LogIndex]ProposeRequest

	quit chan bool
	done chan bool

	logger *slog.Logger

	comms Comms

	fsm FSM
}

func NewNode(config Config, comms Comms, fsm FSM, logStore LogStore, stableStore StableStore) *Node {
	return &Node{
		config: config,
		comms:  comms,

		currentTerm: 0,
		votedFor:    nil,
		logStore:    NewLastLogCache(logStore),
		stableStore: stableStore,

		commitIndex: 0,
		lastApplied: 0,

		state:  Follower,
		leader: nil,

		lastHeartbeat: time.Now(),

		ongoingOperations: make(map[LogIndex]ProposeRequest),

		quit: make(chan bool, 1),
		done: make(chan bool, 1),

		logger: slog.New(slog.Default().Handler()).
			With("prefix", "raft").With("node_id", config.ID),

		fsm: fsm,
	}
}

func (n *Node) Stop() {
	n.quit <- true
	<-n.done
}

func (n *Node) Start() {
	go n.Run()
}

func (n *Node) Run() {
	n.logger.Info("starting node")
	var err error
	n.currentTerm, n.votedFor, err = n.stableStore.Restore()
	if err != nil {
		n.Fatal("failed to restore from stable store: %v", err)
	}

	// jitter for randomizing startup election
	<-time.After(time.Duration(rand.Int63n(int64(HeartbeatTimeout))))
	ticker := time.NewTicker(TickDuration)

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

		case <-n.quit:
			n.logger.Info("quitting")
			break loop
		}
	}

	n.done <- true
}
