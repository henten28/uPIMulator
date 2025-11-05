package interconnect

import (
	"errors"
	"fmt"
	"sync"
)

// TransferRequest represents a data transfer request between DPUs
type TransferRequest struct {
	SrcChannelID int
	SrcRankID    int
	SrcDpuID     int
	DstChannelID int
	DstRankID    int
	DstDpuID     int
	Data         []byte
	Timestamp    int64
}

// Interconnect manages communication between DPUs
type Interconnect struct {
	mu sync.RWMutex

	// Shared memory buffer for inter-DPU communication
	sharedBuffer map[string][]byte

	// Transfer queues for different channels
	transferQueues map[int][]*TransferRequest

	// Statistics
	totalTransfers        int64
	totalBytesTransferred int64
	cycles                int64

	// Configuration
	numChannels int
	numRanks    int
	numDPUs     int
	bandwidth   int64 // bytes per cycle
}

// Init initializes the interconnect
func (ic *Interconnect) Init(numChannels, numRanks, numDPUs int, bandwidth int64) {
	if numChannels <= 0 || numRanks <= 0 || numDPUs <= 0 {
		panic(errors.New("invalid interconnect dimensions"))
	}

	ic.sharedBuffer = make(map[string][]byte)
	ic.transferQueues = make(map[int][]*TransferRequest)

	ic.numChannels = numChannels
	ic.numRanks = numRanks
	ic.numDPUs = numDPUs
	ic.bandwidth = bandwidth

	ic.totalTransfers = 0
	ic.totalBytesTransferred = 0
	ic.cycles = 0

	// Initialize transfer queues for each channel
	for i := 0; i < numChannels; i++ {
		ic.transferQueues[i] = make([]*TransferRequest, 0)
	}
}

// Write data from a DPU to shared buffer
func (ic *Interconnect) Write(channelID, rankID, dpuID int, data []byte) error {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	if err := ic.validateDPUCoordinates(channelID, rankID, dpuID); err != nil {
		return err
	}

	key := ic.makeKey(channelID, rankID, dpuID)
	ic.sharedBuffer[key] = make([]byte, len(data))
	copy(ic.sharedBuffer[key], data)

	ic.totalTransfers++
	ic.totalBytesTransferred += int64(len(data))

	return nil
}

// Read data from shared buffer
func (ic *Interconnect) Read(srcChannelID, srcRankID, srcDpuID int) ([]byte, error) {
	ic.mu.RLock()
	defer ic.mu.RUnlock()

	if err := ic.validateDPUCoordinates(srcChannelID, srcRankID, srcDpuID); err != nil {
		return nil, err
	}

	key := ic.makeKey(srcChannelID, srcRankID, srcDpuID)
	data, exists := ic.sharedBuffer[key]
	if !exists {
		return nil, fmt.Errorf("no data from DPU[%d][%d][%d]",
			srcChannelID, srcRankID, srcDpuID)
	}

	result := make([]byte, len(data))
	copy(result, data)

	return result, nil
}

// Transfer initiates a transfer request between DPUs
func (ic *Interconnect) Transfer(req *TransferRequest) error {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	if err := ic.validateDPUCoordinates(req.SrcChannelID, req.SrcRankID, req.SrcDpuID); err != nil {
		return fmt.Errorf("invalid source: %w", err)
	}

	if err := ic.validateDPUCoordinates(req.DstChannelID, req.DstRankID, req.DstDpuID); err != nil {
		return fmt.Errorf("invalid destination: %w", err)
	}

	// Add to appropriate channel queue
	channelID := req.SrcChannelID
	ic.transferQueues[channelID] = append(ic.transferQueues[channelID], req)

	return nil
}

// Cycle processes transfers for one cycle
func (ic *Interconnect) Cycle() {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	// Process transfers for each channel based on bandwidth
	for channelID := range ic.transferQueues {
		if len(ic.transferQueues[channelID]) > 0 {
			// Process one transfer per cycle (can be extended based on bandwidth)
			req := ic.transferQueues[channelID][0]
			ic.transferQueues[channelID] = ic.transferQueues[channelID][1:]

			// Complete the transfer
			dstKey := ic.makeKey(req.DstChannelID, req.DstRankID, req.DstDpuID)
			ic.sharedBuffer[dstKey] = req.Data

			ic.totalTransfers++
			ic.totalBytesTransferred += int64(len(req.Data))
		}
	}

	ic.cycles++
}

// GetStatistics returns interconnect statistics
func (ic *Interconnect) GetStatistics() map[string]interface{} {
	ic.mu.RLock()
	defer ic.mu.RUnlock()

	stats := make(map[string]interface{})
	stats["total_transfers"] = ic.totalTransfers
	stats["total_bytes_transferred"] = ic.totalBytesTransferred
	stats["cycles"] = ic.cycles

	if ic.totalTransfers > 0 {
		stats["avg_bytes_per_transfer"] = float64(ic.totalBytesTransferred) / float64(ic.totalTransfers)
	}

	if ic.cycles > 0 {
		stats["bandwidth_utilization"] = float64(ic.totalBytesTransferred) / (float64(ic.cycles) * float64(ic.bandwidth))
	}

	return stats
}

// IsEmpty checks if all transfer queues are empty
func (ic *Interconnect) IsEmpty() bool {
	ic.mu.RLock()
	defer ic.mu.RUnlock()

	for _, queue := range ic.transferQueues {
		if len(queue) > 0 {
			return false
		}
	}
	return true
}

// Clear clears data for a specific DPU
func (ic *Interconnect) Clear(channelID, rankID, dpuID int) {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	key := ic.makeKey(channelID, rankID, dpuID)
	delete(ic.sharedBuffer, key)
}

// Helper functions
func (ic *Interconnect) makeKey(channelID, rankID, dpuID int) string {
	return fmt.Sprintf("%d-%d-%d", channelID, rankID, dpuID)
}

func (ic *Interconnect) validateDPUCoordinates(channelID, rankID, dpuID int) error {
	if channelID < 0 || channelID >= ic.numChannels {
		return fmt.Errorf("invalid channel ID: %d", channelID)
	}
	if rankID < 0 || rankID >= ic.numRanks {
		return fmt.Errorf("invalid rank ID: %d", rankID)
	}
	if dpuID < 0 || dpuID >= ic.numDPUs {
		return fmt.Errorf("invalid DPU ID: %d", dpuID)
	}
	return nil
}

func (ic *Interconnect) Fini() {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	ic.sharedBuffer = nil
	ic.transferQueues = nil
}
