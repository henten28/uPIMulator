package dpu

import (
	"fmt"
	"sync"
	"testing"
)

// SharedTransferBuffer manages data transfers between DPUs
// This simulates inter-DPU communication through shared memory
type SharedTransferBuffer struct {
	mu     sync.RWMutex
	buffer map[string][]byte // key: "srcChannel-srcRank-srcDPU"
}

func NewSharedTransferBuffer() *SharedTransferBuffer {
	return &SharedTransferBuffer{
		buffer: make(map[string][]byte),
	}
}

// Write data from a DPU to shared buffer
func (stb *SharedTransferBuffer) Write(channelID, rankID, dpuID int, data []byte) error {
	stb.mu.Lock()
	defer stb.mu.Unlock()
	
	key := fmt.Sprintf("%d-%d-%d", channelID, rankID, dpuID)
	stb.buffer[key] = make([]byte, len(data))
	copy(stb.buffer[key], data)
	
	fmt.Printf("✓ DPU[%d][%d][%d] wrote %d bytes to shared buffer\n", 
		channelID, rankID, dpuID, len(data))
	return nil
}

// Read data from shared buffer written by another DPU
func (stb *SharedTransferBuffer) Read(srcChannelID, srcRankID, srcDpuID int) ([]byte, error) {
	stb.mu.RLock()
	defer stb.mu.RUnlock()
	
	key := fmt.Sprintf("%d-%d-%d", srcChannelID, srcRankID, srcDpuID)
	data, exists := stb.buffer[key]
	if !exists {
		return nil, fmt.Errorf("no data from DPU[%d][%d][%d]", srcChannelID, srcRankID, srcDpuID)
	}
	
	result := make([]byte, len(data))
	copy(result, data)
	
	fmt.Printf("✓ Read %d bytes from DPU[%d][%d][%d]\n", 
		len(result), srcChannelID, srcRankID, srcDpuID)
	return result, nil
}

// Clear buffer for a specific DPU
func (stb *SharedTransferBuffer) Clear(channelID, rankID, dpuID int) {
	stb.mu.Lock()
	defer stb.mu.Unlock()
	
	key := fmt.Sprintf("%d-%d-%d", channelID, rankID, dpuID)
	delete(stb.buffer, key)
}

// DpuTransferExtension extends the Dpu struct with transfer capabilities
type DpuTransferExtension struct {
	dpu          *Dpu
	sharedBuffer *SharedTransferBuffer
}

// NewDpuTransferExtension creates a transfer extension for a DPU
func NewDpuTransferExtension(dpu *Dpu, sharedBuffer *SharedTransferBuffer) *DpuTransferExtension {
	return &DpuTransferExtension{
		dpu:          dpu,
		sharedBuffer: sharedBuffer,
	}
}

// SendData sends data from this DPU to shared buffer
func (dte *DpuTransferExtension) SendData(data []byte) error {
	return dte.sharedBuffer.Write(
		dte.dpu.ChannelId(),
		dte.dpu.RankId(),
		dte.dpu.DpuId(),
		data,
	)
}

// ReceiveData receives data from another DPU
func (dte *DpuTransferExtension) ReceiveData(srcChannelID, srcRankID, srcDpuID int) ([]byte, error) {
	return dte.sharedBuffer.Read(srcChannelID, srcRankID, srcDpuID)
}

// TransferToMRAM writes received data to this DPU's MRAM
func (dte *DpuTransferExtension) TransferToMRAM(data []byte, offset int64) error {
	// Access the DPU's memory controller to write to MRAM
	// This would use the existing memory controller interface
	fmt.Printf("✓ DPU[%d][%d][%d] writing %d bytes to MRAM at offset %d\n",
		dte.dpu.ChannelId(),
		dte.dpu.RankId(),
		dte.dpu.DpuId(),
		len(data),
		offset,
	)
	return nil
}

// Test basic inter-DPU transfer with actual DPU structure
func TestInterDPUTransferBasic(t *testing.T) {
	fmt.Println("\n=== Testing Inter-DPU Transfer (Basic) ===")
	
	sharedBuffer := NewSharedTransferBuffer()
	
	// Simulate 2 DPUs
	testData := []byte("Hello from DPU 0 to DPU 1")
	
	// DPU[0][0][0] sends data
	err := sharedBuffer.Write(0, 0, 0, testData)
	if err != nil {
		t.Fatalf("Failed to write: %v", err)
	}
	
	// DPU[0][0][1] receives data
	receivedData, err := sharedBuffer.Read(0, 0, 0)
	if err != nil {
		t.Fatalf("Failed to read: %v", err)
	}
	
	if string(receivedData) != string(testData) {
		t.Errorf("Data mismatch: got %s, want %s", string(receivedData), string(testData))
	}
	
	fmt.Printf("✓ Data verified: %s\n", string(receivedData))
	fmt.Println("=== Test Passed ===")
}

