// File: simulator/collective/broadcast.go
package collective

import (
	"fmt"
	"uPIMulator/src/device/simulator/interconnect"
)

// BroadcastTopology represents a tree-based broadcast network
type BroadcastTopology struct {
	numNodes int
	network  *interconnect.MeshNetwork
	
	// Tree parameters
	branchingFactor int // Number of children per node (typically 2)
	
	// Node positions in mesh
	nodePositions []struct {
		x, y int
	}
	
	// Statistics
	totalMessages int64
	totalLatency  int64
	cycles        int64
}

// Init initializes the broadcast topology
func (bt *BroadcastTopology) Init(network *interconnect.MeshNetwork, numNodes int) {
	bt.network = network
	bt.numNodes = numNodes
	bt.branchingFactor = 2 // Binary tree
	bt.nodePositions = make([]struct{ x, y int }, numNodes)
	
	// Map nodes to mesh positions (same as ring)
	for i := 0; i < numNodes; i++ {
		bt.nodePositions[i].x = i / 8
		bt.nodePositions[i].y = i % 8
	}
	
	fmt.Printf("✓ Broadcast topology initialized: %d nodes (binary tree)\n", numNodes)
}

// GetParent returns the parent node ID in the tree
func (bt *BroadcastTopology) GetParent(nodeID int) int {
	if nodeID == 0 {
		return -1 // Root has no parent
	}
	return (nodeID - 1) / bt.branchingFactor
}

// GetChildren returns the children node IDs in the tree
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

// GetTreeDepth returns the depth of the tree
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

// SendToChildren sends data from a node to all its children
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

// Broadcast performs tree-based broadcast from root to all nodes
// Root node has the initial data, all others receive it
func (bt *BroadcastTopology) Broadcast(rootID int, data []byte) error {
	if rootID < 0 || rootID >= bt.numNodes {
		return fmt.Errorf("invalid root ID: %d", rootID)
	}
	
	fmt.Printf("\n=== Tree Broadcast from Node %d ===\n", rootID)
	fmt.Printf("Data size: %d bytes\n", len(data))
	fmt.Printf("Tree depth: %d levels\n", bt.GetTreeDepth())
	
	// Track which nodes have received data
	received := make([]bool, bt.numNodes)
	received[rootID] = true
	
	// Broadcast level by level
	depth := bt.GetTreeDepth()
	for level := 0; level < depth; level++ {
		fmt.Printf("\nLevel %d:\n", level)
		
		// Find all nodes that have data and can send
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
			break // No more nodes to send
		}
		
		// All sending nodes transmit to their children
		for _, nodeID := range sendingNodes {
			children := bt.GetChildren(nodeID)
			fmt.Printf("  Node %d → Nodes %v\n", nodeID, children)
			
			err := bt.SendToChildren(nodeID, data)
			if err != nil {
				return err
			}
			
			// Mark children as having received data
			for _, childID := range children {
				received[childID] = true
			}
		}
		
		// Wait for all packets to be delivered
		if !bt.network.RunUntilEmpty(1000) {
			return fmt.Errorf("network timeout at level %d", level)
		}
	}
	
	// Verify all nodes received data
	for nodeID := 0; nodeID < bt.numNodes; nodeID++ {
		if !received[nodeID] {
			return fmt.Errorf("node %d did not receive data", nodeID)
		}
	}
	
	fmt.Printf("\n✓ Broadcast complete: all %d nodes received data\n", bt.numNodes)
	return nil
}

// BroadcastSimple is a simplified synchronous broadcast
func (bt *BroadcastTopology) BroadcastSimple(rootID int, data []byte) (int, error) {
	if rootID < 0 || rootID >= bt.numNodes {
		return 0, fmt.Errorf("invalid root ID: %d", rootID)
	}
	
	// Track which nodes have data
	hasData := make([]bool, bt.numNodes)
	hasData[rootID] = true
	
	steps := 0
	totalReceived := 1 // Root already has data
	
	// Broadcast until all nodes have data
	for totalReceived < bt.numNodes {
		// Find nodes that can send (have data but haven't sent to children yet)
		for nodeID := 0; nodeID < bt.numNodes; nodeID++ {
			if !hasData[nodeID] {
				continue
			}
			
			children := bt.GetChildren(nodeID)
			for _, childID := range children {
				if !hasData[childID] {
					// Send to child
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
		
		// Run network
		bt.network.RunUntilEmpty(1000)
		steps++
		
		if steps > bt.GetTreeDepth()+5 {
			return steps, fmt.Errorf("broadcast timeout")
		}
	}
	
	return steps, nil
}

// MultiRootBroadcast performs broadcast from multiple roots simultaneously
// Useful for multi-source scenarios
func (bt *BroadcastTopology) MultiRootBroadcast(rootIDs []int, data [][]byte) error {
	if len(rootIDs) != len(data) {
		return fmt.Errorf("number of roots and data arrays must match")
	}
	
	fmt.Printf("\n=== Multi-Root Broadcast (%d roots) ===\n", len(rootIDs))
	
	// Perform all broadcasts (may cause congestion)
	for i, rootID := range rootIDs {
		_, err := bt.BroadcastSimple(rootID, data[i])
				if err != nil {
			return fmt.Errorf("broadcast from root %d failed: %w", rootID, err)
		}
	}
	
	fmt.Printf("✓ All %d broadcasts completed\n", len(rootIDs))
	return nil
}

// GetStatistics returns broadcast topology statistics
func (bt *BroadcastTopology) GetStatistics() map[string]interface{} {
	stats := make(map[string]interface{})
	stats["num_nodes"] = bt.numNodes
	stats["branching_factor"] = bt.branchingFactor
	stats["tree_depth"] = bt.GetTreeDepth()
	stats["total_messages"] = bt.totalMessages
	stats["avg_messages_per_node"] = float64(bt.totalMessages) / float64(bt.numNodes)
	
	// Theoretical minimum messages for broadcast
	theoreticalMin := bt.numNodes - 1 // Each non-root receives exactly once
	stats["theoretical_min_messages"] = theoreticalMin
	stats["efficiency"] = float64(theoreticalMin) / float64(bt.totalMessages)
	
	netStats := bt.network.GetStatistics()
	stats["network_latency"] = netStats["avg_latency"]
	stats["network_throughput"] = netStats["throughput"]
	
	return stats
}

// PrintTree prints the tree structure
func (bt *BroadcastTopology) PrintTree() {
	fmt.Println("\n=== Broadcast Tree Structure ===")
	fmt.Printf("Root: Node 0\n")
	
	depth := bt.GetTreeDepth()
	for level := 0; level < depth; level++ {
		fmt.Printf("\nLevel %d:\n", level+1)
		
		// Find nodes at this level
		for nodeID := 1; nodeID < bt.numNodes; nodeID++ {
			parent := bt.GetParent(nodeID)
			
			// Check if this node is at current level
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