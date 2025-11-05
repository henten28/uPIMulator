// File: simulator/interconnect/router_test.go
package interconnect

import (
	"fmt"
	"testing"
)

func TestRouterInit(t *testing.T) {
	fmt.Println("\n=== Test: Router Initialization ===")
	
	router := &Router{}
	router.Init(0, 0, XY_ROUTING)
	
	if router.PositionX != 0 || router.PositionY != 0 {
		t.Error("Router position not set correctly")
	}
	
	// Check all ports exist
	directions := []Direction{NORTH, SOUTH, EAST, WEST, LOCAL}
	for _, dir := range directions {
		if router.InputPorts[dir] == nil {
			t.Errorf("Input port %s not initialized", dir)
		}
		if router.OutputPorts[dir] == nil {
			t.Errorf("Output port %s not initialized", dir)
		}
	}
	
	fmt.Println("✓ Router initialized with all ports")
}

func TestXYRouting(t *testing.T) {
	fmt.Println("\n=== Test: XY Routing Algorithm ===")
	
	// Router at position (2, 2)
	router := &Router{}
	router.Init(2, 2, XY_ROUTING)
	
	testCases := []struct {
		dstX     int
		dstY     int
		expected Direction
		name     string
	}{
		{3, 2, EAST, "Move East"},
		{1, 2, WEST, "Move West"},
		{2, 3, NORTH, "Move North"},
		{2, 1, SOUTH, "Move South"},
		{2, 2, LOCAL, "At Destination"},
		{3, 3, EAST, "Move East first (XY routing)"},
	}
	
	for _, tc := range testCases {
		packet := NewPacket(0, 0, 0, tc.dstX, 0, tc.dstY, []byte("test"))
		direction := router.ComputeNextHop(packet)
		
		if direction != tc.expected {
			t.Errorf("%s: expected %s, got %s", tc.name, tc.expected, direction)
		} else {
			fmt.Printf("✓ %s: %s\n", tc.name, direction)
		}
	}
}

func TestBufferlessBlocking(t *testing.T) {
	fmt.Println("\n=== Test: Bufferless Blocking ===")
	
	router := &Router{}
	router.Init(0, 0, XY_ROUTING)
	
	// Inject first packet to EAST port
	packet1 := NewPacket(0, 0, 0, 1, 0, 0, []byte("packet1"))
	success := router.InjectPacket(packet1)
	if !success {
		t.Fatal("First packet should inject successfully")
	}
	fmt.Println("✓ First packet injected to EAST port")
	
	// Try to inject second packet to same port - should BLOCK
	packet2 := NewPacket(0, 0, 0, 2, 0, 0, []byte("packet2"))
	success = router.InjectPacket(packet2)
	if success {
		t.Error("Second packet should be blocked (bufferless)")
	}
	fmt.Println("✓ Second packet blocked (bufferless design working)")
	
	// After cycle, port should be free
	router.Cycle()
	success = router.InjectPacket(packet2)
	if !success {
		t.Error("After cycle, port should be available")
	}
	fmt.Println("✓ After cycle, port available again")
}

func TestPacketRouting(t *testing.T) {
	fmt.Println("\n=== Test: Packet Routing ===")
	
	router := &Router{}
	router.Init(1, 1, XY_ROUTING)
	
	// Packet going from (0,0) to (2,2) via router at (1,1)
	packet := NewPacket(0, 0, 0, 2, 0, 2, []byte("routing test"))
	
	// Simulate packet arriving from WEST port
	router.ReceivePacket(packet, WEST)
	fmt.Printf("✓ Packet received from WEST\n")
	
	// Cycle - should route packet to EAST (X direction first)
	router.Cycle()
	
	if !router.OutputPorts[EAST].Occupied {
		t.Error("Packet should be routed to EAST port")
	}
	fmt.Printf("✓ Packet routed to EAST port (XY routing)\n")
	
	stats := router.GetStatistics()
	fmt.Printf("✓ Packets routed: %d\n", stats["packets_routed"])
}

func TestMultiHopRouting(t *testing.T) {
	fmt.Println("\n=== Test: Multi-Hop Routing ===")
	
	// Create a chain of 3 routers: (0,0) -> (1,0) -> (2,0)
	router0 := &Router{}
	router1 := &Router{}
	router2 := &Router{}
	
	router0.Init(0, 0, XY_ROUTING)
	router1.Init(1, 0, XY_ROUTING)
	router2.Init(2, 0, XY_ROUTING)
	
	// Send packet from router0 to router2
	packet := NewPacket(0, 0, 0, 2, 0, 0, []byte("multi-hop"))
	
	// Start by receiving packet at router0
	if !router0.ReceivePacket(packet, LOCAL) {
		t.Fatal("Failed to receive packet at router0")
	}
	fmt.Println("✓ Packet received at router (0,0)")
	
	// Hop 1: router0 -> router1
	router0.Cycle()
	if !router0.OutputPorts[EAST].Occupied {
		t.Fatal("Packet should move east from router0")
	}
	
	// Transfer packet
	packet = router0.OutputPorts[EAST].Packet
	if packet == nil {
		t.Fatal("Packet is nil after routing")
	}
	router0.OutputPorts[EAST].Occupied = false // Clear for next cycle
	router0.OutputPorts[EAST].Packet = nil
	
	if !router1.ReceivePacket(packet, WEST) {
		t.Fatal("Failed to receive packet at router1")
	}
	fmt.Printf("✓ Hop 1: Packet at router (1,0), hop count: %d\n", packet.HopCount)
	
	// Hop 2: router1 -> router2
	router1.Cycle()
	if !router1.OutputPorts[EAST].Occupied {
		t.Fatal("Packet should move east from router1")
	}
	
	// Transfer packet
	packet = router1.OutputPorts[EAST].Packet
	if packet == nil {
		t.Fatal("Packet is nil after routing")
	}
	router1.OutputPorts[EAST].Occupied = false
	router1.OutputPorts[EAST].Packet = nil
	
	if !router2.ReceivePacket(packet, WEST) {
		t.Fatal("Failed to receive packet at router2")
	}
	fmt.Printf("✓ Hop 2: Packet at router (2,0), hop count: %d\n", packet.HopCount)
	
	// Hop 3: router2 delivers to local
	router2.Cycle()
	if !router2.OutputPorts[LOCAL].Occupied {
		t.Error("Packet should be delivered to LOCAL")
	}
	fmt.Println("✓ Packet delivered to destination")
	
	if packet.HopCount < 2 {
		t.Errorf("Expected at least 2 hops, got %d", packet.HopCount)
	}
	fmt.Printf("✓ Total hops: %d\n", packet.HopCount)
}

