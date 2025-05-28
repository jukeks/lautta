package lautta

import (
	"errors"
	"math/rand"
	"os"
	"sort"
	"time"
)

func (n *Node) Fatal(s string, args ...any) {
	n.logger.Error(s, args...)
	os.Exit(1)
}

func (n *Node) handleTick() {
	if n.state == Follower && time.Since(n.lastHeartbeat) > HeartbeatTimeout {
		n.logger.Info("starting elections")
		n.startElection()
	}
	if n.state == Candidate && time.Since(n.lastElection) > ElectionTimeout {
		n.logger.Info("Ending elections with timeout")
		n.handleElectionResults()
	}
	if n.state == Leader {
		n.sendHeartbeats()
	}
}

func (n *Node) handleNewerTerm(newTerm TermID) {
	// always reset to follower
	wasLeader := n.state == Leader

	n.logger.Info("newer term", "newTerm", newTerm, "current", n.currentTerm)
	n.state = Follower
	n.votedFor = nil
	n.currentTerm = newTerm

	n.lastHeartbeat = time.Now()
	n.votes = 0
	n.voteResponsesReceived = 0

	if wasLeader {
		n.leader = nil
		// cancel ongoing operations
		if len(n.ongoingOperations) > 0 {
			n.logger.Debug("canceling ongoing operations due to newer term")
			for _, req := range n.ongoingOperations {
				req.Ret <- ProposeResponse{
					Err: errors.New("leader changed"),
				}
			}
		}
	}
	n.ongoingOperations = make(map[LogIndex]ProposeRequest)
}

func (n *Node) handleAppendEntriesRequest(req AppendEntriesRequest) {
	n.logger.Debug("append entries", "req", req, "entries", len(req.Entries))

	if req.Term > n.currentTerm {
		n.handleNewerTerm(req.Term)
	}

	olderTerm := req.Term < n.currentTerm
	for _, newEntry := range req.Entries {
		existingEntry, err := n.logStore.Get(newEntry.Index)
		if err != nil {
			n.Fatal("failed to read log: %v", err)
		}
		if existingEntry.Index != 0 {
			if existingEntry.Term != newEntry.Term {
				n.logStore.DeleteFrom(newEntry.Index)
				n.logStore.Add(newEntry)
				continue
			}
		} else {
			n.logStore.Add(newEntry)
		}
	}

	prevLog, err := n.logStore.Get(req.PrevLogIndex)
	if err != nil {
		n.Fatal("failed to read log: %v", err)
	}

	prevLogMismatch := prevLog.Index == 0 || prevLog.Term != req.PrevLogTerm
	n.logger.Debug("result", "prevLogMismatch", prevLogMismatch,
		"req.PrevLogIndex", req.PrevLogIndex, "req.PrevLogTerm", req.PrevLogTerm)

	if req.PrevLogIndex == 0 {
		// startup
		prevLogMismatch = false
	}

	if req.LeaderCommit > n.commitIndex {
		lastLog, err := n.logStore.GetLastLog()
		if err != nil {
			n.Fatal("failed to read last log: %v", err)
		}

		prevCommit := n.commitIndex
		n.commitIndex = min(lastLog.Index, req.LeaderCommit)
		logs, err := n.logStore.GetBetween(prevCommit+1, n.commitIndex)
		if err != nil {
			n.Fatal("failed to read log: %v", err)
		}

		for _, log := range logs {
			if err := n.fsm.Apply(log); err != nil {
				n.Fatal("failed to apply log: %v", err)
			}
		}
	}

	if err := n.stableStore.Store(n.currentTerm, n.votedFor); err != nil {
		n.Fatal("failed to store state: %v", err)
	}

	req.Ret <- AppendEntriesResponse{
		Term:    n.currentTerm,
		Success: !olderTerm && !prevLogMismatch,
	}
	n.lastHeartbeat = time.Now()
}

func firstOr[T any](arr []T, _default T) T {
	if len(arr) == 0 {
		return _default
	}

	return arr[0]
}

func (n *Node) getMajorityCount() int {
	nodes := len(n.config.Peers) + 1 // including self
	return nodes/2 + 1
}

