package main

import (
	"context"
	"encoding/json"
	"log/slog"

	kvv1 "github.com/jukeks/lautta/proto/gen/lautta/rpc/kv/v1"
	lautta "github.com/jukeks/lautta/raft"
)

type KVServer struct {
	kvv1.UnimplementedKVServiceServer
	logger *slog.Logger
	raft   lautta.RaftServer
}

func NewKVServer(raft lautta.RaftServer) *KVServer {
	return &KVServer{
		logger: slog.New(slog.Default().Handler()).
			With("prefix", "raft-server"),
		raft: raft,
	}
}

type KV struct {
	Key   string
	Value string
}

func (s *KVServer) Write(ctx context.Context, req *kvv1.WriteRequest) (*kvv1.WriteResponse, error) {
	s.logger.Debug("Write called")
	payload := KV{
		Key:   req.Key,
		Value: req.Value,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	resp, err := s.raft.Propose(ctx, lautta.ProposeRequest{
		Payload: raw,
	})
	if err != nil {
		s.logger.Error("error in Propose", "error", err)
		return nil, err
	}

	s.logger.Debug("Write response received")
	if resp.Err != nil {
		return nil, resp.Err
	}
	return &kvv1.WriteResponse{}, nil
}
