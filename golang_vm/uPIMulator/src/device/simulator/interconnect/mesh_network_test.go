// File: simulator/interconnect/mesh_network_test.go
package interconnect

import (
	"fmt"
	"testing"
)

func TestMeshNetworkInit(t *testing.T) {
	fmt.Println("\n=== Test: Mesh Network Initialization ===")
	
	// Create 4x8 mesh (matching 32 DPU configuration)
	network := &MeshNetwork{}
	network.Init(4, 8, XY_ROUTING)
	
	if network.width != 4 || network.height != 8 {
		t.Errorf("Network dimensions incorrect: got %dx%d, want 4x8", 
			network.width, network.height)
	}
	
	// Verify all routers exist
	for x := 0; x < 4; x++ {
		for y := 0; y < 8; y++ {
			router := network.GetRouter(x, y)
			if router == nil {
				t.Errorf("Router at (%d,%d) is nil", x, y)
			}
			if router.PositionX != x || router.PositionY != y {
				t.Errorf("Router position mismatch at (%d,%d)", x, y)
			}
		}
	}
	
	fmt.Println("✓ 4x8 mesh with 32 routers initialized")
	network.Fini()
}

func TestSimplePacketDelivery(t *testing.T) {
	fmt.Println("\n=== Test: Simple Packet Delivery ===")
	
	network := &MeshNetwork{}
	network.Init(4, 4, XY_ROUTING)
	defer network.Fini()
	
	// Send packet from (0,0) to (0,0) - same router
	data := []byte("Hello Local")
	_, err := network.InjectPacket(0, 0, 0, 0, data)
	if err != nil {
		t.Fatalf("Failed to inject packet: %v", err)
	}
	fmt.Println("✓ Packet injected at (0,0) to (0,0)")
	
	// Run until delivered
	if !network.RunUntilEmpty(100) {
		t.Fatal("Packet not delivered within 100 cycles")
	}
	
	stats := network.GetStatistics()
	if stats["packets_delivered"].(int64) != 1 {
		t.Errorf("Expected 1 packet delivered, got %d", stats["packets_delivered"])
	}
	
	fmt.Printf("✓ Packet delivered in %d cycles\n", stats["cycles"])
	fmt.Printf("✓ Average latency: %.2f cycles\n", stats["avg_latency"])
}

func TestSingleHopDelivery(t *testing.T) {
	fmt.Println("\n=== Test: Single Hop Delivery ===")
	
	network := &MeshNetwork{}
	network.Init(4, 4, XY_ROUTING)
	defer network.Fini()
	
	// Send from (0,0) to (1,0) - one hop EAST
	data := []byte("One hop")
	_, err := network.InjectPacket(0, 0, 1, 0, data)
	if err != nil {
		t.Fatalf("Failed to inject: %v", err)
	}
	fmt.Println("✓ Packet injected: (0,0) → (1,0)")
	
	if !network.RunUntilEmpty(100) {
		t.Fatal("Delivery timeout")
	}
	
	stats := network.GetStatistics()
	latency := stats["avg_latency"].(float64)
	
	if latency > 5 {
		t.Errorf("Single hop should take <5 cycles, took %.0f", latency)
	}
	
	fmt.Printf("✓ Delivered in %.0f cycles\n", latency)
}

func TestMultiHopDelivery(t *testing.T) {
	fmt.Println("\n=== Test: Multi-Hop Delivery ===")
	
	network := &MeshNetwork{}
	network.Init(4, 4, XY_ROUTING)
	defer network.Fini()
	
	// Send from (0,0) to (3,3) - diagonal across network
	data := []byte("Long distance")
	_, err := network.InjectPacket(0, 0, 3, 3, data)
	if err != nil {
		t.Fatalf("Failed to inject: %v", err)
	}
	fmt.Println("✓ Packet injected: (0,0) → (3,3)")
	
	if !network.RunUntilEmpty(100) {
		t.Fatal("Delivery timeout")
	}
	
	stats := network.GetStatistics()
	latency := stats["avg_latency"].(float64)
	
	// Should take about 6 hops (3 EAST + 3 NORTH)
	if latency < 6 || latency > 20 {
		t.Errorf("Expected ~6-20 cycles for 6 hops, got %.0f", latency)
	}
	
	fmt.Printf("✓ Delivered in %.0f cycles\n", latency)
}

