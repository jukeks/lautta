package main

import (
	"context"

	raftv1 "github.com/jukeks/lautta/proto/gen/lautta/rpc/raft/v1"
)

type RaftServer struct {
	raftv1.UnimplementedRaftServiceServer
}

func NewRaftServer() *RaftServer {
	return &RaftServer{}
}

func (s *RaftServer) HealthCheck(ctx context.Context, req *raftv1.HealthcheckRequest) (*raftv1.HealthcheckResponse, error) {
	return nil, nil
}

func (s *RaftServer) AppendEntries(ctx context.Context, req *raftv1.AppendEntriesRequest) (*raftv1.AppendEntriesResponse, error) {
	return nil, nil
}

func (s *RaftServer) RequestVote(ctx context.Context, req *raftv1.RequestVoteRequest) (*raftv1.RequestVoteResponse, error) {
	return nil, nil
}
