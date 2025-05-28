package lautta

import "context"

func (n *Node) handleClient() {
	for {
		select {
		case req := <-n.comms.AppendEntriesRequestsOut:
			n.sendAppendEntriesRequest(req.TargetNode, req)
		case req := <-n.comms.RequestVoteRequestsOut:
			n.sendVoteRequest(req.TargetNode, req)
		}
	}
}

func (n *Node) sendAppendEntriesRequest(node NodeID, req AppendEntriesRequest) error {
	go func() {
		resp, err := n.raftClient.AppendEntries(context.Background(), node, req)
		if err != nil {
			n.logger.Error("error requesting append entries", "err", err)
			return
		}
		resp.Request = req
		n.comms.AppendEntriesResponsesIn <- resp
	}()

	return nil
}

func (n *Node) sendVoteRequest(node NodeID, req RequestVoteRequest) error {
	go func() {
		resp, err := n.raftClient.VoteRequest(context.Background(), node, req)
		if err != nil {
			n.logger.Error("error requesting vote", "err", err)
			return
		}
		n.comms.RequestVoteResponsesIn <- resp
	}()

	return nil
}
