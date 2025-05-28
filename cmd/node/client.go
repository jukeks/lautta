package main

import (
	"context"
	"log/slog"

	raftv1 "github.com/jukeks/lautta/proto/gen/lautta/rpc/raft/v1"
	lautta "github.com/jukeks/lautta/raft"
)

type RaftGrpcClient struct {
	peers  map[lautta.NodeID]raftv1.RaftServiceClient
	logger *slog.Logger
}

func NewRaftClient(peers map[lautta.NodeID]raftv1.RaftServiceClient) lautta.RaftClient {
	return &RaftGrpcClient{
		peers: peers,
		logger: slog.New(slog.Default().Handler()).
			With("prefix", "raft-client"),
	}
}

func (c *RaftGrpcClient) AppendEntries(ctx context.Context, node lautta.NodeID, req lautta.AppendEntriesRequest) (lautta.AppendEntriesResponse, error) {
	peer := c.peers[node]

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
		c.logger.Error("error requesting append entries", "err", err)
		return lautta.AppendEntriesResponse{}, err
	}

	return lautta.AppendEntriesResponse{
		Term:    lautta.TermID(resp.Term),
		Success: resp.Success,
		Request: req,
	}, nil

}

func (c *RaftGrpcClient) VoteRequest(ctx context.Context, node lautta.NodeID, req lautta.RequestVoteRequest) (lautta.RequestVoteResponse, error) {
	peer := c.peers[node]

	resp, err := peer.RequestVote(context.Background(), &raftv1.RequestVoteRequest{
		Term:         int64(req.Term),
		CandidateId:  int64(req.CandidateID),
		LastLogIndex: int64(req.LastLogIndex),
		LastLogTerm:  int64(req.LastLogTerm),
	})
	if err != nil {
		c.logger.Error("error requesting vote", "err", err)
		return lautta.RequestVoteResponse{}, err
	}

	return lautta.RequestVoteResponse{
		Term:        lautta.TermID(resp.Term),
		VoteGranted: resp.VoteGranted,
	}, nil
}
