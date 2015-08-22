package proto

import (
	"fmt"
	"math/rand"
	//"reflect"
	//"testing"
	"runtime"
	"testing"
	"time"

	"github.com/libopenstorage/gossip/api"
)

const (
	CPU    string     = "CPU"
	MEMORY string     = "MEMORY"
	ID     api.NodeId = 4
)

func printTestInfo() {
	pc := make([]uintptr, 3) // at least 1 entry needed
	runtime.Callers(2, pc)
	f := runtime.FuncForPC(pc[0])
	fmt.Println("RUNNING TEST: ", f.Name())
}

func flipCoin() bool {
	if rand.Intn(100) < 50 {
		return true
	}
	return false
}

func fillUpNodeInfo(node *api.NodeInfo, i int) {
	node.Id = api.NodeId(i)
	node.LastUpdateTs = time.Now()
	node.Status = api.NODE_STATUS_UP

	value := make(map[string]api.NodeId)
	value[CPU] = node.Id
	value[MEMORY] = node.Id
	node.Value = value
}

func fillUpNodeInfoMap(nodes NodeInfoMap, numOfNodes int) {
	for i := 0; i < numOfNodes; i++ {
		var node api.NodeInfo
		fillUpNodeInfo(&node, i)
		nodes[node.Id] = node
	}
}

func TestGossipStoreUpdateSelf(t *testing.T) {
	printTestInfo()
	// emtpy store
	g := NewGossipStore(ID).(*GossipStoreImpl)

	id := g.NodeId()
	if id != ID {
		t.Error("Incorrect NodeId(), got: ", id,
			" expected: ", ID)
	}

	value := "string"
	key1 := api.StoreKey("key1")
	// key absent
	g.UpdateSelf(key1, value)
	nodeValue, ok := g.kvMap[key1]
	if !ok {
		t.Error("UpdateSelf adding new key failed, after update state: ",
			g.kvMap)
	} else {
		nodeInfo, ok := nodeValue[ID]
		if !ok {
			t.Error("UpdateSelf adding new id failed, nodeMap: ", nodeValue)
		}
		if nodeInfo.Value != value {
			t.Error("UpdateSelf failed, got value: ", nodeInfo.Value,
				" got: ", value)
		}
	}

	// key present id absent
	delete(g.kvMap[key1], ID)
	g.UpdateSelf(key1, value)
	nodeValue = g.kvMap[key1]
	nodeInfo, ok := nodeValue[ID]
	if !ok {
		t.Error("UpdateSelf adding new id failed, nodeMap: ", nodeValue)
	}
	if nodeInfo.Value != value {
		t.Error("UpdateSelf failed, got value: ", nodeInfo.Value,
			" got: ", value)
	}

	// key present id present
	prevTs := nodeInfo.LastUpdateTs
	value = "newValue"
	g.UpdateSelf(key1, value)
	nodeValue = g.kvMap[key1]
	nodeInfo = nodeValue[ID]
	if !nodeInfo.LastUpdateTs.After(prevTs) {
		t.Error("UpdateSelf failed to update timestamp, prev: ", prevTs,
			" got: ", nodeInfo.LastUpdateTs)
	}
	if nodeInfo.Value != value {
		t.Error("UpdateSelf failed, got value: ", nodeInfo.Value,
			" got: ", value)
	}

}

func TestGossipStoreGetStoreKeyValue(t *testing.T) {
	printTestInfo()

	// Case: emtpy store
	// Case: key absent
	g := NewGossipStore(ID).(*GossipStoreImpl)

	keyList := []api.StoreKey{"key1", "key2"}

	nodeInfoList := g.GetStoreKeyValue(keyList[0])
	if len(nodeInfoList.List) != 0 {
		t.Error("Expected empty node info list, got: ", nodeInfoList.List)
	}
	g.kvMap[keyList[0]] = make(NodeInfoMap)
	g.kvMap[keyList[1]] = make(NodeInfoMap)

	// Case: key present but no nodes
	nodeInfoList = g.GetStoreKeyValue(keyList[0])
	if len(nodeInfoList.List) != 0 {
		t.Error("Expected empty node info list, got: ", nodeInfoList.List)
	}

	// Case: key present with nodes with holes in node ids
	fillUpNodeInfoMap(g.kvMap[keyList[0]], 6)
	if len(g.kvMap[keyList[0]]) != 6 {
		t.Error("Failed to fillup node info map properly, got: ",
			g.kvMap[keyList[0]])
	}
	delete(g.kvMap[keyList[0]], 0)
	delete(g.kvMap[keyList[0]], 2)
	delete(g.kvMap[keyList[0]], 4)
	nodeInfoList = g.GetStoreKeyValue(keyList[0])
	if len(nodeInfoList.List) != 6 {
		t.Error("Expected list with atleast 6 elements, got: ", nodeInfoList.List)
	}
	for i := 0; i < len(nodeInfoList.List); i++ {
		if i%2 == 0 {
			if nodeInfoList.List[i].Status != api.NODE_STATUS_INVALID {
				t.Error("Invalid node expected, got: ", nodeInfoList.List[i])
			}
			continue
		}
		infoMap := nodeInfoList.List[i].Value.(map[string]api.NodeId)
		if nodeInfoList.List[i].Id != api.NodeId(i) ||
			nodeInfoList.List[i].Status != api.NODE_STATUS_UP ||
			infoMap[CPU] != api.NodeId(i) ||
			infoMap[MEMORY] != api.NodeId(i) {
			t.Error("Invalid node content received, got: ", nodeInfoList.List[i])
		}
	}

}

