package lautta

type AppendEntriesRequest struct {
	Term     TermID
	LeaderID NodeID

	PrevLogIndex LogIndex
	PrevLogTerm  TermID

	Entries []LogEntry

	LeaderCommit LogIndex
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
}

type RequestVoteResponse struct {
	Term        TermID
	VoteGranted bool
}
