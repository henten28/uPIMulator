// File: simulator/interconnect/inter_chip_switch_test.go
package interconnect

import (
	"fmt"
	"testing"
)

func TestDQPinPartition(t *testing.T) {
	fmt.Println("\n=== Test: DQ Pin Partitioning ===")
	
	dq := &DQPinPartition{}
	err := dq.Init(64, 8) // 64 pins, 8 channels
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	
	if dq.numChannels != 8 {
		t.Errorf("Expected 8 channels, got %d", dq.numChannels)
	}
	
	if dq.pinsPerChannel != 8 {
		t.Errorf("Expected 8 pins per channel, got %d", dq.pinsPerChannel)
	}
	
	// Check pin assignments
	for ch := 0; ch < 8; ch++ {
		pins := dq.GetChannelPins(ch)
		if len(pins) != 8 {
			t.Errorf("Channel %d has %d pins, expected 8", ch, len(pins))
		}
		fmt.Printf("✓ Channel %d: pins %v\n", ch, pins)
	}
	
	fmt.Println("✓ DQ pin partitioning successful")
}

func TestDQPinPartitionInvalid(t *testing.T) {
	fmt.Println("\n=== Test: DQ Pin Partition Invalid ===")
	
	dq := &DQPinPartition{}
	err := dq.Init(64, 7) // Not evenly divisible
	if err == nil {
		t.Error("Expected error for non-divisible partition")
	}
	
	fmt.Println("✓ Invalid partition rejected correctly")
}

func TestCrossbarInit(t *testing.T) {
	fmt.Println("\n=== Test: Crossbar Initialization ===")
	
	crossbar := &CrossbarSwitch{}
	crossbar.Init(4, 4) // 4×4 switch
	
	if crossbar.numInputs != 4 || crossbar.numOutputs != 4 {
		t.Error("Crossbar dimensions incorrect")
	}
	
	// All should be disconnected initially
	for i := 0; i < 4; i++ {
		if crossbar.IsConnected(i) {
			t.Errorf("Input %d should not be connected initially", i)
		}
	}
	
	fmt.Println("✓ 4×4 crossbar initialized")
}

func TestCrossbarConnect(t *testing.T) {
	fmt.Println("\n=== Test: Crossbar Connection ===")
	
	crossbar := &CrossbarSwitch{}
	crossbar.Init(4, 4)
	
	// Connect input 0 to output 2
	success := crossbar.Connect(0, 2)
	if !success {
		t.Error("First connection should succeed")
	}
	
	if !crossbar.IsConnected(0) {
		t.Error("Input 0 should be connected")
	}
	
	conn := crossbar.GetConnection(0)
	if conn != 2 {
		t.Errorf("Input 0 should connect to output 2, got %d", conn)
	}
	
	fmt.Println("✓ Input 0 → Output 2 connected")
}

func TestCrossbarBlocking(t *testing.T) {
	fmt.Println("\n=== Test: Crossbar Blocking ===")
	
	crossbar := &CrossbarSwitch{}
	crossbar.Init(4, 4)
	
	// Connect input 0 to output 1
	crossbar.Connect(0, 1)
	fmt.Println("✓ Input 0 → Output 1")
	
	// Try to connect input 2 to same output 1 (should fail)
	success := crossbar.Connect(2, 1)
	if success {
		t.Error("Second connection to same output should be blocked")
	}
	
	fmt.Println("✓ Blocking works: Output 1 busy, Input 2 blocked")
	
	stats := crossbar.GetStatistics()
	if stats["blocked_attempts"].(int64) != 1 {
		t.Error("Expected 1 blocked attempt")
	}
}

func TestCrossbarDisconnect(t *testing.T) {
	fmt.Println("\n=== Test: Crossbar Disconnect ===")
	
	crossbar := &CrossbarSwitch{}
	crossbar.Init(4, 4)
	
	// Connect and then disconnect
	crossbar.Connect(0, 1)
	fmt.Println("✓ Connected Input 0 → Output 1")
	
	crossbar.Disconnect(0)
	fmt.Println("✓ Disconnected Input 0")
	
	if crossbar.IsConnected(0) {
		t.Error("Input 0 should be disconnected")
	}
	
	// Now should be able to connect another input
	success := crossbar.Connect(2, 1)
	if !success {
		t.Error("Connection should succeed after disconnect")
	}
	fmt.Println("✓ Input 2 → Output 1 (after disconnect)")
}

func TestInterChipSwitchInit(t *testing.T) {
	fmt.Println("\n=== Test: Inter-Chip Switch Init ===")
	
	ics := &InterChipSwitch{}
	err := ics.Init(4, 64, 8) // 4 chips, 64 DQ pins, 8 channels
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	
	if ics.numChips != 4 {
		t.Errorf("Expected 4 chips, got %d", ics.numChips)
	}
	
	fmt.Println("✓ Inter-chip switch initialized")
}

