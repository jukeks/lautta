package lautta

type AppendEntriesRequest struct {
	Term     TermID
	LeaderID NodeID

	PrevLogIndex LogIndex
	PrevLogTerm  TermID

	Entries []LogEntry

	LeaderCommit LogIndex

	ret        chan AppendEntriesResponse
	targetNode NodeID
}

type AppendEntriesResponse struct {
	Term    TermID
	Success bool

	// for matching logic
	request AppendEntriesRequest
}

type RequestVoteRequest struct {
	Term         TermID
	CandidateID  NodeID
	LastLogIndex LogIndex
	LastLogTerm  TermID

	ret        chan RequestVoteResponse
	targetNode NodeID
}

type RequestVoteResponse struct {
	Term        TermID
	VoteGranted bool
}

type ProposeRequest struct {
	Payload []byte

	ret chan ProposeResponse
}

type ProposeResponse struct {
	Err error
}

type comms struct {
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

func newComms() comms {
	return comms{
		AppendEntriesRequestsIn:  make(chan AppendEntriesRequest, 10),
		AppendEntriesResponsesIn: make(chan AppendEntriesResponse, 10),
		RequestVoteRequestsIn:    make(chan RequestVoteRequest, 10),
		RequestVoteResponsesIn:   make(chan RequestVoteResponse, 10),

		ProposeRequestsIn: make(chan ProposeRequest, 10),

		AppendEntriesRequestsOut: make(chan AppendEntriesRequest, 10),
		RequestVoteRequestsOut:   make(chan RequestVoteRequest, 10),
	}
}
