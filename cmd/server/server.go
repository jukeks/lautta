package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"

	"github.com/jukeks/lautta"
	raftv1 "github.com/jukeks/lautta/proto/gen/lautta/rpc/raft/v1"
	"google.golang.org/grpc"
)

var (
	config = flag.String("config", "", "config")
)

func parseConfig() lautta.Config {
	if *config == "" {
		log.Fatal("config is required")
	}

	nodes := strings.Split(*config, ",")
	peers := make([]lautta.Peer, len(nodes))
	for i, node := range nodes {
		components := strings.Split(node, "=")
		if len(components) != 2 {
			log.Fatalf("invalid node format: %s", node)
		}
		id_raw := components[0]
		id, err := strconv.ParseInt(id_raw, 10, 64)
		if err != nil {
			log.Fatalf("invalid node id: %s", id_raw)
		}
		address := components[1]
		peers[i] = lautta.Peer{
			ID:      lautta.NodeID(id),
			Address: address,
		}
	}

	me := peers[0]
	peers = peers[1:]

	return lautta.Config{
		ID:      me.ID,
		Address: me.Address,
		Peers:   peers,
	}
}

func main() {
	flag.Parse()
	cfg := parseConfig()
	lauttaNode := lautta.NewNode(cfg)
	go lauttaNode.Run()
	defer lauttaNode.Stop()
	raftServer := NewRaftServer(lauttaNode)

	ls, err := net.Listen("tcp", fmt.Sprintf(cfg.Address))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	raftv1.RegisterRaftServiceServer(grpcServer, raftServer)
	grpcServer.Serve(ls)
}
