package lautta

import "context"

type RaftServer interface {
	AppendEntries(ctx context.Context, req AppendEntriesRequest) (AppendEntriesResponse, error)
	RequestVote(ctx context.Context, req RequestVoteRequest) (RequestVoteResponse, error)
	Propose(ctx context.Context, req ProposeRequest) (ProposeResponse, error)
}

type RaftClient interface {
	AppendEntries(ctx context.Context, node NodeID, req AppendEntriesRequest) (AppendEntriesResponse, error)
	VoteRequest(ctx context.Context, node NodeID, req RequestVoteRequest) (RequestVoteResponse, error)
}
