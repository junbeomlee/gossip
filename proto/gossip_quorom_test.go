package proto

import (
	"github.com/libopenstorage/gossip/types"
	"strconv"
	"testing"
	"time"
)

func addKey(g *GossiperImpl) types.StoreKey {
	key := types.StoreKey("new_key")
	value := "new_value"
	g.UpdateSelf(key, value)
	return key
}

func startNode(t *testing.T, selfIp string, nodeId types.NodeId, peerIps []string, clusterSize int) (*GossiperImpl, types.StoreKey) {
	g, _ := NewGossiperImpl(selfIp, nodeId, peerIps, types.DEFAULT_GOSSIP_VERSION)
	g.UpdateClusterSize(clusterSize)
	key := addKey(g)
	return g, key
}

func TestQuorumAllNodesUpOneByOne(t *testing.T) {
	printTestInfo()

	nodes := []string{
		"127.0.0.1:9900",
		"127.0.0.2:9901",
	}

	// Start Node0 with cluster size 1
	node0 := types.NodeId("0")
	g0, _ := startNode(t, nodes[0], node0, []string{}, 1)

	if g0.GetSelfStatus() != types.NODE_STATUS_UP {
		t.Error("Expected Node 0 to have status: ", types.NODE_STATUS_UP)
	}

	// Start Node1 with cluster size 2
	node1 := types.NodeId("1")
	g1, _ := startNode(t, nodes[1], node1, []string{nodes[0]}, 2)

	if g1.GetSelfStatus() != types.NODE_STATUS_UP {
		t.Error("Expected Node 1 to have status: ", types.NODE_STATUS_UP)
	}

	// Check if Node0 is still Up
	if g0.GetSelfStatus() != types.NODE_STATUS_UP {
		t.Error("Expected Node 0 to have status: ", types.NODE_STATUS_UP)
	}

	g0.Stop(types.DEFAULT_GOSSIP_INTERVAL * time.Duration(len(nodes)+1))
	g1.Stop(types.DEFAULT_GOSSIP_INTERVAL * time.Duration(len(nodes)+1))
}

func TestQuorumNodeLoosesQuorumAndGainsBack(t *testing.T) {
	printTestInfo()

	nodes := []string{
		"127.0.0.1:9902",
		"127.0.0.2:9903",
	}

	node0 := types.NodeId("0")

	// Start Node 0
	g0, _ := startNode(t, nodes[0], node0, []string{}, 1)

	selfStatus := g0.GetSelfStatus()
	if selfStatus != types.NODE_STATUS_UP {
		t.Error("Expected Node 0 to have status: ", types.NODE_STATUS_UP,
			" Got: ", selfStatus)
	}

	// Simulate new node was added by updating the cluster size, but the new node is not talking to node0
	// Node 0 should loose quorom 1/2
	g0.UpdateClusterSize(2)
	selfStatus = g0.GetSelfStatus()
	if selfStatus != types.NODE_STATUS_UP_AND_WAITING_FOR_QUORUM {
		t.Error("Expected Node 0 to have status: ", types.NODE_STATUS_UP_AND_WAITING_FOR_QUORUM,
			" Got: ", selfStatus)
	}

	// Sleep for quorum timeout
	time.Sleep(g0.quorumTimeout + 2*time.Second)

	selfStatus = g0.GetSelfStatus()
	if selfStatus != types.NODE_STATUS_WAITING_FOR_QUORUM {
		t.Error("Expected Node 0 to have status: ", types.NODE_STATUS_WAITING_FOR_QUORUM,
			" Got: ", selfStatus)
	}

	// Lets start the actual Node 1
	node1 := types.NodeId("1")
	g1, _ := startNode(t, nodes[1], node1, []string{nodes[0]}, 2)

	// Sleep so that nodes gossip
	time.Sleep(g1.GossipInterval() * time.Duration(len(nodes)+1))

	selfStatus = g0.GetSelfStatus()
	if selfStatus != types.NODE_STATUS_UP {
		t.Error("Expected Node 0 to have status: ", types.NODE_STATUS_UP,
			" Got: ", selfStatus)
	}
	selfStatus = g1.GetSelfStatus()
	if selfStatus != types.NODE_STATUS_UP {
		t.Error("Expected Node 1 to have status: ", types.NODE_STATUS_UP,
			" Got: ", selfStatus)
	}
}

