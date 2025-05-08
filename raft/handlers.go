package lautta

import (
	"errors"
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

func (n *Node) handleAppendEntriesRequest(req AppendEntriesRequest) {
	n.logger.Printf("append entries req: %+v", req)

	if req.Term > n.CurrentTerm {
		n.CurrentTerm = req.Term
		n.VotedFor = nil
		n.State = Follower
	}

	n.logger.Printf("got %d entries", len(req.Entries))

	olderTerm := req.Term < n.CurrentTerm
	for _, newEntry := range req.Entries {
		existingEntry := n.getLog(newEntry.Index)
		if existingEntry != nil {
			if existingEntry.Term != newEntry.Term {
				n.deleteFrom(newEntry.Index)
				n.addEntry(newEntry)
				continue
			}
		} else {
			n.addEntry(newEntry)
		}
	}

	// should this be resolved before or after adding entries?
	prevLog := n.getLog(req.PrevLogIndex)
	prevLogMismatch := prevLog == nil || prevLog.Term != req.PrevLogTerm

	if req.LeaderCommit > n.CommitIndex {
		lastLog := n.getLastLog()
		n.CommitIndex = min(lastLog.Index, req.LeaderCommit)
		// TODO commit FSM
	}

	req.Ret <- AppendEntriesResponse{
		Term:    n.CurrentTerm,
		Success: !olderTerm || !prevLogMismatch,
	}
	n.LastHeartbeat = time.Now()
}

func firstOr[T any](arr []T, _default T) T {
	if len(arr) == 0 {
		return _default
	}

	return arr[0]
}

func (n *Node) getMajorityCount() int {
	return len(n.config.Peers)/2 + 1
}

func (n *Node) getMajorityIndex() LogIndex {
	majority := n.getMajorityCount()
	maxIndex := LogIndex(0)
	for _, indexA := range n.Leader.MatchIndex {
		atleastHere := 0
		for _, indexB := range n.Leader.MatchIndex {
			if indexA <= indexB {
				atleastHere++
			}
		}

		if atleastHere >= majority && indexA > maxIndex {
			maxIndex = indexA
		}
	}

	return maxIndex
}

func (n *Node) checkCommitProgress() {
	commitIndex := n.getMajorityIndex()
	if commitIndex <= n.CommitIndex {
		return
	}

	n.logger.Printf("committed %d", commitIndex)

	n.CommitIndex = commitIndex
	toDelete := []LogIndex{}
	for idx, req := range n.ongoingOperations {
		if idx <= commitIndex {
			req.Ret <- ProposeResponse{}
			toDelete = append(toDelete, idx)
		}
	}

	for _, idx := range toDelete {
		delete(n.ongoingOperations, idx)
	}
}

func (n *Node) handleAppendEntriesResponse(resp AppendEntriesResponse) {
	n.logger.Printf("append entries resp: %+v", resp)
	req := resp.Request
	peer := req.TargetNode

	if resp.Success {
		n.Leader.MatchIndex[peer] = req.PrevLogIndex
		n.Leader.NextIndex[peer] = req.PrevLogIndex + 1
		n.checkCommitProgress()
		return
	}

	// one at a time try how far logs are missing
	lastLog := n.getLastLog()
	earliestInLastReq := firstOr(req.Entries, lastLog)
	entries := n.getFrom(earliestInLastReq.Index - 1)

	n.comms.AppendEntriesRequestsOut <- AppendEntriesRequest{
		Term:         n.CurrentTerm,
		LeaderID:     n.config.ID,
		PrevLogIndex: lastLog.Index,
		PrevLogTerm:  lastLog.Term,
		LeaderCommit: n.CommitIndex,
		TargetNode:   peer,
		Entries:      entries,
	}
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

func (n *Node) handleProposeRequest(req ProposeRequest) {
	n.logger.Printf("got write request: %+v. %s", req, req.Payload)
	if n.State != Leader {
		req.Ret <- ProposeResponse{
			Err: errors.New("not a leader"),
		}
		return
	}

	lastLog := n.getLastLog()
	entry := LogEntry{
		Term:    n.CurrentTerm,
		Index:   lastLog.Index + 1,
		Payload: req.Payload,
	}
	n.addEntry(entry)
	n.ongoingOperations[entry.Index] = req

	n.replicate()
}

func (n *Node) replicate() {
	lastLog := n.getLastLog()
	for nodeID, nextIndex := range n.Leader.NextIndex {
		entry := n.getLog(nextIndex)
		n.comms.AppendEntriesRequestsOut <- AppendEntriesRequest{
			Term:         n.CurrentTerm,
			LeaderID:     n.config.ID,
			PrevLogIndex: lastLog.Index,
			PrevLogTerm:  lastLog.Term,
			LeaderCommit: n.CommitIndex,
			TargetNode:   nodeID,
			Entries:      []LogEntry{*entry},
		}
	}
}
