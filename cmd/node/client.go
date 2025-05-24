package main

import (
	"context"

	raftv1 "github.com/jukeks/lautta/proto/gen/lautta/rpc/raft/v1"
	lautta "github.com/jukeks/lautta/raft"
)

type Client struct {
	peers map[lautta.NodeID]raftv1.RaftServiceClient
	comms lautta.Comms
}

func NewClient(peers map[lautta.NodeID]raftv1.RaftServiceClient, comms lautta.Comms) *Client {
	return &Client{
		peers: peers,
		comms: comms,
	}
}

func (c *Client) Run() {
	for {
		select {
		case req := <-c.comms.AppendEntriesRequestsOut:
			c.sendAppendEntriesRequest(req.TargetNode, req)
		case req := <-c.comms.RequestVoteRequestsOut:
			c.sendVoteRequest(req.TargetNode, req)
		}
	}
}

func (c *Client) sendAppendEntriesRequest(node lautta.NodeID, req lautta.AppendEntriesRequest) error {
	peer := c.peers[node]

	go func() {
		entries := make([]*raftv1.Entry, len(req.Entries))
		for i, entry := range req.Entries {
			entries[i] = &raftv1.Entry{
				Index:   int64(entry.Index),
				Term:    int64(entry.Term),
				Payload: entry.Payload,
			}
		}

		resp, err := peer.AppendEntries(context.Background(), &raftv1.AppendEntriesRequest{
			Term:         int64(req.Term),
			LeaderId:     int64(req.LeaderID),
			PrevLogIndex: int64(req.PrevLogIndex),
			PrevLogTerm:  int64(req.PrevLogTerm),
			Entries:      entries,
		})

		if err != nil {
			logger.Printf("error requesting append entries: %v", err)
			return
		}
		c.comms.AppendEntriesResponsesIn <- lautta.AppendEntriesResponse{
			Term:    lautta.TermID(resp.Term),
			Success: resp.Success,
			Request: req,
		}
	}()

	return nil
}

func (c *Client) sendVoteRequest(node lautta.NodeID, req lautta.RequestVoteRequest) error {
	peer := c.peers[node]

	go func() {
		resp, err := peer.RequestVote(context.Background(), &raftv1.RequestVoteRequest{
			Term:         int64(req.Term),
			CandidateId:  int64(req.CandidateID),
			LastLogIndex: int64(req.LastLogIndex),
			LastLogTerm:  int64(req.LastLogTerm),
		})

		if err != nil {
			logger.Printf("error requesting vote: %v", err)
			return
		}
		c.comms.RequestVoteResponsesIn <- lautta.RequestVoteResponse{
			Term:        lautta.TermID(resp.Term),
			VoteGranted: resp.VoteGranted,
		}
	}()

	return nil
}
