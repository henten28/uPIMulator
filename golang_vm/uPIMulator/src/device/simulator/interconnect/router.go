// File: simulator/interconnect/router.go
package interconnect

import (
	"fmt"
)

// Port directions for the router
type Direction int

const (
	NORTH Direction = iota // Up in 2D mesh
	SOUTH                  // Down
	EAST                   // Right
	WEST                   // Left
	LOCAL                  // Connection to local DPU
)

func (d Direction) String() string {
	return [...]string{"NORTH", "SOUTH", "EAST", "WEST", "LOCAL"}[d]
}

// Packet represents a data packet being routed
type Packet struct {
	SrcChannelID int
	SrcRankID    int
	SrcDpuID     int
	DstChannelID int
	DstRankID    int
	DstDpuID     int
	Data         []byte
	HopCount     int    // Number of hops taken
	CurrentX     int    // Current X position in mesh
	CurrentY     int    // Current Y position in mesh
	Timestamp    int64  // When packet was created
}

// RouterPort represents a single input/output port
type RouterPort struct {
	Direction Direction
	Occupied  bool        // Is port currently busy?
	Packet    *Packet     // Current packet (nil if empty)
}

// Router implements a bufferless router for inter-DPU communication
// Key concept: NO BUFFERS - packets must move every cycle or stay at source
type Router struct {
	// Position in the mesh network
	PositionX int // X coordinate (channel, rank)
	PositionY int // Y coordinate (dpu within rank)
	
	// Ports: 4 directions + 1 local
	InputPorts  map[Direction]*RouterPort
	OutputPorts map[Direction]*RouterPort
	
	// Statistics
	packetsRouted    int64
	packetsBlocked   int64  // Packets that couldn't move (backpressure)
	totalHops        int64
	cycles           int64
	
	// Configuration
	routingAlgorithm RoutingAlgorithm
}

// RoutingAlgorithm defines how packets are routed
type RoutingAlgorithm int

const (
	XY_ROUTING RoutingAlgorithm = iota  // X then Y (deterministic)
	YX_ROUTING                           // Y then X (deterministic)
	WEST_FIRST                           // West-first turn model
)

// Init initializes the router at a specific position
func (r *Router) Init(posX, posY int, algorithm RoutingAlgorithm) {
	r.PositionX = posX
	r.PositionY = posY
	r.routingAlgorithm = algorithm
	
	// Initialize ports
	r.InputPorts = make(map[Direction]*RouterPort)
	r.OutputPorts = make(map[Direction]*RouterPort)
	
	directions := []Direction{NORTH, SOUTH, EAST, WEST, LOCAL}
	for _, dir := range directions {
		r.InputPorts[dir] = &RouterPort{Direction: dir, Occupied: false}
		r.OutputPorts[dir] = &RouterPort{Direction: dir, Occupied: false}
	}
	
	r.packetsRouted = 0
	r.packetsBlocked = 0
	r.totalHops = 0
	r.cycles = 0
}

// ComputeNextHop determines which output port to use based on destination
// This is the ROUTING LOGIC - the brain of the router
func (r *Router) ComputeNextHop(packet *Packet) Direction {
	// Calculate relative position to destination
	deltaX := packet.DstChannelID - r.PositionX
	deltaY := packet.DstDpuID - r.PositionY
	
	// If we're at destination, deliver to local DPU
	if deltaX == 0 && deltaY == 0 {
		return LOCAL
	}
	
	// XY Routing: Move in X direction first, then Y
	// This is DETERMINISTIC - same source/dest always takes same path
	switch r.routingAlgorithm {
	case XY_ROUTING:
		if deltaX > 0 {
			return EAST  // Need to go right
		} else if deltaX < 0 {
			return WEST  // Need to go left
		} else if deltaY > 0 {
			return NORTH // X aligned, move up
		} else if deltaY < 0 {
			return SOUTH // X aligned, move down
		}
		
	case YX_ROUTING:
		// Opposite: Y first, then X
		if deltaY > 0 {
			return NORTH
		} else if deltaY < 0 {
			return SOUTH
		} else if deltaX > 0 {
			return EAST
		} else if deltaX < 0 {
			return WEST
		}
		
	case WEST_FIRST:
		// West-first turn model (deadlock-free)
		if deltaX < 0 {
			return WEST  // Always go west first if needed
		} else if deltaY > 0 {
			return NORTH
		} else if deltaY < 0 {
			return SOUTH
		} else if deltaX > 0 {
			return EAST
		}
	}
	
	return LOCAL // Shouldn't reach here
}

