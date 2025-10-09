package jbig2

import (
	"testing"
)

func TestBitStreamReadNBits(t *testing.T) {
	data := []byte{0xb1} // 10110001
	stream := NewBitStream(data, 0)

	// Test reading 1 bit: should get 1
	val1, err := stream.ReadNBits(1)
	if err != nil {
		t.Fatalf("ReadNBits(1) failed: %v", err)
	}
	if val1 != 1 {
		t.Errorf("Expected 1, got %d", val1)
	}

	// Test reading 1 bit: should get 0
	val2, err := stream.ReadNBits(1)
	if err != nil {
		t.Fatalf("ReadNBits(1) failed: %v", err)
	}
	if val2 != 0 {
		t.Errorf("Expected 0, got %d", val2)
	}

	// Test reading 2 bits: should get 3 (11)
	val3, err := stream.ReadNBits(2)
	if err != nil {
		t.Fatalf("ReadNBits(2) failed: %v", err)
	}
	if val3 != 3 {
		t.Errorf("Expected 3, got %d", val3)
	}

	// Test reading 4 bits: should get 1 (0001)
	val4, err := stream.ReadNBits(4)
	if err != nil {
		t.Fatalf("ReadNBits(4) failed: %v", err)
	}
	if val4 != 1 {
		t.Errorf("Expected 1, got %d", val4)
	}
}

func TestBitStreamReadNBitsSigned(t *testing.T) {
	data := []byte{0xb1} // 10110001
	stream := NewBitStream(data, 0)

	// Test reading 1 bit signed: should get 1 (not sign extended for ReadNBitsSigned)
	val1, err := stream.ReadNBitsSigned(1)
	if err != nil {
		t.Fatalf("ReadNBitsSigned(1) failed: %v", err)
	}
	if val1 != 1 {
		t.Errorf("Expected 1, got %d", val1)
	}

	// Test reading 2 bits signed: should get 2 (not sign extended 10)
	stream = NewBitStream([]byte{0x80}, 0) // 10000000
	val2, err := stream.ReadNBitsSigned(2)
	if err != nil {
		t.Fatalf("ReadNBitsSigned(2) failed: %v", err)
	}
	if val2 != 2 {
		t.Errorf("Expected 2, got %d", val2)
	}
}

func TestBitStreamReadNBitsLargerThanData(t *testing.T) {
	data := []byte{0xb1} // 10110001
	stream := NewBitStream(data, 42)

	// Should read all available bits (8 bits)
	val, err := stream.ReadNBits(10)
	if err != nil {
		t.Fatalf("ReadNBits(10) failed: %v", err)
	}
	if val != 0xb1 {
		t.Errorf("Expected 0xb1, got 0x%x", val)
	}
}

func TestBitStreamReadNBitsNullStream(t *testing.T) {
	stream := NewBitStream([]byte{}, 0)

	_, err := stream.ReadNBits(1)
	if err == nil {
		t.Error("Expected error for empty stream, got nil")
	}

	_, err = stream.ReadNBits(2)
	if err == nil {
		t.Error("Expected error for empty stream, got nil")
	}
}

func TestBitStreamReadNBitsOutOfBounds(t *testing.T) {
	data := []byte{0xb1} // 10110001
	stream := NewBitStream(data, 42)

	// Read all 8 bits
	_, err := stream.ReadNBits(8)
	if err != nil {
		t.Fatalf("ReadNBits(8) failed: %v", err)
	}

	// Next read should fail
	_, err = stream.ReadNBits(2)
	if err == nil {
		t.Error("Expected error for out of bounds read, got nil")
	}
}

func TestBitStreamReadNBitsWhereNIs36(t *testing.T) {
	data := []byte{0xb0, 0x01, 0x00, 0x00, 0x40}
	stream := NewBitStream(data, 42)

	// This should read 34 bits, but since we only have 40 bits total,
	// it should read all available bits and shift off the top 2 bits
	val, err := stream.ReadNBits(34)
	if err != nil {
		t.Fatalf("ReadNBits(34) failed: %v", err)
	}
	// Expected: top 2 bits are lost, so 0xc0040001
	if val != 0xc0040001 {
		t.Errorf("Expected 0xc0040001, got 0x%x", val)
	}
}

func TestBitStreamRead1Bit(t *testing.T) {
	data := []byte{0xb1} // 10110001
	stream := NewBitStream(data, 0)

	// First bit should be 1
	bit1, err := stream.Read1Bit()
	if err != nil {
		t.Fatalf("Read1Bit() failed: %v", err)
	}
	if bit1 != 1 {
		t.Errorf("Expected 1, got %d", bit1)
	}

	// Second bit should be 0
	bit2, err := stream.Read1Bit()
	if err != nil {
		t.Fatalf("Read1Bit() failed: %v", err)
	}
	if bit2 != 0 {
		t.Errorf("Expected 0, got %d", bit2)
	}
}

func TestBitStreamRead1BitBool(t *testing.T) {
	data := []byte{0xb1} // 10110001
	stream := NewBitStream(data, 0)

	// First bit should be true
	bit1, err := stream.Read1BitBool()
	if err != nil {
		t.Fatalf("Read1BitBool() failed: %v", err)
	}
	if !bit1 {
		t.Error("Expected true, got false")
	}

	// Second bit should be false
	bit2, err := stream.Read1BitBool()
	if err != nil {
		t.Fatalf("Read1BitBool() failed: %v", err)
	}
	if bit2 {
		t.Error("Expected false, got true")
	}
}

