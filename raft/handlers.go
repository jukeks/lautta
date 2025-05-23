package lautta

import (
	"errors"
	"math/rand"
	"sort"
	"time"
)

func (n *Node) handleTick() {
	if n.State == Follower && time.Since(n.LastHeartbeat) > HeartbeatTimeout {
		n.logger.Println("starting elections")
		n.startElection()
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
		existingEntry, err := n.Log.Get(newEntry.Index)
		if err != nil {
			n.logger.Fatalf("failed to read log: %v", err)
		}
		if existingEntry.Index != 0 {
			if existingEntry.Term != newEntry.Term {
				n.Log.DeleteFrom(newEntry.Index)
				n.Log.Add(newEntry)
				continue
			}
		} else {
			n.Log.Add(newEntry)
		}
	}

	prevLog, err := n.Log.Get(req.PrevLogIndex)
	if err != nil {
		n.logger.Fatalf("failed to read log: %v", err)
	}

	prevLogMismatch := prevLog.Index == 0 || prevLog.Term != req.PrevLogTerm
	n.logger.Printf("prevLogMismatch: %t req.PrevLog: index: %d term: %d",
		prevLogMismatch, req.PrevLogIndex, req.PrevLogTerm)

	if req.PrevLogIndex == 0 {
		// startup
		prevLogMismatch = false
	}

	if req.LeaderCommit > n.CommitIndex {
		lastLog, err := n.Log.GetLastLog()
		if err != nil {
			n.logger.Fatalf("failed to read log: %v", err)
		}

		prevCommit := n.CommitIndex
		n.CommitIndex = min(lastLog.Index, req.LeaderCommit)
		logs, err := n.Log.GetBetween(prevCommit+1, n.CommitIndex)
		if err != nil {
			n.logger.Fatalf("failed to read log: %v", err)
		}

		for _, log := range logs {
			if err := n.fsm.Apply(log); err != nil {
				n.logger.Fatalf("failed to apply log: %v", err)
			}
		}
	}

	req.Ret <- AppendEntriesResponse{
		Term:    n.CurrentTerm,
		Success: !olderTerm && !prevLogMismatch,
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

func (n *Node) getOperationsSorted() []LogIndex {
	operations := []LogIndex{}
	for idx := range n.ongoingOperations {
		operations = append(operations, idx)
	}

	sort.Slice(operations, func(i, j int) bool { return operations[i] < operations[j] })
	return operations
}

func (n *Node) checkCommitProgress() {
	commitIndex := n.getMajorityIndex()
	n.logger.Printf("checkCommitProgress: majority %d vs leader %d", commitIndex, n.CommitIndex)
	if commitIndex <= n.CommitIndex {
		return
	}

	n.logger.Printf("xxxx committed %d", commitIndex)

	n.CommitIndex = commitIndex
	toDelete := []LogIndex{}
	for _, idx := range n.getOperationsSorted() {
		req := n.ongoingOperations[idx]
		if idx <= commitIndex {
			log, err := n.Log.Get(idx)
			if err != nil {
				n.logger.Fatalf("failed to read log: %v", err)
			}
			if err := n.fsm.Apply(log); err != nil {
				n.logger.Fatalf("failed to apply log: %v", err)
			}

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
	lastLog, err := n.Log.GetLastLog()
	if err != nil {
		n.logger.Fatalf("failed to read log: %v", err)
	}

	earliestInLastReq := firstOr(req.Entries, lastLog)
	entries, err := n.Log.GetFrom(earliestInLastReq.Index - 1)
	if err != nil {
		n.logger.Fatalf("failed to read log: %v", err)
	}

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
	if req.Term > n.CurrentTerm {
		n.CurrentTerm = req.Term
		n.VotedFor = nil
		n.State = Follower // TODO check
	}

	olderTerm := req.Term < n.CurrentTerm
	lastLog, err := n.Log.GetLastLog()
	if err != nil {
		n.logger.Fatalf("failed to read log: %v", err)
	}

	olderIndex := lastLog.Index > req.LastLogIndex
	canVote := n.VotedFor == nil || *n.VotedFor == req.CandidateID
	voteGranted := !olderTerm && !olderIndex && canVote

	if voteGranted {
		votedFor := req.CandidateID
		n.VotedFor = &votedFor
	}

	resp := RequestVoteResponse{
		Term:        n.CurrentTerm,
		VoteGranted: voteGranted,
	}

	n.logger.Printf("vote request from node %d: %+v. resp: %+v", req.CandidateID, req, resp)
	req.Ret <- resp
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

		lastLog, err := n.Log.GetLastLog()
		if err != nil {
			n.logger.Fatalf("failed to read log: %v", err)
		}

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
		n.LastHeartbeat = time.Now().Add(time.Duration(rand.Int63n(int64(HeartbeatTimeout * 2))))
	}
}

func (n *Node) startElection() {
	n.State = Candidate
	n.LastElection = time.Now()
	n.CurrentTerm++
	votedFor := n.config.ID
	n.VotedFor = &votedFor
	n.votes = 1
	n.voteResponsesReceived = 0

	lastLog, err := n.Log.GetLastLog()
	if err != nil {
		n.logger.Fatalf("failed to read log: %v", err)
	}

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
	lastLog, err := n.Log.GetLastLog()
	if err != nil {
		n.logger.Fatalf("failed to read log: %v", err)
	}

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

	lastLog, err := n.Log.GetLastLog()
	if err != nil {
		n.logger.Fatalf("failed to read log: %v", err)
	}

	entry := LogEntry{
		Term:    n.CurrentTerm,
		Index:   lastLog.Index + 1,
		Payload: req.Payload,
	}
	n.Log.Add(entry)
	n.ongoingOperations[entry.Index] = req

	n.replicate()
}

func (n *Node) replicate() {
	lastLog, err := n.Log.GetLastLog()
	if err != nil {
		n.logger.Fatalf("failed to read log: %v", err)
	}

	for nodeID, nextIndex := range n.Leader.NextIndex {
		entry, err := n.Log.Get(nextIndex)
		if err != nil {
			n.logger.Fatalf("failed to read log: %v", err)
		}

		n.comms.AppendEntriesRequestsOut <- AppendEntriesRequest{
			Term:         n.CurrentTerm,
			LeaderID:     n.config.ID,
			PrevLogIndex: lastLog.Index,
			PrevLogTerm:  lastLog.Term,
			LeaderCommit: n.CommitIndex,
			TargetNode:   nodeID,
			Entries:      []LogEntry{entry},
		}
	}
}