// TryRoutePacket attempts to route a packet (bufferless - no retry)
// Returns true if successful, false if blocked
func (r *Router) TryRoutePacket(packet *Packet, fromDir Direction) bool {
	// Determine which output port to use
	nextDir := r.ComputeNextHop(packet)
	
	// Check if output port is available
	if r.OutputPorts[nextDir].Occupied {
		// Port busy - packet is BLOCKED (backpressure)
		r.packetsBlocked++
		return false
	}
	
	// Port available - route the packet!
	r.OutputPorts[nextDir].Packet = packet
	r.OutputPorts[nextDir].Occupied = true
	
	packet.HopCount++
	r.packetsRouted++
	r.totalHops += int64(packet.HopCount)
	
	return true
}

// Cycle performs one routing cycle
// Key: In bufferless routing, packets must move or stay at source
func (r *Router) Cycle() {
	// Phase 1: Clear output ports from previous cycle
	for _, port := range r.OutputPorts {
		port.Occupied = false
		port.Packet = nil
	}
	
	// Phase 2: Try to route packets from input ports
	for dir, inputPort := range r.InputPorts {
		if inputPort.Occupied && inputPort.Packet != nil {
			// Try to route this packet
			success := r.TryRoutePacket(inputPort.Packet, dir)
			
			if success {
				// Packet moved - clear input port
				inputPort.Occupied = false
				inputPort.Packet = nil
			}
			// If failed, packet stays in input port (backpressure)
		}
	}
	
	r.cycles++
}

// InjectPacket injects a new packet from local DPU
func (r *Router) InjectPacket(packet *Packet) bool {
	// Update packet's current position
	packet.CurrentX = r.PositionX
	packet.CurrentY = r.PositionY
	
	// Try to place in appropriate output port
	nextDir := r.ComputeNextHop(packet)
	
	if r.OutputPorts[nextDir].Occupied {
		return false // Can't inject - port busy
	}
	
	r.OutputPorts[nextDir].Packet = packet
	r.OutputPorts[nextDir].Occupied = true
	packet.HopCount = 0
	
	return true
}

// ReceivePacket receives a packet from a neighbor router
func (r *Router) ReceivePacket(packet *Packet, fromDir Direction) bool {
	if r.InputPorts[fromDir].Occupied {
		return false // Port busy - reject packet
	}
	
	r.InputPorts[fromDir].Packet = packet
	r.InputPorts[fromDir].Occupied = true
	packet.CurrentX = r.PositionX
	packet.CurrentY = r.PositionY
	
	return true
}

// GetStatistics returns router performance metrics
func (r *Router) GetStatistics() map[string]interface{} {
	stats := make(map[string]interface{})
	stats["position_x"] = r.PositionX
	stats["position_y"] = r.PositionY
	stats["packets_routed"] = r.packetsRouted
	stats["packets_blocked"] = r.packetsBlocked
	stats["total_hops"] = r.totalHops
	stats["cycles"] = r.cycles
	
	if r.packetsRouted > 0 {
		stats["avg_hops"] = float64(r.totalHops) / float64(r.packetsRouted)
		blockRate := float64(r.packetsBlocked) / float64(r.packetsRouted+r.packetsBlocked)
		stats["block_rate"] = blockRate
	}
	
	return stats
}

// IsIdle checks if router has no activity
func (r *Router) IsIdle() bool {
	for _, port := range r.InputPorts {
		if port.Occupied {
			return false
		}
	}
	for _, port := range r.OutputPorts {
		if port.Occupied {
			return false
		}
	}
	return true
}

func (r *Router) Fini() {
	r.InputPorts = nil
	r.OutputPorts = nil
}

// Helper function to create a packet
func NewPacket(srcCh, srcRank, srcDpu, dstCh, dstRank, dstDpu int, data []byte) *Packet {
	return &Packet{
		SrcChannelID: srcCh,
		SrcRankID:    srcRank,
		SrcDpuID:     srcDpu,
		DstChannelID: dstCh,
		DstRankID:    dstRank,
		DstDpuID:     dstDpu,
		Data:         data,
		HopCount:     0,
		Timestamp:    0,
	}
}

// PrintRouterState for debugging
func (r *Router) PrintRouterState() {
	fmt.Printf("\n=== Router at (%d, %d) ===\n", r.PositionX, r.PositionY)
	fmt.Println("Input Ports:")
	for dir, port := range r.InputPorts {
		status := "Empty"
		if port.Occupied {
			status = fmt.Sprintf("Packet to (%d,%d)", 
				port.Packet.DstChannelID, port.Packet.DstDpuID)
		}
		fmt.Printf("  %s: %s\n", dir, status)
	}
	fmt.Println("Output Ports:")
	for dir, port := range r.OutputPorts {
		status := "Empty"
		if port.Occupied {
			status = fmt.Sprintf("Packet to (%d,%d)", 
				port.Packet.DstChannelID, port.Packet.DstDpuID)
		}
		fmt.Printf("  %s: %s\n", dir, status)
	}
}