func TestBitStreamReadByte(t *testing.T) {
	data := []byte{0xb1, 0x42}
	stream := NewBitStream(data, 0)

	// First byte should be 0xb1
	byte1, err := stream.ReadByte()
	if err != nil {
		t.Fatalf("ReadByte() failed: %v", err)
	}
	if byte1 != 0xb1 {
		t.Errorf("Expected 0xb1, got 0x%x", byte1)
	}

	// Second byte should be 0x42
	byte2, err := stream.ReadByte()
	if err != nil {
		t.Fatalf("ReadByte() failed: %v", err)
	}
	if byte2 != 0x42 {
		t.Errorf("Expected 0x42, got 0x%x", byte2)
	}
}

func TestBitStreamInBounds(t *testing.T) {
	data := []byte{0xb1}
	stream := NewBitStream(data, 0)

	// Initially in bounds
	if !stream.InBounds() {
		t.Error("Expected stream to be in bounds initially")
	}

	// Read all bits
	stream.ReadNBits(8)

	// Should be out of bounds now
	if stream.InBounds() {
		t.Error("Expected stream to be out of bounds after reading all bits")
	}
}

func TestBitStreamBitPos(t *testing.T) {
	data := []byte{0xb1}
	stream := NewBitStream(data, 0)

	// Initial position should be 0
	if pos := stream.BitPos(); pos != 0 {
		t.Errorf("Expected initial position 0, got %d", pos)
	}

	// Read 3 bits
	stream.ReadNBits(3)
	if pos := stream.BitPos(); pos != 3 {
		t.Errorf("Expected position 3 after reading 3 bits, got %d", pos)
	}

	// Read 2 more bits
	stream.ReadNBits(2)
	if pos := stream.BitPos(); pos != 5 {
		t.Errorf("Expected position 5 after reading 5 bits, got %d", pos)
	}
}

func TestBitStreamLengthInBits(t *testing.T) {
	data := []byte{0xb1, 0x42} // 16 bits
	stream := NewBitStream(data, 0)

	expected := uint32(16)
	if length := stream.lengthInBits(); length != expected {
		t.Errorf("Expected length %d, got %d", expected, length)
	}
}

func TestBitStreamSetOffset(t *testing.T) {
	data := []byte{0xb1, 0x42, 0x83}
	stream := NewBitStream(data, 0)

	// Set offset to byte 1 (bit 8)
	stream.SetOffset(1)

	// Read 4 bits from second byte
	val, err := stream.ReadNBits(4)
	if err != nil {
		t.Fatalf("ReadNBits(4) after SetOffset failed: %v", err)
	}
	// Bits 0-3 of second byte (0x42): 0100 = 4 (since we're reading from MSB)
	if val != 4 {
		t.Errorf("Expected 4, got %d", val)
	}
}

func TestBitStreamAddOffset(t *testing.T) {
	data := []byte{0xb1, 0x42, 0x83}
	stream := NewBitStream(data, 0)

	// Read 4 bits from first byte (0xb1 = 10110001, bits 7-4: 1011 = 11)
	stream.ReadNBits(4)

	// Add offset by 1 byte
	stream.AddOffset(1)

	// Should now be at byte position 1, bit position 4 (since we read 4 bits from first byte)
	if pos := stream.BitPos(); pos != 12 {
		t.Errorf("Expected position 12, got %d", pos)
	}

	// Read 4 bits from second byte starting at bit 4 of byte 1
	val, err := stream.ReadNBits(4)
	if err != nil {
		t.Fatalf("ReadNBits(4) after AddOffset failed: %v", err)
	}
	// Bits 4-7 of the second byte (0x42): 0010 = 2
	if val != 2 {
		t.Errorf("Expected 2, got %d", val)
	}
}

func TestBitStreamAlignByte(t *testing.T) {
	data := []byte{0xb1, 0x42, 0x83}
	stream := NewBitStream(data, 0)

	// Read 3 bits
	stream.ReadNBits(3)

	// Align to byte boundary
	stream.AlignByte()

	// Should now be at bit position 8 (start of second byte)
	if pos := stream.BitPos(); pos != 8 {
		t.Errorf("Expected position 8 after AlignByte, got %d", pos)
	}
}

func TestBitStreamPointer(t *testing.T) {
	data := []byte{0xb1, 0x42, 0x83}
	stream := NewBitStream(data, 0)

	// Read 3 bits
	stream.ReadNBits(3)

	// Pointer should still point to first byte since we haven't advanced to next byte
	pointer := stream.Pointer()
	if pointer[0] != 0xb1 {
		t.Errorf("Expected pointer to point to 0xb1, got 0x%x", pointer[0])
	}
}

func TestBitStreamBytesLeft(t *testing.T) {
	data := []byte{0xb1, 0x42, 0x83}
	stream := NewBitStream(data, 0)

	// Initially 3 bytes left
	if left := stream.BytesLeft(); left != 3 {
		t.Errorf("Expected 3 bytes left initially, got %d", left)
	}

	// Read 1 byte
	stream.ReadByte()

	// Should have 2 bytes left
	if left := stream.BytesLeft(); left != 2 {
		t.Errorf("Expected 2 bytes left after reading 1 byte, got %d", left)
	}

	// Read 4 bits (half byte)
	stream.ReadNBits(4)

	// Should still have 2 bytes left since we haven't moved to next byte
	if left := stream.BytesLeft(); left != 2 {
		t.Errorf("Expected 2 bytes left after reading 0.5 bytes from second byte, got %d", left)
	}
}
