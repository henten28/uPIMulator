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