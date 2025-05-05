package main

import (
	"flag"
	"fmt"
	"log"
	"net"

	raftv1 "github.com/jukeks/lautta/proto/gen/lautta/rpc/raft/v1"
	"google.golang.org/grpc"
)

var (
	port = flag.Int("port", 40050, "The server port")
)

func main() {
	raftServer := NewRaftServer()

	ls, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	raftv1.RegisterRaftServiceServer(grpcServer, raftServer)
	grpcServer.Serve(ls)
}
