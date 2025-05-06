package lautta

import (
	"context"
	"log"
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

type Node struct {
	config Config

	// persisted
	CurrentTerm TermID
	VotedFor    *NodeID
	Log         []LogEntry

	// volatile
	CommitIndex LogIndex
	LastApplied LogIndex

	LastHeartbeat time.Time

	State NodeState

	Leader *LeaderState

	AppendEntriesRequests  chan AppendEntriesRequest
	AppendEntriesResponses chan AppendEntriesResponse
	VoteRequests           chan RequestVoteRequest
	VoteResponses          chan RequestVoteResponse
	Quit                   chan bool
	Done                   chan bool

	logger *log.Logger
}

type LeaderState struct {
	NextIndex  map[NodeID]LogIndex
	MatchIndex map[NodeID]LogIndex
}

const (
	Tick            = 1000 * time.Millisecond
	ElectionTimeout = 5 * time.Second
)

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

		logger: log.Default(),
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

loop:
	for {
		select {
		case <-time.After(Tick):
			if n.State == Follower && time.Since(n.LastHeartbeat) > ElectionTimeout {
				n.logger.Println("starting elections")
				n.runElection(peerClients)
			}
			if n.State == Leader {
				n.sendHeartbeats(peerClients)
			}

		case appendEntryReq := <-n.AppendEntriesRequests:
			n.logger.Printf("append entries req: %+v", appendEntryReq)

			if appendEntryReq.Term > n.CurrentTerm {
				n.CurrentTerm = appendEntryReq.Term
				n.VotedFor = nil
			}

			appendEntryReq.Ret <- AppendEntriesResponse{
				Term:    n.CurrentTerm,
				Success: true, // XXXX
			}
			n.LastHeartbeat = time.Now()

		case appendEntryResp := <-n.AppendEntriesResponses:
			n.logger.Printf("append entries resp: %+v", appendEntryResp)

		case voteRequest := <-n.VoteRequests:
			n.logger.Printf("vote request: %+v", voteRequest)
			if voteRequest.Term > n.CurrentTerm {
				n.CurrentTerm = voteRequest.Term
				n.VotedFor = nil
			}

			olderTerm := voteRequest.Term < n.CurrentTerm
			lastLog := n.getLastLog()
			olderIndex := lastLog.Index > voteRequest.LastLogIndex
			canVote := n.VotedFor == nil || *n.VotedFor == voteRequest.CandidateID
			voteGranted := !olderTerm && !olderIndex && canVote

			if voteGranted {
				votedFor := voteRequest.CandidateID
				n.VotedFor = &votedFor
			}

			voteRequest.Ret <- RequestVoteResponse{
				Term:        n.CurrentTerm,
				VoteGranted: voteGranted,
			}
		case <-n.Quit:
			n.logger.Println("quitting")
			break loop
		}
	}

	n.Done <- true
}

func (n *Node) runElection(peers map[NodeID]raftv1.RaftServiceClient) {
	n.State = Candidate
	n.CurrentTerm++

	votes := 1
	for _, peer := range peers {
		resp, err := peer.RequestVote(context.Background(), &raftv1.RequestVoteRequest{
			Term:         int64(n.CurrentTerm),
			CandidateId:  int64(n.config.ID),
			LastLogIndex: int64(n.CommitIndex),
		})

		if err != nil {
			n.logger.Printf("error requesting vote: %v", err)
			continue
		}

		if resp.VoteGranted {
			votes += 1
		}
	}

	nNodes := len(peers) + 1
	if votes > (nNodes/2)+1 {
		n.logger.Printf("node %d won election", n.config.ID)
		n.State = Leader

		lastLog := n.getLastLog()

		n.Leader = &LeaderState{
			NextIndex:  make(map[NodeID]LogIndex),
			MatchIndex: make(map[NodeID]LogIndex),
		}
		for _, peer := range n.config.Peers {
			n.Leader.NextIndex[peer.ID] = lastLog.Index + 1
			n.Leader.MatchIndex[peer.ID] = 0
		}

		n.sendHeartbeats(peers)
	}
}

func (n *Node) sendHeartbeats(peers map[NodeID]raftv1.RaftServiceClient) {
	lastLog := n.getLastLog()
	for _, peer := range peers {
		go func() {
			resp, err := peer.AppendEntries(context.Background(), &raftv1.AppendEntriesRequest{
				Term:         int64(n.CurrentTerm),
				LeaderId:     int64(n.config.ID),
				PrevLogIndex: int64(lastLog.Index),
				PrevLogTerm:  int64(lastLog.Term),
				LeaderCommit: int64(n.CommitIndex),
			})

			if err != nil {
				n.logger.Printf("error heartbeating: %v", err)

			}
			n.AppendEntriesResponses <- AppendEntriesResponse{
				Term:    TermID(resp.Term),
				Success: resp.Success,
			}
		}()
	}
}
