package lautta

type AppendEntriesRequest struct {
	Term     TermID
	LeaderID NodeID

	PrevLogIndex LogIndex
	PrevLogTerm  TermID

	Entries []LogEntry

	LeaderCommit LogIndex

	Ret chan AppendEntriesResponse
}

type AppendEntriesResponse struct {
	Term    TermID
	Success bool
}

type RequestVoteRequest struct {
	Term         TermID
	CandidateID  NodeID
	LastLogIndex LogIndex
	LastLogTerm  TermID

	Ret chan RequestVoteResponse
}

type RequestVoteResponse struct {
	Term        TermID
	VoteGranted bool
}

type HeartbeatRequest struct {
	NodeID NodeID
	Ret    chan HeartbeatResponse
}

type HeartbeatResponse struct {
	NodeID NodeID
}
