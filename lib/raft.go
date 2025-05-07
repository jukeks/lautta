package lautta

import (
	"log"
	"os"
	"time"

	raftv1 "github.com/jukeks/lautta/proto/gen/lautta/rpc/raft/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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

	AppendEntriesRequests  chan AppendEntriesRequest
	AppendEntriesResponses chan AppendEntriesResponse
	VoteRequests           chan RequestVoteRequest
	VoteResponses          chan RequestVoteResponse
	Quit                   chan bool
	Done                   chan bool

	logger *log.Logger

	peerClients map[NodeID]raftv1.RaftServiceClient
}

func NewNode(config Config) *Node {
	return &Node{
		config: config,

		CurrentTerm: 0,
		VotedFor:    nil,
		Log:         []LogEntry{},

		CommitIndex: 0,
		LastApplied: 0,

		State:  Follower,
		Leader: nil,

		LastHeartbeat: time.Now(),

		AppendEntriesRequests:  make(chan AppendEntriesRequest, 10),
		AppendEntriesResponses: make(chan AppendEntriesResponse, 10),
		VoteRequests:           make(chan RequestVoteRequest, 10),
		VoteResponses:          make(chan RequestVoteResponse, 10),
		Quit:                   make(chan bool, 1),
		Done:                   make(chan bool, 1),

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

func (n *Node) initPeerClients() {
	peerClients := make(map[NodeID]raftv1.RaftServiceClient)
	for _, peer := range n.config.Peers {
		n.logger.Printf("peer: %d -> %s", peer.ID, peer.Address)
		c, err := grpc.NewClient(peer.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			n.logger.Fatalf("failed to connect to peer %d: %v", peer.ID, err)
		}
		client := raftv1.NewRaftServiceClient(c)
		peerClients[peer.ID] = client
	}

	n.peerClients = peerClients
}

func (n *Node) Run() {
	n.logger.Printf("starting node %d", n.config.ID)
	n.initPeerClients()

loop:
	for {
		select {
		case <-time.After(Tick):
			n.handleTick()

		case appendEntryReq := <-n.AppendEntriesRequests:
			n.handleAppendEntriesRequest(appendEntryReq)

		case appendEntryResp := <-n.AppendEntriesResponses:
			n.handleAppendEntriesResponse(appendEntryResp)

		case voteRequest := <-n.VoteRequests:
			n.handleVoteRequest(voteRequest)

		case voteResponse := <-n.VoteResponses:
			n.handleVoteResponse(voteResponse)

		case <-n.Quit:
			n.logger.Println("quitting")
			break loop
		}
	}

	n.Done <- true
}
