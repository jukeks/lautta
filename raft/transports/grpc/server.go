package grpc

import (
	"context"
	"log/slog"

	raftv1 "github.com/jukeks/lautta/proto/gen/lautta/rpc/raft/v1"
	lautta "github.com/jukeks/lautta/raft"
)

type RaftServer struct {
	raftv1.UnimplementedRaftServiceServer
	logger *slog.Logger
	raft   lautta.RaftServer
}

func NewRaftServer(raft lautta.RaftServer) *RaftServer {
	return &RaftServer{
		logger: slog.New(slog.Default().Handler()).
			With("prefix", "raft-server"),
		raft: raft,
	}
}

func (s *RaftServer) AppendEntries(ctx context.Context, req *raftv1.AppendEntriesRequest) (*raftv1.AppendEntriesResponse, error) {
	s.logger.Debug("AppendEntries called")
	entries := make([]lautta.LogEntry, len(req.Entries))
	for i, entry := range req.Entries {
		entries[i].Index = lautta.LogIndex(entry.Index)
		entries[i].Term = lautta.TermID(entry.Term)
		entries[i].Payload = entry.Payload
	}

	resp, err := s.raft.AppendEntries(ctx, lautta.AppendEntriesRequest{
		Term:         lautta.TermID(req.Term),
		LeaderID:     lautta.NodeID(req.LeaderId),
		PrevLogIndex: lautta.LogIndex(req.PrevLogIndex),
		PrevLogTerm:  lautta.TermID(req.PrevLogTerm),
		Entries:      entries,
		LeaderCommit: lautta.LogIndex(req.LeaderCommit),
	})
	if err != nil {
		s.logger.Error("error in AppendEntries", "error", err)
		return nil, err
	}

	return &raftv1.AppendEntriesResponse{
		Term:    int64(resp.Term),
		Success: resp.Success,
	}, nil
}

func (s *RaftServer) RequestVote(ctx context.Context, req *raftv1.RequestVoteRequest) (*raftv1.RequestVoteResponse, error) {
	s.logger.Debug("RequestVote called")

	resp, err := s.raft.RequestVote(ctx, lautta.RequestVoteRequest{
		Term:         lautta.TermID(req.Term),
		CandidateID:  lautta.NodeID(req.CandidateId),
		LastLogIndex: lautta.LogIndex(req.LastLogIndex),
		LastLogTerm:  lautta.TermID(req.LastLogTerm),
	})
	if err != nil {
		s.logger.Error("error in RequestVote", "error", err)
		return nil, err
	}

	s.logger.Debug("request vote resp received")
	return &raftv1.RequestVoteResponse{
		Term:        int64(resp.Term),
		VoteGranted: resp.VoteGranted,
	}, nil
}
