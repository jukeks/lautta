package main

import (
	"flag"
	"fmt"
	"log"
	"net"

	"github.com/jukeks/lautta"
	raftv1 "github.com/jukeks/lautta/proto/gen/lautta/rpc/raft/v1"
	"google.golang.org/grpc"
)

var (
	port = flag.Int("port", 40050, "The server port")
)

func main() {
	lauttaNode := lautta.NewNode()
	go lauttaNode.Run()
	defer lauttaNode.Stop()
	raftServer := NewRaftServer(lauttaNode)

	ls, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	raftv1.RegisterRaftServiceServer(grpcServer, raftServer)
	grpcServer.Serve(ls)
}