func TestGossipStoreMetaInfo(t *testing.T) {
	printTestInfo()

	g := NewGossipStore(ID).(*GossipStoreImpl)

	// Case: store empty
	m := g.MetaInfo()
	if len(m) != 0 {
		t.Error("Empty meta info expected from empty store, got: ", m)
	}

	nodeLen := 10
	// Case: store with keys, some keys have no ids, other have ids,
	keyList := []api.StoreKey{"key1", "key2", "key3"}
	for i, key := range keyList {
		g.kvMap[key] = make(NodeInfoMap)
		fillUpNodeInfoMap(g.kvMap[key], nodeLen)

		for j := 0; j < nodeLen; j++ {
			if i%2 == 0 {
				if j%2 == 0 {
					delete(g.kvMap[key], api.NodeId(j))
				}
			} else {
				if j%2 == 1 {
					delete(g.kvMap[key], api.NodeId(j))
				}
			}
		}
	}

	m = g.MetaInfo()
	if len(m) != 3 {
		t.Error("Meta info len error, got: ", len(m), " expected: ", len(keyList))
	}
	for key, metaInfoList := range m {
		if len(metaInfoList.List) != len(g.kvMap[key]) {
			t.Error("Unexpected meta info returned, expected: ", nodeLen/2,
				" got: ", len(metaInfoList.List))
		}

		for _, metaInfo := range metaInfoList.List {
			nodeInfo, ok := g.kvMap[key][metaInfo.Id]
			if !ok {
				t.Error("Unexpected id returned, meta info: ", metaInfo,
					" store: ", g.kvMap[key])
				continue
			}

			if nodeInfo.Id != metaInfo.Id ||
				nodeInfo.LastUpdateTs != metaInfo.LastUpdateTs {
				t.Error("MetaInfo mismatch, nodeInfo: ", nodeInfo,
					" metaInfo: ", metaInfo)
			}
		}
	}
}

