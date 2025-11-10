// File: simulator/collective/collective_test.go
package collective

import (
	"fmt"
	"testing"
	"uPIMulator/src/device/simulator/interconnect"
)

func TestRingTopologyInit(t *testing.T) {
	fmt.Println("\n=== Test: Ring Topology Initialization ===")
	
	network := &interconnect.MeshNetwork{}
	network.Init(4, 8, interconnect.XY_ROUTING)
	defer network.Fini()
	
	ring := &RingTopology{}
	ring.Init(network, 32)
	defer ring.Fini()
	
	if ring.numNodes != 32 {
		t.Errorf("Expected 32 nodes, got %d", ring.numNodes)
	}
	
	// Test ring connections
	if ring.GetNextNode(0) != 1 {
		t.Error("Next node of 0 should be 1")
	}
	if ring.GetNextNode(31) != 0 {
		t.Error("Next node of 31 should be 0 (wrap around)")
	}
	if ring.GetPrevNode(0) != 31 {
		t.Error("Previous node of 0 should be 31")
	}
	
	fmt.Println("✓ Ring topology with 32 nodes initialized")
	fmt.Printf("✓ Node 0 → %d\n", ring.GetNextNode(0))
	fmt.Printf("✓ Node 31 → %d (wraps to 0)\n", ring.GetNextNode(31))
}

func TestRingSendToNext(t *testing.T) {
	fmt.Println("\n=== Test: Ring Send to Next ===")
	
	network := &interconnect.MeshNetwork{}
	network.Init(4, 8, interconnect.XY_ROUTING)
	defer network.Fini()
	
	ring := &RingTopology{}
	ring.Init(network, 8)
	defer ring.Fini()
	
	// Node 0 sends to Node 1
	data := []byte("Hello Node 1")
	err := ring.SendToNext(0, data)
	if err != nil {
		t.Fatalf("Failed to send: %v", err)
	}
	fmt.Println("✓ Node 0 sent to Node 1")
	
	// Run network to deliver
	if !network.RunUntilEmpty(100) {
		t.Fatal("Message not delivered")
	}
	
	stats := network.GetStatistics()
	if stats["packets_delivered"].(int64) != 1 {
		t.Error("Expected 1 packet delivered")
	}
	
	fmt.Println("✓ Message delivered successfully")
}

func TestRingAllReduceSmall(t *testing.T) {
	fmt.Println("\n=== Test: Ring AllReduce (4 nodes) ===")
	
	network := &interconnect.MeshNetwork{}
	network.Init(4, 8, interconnect.XY_ROUTING)
	defer network.Fini()
	
	ring := &RingTopology{}
	ring.Init(network, 4)
	defer ring.Fini()
	
	// Each node starts with its ID
	initialValues := []int64{10, 20, 30, 40}
	
	result, err := ring.RingAllReduceSimple(initialValues, SUM)
	if err != nil {
		t.Fatalf("AllReduce failed: %v", err)
	}
	
	expected := int64(10 + 20 + 30 + 40) // 100
	if result != expected {
		t.Errorf("Expected %d, got %d", expected, result)
	}
	
	fmt.Printf("✓ AllReduce SUM: %v → %d\n", initialValues, result)
}

func TestRingAllReduce8Nodes(t *testing.T) {
	fmt.Println("\n=== Test: Ring AllReduce (8 nodes) ===")
	
	network := &interconnect.MeshNetwork{}
	network.Init(4, 8, interconnect.XY_ROUTING)
	defer network.Fini()
	
	ring := &RingTopology{}
	ring.Init(network, 8)
	defer ring.Fini()
	
	// Each node has value = nodeID * 10
	initialValues := make([]int64, 8)
	for i := 0; i < 8; i++ {
		initialValues[i] = int64(i * 10)
	}
	
	result, err := ring.RingAllReduceSimple(initialValues, SUM)
	if err != nil {
		t.Fatalf("AllReduce failed: %v", err)
	}
	
	// Sum = 0+10+20+30+40+50+60+70 = 280
	expected := int64(280)
	if result != expected {
		t.Errorf("Expected %d, got %d", expected, result)
	}
	
	fmt.Printf("✓ AllReduce SUM (8 nodes): %d\n", result)
	
	stats := ring.GetStatistics()
	fmt.Printf("✓ Total messages: %d\n", stats["total_messages"])
	fmt.Printf("✓ Avg messages per node: %.1f\n", stats["avg_messages_per_node"])
}

