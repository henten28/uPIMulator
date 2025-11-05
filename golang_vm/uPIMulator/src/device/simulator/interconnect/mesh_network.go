// File: simulator/interconnect/mesh_network.go
package interconnect

import (
	"fmt"
)

// MeshNetwork represents a 2D mesh topology connecting routers
type MeshNetwork struct {
	// Network dimensions
	width  int // X dimension (channels * ranks)
	height int // Y dimension (DPUs per rank)
	
	// Router grid [x][y]
	routers [][]*Router
	
	// Packets in flight
	activePackets map[int]*Packet // key: packet ID
	nextPacketID  int
	
	// Statistics
	totalPacketsDelivered int64
	totalPacketLatency    int64
	totalPacketsInjected  int64
	cycles                int64
	
	// Configuration
	routingAlgorithm RoutingAlgorithm
}

// Init initializes the mesh network
func (mn *MeshNetwork) Init(width, height int, algorithm RoutingAlgorithm) {
	mn.width = width
	mn.height = height
	mn.routingAlgorithm = algorithm
	mn.activePackets = make(map[int]*Packet)
	mn.nextPacketID = 0
	
	// Create router grid
	mn.routers = make([][]*Router, width)
	for x := 0; x < width; x++ {
		mn.routers[x] = make([]*Router, height)
		for y := 0; y < height; y++ {
			router := &Router{}
			router.Init(x, y, algorithm)
			mn.routers[x][y] = router
		}
	}
	
	fmt.Printf("✓ Mesh network initialized: %dx%d = %d routers\n", 
		width, height, width*height)
}

// InjectPacket injects a packet into the network from a source DPU
func (mn *MeshNetwork) InjectPacket(srcX, srcY, dstX, dstY int, data []byte) (int, error) {
	if !mn.isValidPosition(srcX, srcY) {
		return -1, fmt.Errorf("invalid source position (%d,%d)", srcX, srcY)
	}
	if !mn.isValidPosition(dstX, dstY) {
		return -1, fmt.Errorf("invalid destination position (%d,%d)", dstX, dstY)
	}
	
	packet := NewPacket(srcX, 0, srcY, dstX, 0, dstY, data)
	packet.Timestamp = mn.cycles
	
	router := mn.routers[srcX][srcY]
	// Inject into LOCAL input port (from DPU to router)
	if !router.ReceivePacket(packet, LOCAL) {
		return -1, fmt.Errorf("router at (%d,%d) busy, cannot inject", srcX, srcY)
	}
	
	packetID := mn.nextPacketID
	mn.nextPacketID++
	mn.activePackets[packetID] = packet
	mn.totalPacketsInjected++
	
	return packetID, nil
}

// Cycle performs one network cycle
// This is where the magic happens - all routers operate in parallel
func (mn *MeshNetwork) Cycle() {
	// Phase 1: All routers route packets simultaneously
	for x := 0; x < mn.width; x++ {
		for y := 0; y < mn.height; y++ {
			mn.routers[x][y].Cycle()
		}
	}
	
	// Phase 2: Transfer packets between routers
	// Check each router's output ports and transfer to neighbor's input ports
	for x := 0; x < mn.width; x++ {
		for y := 0; y < mn.height; y++ {
			router := mn.routers[x][y]
			
			// Check NORTH output
			if router.OutputPorts[NORTH].Occupied && y < mn.height-1 {
				packet := router.OutputPorts[NORTH].Packet
				neighborRouter := mn.routers[x][y+1]
				if neighborRouter.ReceivePacket(packet, SOUTH) {
					router.OutputPorts[NORTH].Occupied = false
					router.OutputPorts[NORTH].Packet = nil
				}
			}
			
			// Check SOUTH output
			if router.OutputPorts[SOUTH].Occupied && y > 0 {
				packet := router.OutputPorts[SOUTH].Packet
				neighborRouter := mn.routers[x][y-1]
				if neighborRouter.ReceivePacket(packet, NORTH) {
					router.OutputPorts[SOUTH].Occupied = false
					router.OutputPorts[SOUTH].Packet = nil
				}
			}
			
			// Check EAST output
			if router.OutputPorts[EAST].Occupied && x < mn.width-1 {
				packet := router.OutputPorts[EAST].Packet
				neighborRouter := mn.routers[x+1][y]
				if neighborRouter.ReceivePacket(packet, WEST) {
					router.OutputPorts[EAST].Occupied = false
					router.OutputPorts[EAST].Packet = nil
				}
			}
			
			// Check WEST output
			if router.OutputPorts[WEST].Occupied && x > 0 {
				packet := router.OutputPorts[WEST].Packet
				neighborRouter := mn.routers[x-1][y]
				if neighborRouter.ReceivePacket(packet, EAST) {
					router.OutputPorts[WEST].Occupied = false
					router.OutputPorts[WEST].Packet = nil
				}
			}
			
			// Check LOCAL output (packet delivered to DPU)
			if router.OutputPorts[LOCAL].Occupied {
				packet := router.OutputPorts[LOCAL].Packet
				if packet != nil {
					// Packet delivered!
					latency := mn.cycles - packet.Timestamp
					mn.totalPacketsDelivered++
					mn.totalPacketLatency += latency
					
					// Remove from active packets
					for id, p := range mn.activePackets {
						if p == packet {
							delete(mn.activePackets, id)
							break
						}
					}
					
					router.OutputPorts[LOCAL].Occupied = false
					router.OutputPorts[LOCAL].Packet = nil
				}
			}
		}
	}
	
	mn.cycles++
}