func TestQuorumTwoNodesLooseConnectivity(t *testing.T) {
	printTestInfo()

	nodes := []string{
		"127.0.0.1:9904",
		"127.0.0.2:9905",
	}

	node0 := types.NodeId("0")
	g0, _ := startNode(t, nodes[0], node0, []string{}, 1)

	if g0.GetSelfStatus() != types.NODE_STATUS_UP {
		t.Error("Expected Node 0 to have status: ", types.NODE_STATUS_UP)
	}

	// Simulate new node was added by updating the cluster size, but the new node is not talking to node0
	// Node 0 should loose quorom 1/2
	g0.UpdateClusterSize(2)
	if g0.GetSelfStatus() != types.NODE_STATUS_UP_AND_WAITING_FOR_QUORUM {
		t.Error("Expected Node 0 to have status: ", types.NODE_STATUS_UP_AND_WAITING_FOR_QUORUM)
	}

	// Lets start the actual node 1. We do not supply node 0 Ip address here so that node 1 does not talk to node 0
	// to simulate NO connectivity between node 0 and node 1
	node1 := types.NodeId("1")
	g1, _ := startNode(t, nodes[1], node1, []string{}, 2)

	// For node 0 the status will change from UP_WAITING_QUORUM to WAITING_QUORUM after
	// the quorum timeout
	time.Sleep(g0.quorumTimeout + 2*time.Second)

	if g0.GetSelfStatus() != types.NODE_STATUS_WAITING_FOR_QUORUM {
		t.Error("Expected Node 0 to have status: ", types.NODE_STATUS_WAITING_FOR_QUORUM)
	}
	if g1.GetSelfStatus() != types.NODE_STATUS_WAITING_FOR_QUORUM {
		t.Error("Expected Node 1 to have status: ", types.NODE_STATUS_WAITING_FOR_QUORUM)
	}
}

func TestQuorumOneNodeIsolated(t *testing.T) {
	printTestInfo()

	nodes := []string{
		"127.0.0.1:9906",
		"127.0.0.2:9907",
		"127.0.0.3:9908",
	}

	var gossipers []*GossiperImpl
	for i, ip := range nodes {
		nodeId := types.NodeId(strconv.FormatInt(int64(i), 10))
		var g *GossiperImpl
		if i == 0 {
			g, _ = startNode(t, ip, nodeId, []string{}, len(nodes))
		} else {
			g, _ = startNode(t, ip, nodeId, []string{nodes[0]}, len(nodes))
		}

		gossipers = append(gossipers, g)
	}

	// Lets sleep so that the nodes gossip and update their quorum
	time.Sleep(types.DEFAULT_GOSSIP_INTERVAL * time.Duration(len(nodes)+1))

	for i, g := range gossipers {
		if g.GetSelfStatus() != types.NODE_STATUS_UP {
			t.Error("Expected Node ", i, " status to be ", types.NODE_STATUS_UP, " Got: ", g.GetSelfStatus())
		}
	}

	// Isolate node 1
	// Simulate isolation by stopping gossiper for node 1 and starting it back,
	// but by not providing peer IPs and setting cluster size to 3.
	gossipers[1].Stop(time.Duration(10) * time.Second)
	gossipers[1].InitStore(types.NodeId("1"), "v1")
	gossipers[1].Start([]string{})

	// Lets sleep so that the nodes gossip and update their quorum
	time.Sleep(types.DEFAULT_GOSSIP_INTERVAL * time.Duration(len(nodes)+1))

	for i, g := range gossipers {
		if i == 1 {
			if g.GetSelfStatus() != types.NODE_STATUS_WAITING_FOR_QUORUM {
				t.Error("Expected Node ", i, " status to be ", types.NODE_STATUS_WAITING_FOR_QUORUM, " Got: ", g.GetSelfStatus())
			}
			continue
		}
		if g.GetSelfStatus() != types.NODE_STATUS_UP {
			t.Error("Expected Node ", i, " status to be ", types.NODE_STATUS_UP, " Got: ", g.GetSelfStatus())
		}
	}
}

func TestQuorumNetworkPartition(t *testing.T) {
	printTestInfo()
	nodes := []string{
		"127.0.0.1:9909",
		"127.0.0.2:9910",
		"127.0.0.3:9911",
		"127.0.0.4:9912",
		"127.0.0.5:9913",
	}

	// Simulate a network parition. Node 0-2 in parition 1. Node 3-4 in partition 2.
	var gossipers []*GossiperImpl
	// Partition 1
	for i := 0; i < 3; i++ {
		nodeId := types.NodeId(strconv.FormatInt(int64(i), 10))
		var g *GossiperImpl
		g, _ = startNode(t, nodes[i], nodeId, []string{nodes[0], nodes[1], nodes[2]}, 3)
		gossipers = append(gossipers, g)
	}
	// Parition 2
	for i := 3; i < 5; i++ {
		nodeId := types.NodeId(strconv.FormatInt(int64(i), 10))
		var g *GossiperImpl
		g, _ = startNode(t, nodes[i], nodeId, []string{nodes[3], nodes[4]}, 2)
		gossipers = append(gossipers, g)
	}
	// Let the nodes gossip
	time.Sleep(types.DEFAULT_GOSSIP_INTERVAL * time.Duration(len(nodes)))
	for i, g := range gossipers {
		if g.GetSelfStatus() != types.NODE_STATUS_UP {
			t.Error("Expected Node ", i, " status to be ", types.NODE_STATUS_UP, " Got: ", g.GetSelfStatus())
		}
	}

	// Setup the partition by updating the cluster size
	for _, g := range gossipers {
		g.UpdateClusterSize(5)
	}

	// Let the nodes update their quorum
	time.Sleep(time.Duration(3) * time.Second)
	// Partition 1
	for i := 0; i < 3; i++ {
		if gossipers[i].GetSelfStatus() != types.NODE_STATUS_UP {
			t.Error("Expected Node ", i, " status to be ", types.NODE_STATUS_UP, " Got: ", gossipers[i].GetSelfStatus())
		}

	}
	// Parition 2
	for i := 3; i < 5; i++ {
		if gossipers[i].GetSelfStatus() != types.NODE_STATUS_UP_AND_WAITING_FOR_QUORUM {
			t.Error("Expected Node ", i, " status to be ", types.NODE_STATUS_UP_AND_WAITING_FOR_QUORUM, " Got: ", gossipers[i].GetSelfStatus())
		}
	}

	time.Sleep(TestQuorumTimeout)
	// Parition 2
	for i := 3; i < 5; i++ {
		if gossipers[i].GetSelfStatus() != types.NODE_STATUS_WAITING_FOR_QUORUM {
			t.Error("Expected Node ", i, " status to be ", types.NODE_STATUS_WAITING_FOR_QUORUM, " Got: ", gossipers[i].GetSelfStatus())
		}
	}
}