func TestMultiplePackets(t *testing.T) {
	fmt.Println("\n=== Test: Multiple Packets ===")
	
	network := &MeshNetwork{}
	network.Init(4, 4, XY_ROUTING)
	defer network.Fini()
	
	// Inject 5 packets to different destinations
	packets := []struct {
		srcX, srcY, dstX, dstY int
		data                    string
	}{
		{0, 0, 3, 0, "packet1"},
		{0, 1, 3, 1, "packet2"},
		{0, 2, 3, 2, "packet3"},
		{1, 0, 2, 3, "packet4"},
		{2, 0, 1, 3, "packet5"},
	}
	
	for i, p := range packets {
		_, err := network.InjectPacket(p.srcX, p.srcY, p.dstX, p.dstY, []byte(p.data))
		if err != nil {
			t.Errorf("Failed to inject packet %d: %v", i, err)
		}
	}
	fmt.Printf("✓ Injected %d packets\n", len(packets))
	
	if !network.RunUntilEmpty(200) {
		t.Fatal("Not all packets delivered within 200 cycles")
	}
	
	stats := network.GetStatistics()
	if stats["packets_delivered"].(int64) != int64(len(packets)) {
		t.Errorf("Expected %d delivered, got %d", 
			len(packets), stats["packets_delivered"])
	}
	
	fmt.Printf("✓ All %d packets delivered\n", len(packets))
	fmt.Printf("✓ Average latency: %.2f cycles\n", stats["avg_latency"])
	fmt.Printf("✓ Throughput: %.4f packets/cycle\n", stats["throughput"])
}

func TestAllToAll(t *testing.T) {
	fmt.Println("\n=== Test: All-to-All Communication ===")
	
	network := &MeshNetwork{}
	network.Init(4, 4, XY_ROUTING)
	defer network.Fini()
	
	// Every router sends to every other router (simplified)
	// Just test a subset to keep test fast
	injected := 0
	for srcX := 0; srcX < 2; srcX++ {
		for srcY := 0; srcY < 2; srcY++ {
			for dstX := 2; dstX < 4; dstX++ {
				for dstY := 2; dstY < 4; dstY++ {
					data := []byte(fmt.Sprintf("(%d,%d)->(%d,%d)", srcX, srcY, dstX, dstY))
					_, err := network.InjectPacket(srcX, srcY, dstX, dstY, data)
					if err != nil {
						// Router busy, skip
						continue
					}
					injected++
					
					// Run a few cycles to make room
					if injected%4 == 0 {
						for i := 0; i < 5; i++ {
							network.Cycle()
						}
					}
				}
			}
		}
	}
	
	fmt.Printf("✓ Injected %d packets in all-to-all pattern\n", injected)
	
	if !network.RunUntilEmpty(1000) {
		t.Fatal("All-to-all not complete within 1000 cycles")
	}
	
	stats := network.GetStatistics()
	fmt.Printf("✓ All packets delivered\n")
	fmt.Printf("✓ Total cycles: %d\n", stats["cycles"])
	fmt.Printf("✓ Average latency: %.2f cycles\n", stats["avg_latency"])
	fmt.Printf("✓ Block rate: %.4f\n", stats["network_block_rate"])
}


func TestNetworkStatistics(t *testing.T) {
	fmt.Println("\n=== Test: Network Statistics ===")
	
	network := &MeshNetwork{}
	network.Init(4, 8, XY_ROUTING)
	defer network.Fini()
	
	// Send some packets
	for i := 0; i < 10; i++ {
		network.InjectPacket(0, i%8, 3, (i+4)%8, []byte(fmt.Sprintf("pkt%d", i)))
		network.Cycle()
		network.Cycle()
	}
	
	network.RunUntilEmpty(500)
	
	stats := network.GetStatistics()
	
	fmt.Println("\nNetwork Statistics:")
	for key, value := range stats {
		fmt.Printf("  %s: %v\n", key, value)
	}
	
	if stats["total_routers"].(int) != 32 {
		t.Errorf("Expected 32 routers, got %d", stats["total_routers"])
	}
	
	fmt.Println("✓ Statistics collection working")
}

func BenchmarkMeshNetworkSinglePacket(b *testing.B) {
	network := &MeshNetwork{}
	network.Init(4, 4, XY_ROUTING)
	defer network.Fini()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		network.InjectPacket(0, 0, 3, 3, []byte("benchmark"))
		network.RunUntilEmpty(100)
	}
}

func BenchmarkMeshNetworkCycle(b *testing.B) {
	network := &MeshNetwork{}
	network.Init(4, 8, XY_ROUTING)
	defer network.Fini()
	
	// Fill network with some packets
	for i := 0; i < 16; i++ {
		network.InjectPacket(i/8, i%8, (i+2)/8, (i+2)%8, []byte("data"))
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		network.Cycle()
	}
}