func TestBackpressure(t *testing.T) {
	fmt.Println("\n=== Test: Backpressure Mechanism ===")
	
	router := &Router{}
	router.Init(1, 1, XY_ROUTING)
	
	// Create congestion: two packets wanting same output port
	packet1 := NewPacket(0, 0, 0, 2, 0, 1, []byte("p1"))
	packet2 := NewPacket(0, 0, 1, 2, 0, 1, []byte("p2"))
	
	// Both need to go EAST
	router.ReceivePacket(packet1, WEST)
	router.ReceivePacket(packet2, SOUTH)
	
	fmt.Println("✓ Two packets received, both need EAST port")
	
	// Cycle - only one can proceed
	router.Cycle()
	
	// Check that exactly one packet moved
	movedCount := 0
	if !router.InputPorts[WEST].Occupied {
		movedCount++
	}
	if !router.InputPorts[SOUTH].Occupied {
		movedCount++
	}
	
	if movedCount != 1 {
		t.Errorf("Expected 1 packet to move, got %d", movedCount)
	}
	
	fmt.Printf("✓ Backpressure working: 1 packet moved, 1 blocked\n")
	
	stats := router.GetStatistics()
	if stats["packets_blocked"].(int64) < 1 {
		t.Error("Should have at least 1 blocked packet")
	}
	fmt.Printf("✓ Blocked packets: %d\n", stats["packets_blocked"])
}

func TestRoutingAlgorithms(t *testing.T) {
	fmt.Println("\n=== Test: Different Routing Algorithms ===")
	
	// Test packet from (0,0) to (2,2)
	packet := NewPacket(0, 0, 0, 2, 0, 2, []byte("test"))
	
	// XY Routing: should go EAST first
	routerXY := &Router{}
	routerXY.Init(0, 0, XY_ROUTING)
	dirXY := routerXY.ComputeNextHop(packet)
	if dirXY != EAST {
		t.Errorf("XY routing: expected EAST, got %s", dirXY)
	}
	fmt.Printf("✓ XY Routing: %s (X first)\n", dirXY)
	
	// YX Routing: should go NORTH first
	routerYX := &Router{}
	routerYX.Init(0, 0, YX_ROUTING)
	dirYX := routerYX.ComputeNextHop(packet)
	if dirYX != NORTH {
		t.Errorf("YX routing: expected NORTH, got %s", dirYX)
	}
	fmt.Printf("✓ YX Routing: %s (Y first)\n", dirYX)
	
	// West-first routing
	routerWF := &Router{}
	routerWF.Init(0, 0, WEST_FIRST)
	dirWF := routerWF.ComputeNextHop(packet)
	if dirWF != NORTH {
		t.Errorf("West-first routing: expected NORTH, got %s", dirWF)
	}
	fmt.Printf("✓ West-First Routing: %s\n", dirWF)
}

func TestRouterStatistics(t *testing.T) {
	fmt.Println("\n=== Test: Router Statistics ===")
	
	router := &Router{}
	router.Init(0, 0, XY_ROUTING)
	
	// Route several packets - receive them first, then route
	for i := 0; i < 5; i++ {
		packet := NewPacket(0, 0, 0, 1, 0, 0, []byte(fmt.Sprintf("p%d", i)))
		router.ReceivePacket(packet, LOCAL)
		router.Cycle()
	}
	
	stats := router.GetStatistics()
	
	fmt.Println("\nRouter Statistics:")
	for key, value := range stats {
		fmt.Printf("  %s: %v\n", key, value)
	}
	
	if stats["packets_routed"].(int64) != 5 {
		t.Errorf("Expected 5 packets routed, got %d", stats["packets_routed"])
	}
	
	fmt.Println("✓ Statistics tracking working")
}

func BenchmarkRouterCycle(b *testing.B) {
	router := &Router{}
	router.Init(1, 1, XY_ROUTING)
	
	packet := NewPacket(0, 0, 0, 2, 0, 2, make([]byte, 64))
	router.ReceivePacket(packet, WEST)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		router.Cycle()
		if router.IsIdle() {
			router.ReceivePacket(packet, WEST)
		}
	}
}

func BenchmarkRoutingDecision(b *testing.B) {
	router := &Router{}
	router.Init(1, 1, XY_ROUTING)
	
	packet := NewPacket(0, 0, 0, 3, 0, 3, []byte("benchmark"))
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = router.ComputeNextHop(packet)
	}
}