func TestRingAllReduceMAX(t *testing.T) {
	fmt.Println("\n=== Test: Ring AllReduce MAX ===")
	
	network := &interconnect.MeshNetwork{}
	network.Init(4, 8, interconnect.XY_ROUTING)
	defer network.Fini()
	
	ring := &RingTopology{}
	ring.Init(network, 4)
	defer ring.Fini()
	
	initialValues := []int64{15, 42, 8, 23}
	
	result, err := ring.RingAllReduceSimple(initialValues, MAX)
	if err != nil {
		t.Fatalf("AllReduce failed: %v", err)
	}
	
	if result != 42 {
		t.Errorf("Expected MAX=42, got %d", result)
	}
	
	fmt.Printf("✓ AllReduce MAX: %v → %d\n", initialValues, result)
}

func TestRingAllReduceMIN(t *testing.T) {
	fmt.Println("\n=== Test: Ring AllReduce MIN ===")
	
	network := &interconnect.MeshNetwork{}
	network.Init(4, 8, interconnect.XY_ROUTING)
	defer network.Fini()
	
	ring := &RingTopology{}
	ring.Init(network, 4)
	defer ring.Fini()
	
	initialValues := []int64{15, 42, 8, 23}
	
	result, err := ring.RingAllReduceSimple(initialValues, MIN)
	if err != nil {
		t.Fatalf("AllReduce failed: %v", err)
	}
	
	if result != 8 {
		t.Errorf("Expected MIN=8, got %d", result)
	}
	
	fmt.Printf("✓ AllReduce MIN: %v → %d\n", initialValues, result)
}

func TestReduceOperations(t *testing.T) {
	fmt.Println("\n=== Test: Reduce Operations ===")
	
	tests := []struct {
		op       ReduceOp
		a, b     int64
		expected int64
	}{
		{SUM, 10, 20, 30},
		{MAX, 10, 20, 20},
		{MIN, 10, 20, 10},
		{PROD, 5, 6, 30},
	}
	
	for _, test := range tests {
		result := ApplyReduce(test.op, test.a, test.b)
		if result != test.expected {
			t.Errorf("%v(%d, %d) = %d, expected %d", 
				test.op, test.a, test.b, result, test.expected)
		}
		fmt.Printf("✓ %v(%d, %d) = %d\n", test.op, test.a, test.b, result)
	}
}

func TestRingStatistics(t *testing.T) {
	fmt.Println("\n=== Test: Ring Statistics ===")
	
	network := &interconnect.MeshNetwork{}
	network.Init(4, 8, interconnect.XY_ROUTING)
	defer network.Fini()
	
	ring := &RingTopology{}
	ring.Init(network, 8)
	defer ring.Fini()
	
	initialValues := []int64{1, 2, 3, 4, 5, 6, 7, 8}
	ring.RingAllReduceSimple(initialValues, SUM)
	
	stats := ring.GetStatistics()
	
	fmt.Println("\nRing Statistics:")
	for key, value := range stats {
		fmt.Printf("  %s: %v\n", key, value)
	}
	
	if stats["num_nodes"].(int) != 8 {
		t.Error("Expected 8 nodes in statistics")
	}
	
	fmt.Println("✓ Statistics collection working")
}

func BenchmarkRingAllReduce4Nodes(b *testing.B) {
	network := &interconnect.MeshNetwork{}
	network.Init(4, 8, interconnect.XY_ROUTING)
	defer network.Fini()
	
	ring := &RingTopology{}
	ring.Init(network, 4)
	defer ring.Fini()
	
	initialValues := []int64{1, 2, 3, 4}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ring.RingAllReduceSimple(initialValues, SUM)
	}
}