func TestInterChipTransfer(t *testing.T) {
	fmt.Println("\n=== Test: Inter-Chip Transfer ===")
	
	ics := &InterChipSwitch{}
	ics.Init(4, 64, 8)
	
	// Start transfer from chip 0 to chip 2 on channel 3
	data := []byte("Inter-chip data")
	transferID, err := ics.StartTransfer(0, 2, 3, data)
	if err != nil {
		t.Fatalf("Transfer failed: %v", err)
	}
	
	fmt.Printf("✓ Transfer started: Chip 0 → Chip 2 (ID=%d)\n", transferID)
	
	// Simulate some cycles
	for i := 0; i < 10; i++ {
		ics.Cycle()
	}
	
	// Complete transfer
	err = ics.CompleteTransfer(transferID)
	if err != nil {
		t.Errorf("Complete failed: %v", err)
	}
	
	fmt.Println("✓ Transfer completed after 10 cycles")
}

func TestInterChipMultipleTransfers(t *testing.T) {
	fmt.Println("\n=== Test: Multiple Inter-Chip Transfers ===")
	
	ics := &InterChipSwitch{}
	ics.Init(4, 64, 8)
	
	// Start multiple transfers
	transfers := []struct {
		src, dst, channel int
	}{
		{0, 1, 0},
		{2, 3, 1},
		{1, 2, 2},
	}
	
	transferIDs := make([]int, 0)
	for _, tf := range transfers {
		data := []byte(fmt.Sprintf("Data %d->%d", tf.src, tf.dst))
		id, err := ics.StartTransfer(tf.src, tf.dst, tf.channel, data)
		if err != nil {
			fmt.Printf("Transfer %d→%d blocked (expected)\n", tf.src, tf.dst)
		} else {
			transferIDs = append(transferIDs, id)
			fmt.Printf("✓ Transfer started: Chip %d → Chip %d\n", tf.src, tf.dst)
		}
	}
	
	// Complete transfers
	for _, id := range transferIDs {
		ics.Cycle()
		ics.CompleteTransfer(id)
	}
	
	fmt.Printf("✓ Completed %d transfers\n", len(transferIDs))
}

func TestInterChipBlocking(t *testing.T) {
	fmt.Println("\n=== Test: Inter-Chip Blocking ===")
	
	ics := &InterChipSwitch{}
	ics.Init(4, 64, 8)
	
	// Start transfer from chip 0 to chip 1
	_, err := ics.StartTransfer(0, 1, 0, []byte("first"))
	if err != nil {
		t.Fatalf("First transfer failed: %v", err)
	}
	fmt.Println("✓ Chip 0 → Chip 1")
	
	// Try to start another transfer to same destination (should fail)
	_, err = ics.StartTransfer(2, 1, 1, []byte("second"))
	if err == nil {
		t.Error("Second transfer should be blocked")
	}
	fmt.Println("✓ Chip 2 → Chip 1 blocked (chip 1 busy)")
}

func TestInterChipStatistics(t *testing.T) {
	fmt.Println("\n=== Test: Inter-Chip Statistics ===")
	
	ics := &InterChipSwitch{}
	ics.Init(4, 64, 8)
	
	// Perform some transfers
	for i := 0; i < 5; i++ {
		data := make([]byte, 128)
		id, err := ics.StartTransfer(i%4, (i+1)%4, i%8, data)
		if err == nil {
			ics.Cycle()
			ics.CompleteTransfer(id)
		}
	}
	
	stats := ics.GetStatistics()
	
	fmt.Println("\nInter-Chip Switch Statistics:")
	for key, value := range stats {
		fmt.Printf("  %s: %v\n", key, value)
	}
	
	if stats["num_chips"].(int) != 4 {
		t.Error("Chip count incorrect")
	}
	
	if stats["num_channels"].(int) != 8 {
		t.Error("Channel count incorrect")
	}
	
	fmt.Println("✓ Statistics collection working")
}

func TestDQPartitionBandwidth(t *testing.T) {
	fmt.Println("\n=== Test: DQ Partition Bandwidth ===")
	
	dq := &DQPinPartition{}
	dq.Init(64, 8)
	
	bandwidth := dq.GetChannelBandwidth()
	if bandwidth != 8 {
		t.Errorf("Expected 8 bits per channel, got %d", bandwidth)
	}
	
	fmt.Printf("✓ Bandwidth per channel: %d bits\n", bandwidth)
	fmt.Printf("✓ Total bandwidth: %d bits (%d channels)\n", 
		bandwidth*dq.numChannels, dq.numChannels)
}

func BenchmarkCrossbarConnect(b *testing.B) {
	crossbar := &CrossbarSwitch{}
	crossbar.Init(32, 32)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		inputID := i % 32
		outputID := (i + 1) % 32
		crossbar.Connect(inputID, outputID)
		crossbar.Disconnect(inputID)
	}
}

func BenchmarkInterChipTransfer(b *testing.B) {
	ics := &InterChipSwitch{}
	ics.Init(8, 64, 8)
	
	data := make([]byte, 64)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id, err := ics.StartTransfer(i%8, (i+1)%8, i%8, data)
		if err == nil {
			ics.Cycle()
			ics.CompleteTransfer(id)
		}
	}
}