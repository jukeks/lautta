package lautta

import "context"

func (n *Node) AppendEntries(ctx context.Context, req AppendEntriesRequest) (AppendEntriesResponse, error) {
	req.ret = make(chan AppendEntriesResponse, 1)
	select {
	case n.comms.AppendEntriesRequestsIn <- req:
	case <-ctx.Done():
		return AppendEntriesResponse{}, ctx.Err()
	}

	select {
	case resp := <-req.ret:
		return resp, nil
	case <-ctx.Done():
		return AppendEntriesResponse{}, ctx.Err()
	}
}

func (n *Node) RequestVote(ctx context.Context, req RequestVoteRequest) (RequestVoteResponse, error) {
	req.ret = make(chan RequestVoteResponse, 1)
	select {
	case n.comms.RequestVoteRequestsIn <- req:
	case <-ctx.Done():
		return RequestVoteResponse{}, ctx.Err()
	}

	select {
	case resp := <-req.ret:
		return resp, nil
	case <-ctx.Done():
		return RequestVoteResponse{}, ctx.Err()
	}

}

func (n *Node) Propose(ctx context.Context, req ProposeRequest) (ProposeResponse, error) {
	req.ret = make(chan ProposeResponse, 1)
	select {
	case n.comms.ProposeRequestsIn <- req:
	case <-ctx.Done():
		return ProposeResponse{}, ctx.Err()
	}

	select {
	case resp := <-req.ret:
		return resp, nil
	case <-ctx.Done():
		return ProposeResponse{}, ctx.Err()
	}
}
