package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"

	kvv1 "github.com/jukeks/lautta/proto/gen/lautta/rpc/kv/v1"
	raftv1 "github.com/jukeks/lautta/proto/gen/lautta/rpc/raft/v1"
	lautta "github.com/jukeks/lautta/raft"
	tukki "github.com/jukeks/tukki/pkg/db"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	config = flag.String("config", "", "config")

	logger = log.New(os.Stderr, "[server] ", log.Lmicroseconds)
)

func parseConfig(config string) lautta.Config {
	if config == "" {
		logger.Fatal("config is required")
	}

	nodes := strings.Split(config, ",")
	peers := make([]lautta.Peer, len(nodes))
	for i, node := range nodes {
		components := strings.Split(node, "=")
		if len(components) != 2 {
			logger.Fatalf("invalid node format: %s", node)
		}
		id_raw := components[0]
		id, err := strconv.ParseInt(id_raw, 10, 64)
		if err != nil {
			logger.Fatalf("invalid node id: %s", id_raw)
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

func initPeerClients(peers []lautta.Peer) map[lautta.NodeID]raftv1.RaftServiceClient {
	peerClients := make(map[lautta.NodeID]raftv1.RaftServiceClient)
	for _, peer := range peers {
		logger.Printf("peer: %d -> %s", peer.ID, peer.Address)
		c, err := grpc.NewClient(peer.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			logger.Fatalf("failed to connect to peer %d: %v", peer.ID, err)
		}
		client := raftv1.NewRaftServiceClient(c)
		peerClients[peer.ID] = client
	}

	return peerClients
}

type fsm struct {
	logs []lautta.LogEntry
}

func (f *fsm) Apply(log lautta.LogEntry) error {
	f.logs = append(f.logs, log)
	return nil
}

func main() {
	flag.Parse()
	cfg := parseConfig(*config)

	db, err := tukki.OpenDatabase("asdf", tukki.GetDefaultConfig())
	if err != nil {
		logger.Fatalf("failed to open db")
	}

	db.Set("lol", "jes")
	peers := initPeerClients(cfg.Peers)
	comms := lautta.NewComms()

	client := NewRaftClient(peers, comms)
	go client.Run()

	fsm := &fsm{}
	lauttaNode := lautta.NewNode(cfg, comms, fsm,
		lautta.NewInMemLog(), lautta.NewInMemStableStore())
	go lauttaNode.Run()
	defer lauttaNode.Stop()

	raftServer := NewRaftServer(comms)

	ls, err := net.Listen("tcp", fmt.Sprintf(cfg.Address))
	if err != nil {
		logger.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	raftv1.RegisterRaftServiceServer(grpcServer, raftServer)
	kvv1.RegisterKVServiceServer(grpcServer, raftServer)
	grpcServer.Serve(ls)
}