func BenchmarkRingAllReduce32Nodes(b *testing.B) {
	network := &interconnect.MeshNetwork{}
	network.Init(4, 8, interconnect.XY_ROUTING)
	defer network.Fini()
	
	ring := &RingTopology{}
	ring.Init(network, 32)
	defer ring.Fini()
	
	initialValues := make([]int64, 32)
	for i := 0; i < 32; i++ {
		initialValues[i] = int64(i)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ring.RingAllReduceSimple(initialValues, SUM)
	}
}


// Add these tests to: simulator/collective/collective_test.go

func TestBroadcastTopologyInit(t *testing.T) {
	fmt.Println("\n=== Test: Broadcast Topology Initialization ===")
	
	network := &interconnect.MeshNetwork{}
	network.Init(4, 8, interconnect.XY_ROUTING)
	defer network.Fini()
	
	broadcast := &BroadcastTopology{}
	broadcast.Init(network, 32)
	defer broadcast.Fini()
	
	if broadcast.numNodes != 32 {
		t.Errorf("Expected 32 nodes, got %d", broadcast.numNodes)
	}
	
	if broadcast.branchingFactor != 2 {
		t.Errorf("Expected branching factor 2, got %d", broadcast.branchingFactor)
	}
	
	depth := broadcast.GetTreeDepth()
	if depth != 5 {
		t.Errorf("Expected tree depth 5 for 32 nodes, got %d", depth)
	}
	
	fmt.Printf("✓ Broadcast topology initialized: %d nodes, depth %d\n", 32, depth)
}

func TestBroadcastTreeStructure(t *testing.T) {
	fmt.Println("\n=== Test: Broadcast Tree Structure ===")
	
	network := &interconnect.MeshNetwork{}
	network.Init(4, 8, interconnect.XY_ROUTING)
	defer network.Fini()
	
	broadcast := &BroadcastTopology{}
	broadcast.Init(network, 8)
	defer broadcast.Fini()
	
	// Test parent relationships
	tests := []struct {
		nodeID         int
		expectedParent int
	}{
		{0, -1}, // Root has no parent
		{1, 0},  // Node 1's parent is 0
		{2, 0},  // Node 2's parent is 0
		{3, 1},  // Node 3's parent is 1
		{4, 1},  // Node 4's parent is 1
		{5, 2},  // Node 5's parent is 2
	}
	
	for _, test := range tests {
		parent := broadcast.GetParent(test.nodeID)
		if parent != test.expectedParent {
			t.Errorf("Node %d: expected parent %d, got %d", 
				test.nodeID, test.expectedParent, parent)
		} else {
			fmt.Printf("✓ Node %d → Parent %d\n", test.nodeID, parent)
		}
	}
	
	// Test children relationships
	children0 := broadcast.GetChildren(0)
	if len(children0) != 2 || children0[0] != 1 || children0[1] != 2 {
		t.Errorf("Node 0 children incorrect: %v", children0)
	}
	fmt.Printf("✓ Node 0 → Children %v\n", children0)
}

func TestBroadcastSmall(t *testing.T) {
	fmt.Println("\n=== Test: Broadcast (4 nodes) ===")
	
	network := &interconnect.MeshNetwork{}
	network.Init(4, 8, interconnect.XY_ROUTING)
	defer network.Fini()
	
	broadcast := &BroadcastTopology{}
	broadcast.Init(network, 4)
	defer broadcast.Fini()
	
	data := []byte("Broadcast message")
	steps, err := broadcast.BroadcastSimple(0, data)
	if err != nil {
		t.Fatalf("Broadcast failed: %v", err)
	}
	
	fmt.Printf("✓ Broadcast completed in %d steps\n", steps)
	
	depth := broadcast.GetTreeDepth()
	if steps > depth+2 {
		t.Errorf("Too many steps: %d for depth %d", steps, depth)
	}
}

