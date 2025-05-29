package main

import (
	"flag"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	kvv1 "github.com/jukeks/lautta/proto/gen/example/rpc/kv/v1"
	raftv1 "github.com/jukeks/lautta/proto/gen/lautta/rpc/raft/v1"
	lautta "github.com/jukeks/lautta/raft"
	lauttaGrpc "github.com/jukeks/lautta/raft/transports/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	config = flag.String("config", "", "config")
	inMem  = flag.Bool("in-mem", false, "use in-memory log and stable store")
	dbDir  = flag.String("db-dir", "", "directory for raft state")

	logger = slog.New(slog.Default().Handler()).With("prefix", "node")
)

func parseConfig(config string) lautta.Config {
	if config == "" {
		logger.Error("config is required")
		os.Exit(1)
	}

	nodes := strings.Split(config, ",")
	peers := make([]lautta.Peer, len(nodes))
	for i, node := range nodes {
		components := strings.Split(node, "=")
		if len(components) != 2 {
			logger.Error("invalid node format", "node", node)
			os.Exit(1)
		}
		id_raw := components[0]
		id, err := strconv.ParseInt(id_raw, 10, 64)
		if err != nil {
			logger.Error("invalid node id", "id", id_raw)
			os.Exit(1)
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
	logger.Info("peers", "peers", peers)
	for _, peer := range peers {
		c, err := grpc.NewClient(peer.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			logger.Error("failed to connect to peer", "peer", peer.ID, "err", err)
			os.Exit(1)
		}
		client := raftv1.NewRaftServiceClient(c)
		peerClients[peer.ID] = client
	}

	return peerClients
}

type dummyFSM struct {
	logs []lautta.LogEntry
}

func (f *dummyFSM) Apply(log lautta.LogEntry) error {
	f.logs = append(f.logs, log)
	return nil
}

func main() {
	flag.Parse()

	if !*inMem && *dbDir == "" {
		logger.Error("db-dir is required when not using in-memory log and stable store")
		os.Exit(1)
	}

	cfg := parseConfig(*config)
	peers := initPeerClients(cfg.Peers)
	client := lauttaGrpc.NewRaftClient(peers)

	fsm := &dummyFSM{}
	logStore := lautta.NewInMemLogStore()
	stableStore := lautta.NewInMemStableStore()
	if !*inMem {
		store, err := NewTukkiStore(*dbDir)
		if err != nil {
			logger.Error("failed to create tukki store", "err", err)
			os.Exit(1)
		}
		defer store.Close()

		logStore = store
		stableStore = store
		logger.Info("using tukki store", "db-dir", *dbDir)
	}

	lauttaNode := lautta.NewNode(cfg, client, fsm, logStore, stableStore)
	lauttaNode.Start()
	defer lauttaNode.Stop()

	kvServer := NewKVServer(lauttaNode)
	raftServer := lauttaGrpc.NewRaftServer(lauttaNode)

	ls, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		logger.Error("failed to listen", "err", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer()
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		grpcServer.GracefulStop()
	}()

	raftv1.RegisterRaftServiceServer(grpcServer, raftServer)
	kvv1.RegisterKVServiceServer(grpcServer, kvServer)
	grpcServer.Serve(ls)
}
