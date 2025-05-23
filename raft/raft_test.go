package lautta

import (
	"bytes"
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
		target.AppendEntriesRequestsIn <- req
		resp := <-req.Ret
		resp.Request = req
		origin.AppendEntriesResponsesIn <- resp
	}

	relayRequestVoteRequest := func(req RequestVoteRequest, origin, target Comms) {
		req.Ret = make(chan RequestVoteResponse, 1)
		target.RequestVoteRequestsIn <- req
		resp := <-req.Ret
		origin.RequestVoteResponsesIn <- resp
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

func getCluster() ([]*Node, []*fsm, func()) {
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

	node1 := NewNode(config1, comms1, fsm1, NewInMemLog(), NewInMemStableStore())
	node2 := NewNode(config2, comms2, fsm2, NewInMemLog(), NewInMemStableStore())
	node3 := NewNode(config3, comms3, fsm3, NewInMemLog(), NewInMemStableStore())

	go node1.Run()
	go node2.Run()
	go node3.Run()

	cleanup := func() {
		node1.Stop()
		node2.Stop()
		node3.Stop()
		stop <- true
	}

	return []*Node{node1, node2, node3}, []*fsm{fsm1, fsm2, fsm3}, cleanup
}

func getLeader(cluster []*Node) *Node {
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)

		for _, node := range cluster {
			if node.State == Leader {
				return node
			}
		}
	}

	return nil
}

func TestElection(t *testing.T) {
	cluster, _, cleanup := getCluster()

	leaders := 0
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)

		leaders = 0
		for _, node := range cluster {
			if node.State == Leader {
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

func TestPropose(t *testing.T) {
	cluster, fsms, cleanup := getCluster()
	defer cleanup()

	leader := getLeader(cluster)
	if leader == nil {
		t.Fatalf("failed to get leader")
	}

	payload := []byte("test payload")
	ret := make(chan ProposeResponse, 1)
	leader.comms.ProposeRequestsIn <- ProposeRequest{
		Payload: payload,
		Ret:     ret,
	}

	select {
	case resp := <-ret:
		if resp.Err != nil {
			t.Fatalf("failed to propose: %v", resp.Err)
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("timed out")
	}

	time.Sleep(50 * time.Millisecond)
	for _, f := range fsms {
		if !bytes.Equal(f.logs[0].Payload, payload) {
			t.Errorf("leader fsm doesn't contain log")
		}
	}

	for _, node := range cluster {
		if node.CommitIndex != 1 {
			t.Errorf("commit index not progressed")
		}
	}
}