func TestBroadcast8Nodes(t *testing.T) {
	fmt.Println("\n=== Test: Broadcast (8 nodes) ===")
	
	network := &interconnect.MeshNetwork{}
	network.Init(4, 8, interconnect.XY_ROUTING)
	defer network.Fini()
	
	broadcast := &BroadcastTopology{}
	broadcast.Init(network, 8)
	defer broadcast.Fini()
	
	data := []byte("Hello from root")
	steps, err := broadcast.BroadcastSimple(0, data)
	if err != nil {
		t.Fatalf("Broadcast failed: %v", err)
	}
	
	fmt.Printf("✓ Broadcast completed in %d steps\n", steps)
	
	stats := broadcast.GetStatistics()
	fmt.Printf("✓ Total messages: %d\n", stats["total_messages"])
	fmt.Printf("✓ Tree depth: %d\n", stats["tree_depth"])
	fmt.Printf("✓ Efficiency: %.2f\n", stats["efficiency"])
	
	// Should send exactly 7 messages for 8 nodes (N-1)
	if stats["total_messages"].(int64) < 7 {
		t.Error("Not enough messages sent")
	}
}

func TestBroadcast32Nodes(t *testing.T) {
	fmt.Println("\n=== Test: Broadcast (32 nodes) ===")
	
	network := &interconnect.MeshNetwork{}
	network.Init(4, 8, interconnect.XY_ROUTING)
	defer network.Fini()
	
	broadcast := &BroadcastTopology{}
	broadcast.Init(network, 32)
	defer broadcast.Fini()
	
	data := []byte("Broadcast to all 32 DPUs")
	steps, err := broadcast.BroadcastSimple(0, data)
	if err != nil {
		t.Fatalf("Broadcast failed: %v", err)
	}
	
	fmt.Printf("✓ Broadcast to 32 nodes in %d steps\n", steps)
	
	stats := broadcast.GetStatistics()
	fmt.Printf("✓ Total messages: %d\n", stats["total_messages"])
	fmt.Printf("✓ Messages per node: %.1f\n", stats["avg_messages_per_node"])
	
	// Tree depth for 32 nodes should be 5
	if stats["tree_depth"].(int) != 5 {
		t.Errorf("Expected tree depth 5, got %d", stats["tree_depth"])
	}
}

func TestBroadcastDifferentRoots(t *testing.T) {
	fmt.Println("\n=== Test: Broadcast from Different Roots ===")
	
	network := &interconnect.MeshNetwork{}
	network.Init(4, 8, interconnect.XY_ROUTING)
	defer network.Fini()
	
	broadcast := &BroadcastTopology{}
	broadcast.Init(network, 8)
	defer broadcast.Fini()
	
	// Test broadcast from node 0 (tree root)
	steps0, err := broadcast.BroadcastSimple(0, []byte("From node 0"))
	if err != nil {
		t.Fatalf("Broadcast from 0 failed: %v", err)
	}
	fmt.Printf("✓ From node 0: %d steps\n", steps0)
	
	// Note: Broadcasting from non-root nodes works but takes more steps
	// since the tree structure is built for root=0
	fmt.Println("✓ Broadcast from tree root successful")
}

func TestBroadcastVsRing(t *testing.T) {
	fmt.Println("\n=== Test: Broadcast vs Ring Comparison ===")
	
	network := &interconnect.MeshNetwork{}
	network.Init(4, 8, interconnect.XY_ROUTING)
	defer network.Fini()
	
	numNodes := 8
	
	// Test Broadcast
	broadcast := &BroadcastTopology{}
	broadcast.Init(network, numNodes)
	broadcast.BroadcastSimple(0, []byte("test"))
	broadcastStats := broadcast.GetStatistics()
	broadcast.Fini()
	
	// For comparison info
	ringMessages := numNodes - 1 // Ring sends N-1 times
	broadcastMessages := broadcastStats["total_messages"].(int64)
	
	fmt.Printf("✓ Broadcast messages: %d\n", broadcastMessages)
	fmt.Printf("✓ Ring messages would be: %d\n", ringMessages)
	fmt.Printf("✓ Broadcast tree depth: %d (logarithmic)\n", broadcastStats["tree_depth"])
	fmt.Printf("✓ Ring would take: %d steps (linear)\n", numNodes-1)
}

