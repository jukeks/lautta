package main

import (
	"context"
	"log"
	"os"

	lautta "github.com/jukeks/lautta/lib"
	raftv1 "github.com/jukeks/lautta/proto/gen/lautta/rpc/raft/v1"
)

type RaftServer struct {
	raftv1.UnimplementedRaftServiceServer
	logger *log.Logger
	comms  lautta.Comms
}

func NewRaftServer(comms lautta.Comms) *RaftServer {
	return &RaftServer{
		logger: log.New(os.Stderr, "[grpc] ", log.Lmicroseconds),
		comms:  comms,
	}
}

func (s *RaftServer) AppendEntries(ctx context.Context, req *raftv1.AppendEntriesRequest) (*raftv1.AppendEntriesResponse, error) {
	s.logger.Println("AppendEntries called")
	ret := make(chan lautta.AppendEntriesResponse, 1)
	entries := make([]lautta.LogEntry, len(req.Entries))
	for i, entry := range req.Entries {
		entries[i].Index = lautta.LogIndex(entry.Index)
		entries[i].Term = lautta.TermID(entry.Term)
		entries[i].Payload = entry.Payload
	}

	s.comms.AppendEntriesRequestsIn <- lautta.AppendEntriesRequest{
		Term:         lautta.TermID(req.Term),
		LeaderID:     lautta.NodeID(req.LeaderId),
		PrevLogIndex: lautta.LogIndex(req.PrevLogIndex),
		PrevLogTerm:  lautta.TermID(req.PrevLogTerm),
		Entries:      entries,
		LeaderCommit: lautta.LogIndex(req.LeaderCommit),
		Ret:          ret,
	}

	resp := <-ret
	return &raftv1.AppendEntriesResponse{
		Term:    int64(resp.Term),
		Success: resp.Success,
	}, nil
}

func (s *RaftServer) RequestVote(ctx context.Context, req *raftv1.RequestVoteRequest) (*raftv1.RequestVoteResponse, error) {
	s.logger.Println("RequestVote called")

	ret := make(chan lautta.RequestVoteResponse, 1)
	s.comms.RequestVoteRequestsIn <- lautta.RequestVoteRequest{
		Term:         lautta.TermID(req.Term),
		CandidateID:  lautta.NodeID(req.CandidateId),
		LastLogIndex: lautta.LogIndex(req.LastLogIndex),
		LastLogTerm:  lautta.TermID(req.LastLogTerm),
		Ret:          ret,
	}

	resp := <-ret
	s.logger.Printf("request vote resp received")
	return &raftv1.RequestVoteResponse{
		Term:        int64(resp.Term),
		VoteGranted: resp.VoteGranted,
	}, nil
}
