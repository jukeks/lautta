package lautta

import (
	"math/rand"
	"time"
)

func (n *Node) handleTick() {
	if n.State == Follower && time.Since(n.LastHeartbeat) > HeartbeatTimeout {
		n.logger.Println("starting elections")
		n.runElection()
	}
	if n.State == Candidate && time.Since(n.LastElection) > ElectionTimeout {
		n.logger.Println("Ending elections with timeout")
		n.handleElectionResults()
	}
	if n.State == Leader {
		n.sendHeartbeats()
	}
}

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

func (n *Node) handleAppendEntriesRequest(req AppendEntriesRequest) {
	n.logger.Printf("append entries req: %+v", req)

	if req.Term > n.CurrentTerm {
		n.CurrentTerm = req.Term
		n.VotedFor = nil
		n.State = Follower
	}

	olderTerm := req.Term < n.CurrentTerm
	prevLog := n.findLog(req.PrevLogIndex)
	prevLogMismatch := prevLog == nil || prevLog.Term != req.PrevLogTerm

	for _, newEntry := range req.Entries {
		existingEntry := n.findLog(newEntry.Index)
		if existingEntry != nil {
			if existingEntry.Term != newEntry.Term {
				n.deleteAndAfter(newEntry.Index)
				break
			}
		} else {
			n.addEntry(newEntry)
		}
	}

	if req.LeaderCommit > n.CommitIndex {
		lastLog := n.getLastLog()
		n.CommitIndex = min(lastLog.Index, req.LeaderCommit)
	}

	req.Ret <- AppendEntriesResponse{
		Term:    n.CurrentTerm,
		Success: !olderTerm || prevLogMismatch,
	}
	n.LastHeartbeat = time.Now()
}

func (n *Node) handleAppendEntriesResponse(resp AppendEntriesResponse) {
	n.logger.Printf("append entries resp: %+v", resp)
}

func (n *Node) handleVoteRequest(req RequestVoteRequest) {
	n.logger.Printf("vote request: %+v", req)
	if req.Term > n.CurrentTerm {
		n.CurrentTerm = req.Term
		n.VotedFor = nil
		n.State = Follower // TODO check
	}

	olderTerm := req.Term < n.CurrentTerm
	lastLog := n.getLastLog()
	olderIndex := lastLog.Index > req.LastLogIndex
	canVote := n.VotedFor == nil || *n.VotedFor == req.CandidateID
	voteGranted := !olderTerm && !olderIndex && canVote

	if voteGranted {
		votedFor := req.CandidateID
		n.VotedFor = &votedFor
	}

	req.Ret <- RequestVoteResponse{
		Term:        n.CurrentTerm,
		VoteGranted: voteGranted,
	}
}

func (n *Node) handleVoteResponse(resp RequestVoteResponse) {
	n.logger.Printf("vote response: %+v", resp)
	if n.State != Candidate {
		n.logger.Printf("got vote response but not a candidate")
		return
	}
	n.voteResponsesReceived += 1
	if resp.Term > n.CurrentTerm {
		n.CurrentTerm = resp.Term
		n.VotedFor = nil
		n.State = Follower
	}
	if resp.VoteGranted {
		n.votes += 1
	}

	if n.voteResponsesReceived == len(n.config.Peers) {
		n.logger.Printf("got all vote responses")
		n.handleElectionResults()
	}
}

func (n *Node) handleElectionResults() {
	nNodes := len(n.config.Peers) + 1
	if n.votes >= (nNodes/2)+1 {
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

		n.sendHeartbeats()
	} else {
		n.logger.Printf("lost election")
		n.State = Follower
		// add jitter to prevent stalemate
		n.LastHeartbeat = time.Now().Add(time.Duration(rand.Int63n(int64(HeartbeatTimeout))))
	}
}

func (n *Node) runElection() {
	n.State = Candidate
	n.LastElection = time.Now()
	n.CurrentTerm++
	n.votes = 1
	n.voteResponsesReceived = 0

	lastLog := n.getLastLog()

	for _, peer := range n.config.Peers {
		n.comms.RequestVoteRequestsOut <- RequestVoteRequest{
			Term:         n.CurrentTerm,
			CandidateID:  n.config.ID,
			LastLogIndex: lastLog.Index,
			LastLogTerm:  lastLog.Term,
			TargetNode:   peer.ID,
		}
	}
}

func (n *Node) sendHeartbeats() {
	lastLog := n.getLastLog()
	for _, peer := range n.config.Peers {
		n.comms.AppendEntriesRequestsOut <- AppendEntriesRequest{
			Term:         n.CurrentTerm,
			LeaderID:     n.config.ID,
			PrevLogIndex: lastLog.Index,
			PrevLogTerm:  lastLog.Term,
			LeaderCommit: n.CommitIndex,
			TargetNode:   peer.ID,
		}
	}
}