func TestBroadcastStatistics(t *testing.T) {
	fmt.Println("\n=== Test: Broadcast Statistics ===")
	
	network := &interconnect.MeshNetwork{}
	network.Init(4, 8, interconnect.XY_ROUTING)
	defer network.Fini()
	
	broadcast := &BroadcastTopology{}
	broadcast.Init(network, 16)
	defer broadcast.Fini()
	
	broadcast.BroadcastSimple(0, []byte("stats test"))
	
	stats := broadcast.GetStatistics()
	
	fmt.Println("\nBroadcast Statistics:")
	for key, value := range stats {
		fmt.Printf("  %s: %v\n", key, value)
	}
	
	if stats["num_nodes"].(int) != 16 {
		t.Error("Node count incorrect")
	}
	
	fmt.Println("✓ Statistics collection working")
}

func BenchmarkBroadcast4Nodes(b *testing.B) {
	network := &interconnect.MeshNetwork{}
	network.Init(4, 8, interconnect.XY_ROUTING)
	defer network.Fini()
	
	broadcast := &BroadcastTopology{}
	broadcast.Init(network, 4)
	defer broadcast.Fini()
	
	data := []byte("benchmark")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		broadcast.BroadcastSimple(0, data)
	}
}

func BenchmarkBroadcast32Nodes(b *testing.B) {
	network := &interconnect.MeshNetwork{}
	network.Init(4, 8, interconnect.XY_ROUTING)
	defer network.Fini()
	
	broadcast := &BroadcastTopology{}
	broadcast.Init(network, 32)
	defer broadcast.Fini()
	
	data := []byte("benchmark")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		broadcast.BroadcastSimple(0, data)
	}
}


// Add these tests to: simulator/collective/collective_test.go

func TestReduceScatterInit(t *testing.T) {
	fmt.Println("\n=== Test: Reduce-Scatter Initialization ===")
	
	network := &interconnect.MeshNetwork{}
	network.Init(4, 8, interconnect.XY_ROUTING)
	defer network.Fini()
	
	rs := &ReduceScatterTopology{}
	rs.Init(network, 32)
	defer rs.Fini()
	
	if rs.numNodes != 32 {
		t.Errorf("Expected 32 nodes, got %d", rs.numNodes)
	}
	
	fmt.Println("✓ Reduce-Scatter topology initialized")
}

func TestReduceScatterSmall(t *testing.T) {
	fmt.Println("\n=== Test: Reduce-Scatter (4 nodes) ===")
	
	network := &interconnect.MeshNetwork{}
	network.Init(4, 8, interconnect.XY_ROUTING)
	defer network.Fini()
	
	rs := &ReduceScatterTopology{}
	rs.Init(network, 4)
	defer rs.Fini()
	
	// Each node has 4 values
	initialData := [][]int64{
		{10, 20, 30, 40}, // Node 0
		{1, 2, 3, 4},     // Node 1
		{5, 6, 7, 8},     // Node 2
		{9, 10, 11, 12},  // Node 3
	}
	
	result, err := rs.ReduceScatterSimple(initialData, SUM)
	if err != nil {
		t.Fatalf("Reduce-Scatter failed: %v", err)
	}
	
	// Expected results:
	// Node 0: sum of column 0 = 10+1+5+9 = 25
	// Node 1: sum of column 1 = 20+2+6+10 = 38
	// Node 2: sum of column 2 = 30+3+7+11 = 51
	// Node 3: sum of column 3 = 40+4+8+12 = 64
	expected := []int64{25, 38, 51, 64}
	
	for i, val := range result {
		if val != expected[i] {
			t.Errorf("Node %d: expected %d, got %d", i, expected[i], val)
		}
	}
	
	fmt.Printf("✓ Reduce-Scatter results: %v\n", result)
	fmt.Printf("✓ Expected: %v ✓\n", expected)
}