func TestGossipStoreDiff(t *testing.T) {
	printTestInfo()

	nodeLen := 10
	g1 := NewGossipStore(ID).(*GossipStoreImpl)
	g2 := NewGossipStore(ID).(*GossipStoreImpl)

	// Case: empty store and emtpy meta info
	g2New, g1New := g1.Diff(g2.MetaInfo())
	if len(g2New) != 0 || len(g1New) != 0 {
		t.Error("Diff of empty stores not empty, g2: ", g2,
			" g1: ", g1)
	}

	// Case: empty store and non-empty meta info
	keyList := []api.StoreKey{"key1", "key2", "key3"}
	for _, key := range keyList {
		g2.kvMap[key] = make(NodeInfoMap)
		fillUpNodeInfoMap(g2.kvMap[key], nodeLen)
	}

	g2New, g1New = g1.Diff(g2.MetaInfo())
	if len(g2New) != len(g2.kvMap) ||
		len(g1New) != 0 {
		t.Error("Diff lens unexpected, g1New: ", len(g1New),
			", g2New: ", len(g2New), " g2: ", len(g2.kvMap))
	}

	for key, nodeIds := range g2New {
		if len(nodeIds) != len(g2.kvMap[key]) {
			t.Error("Nodes mismatch, got ids: ", nodeIds,
				", expected: ", g2.kvMap[key])
		}
	}

	// Case: diff of itself should return empty
	g2New, g1New = g2.Diff(g2.MetaInfo())
	if len(g2New) != 0 || len(g1New) != 0 {
		t.Error("Diff of empty stores not empty, g2New: ", g2New,
			" g1New: ", g1New)
	}

	// Case: empty store meta info with store value
	g1New, g2New = g2.Diff(g1.MetaInfo())
	if len(g2New) != len(g2.kvMap) ||
		len(g1New) != 0 {
		t.Error("Diff lens unexpected, g1New: ", len(g1New),
			", g2New: ", len(g2New), " g2: ", len(g2.kvMap))
	}

	for key, nodeIds := range g2New {
		if len(nodeIds) != len(g2.kvMap[key]) {
			t.Error("Nodes mismatch, got ids: ", nodeIds,
				", expected: ", g2.kvMap[key])
		}
	}

	// Case: diff with meta info such that
	//   - keys are absent from store
	//   - keys are present but no node ids
	//   - keys are present, some have old and some have new ts,
	//      some have new ids and some ids from meta are missing
	keyIdMap := make(map[api.StoreKey]api.NodeId)
	for i, key := range keyList {
		g2.kvMap[key] = make(NodeInfoMap)
		fillUpNodeInfoMap(g2.kvMap[key], nodeLen)
		g1.kvMap[key] = make(NodeInfoMap)
		for id, info := range g2.kvMap[key] {
			g1.kvMap[key][id] = info
		}
		if i < 2 {
			// key3 values are same
			keyIdMap[key] = api.NodeId(i)
		} else {
			continue
		}

		// g2 has newer nodes with even id
		for id, _ := range g2.kvMap[key] {
			if id%2 == 0 {
				nodeInfo := g2.kvMap[key][id]
				nodeInfo.LastUpdateTs = time.Now()
				g2.kvMap[key][id] = nodeInfo
			}
		}
		// g1 has newer nodes with od ids
		for id, _ := range g1.kvMap[key] {
			if id%2 == 1 {
				nodeInfo := g1.kvMap[key][id]
				nodeInfo.LastUpdateTs = time.Now()
				g1.kvMap[key][id] = nodeInfo
			}
		}
	}

	g2New, g1New = g1.Diff(g2.MetaInfo())
	if len(g2New) != len(g1New) || len(g2New) != 2 {
		t.Error("Diff returned more than 2 keys, g2New: ", g2New,
			" g1New: ", g1New)
	}
	for key, nodeIds := range g2New {
		_, ok := keyIdMap[key]
		if !ok {
			t.Error("Invalid key returned: ", key)
		}

		for _, id := range nodeIds {
			if id%2 != 0 {
				t.Error("g2New has invalid node id: ", id)
			}
		}
	}
	for key, nodeIds := range g1New {
		_, ok := keyIdMap[key]
		if !ok {
			t.Error("Invalid key returned: ", key)
		}

		for _, id := range nodeIds {
			if id%2 != 1 {
				t.Error("g2New has invalid node id: ", id)
			}
		}
	}
}

func TestGossipStoreSubset(t *testing.T) {
	printTestInfo()

	g := NewGossipStore(ID).(*GossipStoreImpl)

	// empty store and empty nodelist and non-empty nodelist
	diff := api.StoreNodes{}
	sv := g.Subset(diff)
	if len(sv) != 0 {
		t.Error("Emtpy subset expected, got: ", sv)
	}

	nodeLen := 10
	keyList := []api.StoreKey{"key1", "key2", "key3"}
	for _, key := range keyList {
		nodeIds := make([]api.NodeId, nodeLen*2)
		for i := 0; i < nodeLen*2; i++ {
			nodeIds[i] = api.NodeId(i)
		}
		diff[key] = nodeIds
	}

	sv = g.Subset(diff)
	if len(sv) != 0 {
		t.Error("Emtpy subset expected, got: ", sv)
	}

	// store and diff asks for 20 nodes but store
	// has only a subset of it, as well as some keys
	// it does not know about
	for i, key := range keyList {
		if i > 1 {
			continue
		}
		g.kvMap[key] = make(NodeInfoMap)
		fillUpNodeInfoMap(g.kvMap[key], nodeLen)
	}

	sv = g.Subset(diff)
	if len(sv) != 2 {
		t.Error("Subset has more keys then requested: ", sv)
	}
	for i, key := range keyList {
		nodeInfoMap, ok := sv[key]
		if i > 1 {
			if ok {
				t.Error("Subset has a key not requested: ", key)
			}
			continue
		}

		if len(nodeInfoMap) != nodeLen {
			t.Error("Subset has more keys than store: ", nodeInfoMap)
		}

		storeInfoMap := g.kvMap[key]

		if len(storeInfoMap) != len(nodeInfoMap) {
			t.Error("Subset is different then expected, got: ",
				len(nodeInfoMap), " expected: ",
				len(storeInfoMap))
		}
	}

}

/*
 GetStoreKeys
 // no keys
 // some keys

 Update
 // empty store and empty diff and non-empty diff
 // non-empty store and empty diff
 // store and diff has values such that -
 //   - diff has new keys
 //   - diff has same keys but some ids are newer
 //   - diff has same keys and same ids but content is newer
*/