func (n *Node) getMajorityIndex() LogIndex {
	majority := n.getMajorityCount()
	maxIndex := LogIndex(0)
	for _, indexA := range n.leader.MatchIndex {
		atleastHere := 1 // count self
		for _, indexB := range n.leader.MatchIndex {
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
	n.logger.Debug("checkCommitProgress", "majority", commitIndex, "leader", n.commitIndex)
	if commitIndex <= n.commitIndex {
		return
	}

	n.logger.Debug("committed", "commitIndex", commitIndex)

	n.commitIndex = commitIndex
	toDelete := []LogIndex{}
	for _, idx := range n.getOperationsSorted() {
		req := n.ongoingOperations[idx]
		if idx <= commitIndex {
			log, err := n.logStore.Get(idx)
			if err != nil {
				n.Fatal("failed to read log: %v", err)
			}
			if err := n.fsm.Apply(log); err != nil {
				n.Fatal("failed to apply log: %v", err)
			}

			req.Ret <- ProposeResponse{}
			toDelete = append(toDelete, idx)
		}
	}

	for _, idx := range toDelete {
		delete(n.ongoingOperations, idx)
	}

	n.sendHeartbeats()
}

func (n *Node) handleAppendEntriesResponse(resp AppendEntriesResponse) {
	n.logger.Debug("append entries resp", "resp", resp)
	req := resp.Request
	peer := req.TargetNode
	if n.state != Leader {
		n.logger.Info("got append entries response but not a leader, ignoring")
		return
	}

	if resp.Term > n.currentTerm {
		n.handleNewerTerm(resp.Term)
		if err := n.stableStore.Store(n.currentTerm, n.votedFor); err != nil {
			n.Fatal("failed to store state: %v", err)
		}
		n.logger.Info("newer term received from append entries response, is now follower")
		return
	}

	if resp.Success {
		n.leader.MatchIndex[peer] = req.PrevLogIndex
		n.leader.NextIndex[peer] = req.PrevLogIndex + 1
		n.checkCommitProgress()
		return
	}

	// one at a time try how far logs are missing
	lastLog, err := n.logStore.GetLastLog()
	if err != nil {
		n.Fatal("failed to read log: %v", err)
	}

	earliestInLastReq := firstOr(req.Entries, lastLog)
	entries, err := n.logStore.GetFrom(earliestInLastReq.Index - 1)
	if err != nil {
		n.Fatal("failed to read log: %v", err)
	}

	n.comms.AppendEntriesRequestsOut <- AppendEntriesRequest{
		Term:         n.currentTerm,
		LeaderID:     n.config.ID,
		PrevLogIndex: lastLog.Index,
		PrevLogTerm:  lastLog.Term,
		LeaderCommit: n.commitIndex,
		TargetNode:   peer,
		Entries:      entries,
	}
}

func (n *Node) handleVoteRequest(req RequestVoteRequest) {
	if req.Term > n.currentTerm {
		n.handleNewerTerm(req.Term)
	}

	olderTerm := req.Term < n.currentTerm
	lastLog, err := n.logStore.GetLastLog()
	if err != nil {
		n.Fatal("failed to read log: %v", err)
	}

	olderIndex := lastLog.Index > req.LastLogIndex
	canVote := n.votedFor == nil || *n.votedFor == req.CandidateID
	voteGranted := !olderTerm && !olderIndex && canVote

	if voteGranted {
		votedFor := req.CandidateID
		n.votedFor = &votedFor
	}

	// otherwise we would start a new election before the current one is done
	n.lastHeartbeat = time.Now()

	if err := n.stableStore.Store(n.currentTerm, n.votedFor); err != nil {
		n.Fatal("failed to store state: %v", err)
	}

	resp := RequestVoteResponse{
		Term:        n.currentTerm,
		VoteGranted: voteGranted,
	}

	n.logger.Debug("vote request node", "from", req.CandidateID, "req", req, "resp", resp)
	req.Ret <- resp
}

func (n *Node) handleVoteResponse(resp RequestVoteResponse) {
	n.logger.Info("vote response", "resp", resp)
	if n.state != Candidate {
		n.logger.Info("got vote response but not a candidate, ignoring")
		return
	}

	if resp.Term > n.currentTerm {
		n.handleNewerTerm(resp.Term)

		if err := n.stableStore.Store(n.currentTerm, n.votedFor); err != nil {
			n.Fatal("failed to store state: %v", err)
		}

		n.logger.Info("newer term received from vote response, node is now follower")
		return
	}

	n.voteResponsesReceived += 1
	if resp.VoteGranted {
		n.votes += 1
	}

	if n.voteResponsesReceived == len(n.config.Peers) {
		n.logger.Info("got all vote responses")
		n.handleElectionResults()
	}
}

func (n *Node) handleElectionResults() {
	nNodes := len(n.config.Peers) + 1
	if n.votes >= (nNodes/2)+1 {
		n.logger.Info("won election")
		n.state = Leader

		lastLog, err := n.logStore.GetLastLog()
		if err != nil {
			n.Fatal("failed to read log: %v", err)
		}

		n.leader = &LeaderState{
			NextIndex:  make(map[NodeID]LogIndex),
			MatchIndex: make(map[NodeID]LogIndex),
		}
		for _, peer := range n.config.Peers {
			n.leader.NextIndex[peer.ID] = lastLog.Index + 1
			n.leader.MatchIndex[peer.ID] = 0
		}

		n.sendHeartbeats()
	} else {
		n.logger.Info("lost election")
		n.state = Follower
		// add jitter to prevent stalemate
		n.lastHeartbeat = time.Now().Add(time.Duration(rand.Int63n(int64(HeartbeatTimeout * 2))))
	}
}

func (n *Node) startElection() {
	n.state = Candidate
	n.lastElection = time.Now()
	n.currentTerm++
	votedFor := n.config.ID
	n.votedFor = &votedFor
	n.votes = 1
	n.voteResponsesReceived = 0

	lastLog, err := n.logStore.GetLastLog()
	if err != nil {
		n.Fatal("failed to read log: %v", err)
	}

	if err := n.stableStore.Store(n.currentTerm, n.votedFor); err != nil {
		n.Fatal("failed to store state: %v", err)
	}

	for _, peer := range n.config.Peers {
		n.comms.RequestVoteRequestsOut <- RequestVoteRequest{
			Term:         n.currentTerm,
			CandidateID:  n.config.ID,
			LastLogIndex: lastLog.Index,
			LastLogTerm:  lastLog.Term,
			TargetNode:   peer.ID,
		}
	}
}

func (n *Node) sendHeartbeats() {
	lastLog, err := n.logStore.GetLastLog()
	if err != nil {
		n.Fatal("failed to read log: %v", err)
	}

	for _, peer := range n.config.Peers {
		n.comms.AppendEntriesRequestsOut <- AppendEntriesRequest{
			Term:         n.currentTerm,
			LeaderID:     n.config.ID,
			PrevLogIndex: lastLog.Index,
			PrevLogTerm:  lastLog.Term,
			LeaderCommit: n.commitIndex,
			TargetNode:   peer.ID,
		}
	}
}

func (n *Node) handleProposeRequest(req ProposeRequest) {
	n.logger.Debug("got write request", "req", req, "payload", req.Payload)
	if n.state != Leader {
		req.Ret <- ProposeResponse{
			Err: errors.New("not a leader"),
		}
		return
	}

	lastLog, err := n.logStore.GetLastLog()
	if err != nil {
		n.Fatal("failed to read log: %v", err)
	}

	entry := LogEntry{
		Term:    n.currentTerm,
		Index:   lastLog.Index + 1,
		Payload: req.Payload,
	}

	if err := n.logStore.Add(entry); err != nil {
		n.Fatal("failed to add log entry: %v", err)
	}
	n.ongoingOperations[entry.Index] = req

	n.replicate()
}

func (n *Node) replicate() {
	lastLog, err := n.logStore.GetLastLog()
	if err != nil {
		n.Fatal("failed to read log: %v", err)
	}

	for nodeID, nextIndex := range n.leader.NextIndex {
		entry, err := n.logStore.Get(nextIndex)
		if err != nil {
			n.Fatal("failed to read log: %v", err)
		}

		n.comms.AppendEntriesRequestsOut <- AppendEntriesRequest{
			Term:         n.currentTerm,
			LeaderID:     n.config.ID,
			PrevLogIndex: lastLog.Index,
			PrevLogTerm:  lastLog.Term,
			LeaderCommit: n.commitIndex,
			TargetNode:   nodeID,
			Entries:      []LogEntry{entry},
		}
	}
}
