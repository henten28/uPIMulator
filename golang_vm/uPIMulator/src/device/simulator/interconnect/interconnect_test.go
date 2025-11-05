package interconnect

import (
	"fmt"
	"sync"
	"testing"
)

func TestInterconnectInit(t *testing.T) {
	ic := &Interconnect{}
	ic.Init(2, 2, 8, 1024) // 2 channels, 2 ranks, 8 DPUs per rank, 1KB/cycle bandwidth

	if ic.numChannels != 2 || ic.numRanks != 2 || ic.numDPUs != 8 {
		t.Error("Interconnect initialization failed")
	}

	ic.Fini()
}

func TestBasicTransfer(t *testing.T) {
	fmt.Println("\n=== Test: Basic Transfer ===")

	ic := &Interconnect{}
	ic.Init(2, 2, 8, 1024)
	defer ic.Fini()

	testData := []byte("Hello from DPU[0][0][0]")

	// Write from DPU[0][0][0]
	err := ic.Write(0, 0, 0, testData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read from DPU[0][0][1]
	data, err := ic.Read(0, 0, 0)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if string(data) != string(testData) {
		t.Errorf("Data mismatch: got %s, want %s", string(data), string(testData))
	}

	fmt.Printf("✓ Successfully transferred: %s\n", string(data))
}

func TestMultiDPUTransfer(t *testing.T) {
	fmt.Println("\n=== Test: 32 DPU Transfer ===")

	ic := &Interconnect{}
	ic.Init(2, 2, 8, 1024)
	defer ic.Fini()

	var wg sync.WaitGroup

	// All 32 DPUs write data
	for ch := 0; ch < 2; ch++ {
		for rank := 0; rank < 2; rank++ {
			for dpu := 0; dpu < 8; dpu++ {
				wg.Add(1)
				go func(c, r, d int) {
					defer wg.Done()
					data := []byte(fmt.Sprintf("DPU[%d][%d][%d]", c, r, d))
					if err := ic.Write(c, r, d, data); err != nil {
						t.Errorf("Write failed: %v", err)
					}
				}(ch, rank, dpu)
			}
		}
	}
	wg.Wait()

	// Verify all writes
	successCount := 0
	for ch := 0; ch < 2; ch++ {
		for rank := 0; rank < 2; rank++ {
			for dpu := 0; dpu < 8; dpu++ {
				data, err := ic.Read(ch, rank, dpu)
				if err == nil {
					expected := fmt.Sprintf("DPU[%d][%d][%d]", ch, rank, dpu)
					if string(data) == expected {
						successCount++
					}
				}
			}
		}
	}

	if successCount != 32 {
		t.Errorf("Expected 32 successful transfers, got %d", successCount)
	}

	fmt.Printf("✓ Successfully transferred data for %d/32 DPUs\n", successCount)
}

func TestTransferRequest(t *testing.T) {
	fmt.Println("\n=== Test: Transfer Request ===")

	ic := &Interconnect{}
	ic.Init(2, 2, 8, 1024)
	defer ic.Fini()

	req := &TransferRequest{
		SrcChannelID: 0,
		SrcRankID:    0,
		SrcDpuID:     0,
		DstChannelID: 0,
		DstRankID:    0,
		DstDpuID:     1,
		Data:         []byte("Transfer via request"),
		Timestamp:    0,
	}

	err := ic.Transfer(req)
	if err != nil {
		t.Fatalf("Transfer request failed: %v", err)
	}

	// Process the transfer
	ic.Cycle()

	// Verify destination received data
	data, err := ic.Read(0, 0, 1)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if string(data) != "Transfer via request" {
		t.Errorf("Data mismatch: got %s", string(data))
	}

	fmt.Printf("✓ Transfer request completed successfully\n")
}

func TestInvalidCoordinates(t *testing.T) {
	fmt.Println("\n=== Test: Invalid Coordinates ===")

	ic := &Interconnect{}
	ic.Init(2, 2, 8, 1024)
	defer ic.Fini()

	// Test invalid channel
	err := ic.Write(5, 0, 0, []byte("test"))
	if err == nil {
		t.Error("Expected error for invalid channel")
	}

	// Test invalid rank
	err = ic.Write(0, 5, 0, []byte("test"))
	if err == nil {
		t.Error("Expected error for invalid rank")
	}

	// Test invalid DPU
	err = ic.Write(0, 0, 15, []byte("test"))
	if err == nil {
		t.Error("Expected error for invalid DPU")
	}

	fmt.Println("✓ Invalid coordinate handling works correctly")
}

func TestStatistics(t *testing.T) {
	fmt.Println("\n=== Test: Statistics ===")

	ic := &Interconnect{}
	ic.Init(2, 2, 8, 1024)
	defer ic.Fini()

	// Perform some transfers
	for i := 0; i < 10; i++ {
		data := []byte(fmt.Sprintf("Transfer %d", i))
		ic.Write(0, 0, i%8, data)
	}

	stats := ic.GetStatistics()

	fmt.Println("\nStatistics:")
	for key, value := range stats {
		fmt.Printf("  %s: %v\n", key, value)
	}

	if stats["total_transfers"].(int64) != 10 {
		t.Errorf("Expected 10 transfers, got %d", stats["total_transfers"])
	}
}

func TestCycleProcessing(t *testing.T) {
	fmt.Println("\n=== Test: Cycle Processing ===")

	ic := &Interconnect{}
	ic.Init(2, 2, 8, 1024)
	defer ic.Fini()

	// Queue multiple transfers
	for i := 0; i < 5; i++ {
		req := &TransferRequest{
			SrcChannelID: 0,
			SrcRankID:    0,
			SrcDpuID:     i,
			DstChannelID: 0,
			DstRankID:    0,
			DstDpuID:     i + 1,
			Data:         []byte(fmt.Sprintf("Transfer %d", i)),
		}
		ic.Transfer(req)
	}

	// Process cycles until empty
	cycles := 0
	for !ic.IsEmpty() {
		ic.Cycle()
		cycles++
	}

	fmt.Printf("✓ Processed 5 transfers in %d cycles\n", cycles)

	if cycles < 5 {
		t.Error("Expected at least 5 cycles for 5 transfers")
	}
}

func BenchmarkInterconnectWrite(b *testing.B) {
	ic := &Interconnect{}
	ic.Init(2, 2, 8, 1024)
	defer ic.Fini()

	testData := make([]byte, 1024)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ic.Write(0, 0, i%8, testData)
	}
}

func BenchmarkInterconnect32DPUsConcurrent(b *testing.B) {
	ic := &Interconnect{}
	ic.Init(2, 2, 8, 1024)
	defer ic.Fini()

	testData := make([]byte, 1024)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		for ch := 0; ch < 2; ch++ {
			for rank := 0; rank < 2; rank++ {
				for dpu := 0; dpu < 8; dpu++ {
					wg.Add(1)
					go func(c, r, d int) {
						defer wg.Done()
						ic.Write(c, r, d, testData)
					}(ch, rank, dpu)
				}
			}
		}
		wg.Wait()
	}
}