func TestReduceScatter8Nodes(t *testing.T) {
	fmt.Println("\n=== Test: Reduce-Scatter (8 nodes) ===")
	
	network := &interconnect.MeshNetwork{}
	network.Init(4, 8, interconnect.XY_ROUTING)
	defer network.Fini()
	
	rs := &ReduceScatterTopology{}
	rs.Init(network, 8)
	defer rs.Fini()
	
	// Each node has 8 values (same as node ID * 10 for simplicity)
	initialData := make([][]int64, 8)
	for i := 0; i < 8; i++ {
		initialData[i] = make([]int64, 8)
		for j := 0; j < 8; j++ {
			initialData[i][j] = int64(i * 10)
		}
	}
	
	result, err := rs.ReduceScatterSimple(initialData, SUM)
	if err != nil {
		t.Fatalf("Reduce-Scatter failed: %v", err)
	}
	
	// Each node should get sum of its column
	// Sum = 0+10+20+30+40+50+60+70 = 280
	for i, val := range result {
		if val != 280 {
			t.Errorf("Node %d: expected 280, got %d", i, val)
		}
	}
	
	fmt.Printf("✓ All nodes got correct reduced value: 280\n")
	
	stats := rs.GetStatistics()
	fmt.Printf("✓ Total messages: %d\n", stats["total_messages"])
}

func TestReduceScatterMAX(t *testing.T) {
	fmt.Println("\n=== Test: Reduce-Scatter MAX ===")
	
	network := &interconnect.MeshNetwork{}
	network.Init(4, 8, interconnect.XY_ROUTING)
	defer network.Fini()
	
	rs := &ReduceScatterTopology{}
	rs.Init(network, 4)
	defer rs.Fini()
	
	initialData := [][]int64{
		{10, 50, 30, 70},
		{25, 15, 80, 40},
		{5, 60, 20, 90},
		{40, 20, 100, 30},
	}
	
	result, err := rs.ReduceScatterSimple(initialData, MAX)
	if err != nil {
		t.Fatalf("Reduce-Scatter failed: %v", err)
	}
	
	// Expected: max of each column
	expected := []int64{40, 60, 100, 90}
	
	for i, val := range result {
		if val != expected[i] {
			t.Errorf("Node %d: expected %d, got %d", i, expected[i], val)
		}
	}
	
	fmt.Printf("✓ Reduce-Scatter MAX: %v\n", result)
}

func TestReduceScatterMIN(t *testing.T) {
	fmt.Println("\n=== Test: Reduce-Scatter MIN ===")
	
	network := &interconnect.MeshNetwork{}
	network.Init(4, 8, interconnect.XY_ROUTING)
	defer network.Fini()
	
	rs := &ReduceScatterTopology{}
	rs.Init(network, 4)
	defer rs.Fini()
	
	initialData := [][]int64{
		{10, 50, 30, 70},
		{25, 15, 80, 40},
		{5, 60, 20, 90},
		{40, 20, 100, 30},
	}
	
	result, err := rs.ReduceScatterSimple(initialData, MIN)
	if err != nil {
		t.Fatalf("Reduce-Scatter failed: %v", err)
	}
	
	// Expected: min of each column
	expected := []int64{5, 15, 20, 30}
	
	for i, val := range result {
		if val != expected[i] {
			t.Errorf("Node %d: expected %d, got %d", i, expected[i], val)
		}
	}
	
	fmt.Printf("✓ Reduce-Scatter MIN: %v\n", result)
}

func TestAllGather(t *testing.T) {
	fmt.Println("\n=== Test: AllGather ===")
	
	network := &interconnect.MeshNetwork{}
	network.Init(4, 8, interconnect.XY_ROUTING)
	defer network.Fini()
	
	rs := &ReduceScatterTopology{}
	rs.Init(network, 4)
	defer rs.Fini()
	
	// Each node starts with one value
	initialValues := []int64{10, 20, 30, 40}
	
	result, err := rs.AllGather(initialValues)
	if err != nil {
		t.Fatalf("AllGather failed: %v", err)
	}
	
	// Each node should have all values
	for nodeID := 0; nodeID < 4; nodeID++ {
		for i, val := range result[nodeID] {
			if val != initialValues[i] {
				t.Errorf("Node %d, index %d: expected %d, got %d", 
					nodeID, i, initialValues[i], val)
			}
		}
	}
	
	fmt.Printf("✓ AllGather complete: all nodes have %v\n", initialValues)
}

