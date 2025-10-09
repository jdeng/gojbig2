package jbig2

import "testing"

func TestStripJBIG2FileHeader_NoSignature(t *testing.T) {
	data := []byte{0x00, 0x01, 0x02, 0x03}
	trimmed, header, err := stripJBIG2FileHeader(data)
	if err != nil {
		t.Fatalf("stripJBIG2FileHeader returned error: %v", err)
	}
	if header != nil {
		t.Fatalf("unexpected header: %#v", header)
	}
	if &trimmed[0] != &data[0] {
		t.Fatalf("expected original slice, got new slice")
	}
}

func TestStripJBIG2FileHeader_WithSignature(t *testing.T) {
	head := append([]byte{}, jbig2FileSignature...)
	head = append(head, 0x00)                   // flags: page count present
	head = append(head, 0x00, 0x00, 0x00, 0x03) // num pages = 3 (big-endian)
	payload := []byte{0xAA, 0xBB, 0xCC}
	data := append(head, payload...)

	trimmed, header, err := stripJBIG2FileHeader(data)
	if err != nil {
		t.Fatalf("stripJBIG2FileHeader returned error: %v", err)
	}
	if header == nil {
		t.Fatalf("expected header, got nil")
	}
	if header.Flags != 0 {
		t.Fatalf("unexpected flags: got %d, want 0", header.Flags)
	}
	if !header.HasNumPage || header.NumPages != 3 {
		t.Fatalf("unexpected pages: got %d, want 3 (has=%v)", header.NumPages, header.HasNumPage)
	}
	if len(trimmed) != len(payload) {
		t.Fatalf("unexpected trimmed length: got %d, want %d", len(trimmed), len(payload))
	}
	for i, b := range payload {
		if trimmed[i] != b {
			t.Fatalf("trimmed payload mismatch at %d: got 0x%02x want 0x%02x", i, trimmed[i], b)
		}
	}
}

func TestStripJBIG2FileHeader_Truncated(t *testing.T) {
	head := append([]byte{}, jbig2FileSignature...)
	data := append(head, 0x00, 0x00)
	if _, _, err := stripJBIG2FileHeader(data); err == nil {
		t.Fatal("expected error for truncated header")
	}
}
