package lautta

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"testing"
	"time"
)

type testClient struct {
	servers map[NodeID]RaftServer
}

func (c *testClient) AppendEntries(ctx context.Context, node NodeID, req AppendEntriesRequest) (AppendEntriesResponse, error) {
	if server, ok := c.servers[node]; ok {
		return server.AppendEntries(ctx, req)
	}
	return AppendEntriesResponse{}, fmt.Errorf("server not found for node %d: %+v", node, c.servers)
}

func (c *testClient) VoteRequest(ctx context.Context, node NodeID, req RequestVoteRequest) (RequestVoteResponse, error) {
	if server, ok := c.servers[node]; ok {
		return server.RequestVote(ctx, req)
	}
	return RequestVoteResponse{}, fmt.Errorf("server not found for node %d", node)
}

type fsm struct {
	logs []LogEntry
}

func (f *fsm) Apply(log LogEntry) error {
	f.logs = append(f.logs, log)
	return nil
}

func getCluster(started bool) (Cluster, func()) {
	nodeID1 := NodeID(1)
	nodeID2 := NodeID(2)
	nodeID3 := NodeID(3)
	stop := make(chan bool, 1)

	config1 := Config{
		ID: nodeID1,
		Peers: []Peer{
			{ID: nodeID2},
			{ID: nodeID3},
		},
	}
	config2 := Config{
		ID: nodeID2,
		Peers: []Peer{
			{ID: nodeID1},
			{ID: nodeID3},
		},
	}
	config3 := Config{
		ID: nodeID3,
		Peers: []Peer{
			{ID: nodeID2},
			{ID: nodeID1},
		},
	}
	fsm1 := &fsm{}
	fsm2 := &fsm{}
	fsm3 := &fsm{}

	client := &testClient{servers: map[NodeID]RaftServer{}}

	node1 := NewNode(config1, client, fsm1, NewInMemLogStore(), NewInMemStableStore())
	node2 := NewNode(config2, client, fsm2, NewInMemLogStore(), NewInMemStableStore())
	node3 := NewNode(config3, client, fsm3, NewInMemLogStore(), NewInMemStableStore())

	client.servers[nodeID1] = node1
	client.servers[nodeID2] = node2
	client.servers[nodeID3] = node3

	if started {
		node1.Start()
		node2.Start()
		node3.Start()
	}

	cleanup := func() {
		node1.Stop()
		node2.Stop()
		node3.Stop()
		stop <- true
	}

	nodes := []*NodeFSM{
		{Node: node1, FSM: fsm1},
		{Node: node2, FSM: fsm2},
		{Node: node3, FSM: fsm3},
	}

	return Cluster{Members: nodes}, cleanup
}

func getLeader(cluster Cluster) *NodeFSM {
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		for _, member := range cluster.Members {
			if member.Node.state == Leader {
				return member
			}
		}

		time.Sleep(1 * time.Millisecond)
	}

	return nil
}

func getFollower(cluster Cluster) *NodeFSM {
	for _, member := range cluster.Members {
		if member.Node.state == Follower {
			return member
		}
	}

	return nil
}

func TestElection(t *testing.T) {
	cluster, cleanup := getCluster(true)

	leaders := 0
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)

		leaders = 0
		for _, member := range cluster.Members {
			if member.Node.state == Leader {
				leaders++
			}
		}

		if leaders > 0 {
			break
		}
	}

	cleanup()

	if leaders != 1 {
		t.Errorf("leader count mismatch: %d", leaders)
	}
}

func propose(t *testing.T, leader *Node, payload []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	_, err := leader.Propose(ctx, ProposeRequest{
		Payload: payload,
	})
	defer cancel()
	return err
}

type NodeFSM struct {
	Node *Node
	FSM  *fsm
}

type Cluster struct {
	Members []*NodeFSM
}

func ensurePropagation(t *testing.T, skip []NodeID, cluster Cluster, payload []byte) {
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		found := 0
		for _, member := range cluster.Members {
			if slices.Contains(skip, member.Node.config.ID) {
				continue
			}

			for _, log := range member.FSM.logs {
				if bytes.Equal(log.Payload, payload) {
					found++
					break
				}
			}
		}

		if found == len(cluster.Members)-len(skip) {
			return
		}

		time.Sleep(1 * time.Millisecond)
	}

	t.Fatalf("failed to propagate log entry with payload %s", payload)
}

func TestPropose(t *testing.T) {
	cluster, cleanup := getCluster(true)
	defer cleanup()

	leader := getLeader(cluster)
	if leader == nil {
		t.Fatalf("failed to get leader")
	}

	payload := []byte("test payload")
	if err := propose(t, leader.Node, payload); err != nil {
		t.Fatalf("failed to propose: %v", err)
	}

	ensurePropagation(t, nil, cluster, payload)

	for _, member := range cluster.Members {
		if member.Node.commitIndex != 1 {
			t.Errorf("commit index not progressed")
		}
	}
}

func TestReplay(t *testing.T) {
	cluster, _ := getCluster(true)

	leader := getLeader(cluster)
	if leader == nil {
		t.Fatalf("failed to get leader")
	}

	follower := getFollower(cluster)
	if follower == nil {
		t.Fatalf("failed to get follower")
	}
	follower.Node.Stop()

	payload1 := []byte("1")
	if err := propose(t, leader.Node, payload1); err != nil {
		t.Fatalf("failed to propose: %v", err)
	}
	ensurePropagation(t, []NodeID{follower.Node.config.ID}, cluster, payload1)

	payload2 := []byte("2")
	if err := propose(t, leader.Node, payload2); err != nil {
		t.Fatalf("failed to propose: %v", err)
	}
	ensurePropagation(t, []NodeID{follower.Node.config.ID}, cluster, payload2)

	follower.Node.Start()
	// follower should catch up with the leader
	ensurePropagation(t, nil, cluster, payload1)
	ensurePropagation(t, nil, cluster, payload2)
}

func TestElectionWithMajority(t *testing.T) {
	cluster, _ := getCluster(false)

	cluster.Members[0].Node.Start()
	cluster.Members[1].Node.Start()

	leader := getLeader(cluster)
	if leader == nil {
		t.Fatalf("failed to get leader")
	}
}