func TestQuorumEventHandling(t *testing.T) {
	printTestInfo()

	nodes := []string{
		"127.0.0.1:9914",
		"127.0.0.2:9915",
		"127.0.0.3:9916",
		"127.0.0.4:9917",
		"127.0.0.5:9918",
	}

	// Start all nodes
	var gossipers []*GossiperImpl
	for i := 0; i < len(nodes); i++ {
		nodeId := types.NodeId(strconv.FormatInt(int64(i), 10))
		var g *GossiperImpl
		g, _ = startNode(t, nodes[i], nodeId, []string{nodes[0]}, 3)
		gossipers = append(gossipers, g)
	}

	// Bring node 4 down. Quorum is still up
	gossipers[4].Stop(types.DEFAULT_GOSSIP_INTERVAL * time.Duration(len(nodes)))

	time.Sleep(types.DEFAULT_GOSSIP_INTERVAL * time.Duration(len(nodes)))

	for i := 0; i < len(nodes)-1; i++ {
		if gossipers[i].GetSelfStatus() != types.NODE_STATUS_UP {
			t.Error("Expected Node ", i, " status to be ", types.NODE_STATUS_UP, " Got: ", gossipers[i].GetSelfStatus())
		}
	}

	// Bring node 3,node 2, node 1 down
	gossipers[3].Stop(types.DEFAULT_GOSSIP_INTERVAL * time.Duration(len(nodes)))
	gossipers[2].Stop(types.DEFAULT_GOSSIP_INTERVAL * time.Duration(len(nodes)))
	gossipers[1].Stop(types.DEFAULT_GOSSIP_INTERVAL * time.Duration(len(nodes)))

	time.Sleep(types.DEFAULT_GOSSIP_INTERVAL * time.Duration(len(nodes) + 1))
	//time.Sleep(types.DEFAULT_GOSSIP_INTERVAL)

	if gossipers[0].GetSelfStatus() != types.NODE_STATUS_UP_AND_WAITING_FOR_QUORUM {
		t.Error("Expected Node 0 status to be ", types.NODE_STATUS_UP_AND_WAITING_FOR_QUORUM, " Got: ", gossipers[0].GetSelfStatus())
	}

	// Start Node 2
	gossipers[2].Start([]string{nodes[0]})
	gossipers[2].UpdateClusterSize(5)

	time.Sleep(types.DEFAULT_GOSSIP_INTERVAL)

	// Node 0 still not in quorum. But should be up as quorum timeout not occured yet
	if gossipers[0].GetSelfStatus() != types.NODE_STATUS_UP_AND_WAITING_FOR_QUORUM {
		t.Error("Expected Node 0  status to be ", types.NODE_STATUS_UP_AND_WAITING_FOR_QUORUM, " Got: ", gossipers[0].GetSelfStatus())
	}

	// Sleep for quorum timeout to occur
	time.Sleep(gossipers[0].quorumTimeout + 2*time.Second)

	if gossipers[0].GetSelfStatus() != types.NODE_STATUS_WAITING_FOR_QUORUM {
		t.Error("Expected Node 0 status to be ", types.NODE_STATUS_WAITING_FOR_QUORUM, " Got: ", gossipers[0].GetSelfStatus())
	}

	// Start Node 1
	gossipers[1].Start([]string{nodes[0]})
	gossipers[1].UpdateClusterSize(5)

	time.Sleep(time.Duration(2) * types.DEFAULT_GOSSIP_INTERVAL)

	// Node 0 should now be up
	if gossipers[0].GetSelfStatus() != types.NODE_STATUS_UP {
		t.Error("Expected Node 0 status to be ", types.NODE_STATUS_UP, " Got: ", gossipers[0].GetSelfStatus())
	}

}