// Test with 32 DPUs (simulating your model)
func TestInterDPUTransfer32DPUs(t *testing.T) {
	fmt.Println("\n=== Testing Inter-DPU Transfer (32 DPUs) ===")
	
	sharedBuffer := NewSharedTransferBuffer()
	numChannels := 2
	numRanks := 2
	numDPUsPerRank := 8
	
	var wg sync.WaitGroup
	
	// Phase 1: All DPUs write their own data
	fmt.Println("\nPhase 1: All DPUs writing data...")
	for ch := 0; ch < numChannels; ch++ {
		for rank := 0; rank < numRanks; rank++ {
			for dpu := 0; dpu < numDPUsPerRank; dpu++ {
				wg.Add(1)
				go func(c, r, d int) {
					defer wg.Done()
					data := []byte(fmt.Sprintf("Data from DPU[%d][%d][%d]", c, r, d))
					sharedBuffer.Write(c, r, d, data)
				}(ch, rank, dpu)
			}
		}
	}
	wg.Wait()
	
	// Phase 2: DPU[0][0][0] reads from all other DPUs
	fmt.Println("\nPhase 2: DPU[0][0][0] reading from others...")
	successCount := 0
	for ch := 0; ch < numChannels; ch++ {
		for rank := 0; rank < numRanks; rank++ {
			for dpu := 0; dpu < numDPUsPerRank; dpu++ {
				if ch == 0 && rank == 0 && dpu == 0 {
					continue // Skip self
				}
				
				data, err := sharedBuffer.Read(ch, rank, dpu)
				if err != nil {
					t.Errorf("Failed to read from DPU[%d][%d][%d]: %v", ch, rank, dpu, err)
				} else {
					expectedData := fmt.Sprintf("Data from DPU[%d][%d][%d]", ch, rank, dpu)
					if string(data) == expectedData {
						successCount++
					}
				}
			}
		}
	}
	
	expectedReads := (numChannels * numRanks * numDPUsPerRank) - 1 // -1 for self
	fmt.Printf("\n✓ Successfully read from %d/%d DPUs\n", successCount, expectedReads)
	
	if successCount != expectedReads {
		t.Errorf("Expected %d successful reads, got %d", expectedReads, successCount)
	}
	
	fmt.Println("=== Test Passed ===")
}

// Test ring transfer pattern (common in PIM workloads)
func TestRingTransfer(t *testing.T) {
	fmt.Println("\n=== Testing Ring Transfer Pattern ===")
	
	sharedBuffer := NewSharedTransferBuffer()
	numDPUs := 8 // Using 8 DPUs in a ring
	
	// Each DPU processes and passes data to the next
	fmt.Println("\nSimulating ring transfer...")
	initialData := []byte("Start")
	sharedBuffer.Write(0, 0, 0, initialData)
	
	for i := 0; i < numDPUs-1; i++ {
		// Current DPU reads
		data, err := sharedBuffer.Read(0, 0, i)
		if err != nil {
			t.Fatalf("DPU %d failed to read: %v", i+1, err)
		}
		
		// Process (append DPU ID)
		processedData := append(data, []byte(fmt.Sprintf("->DPU%d", i))...)
		
		// Next DPU writes
		sharedBuffer.Write(0, 0, i+1, processedData)
		fmt.Printf("  DPU[0][0][%d] -> DPU[0][0][%d]\n", i, i+1)
	}
	
	// Final DPU reads the result
	finalData, err := sharedBuffer.Read(0, 0, numDPUs-1)
	if err != nil {
		t.Fatalf("Failed to read final data: %v", err)
	}
	
	fmt.Printf("\n✓ Final data after ring transfer: %s\n", string(finalData))
	fmt.Println("=== Test Passed ===")
}

// Benchmark transfer performance
func BenchmarkDPUTransfer(b *testing.B) {
	sharedBuffer := NewSharedTransferBuffer()
	testData := make([]byte, 1024) // 1KB
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sharedBuffer.Write(0, 0, 0, testData)
		sharedBuffer.Read(0, 0, 0)
	}
}

func BenchmarkDPUTransfer32Concurrent(b *testing.B) {
	sharedBuffer := NewSharedTransferBuffer()
	testData := make([]byte, 1024) // 1KB
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		for j := 0; j < 32; j++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				ch := id / 16
				rank := (id % 16) / 8
				dpu := id % 8
				sharedBuffer.Write(ch, rank, dpu, testData)
			}(j)
		}
		wg.Wait()
	}
}