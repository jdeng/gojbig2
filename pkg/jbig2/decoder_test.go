package jbig2

import (
	"testing"
)

func TestDecoderCreation(t *testing.T) {
	// Test creating decoder with empty data
	_, err := New(Options{})
	if err == nil {
		t.Error("Expected error for empty source data, got nil")
	}

	// Test creating decoder with valid data
	data := []byte{0x00, 0x01, 0x02, 0x03}
	opts := Options{
		SrcData: data,
		SrcKey:  0,
	}
	decoder, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create decoder: %v", err)
	}
	if decoder == nil {
		t.Fatal("Expected decoder to be non-nil")
	}
}

func TestDecoderOptions(t *testing.T) {
	data := []byte{0x00, 0x01, 0x02, 0x03}
	globalData := []byte{0x10, 0x11, 0x12, 0x13}

	opts := Options{
		GlobalData: globalData,
		GlobalKey:  0x12345678,
		SrcData:    data,
		SrcKey:     0x87654321,
	}

	decoder, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create decoder with options: %v", err)
	}
	if decoder == nil {
		t.Fatal("Expected decoder to be non-nil")
	}
}

func TestDecoderSegments(t *testing.T) {
	data := []byte{0x00, 0x01, 0x02, 0x03}
	opts := Options{
		SrcData: data,
		SrcKey:  0,
	}

	decoder, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create decoder: %v", err)
	}

	// Initially should have no segments
	segments := decoder.GetSegments()
	if len(segments) != 0 {
		t.Errorf("Expected 0 segments initially, got %d", len(segments))
	}
}

func TestDecoderProcessingStatus(t *testing.T) {
	data := []byte{0x00, 0x01, 0x02, 0x03}
	opts := Options{
		SrcData: data,
		SrcKey:  0,
	}

	decoder, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create decoder: %v", err)
	}

	// Should be ready initially
	status := decoder.GetProcessingStatus()
	if status != CodecStatusReady {
		t.Errorf("Expected status CodecStatusReady, got %v", status)
	}
}

func TestResultTypeString(t *testing.T) {
	tests := []struct {
		rt       ResultType
		expected string
	}{
		{ResultTypeVoid, "Void"},
		{ResultTypeImage, "Image"},
		{ResultTypeSymbolDict, "SymbolDict"},
		{ResultTypePatternDict, "PatternDict"},
		{ResultTypeHuffmanTable, "HuffmanTable"},
	}

	for _, test := range tests {
		if got := test.rt.String(); got != test.expected {
			t.Errorf("ResultType %d string mismatch: got %q, want %q", test.rt, got, test.expected)
		}
	}

	if got := ResultType(42).String(); got != "ResultType(42)" {
		t.Errorf("Unexpected fallback string: got %q", got)
	}
}

func TestCodecStatusString(t *testing.T) {
	tests := []struct {
		status   CodecStatus
		expected string
	}{
		{CodecStatusReady, "Ready"},
		{CodecStatusToBeContinued, "ToBeContinued"},
		{CodecStatusFinished, "Finished"},
		{CodecStatusError, "Error"},
	}

	for _, test := range tests {
		if got := test.status.String(); got != test.expected {
			t.Errorf("CodecStatus %d string mismatch: got %q, want %q", test.status, got, test.expected)
		}
	}

	if got := CodecStatus(99).String(); got != "CodecStatus(99)" {
		t.Errorf("Unexpected fallback codec status string: got %q", got)
	}
}
