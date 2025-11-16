package collective

import (
	"fmt"
	"uPIMulator/src/device/simulator/interconnect"
)

type BroadcastTopology struct {
	numNodes int
	network  *interconnect.MeshNetwork
	
	branchingFactor int
	
	nodePositions []struct {
		x, y int
	}
	
	totalMessages int64
	totalLatency  int64
	cycles        int64
}

func (bt *BroadcastTopology) Init(network *interconnect.MeshNetwork, numNodes int) {
	bt.network = network
	bt.numNodes = numNodes
	bt.branchingFactor = 2
	bt.nodePositions = make([]struct{ x, y int }, numNodes)
	
	for i := 0; i < numNodes; i++ {
		bt.nodePositions[i].x = i / 8
		bt.nodePositions[i].y = i % 8
	}
	
	fmt.Printf("✓ Broadcast topology initialized: %d nodes (binary tree)\n", numNodes)
}

func (bt *BroadcastTopology) GetParent(nodeID int) int {
	if nodeID == 0 {
		return -1
	}
	return (nodeID - 1) / bt.branchingFactor
}

func (bt *BroadcastTopology) GetChildren(nodeID int) []int {
	children := make([]int, 0, bt.branchingFactor)
	
	for i := 0; i < bt.branchingFactor; i++ {
		childID := nodeID*bt.branchingFactor + 1 + i
		if childID < bt.numNodes {
			children = append(children, childID)
		}
	}
	
	return children
}

func (bt *BroadcastTopology) GetTreeDepth() int {
	if bt.numNodes <= 1 {
		return 0
	}
	
	depth := 0
	nodes := 1
	for nodes < bt.numNodes {
		nodes *= bt.branchingFactor
		depth++
	}
	
	return depth
}

func (bt *BroadcastTopology) SendToChildren(nodeID int, data []byte) error {
	children := bt.GetChildren(nodeID)
	
	for _, childID := range children {
		srcX := bt.nodePositions[nodeID].x
		srcY := bt.nodePositions[nodeID].y
		dstX := bt.nodePositions[childID].x
		dstY := bt.nodePositions[childID].y
		
		_, err := bt.network.InjectPacket(srcX, srcY, dstX, dstY, data)
		if err != nil {
			return fmt.Errorf("node %d failed to send to child %d: %w", nodeID, childID, err)
		}
		
		bt.totalMessages++
	}
	
	return nil
}

func (bt *BroadcastTopology) Broadcast(rootID int, data []byte) error {
	if rootID < 0 || rootID >= bt.numNodes {
		return fmt.Errorf("invalid root ID: %d", rootID)
	}
	
	fmt.Printf("\n=== Tree Broadcast from Node %d ===\n", rootID)
	fmt.Printf("Data size: %d bytes\n", len(data))
	fmt.Printf("Tree depth: %d levels\n", bt.GetTreeDepth())
	
	received := make([]bool, bt.numNodes)
	received[rootID] = true
	
	depth := bt.GetTreeDepth()
	for level := 0; level < depth; level++ {
		fmt.Printf("\nLevel %d:\n", level)
		
		sendingNodes := make([]int, 0)
		for nodeID := 0; nodeID < bt.numNodes; nodeID++ {
			if received[nodeID] {
				children := bt.GetChildren(nodeID)
				if len(children) > 0 {
					sendingNodes = append(sendingNodes, nodeID)
				}
			}
		}
		
		if len(sendingNodes) == 0 {
			break
		}
		
		for _, nodeID := range sendingNodes {
			children := bt.GetChildren(nodeID)
			fmt.Printf("  Node %d → Nodes %v\n", nodeID, children)
			
			err := bt.SendToChildren(nodeID, data)
			if err != nil {
				return err
			}
			
			for _, childID := range children {
				received[childID] = true
			}
		}
		
		if !bt.network.RunUntilEmpty(1000) {
			return fmt.Errorf("network timeout at level %d", level)
		}
	}
	
	for nodeID := 0; nodeID < bt.numNodes; nodeID++ {
		if !received[nodeID] {
			return fmt.Errorf("node %d did not receive data", nodeID)
		}
	}
	
	fmt.Printf("\n✓ Broadcast complete: all %d nodes received data\n", bt.numNodes)
	return nil
}