// RunUntilEmpty runs the network until all packets are delivered
func (mn *MeshNetwork) RunUntilEmpty(maxCycles int64) bool {
	startCycle := mn.cycles
	
	for len(mn.activePackets) > 0 {
		if mn.cycles-startCycle >= maxCycles {
			return false // Timeout
		}
		mn.Cycle()
	}
	
	return true
}

// IsEmpty checks if network has no packets in flight
func (mn *MeshNetwork) IsEmpty() bool {
	return len(mn.activePackets) == 0
}

// GetStatistics returns network statistics
func (mn *MeshNetwork) GetStatistics() map[string]interface{} {
	stats := make(map[string]interface{})
	stats["width"] = mn.width
	stats["height"] = mn.height
	stats["total_routers"] = mn.width * mn.height
	stats["packets_injected"] = mn.totalPacketsInjected
	stats["packets_delivered"] = mn.totalPacketsDelivered
	stats["packets_in_flight"] = len(mn.activePackets)
	stats["cycles"] = mn.cycles
	
	if mn.totalPacketsDelivered > 0 {
		avgLatency := float64(mn.totalPacketLatency) / float64(mn.totalPacketsDelivered)
		stats["avg_latency"] = avgLatency
		
		throughput := float64(mn.totalPacketsDelivered) / float64(mn.cycles)
		stats["throughput"] = throughput
	}
	
	// Aggregate router statistics
	totalRouted := int64(0)
	totalBlocked := int64(0)
	for x := 0; x < mn.width; x++ {
		for y := 0; y < mn.height; y++ {
			routerStats := mn.routers[x][y].GetStatistics()
			totalRouted += routerStats["packets_routed"].(int64)
			totalBlocked += routerStats["packets_blocked"].(int64)
		}
	}
	stats["total_packets_routed"] = totalRouted
	stats["total_packets_blocked"] = totalBlocked
	
	if totalRouted > 0 {
		stats["network_block_rate"] = float64(totalBlocked) / float64(totalRouted+totalBlocked)
	}
	
	return stats
}

// GetRouter returns the router at position (x, y)
func (mn *MeshNetwork) GetRouter(x, y int) *Router {
	if !mn.isValidPosition(x, y) {
		return nil
	}
	return mn.routers[x][y]
}

// PrintNetworkState prints the current state of all routers
func (mn *MeshNetwork) PrintNetworkState() {
	fmt.Printf("\n=== Network State (Cycle %d) ===\n", mn.cycles)
	fmt.Printf("Active packets: %d\n", len(mn.activePackets))
	
	for y := mn.height - 1; y >= 0; y-- {
		for x := 0; x < mn.width; x++ {
			router := mn.routers[x][y]
			if router.IsIdle() {
				fmt.Print("[ ]")
			} else {
				fmt.Print("[*]")
			}
		}
		fmt.Println()
	}
	fmt.Println()
}

// Helper functions
func (mn *MeshNetwork) isValidPosition(x, y int) bool {
	return x >= 0 && x < mn.width && y >= 0 && y < mn.height
}

func (mn *MeshNetwork) Fini() {
	for x := 0; x < mn.width; x++ {
		for y := 0; y < mn.height; y++ {
			mn.routers[x][y].Fini()
		}
	}
	mn.routers = nil
	mn.activePackets = nil
}

// SendPacketBlocking is a convenience function that waits for packet delivery
func (mn *MeshNetwork) SendPacketBlocking(srcX, srcY, dstX, dstY int, data []byte, timeout int64) error {
	_, err := mn.InjectPacket(srcX, srcY, dstX, dstY, data)
	if err != nil {
		return err
	}
	
	if !mn.RunUntilEmpty(timeout) {
		return fmt.Errorf("packet delivery timeout after %d cycles", timeout)
	}
	
	return nil
}