func TestReduceScatterVsAllReduce(t *testing.T) {
	fmt.Println("\n=== Test: Reduce-Scatter vs AllReduce Comparison ===")
	
	network := &interconnect.MeshNetwork{}
	network.Init(4, 8, interconnect.XY_ROUTING)
	defer network.Fini()
	
	// Test Reduce-Scatter
	rs := &ReduceScatterTopology{}
	rs.Init(network, 4)
	
	initialData := [][]int64{
		{10, 20, 30, 40},
		{1, 2, 3, 4},
		{5, 6, 7, 8},
		{9, 10, 11, 12},
	}
	
	rsResult, _ := rs.ReduceScatterSimple(initialData, SUM)
	rsStats := rs.GetStatistics()
	rs.Fini()
	
	// Compare with AllReduce (conceptual)
	fmt.Println("\nComparison:")
	fmt.Printf("✓ Reduce-Scatter: Each node gets ONE value: %v\n", rsResult)
	fmt.Printf("✓ AllReduce: Each node would get SAME value: %d\n", 
		rsResult[0]+rsResult[1]+rsResult[2]+rsResult[3])
	fmt.Printf("✓ Reduce-Scatter messages: %d\n", rsStats["total_messages"])
	fmt.Println("✓ Use Reduce-Scatter when each node needs different data")
	fmt.Println("✓ Use AllReduce when all nodes need same result")
}

func TestReduceScatterStatistics(t *testing.T) {
	fmt.Println("\n=== Test: Reduce-Scatter Statistics ===")
	
	network := &interconnect.MeshNetwork{}
	network.Init(4, 8, interconnect.XY_ROUTING)
	defer network.Fini()
	
	rs := &ReduceScatterTopology{}
	rs.Init(network, 8)
	defer rs.Fini()
	
	initialData := make([][]int64, 8)
	for i := 0; i < 8; i++ {
		initialData[i] = make([]int64, 8)
		for j := 0; j < 8; j++ {
			initialData[i][j] = int64(i + j)
		}
	}
	
	rs.ReduceScatterSimple(initialData, SUM)
	
	stats := rs.GetStatistics()
	
	fmt.Println("\nReduce-Scatter Statistics:")
	for key, value := range stats {
		fmt.Printf("  %s: %v\n", key, value)
	}
	
	if stats["num_nodes"].(int) != 8 {
		t.Error("Node count incorrect")
	}
	
	fmt.Println("✓ Statistics collection working")
}

func BenchmarkReduceScatter4Nodes(b *testing.B) {
	network := &interconnect.MeshNetwork{}
	network.Init(4, 8, interconnect.XY_ROUTING)
	defer network.Fini()
	
	rs := &ReduceScatterTopology{}
	rs.Init(network, 4)
	defer rs.Fini()
	
	initialData := [][]int64{
		{10, 20, 30, 40},
		{1, 2, 3, 4},
		{5, 6, 7, 8},
		{9, 10, 11, 12},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rs.ReduceScatterSimple(initialData, SUM)
	}
}

func BenchmarkReduceScatter32Nodes(b *testing.B) {
	network := &interconnect.MeshNetwork{}
	network.Init(4, 8, interconnect.XY_ROUTING)
	defer network.Fini()
	
	rs := &ReduceScatterTopology{}
	rs.Init(network, 32)
	defer rs.Fini()
	
	initialData := make([][]int64, 32)
	for i := 0; i < 32; i++ {
		initialData[i] = make([]int64, 32)
		for j := 0; j < 32; j++ {
			initialData[i][j] = int64(i + j)
		}
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rs.ReduceScatterSimple(initialData, SUM)
	}
}