func (bt *BroadcastTopology) BroadcastSimple(rootID int, data []byte) (int, error) {
	if rootID < 0 || rootID >= bt.numNodes {
		return 0, fmt.Errorf("invalid root ID: %d", rootID)
	}
	
	hasData := make([]bool, bt.numNodes)
	hasData[rootID] = true
	
	steps := 0
	totalReceived := 1

	for totalReceived < bt.numNodes {
		for nodeID := 0; nodeID < bt.numNodes; nodeID++ {
			if !hasData[nodeID] {
				continue
			}
			
			children := bt.GetChildren(nodeID)
			for _, childID := range children {
				if !hasData[childID] {
					srcX := bt.nodePositions[nodeID].x
					srcY := bt.nodePositions[nodeID].y
					dstX := bt.nodePositions[childID].x
					dstY := bt.nodePositions[childID].y
					
					bt.network.InjectPacket(srcX, srcY, dstX, dstY, data)
					hasData[childID] = true
					totalReceived++
					bt.totalMessages++
				}
			}
		}
		
		bt.network.RunUntilEmpty(1000)
		steps++
		
		if steps > bt.GetTreeDepth()+5 {
			return steps, fmt.Errorf("broadcast timeout")
		}
	}
	
	return steps, nil
}

func (bt *BroadcastTopology) MultiRootBroadcast(rootIDs []int, data [][]byte) error {
	if len(rootIDs) != len(data) {
		return fmt.Errorf("number of roots and data arrays must match")
	}
	
	fmt.Printf("\n=== Multi-Root Broadcast (%d roots) ===\n", len(rootIDs))
	
	for i, rootID := range rootIDs {
		_, err := bt.BroadcastSimple(rootID, data[i])
				if err != nil {
			return fmt.Errorf("broadcast from root %d failed: %w", rootID, err)
		}
	}
	
	fmt.Printf("✓ All %d broadcasts completed\n", len(rootIDs))
	return nil
}

func (bt *BroadcastTopology) GetStatistics() map[string]interface{} {
	stats := make(map[string]interface{})
	stats["num_nodes"] = bt.numNodes
	stats["branching_factor"] = bt.branchingFactor
	stats["tree_depth"] = bt.GetTreeDepth()
	stats["total_messages"] = bt.totalMessages
	stats["avg_messages_per_node"] = float64(bt.totalMessages) / float64(bt.numNodes)
	
	theoreticalMin := bt.numNodes - 1
	stats["theoretical_min_messages"] = theoreticalMin
	stats["efficiency"] = float64(theoreticalMin) / float64(bt.totalMessages)
	
	netStats := bt.network.GetStatistics()
	stats["network_latency"] = netStats["avg_latency"]
	stats["network_throughput"] = netStats["throughput"]
	
	return stats
}

func (bt *BroadcastTopology) PrintTree() {
	fmt.Println("\n=== Broadcast Tree Structure ===")
	fmt.Printf("Root: Node 0\n")
	
	depth := bt.GetTreeDepth()
	for level := 0; level < depth; level++ {
		fmt.Printf("\nLevel %d:\n", level+1)
		
		for nodeID := 1; nodeID < bt.numNodes; nodeID++ {
			parent := bt.GetParent(nodeID)
			
			nodeLevel := 0
			tempID := nodeID
			for tempID > 0 {
				tempID = bt.GetParent(tempID)
				nodeLevel++
			}
			
			if nodeLevel == level+1 {
				children := bt.GetChildren(nodeID)
				if len(children) > 0 {
					fmt.Printf("  Node %d (parent: %d) → Children: %v\n", nodeID, parent, children)
				} else {
					fmt.Printf("  Node %d (parent: %d) → Leaf\n", nodeID, parent)
				}
			}
		}
	}
}

func (bt *BroadcastTopology) Fini() {
	bt.nodePositions = nil
}
