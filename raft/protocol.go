package lautta

type AppendEntriesRequest struct {
	Term     TermID
	LeaderID NodeID

	PrevLogIndex LogIndex
	PrevLogTerm  TermID

	Entries []LogEntry

	LeaderCommit LogIndex

	Ret        chan AppendEntriesResponse
	TargetNode NodeID
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

	Ret        chan RequestVoteResponse
	TargetNode NodeID
}

type RequestVoteResponse struct {
	Term        TermID
	VoteGranted bool
}

type ProposeRequest struct {
	Payload []byte

	Ret chan ProposeResponse
}

type ProposeResponse struct{}

type Comms struct {
	// messages to node
	AppendEntriesRequestsIn  chan AppendEntriesRequest
	AppendEntriesResponsesIn chan AppendEntriesResponse
	RequestVoteRequestsIn    chan RequestVoteRequest
	RequestVoteResponsesIn   chan RequestVoteResponse

	ProposeRequestsIn chan ProposeRequest

	// messages from node
	AppendEntriesRequestsOut chan AppendEntriesRequest
	RequestVoteRequestsOut   chan RequestVoteRequest
}

func NewComms() Comms {
	return Comms{
		AppendEntriesRequestsIn:  make(chan AppendEntriesRequest, 10),
		AppendEntriesResponsesIn: make(chan AppendEntriesResponse, 10),
		RequestVoteRequestsIn:    make(chan RequestVoteRequest, 10),
		RequestVoteResponsesIn:   make(chan RequestVoteResponse, 10),

		ProposeRequestsIn: make(chan ProposeRequest, 10),

		AppendEntriesRequestsOut: make(chan AppendEntriesRequest, 10),
		RequestVoteRequestsOut:   make(chan RequestVoteRequest, 10),
	}
}
