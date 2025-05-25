package lautta

import (
	"bytes"
	"slices"
	"testing"
	"time"
)

func serve(stop chan bool, node1 NodeID, comms1 Comms, node2 NodeID, comms2 Comms, node3 NodeID, comms3 Comms) {
	m := map[NodeID]Comms{
		node1: comms1,
		node2: comms2,
		node3: comms3,
	}

	relayAppendEntriesRequest := func(req AppendEntriesRequest, origin, target Comms) {
		req.Ret = make(chan AppendEntriesResponse, 1)
		select {
		case target.AppendEntriesRequestsIn <- req:
		case <-time.After(50 * time.Millisecond):
			return
		}
		select {
		case resp := <-req.Ret:
			resp.Request = req
			origin.AppendEntriesResponsesIn <- resp
		case <-time.After(50 * time.Millisecond):
			return
		}
	}

	relayRequestVoteRequest := func(req RequestVoteRequest, origin, target Comms) {
		req.Ret = make(chan RequestVoteResponse, 1)
		select {
		case target.RequestVoteRequestsIn <- req:
		case <-time.After(50 * time.Millisecond):
			return
		}

		select {
		case resp := <-req.Ret:
			origin.RequestVoteResponsesIn <- resp
		case <-time.After(50 * time.Millisecond):
			return
		}
	}

loop:
	for {
		select {
		case <-stop:
			break loop
		case req := <-comms1.AppendEntriesRequestsOut:
			target := m[req.TargetNode]
			go relayAppendEntriesRequest(req, comms1, target)
		case req := <-comms2.AppendEntriesRequestsOut:
			target := m[req.TargetNode]
			go relayAppendEntriesRequest(req, comms2, target)
		case req := <-comms3.AppendEntriesRequestsOut:
			target := m[req.TargetNode]
			go relayAppendEntriesRequest(req, comms3, target)
		case req := <-comms1.RequestVoteRequestsOut:
			target := m[req.TargetNode]
			go relayRequestVoteRequest(req, comms1, target)
		case req := <-comms2.RequestVoteRequestsOut:
			target := m[req.TargetNode]
			go relayRequestVoteRequest(req, comms2, target)
		case req := <-comms3.RequestVoteRequestsOut:
			target := m[req.TargetNode]
			go relayRequestVoteRequest(req, comms3, target)
		}
	}
}

type fsm struct {
	logs []LogEntry
}

func (f *fsm) Apply(log LogEntry) error {
	f.logs = append(f.logs, log)
	return nil
}

func getCluster() (Cluster, func()) {
	comms1 := NewComms()
	comms2 := NewComms()
	comms3 := NewComms()

	nodeID1 := NodeID(1)
	nodeID2 := NodeID(2)
	nodeID3 := NodeID(3)
	stop := make(chan bool, 1)
	go serve(stop, nodeID1, comms1, nodeID2, comms2, nodeID3, comms3)

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

	node1 := NewNode(config1, comms1, fsm1, NewInMemLogStore(), NewInMemStableStore())
	node2 := NewNode(config2, comms2, fsm2, NewInMemLogStore(), NewInMemStableStore())
	node3 := NewNode(config3, comms3, fsm3, NewInMemLogStore(), NewInMemStableStore())

	node1.Start()
	node2.Start()
	node3.Start()

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

	return Cluster{Nodes: nodes}, cleanup
}

func getLeader(cluster Cluster) *NodeFSM {
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		for _, node := range cluster.Nodes {
			if node.Node.state == Leader {
				return node
			}
		}

		time.Sleep(1 * time.Millisecond)
	}

	return nil
}

func getFollower(cluster Cluster) *NodeFSM {
	for _, node := range cluster.Nodes {
		if node.Node.state == Follower {
			return node
		}
	}

	return nil
}

func TestElection(t *testing.T) {
	cluster, cleanup := getCluster()

	leaders := 0
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)

		leaders = 0
		for _, node := range cluster.Nodes {
			if node.Node.state == Leader {
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

func propose(t *testing.T, leader Comms, payload []byte) error {
	ret := make(chan ProposeResponse, 1)
	leader.ProposeRequestsIn <- ProposeRequest{
		Payload: payload,
		Ret:     ret,
	}

	select {
	case resp := <-ret:
		if resp.Err != nil {
			return resp.Err
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("timed out waiting for propose response")
	}
	return nil
}

type NodeFSM struct {
	Node *Node
	FSM  *fsm
}

type Cluster struct {
	Nodes []*NodeFSM
}

func ensurePropagation(t *testing.T, skip []NodeID, cluster Cluster, payload []byte) {
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		found := 0
		for _, node := range cluster.Nodes {
			if slices.Contains(skip, node.Node.config.ID) {
				continue
			}

			for _, log := range node.FSM.logs {
				if bytes.Equal(log.Payload, payload) {
					found++
					break
				}
			}
		}

		if found == len(cluster.Nodes)-len(skip) {
			return
		}

		time.Sleep(1 * time.Millisecond)
	}

	t.Fatalf("failed to propagate log entry with payload %s", payload)
}

func TestPropose(t *testing.T) {
	cluster, cleanup := getCluster()
	defer cleanup()

	leader := getLeader(cluster)
	if leader == nil {
		t.Fatalf("failed to get leader")
	}

	payload := []byte("test payload")
	if err := propose(t, leader.Node.comms, payload); err != nil {
		t.Fatalf("failed to propose: %v", err)
	}

	ensurePropagation(t, nil, cluster, payload)

	for _, node := range cluster.Nodes {
		if node.Node.commitIndex != 1 {
			t.Errorf("commit index not progressed")
		}
	}
}

func TestReplay(t *testing.T) {
	cluster, _ := getCluster()

	leader := getLeader(cluster)
	if leader == nil {
		t.Fatalf("failed to get leader")
	}

	follower := getFollower(cluster)
	if follower == nil {
		t.Fatalf("failed to get follower")
	}
	follower.Node.Stop()

	payload := []byte("1")
	if err := propose(t, leader.Node.comms, payload); err != nil {
		t.Fatalf("failed to propose: %v", err)
	}

	ensurePropagation(t, []NodeID{follower.Node.config.ID}, cluster, payload)

	follower.Node.Start()
	// follower should catch up with the leader
	ensurePropagation(t, nil, cluster, payload)
}
