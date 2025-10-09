package jbig2

import "testing"

func TestHTRDProcDecodeImageClampsPatternIndex(t *testing.T) {
	proc := NewHTRDProc()
	proc.HBWidth = 2
	proc.HBHeight = 2
	proc.HGWidth = 2
	proc.HGHeight = 2
	proc.HGX = 0
	proc.HGY = 0
	proc.HRX = 256
	proc.HRY = 0
	proc.HPW = 1
	proc.HPH = 1
	proc.HDefPixel = false
	proc.HCombOp = ComposeReplace
	proc.HNumPats = 3
	proc.HPats = make([]*Image, proc.HNumPats)

	// Pattern 0: empty
	proc.HPats[0] = NewImage(1, 1)
	// Pattern 1: clears to 0 (already zero)
	proc.HPats[1] = NewImage(1, 1)
	// Pattern 2: sets pixel to 1
	proc.HPats[2] = NewImage(1, 1)
	proc.HPats[2].SetPixel(0, 0, 1)

	// Two bitplanes (LSB then MSB)
	plane0 := NewImage(2, 2)
	plane1 := NewImage(2, 2)
	if plane0 == nil || plane1 == nil {
		t.Fatalf("failed to allocate bitplanes")
	}

	// pattern indexes: (0,0)->0, (1,0)->2, (0,1)->1, (1,1)->3 (clamped to 2)
	plane0.SetPixel(0, 1, 1) // index 1 at (0,1)
	plane0.SetPixel(1, 1, 1) // contributes to 3 at (1,1)
	plane0.SetPixel(1, 0, 0)
	plane1.SetPixel(1, 0, 1) // index 2 at (1,0)
	plane1.SetPixel(1, 1, 1) // with plane0 gives 3 at (1,1)

	img, err := proc.decodeImage([]*Image{plane0, plane1})
	if err != nil {
		t.Fatalf("decodeImage returned error: %v", err)
	}

	if got := img.GetPixel(1, 0); got != 1 {
		t.Errorf("pixel (1,0) = %d, want 1", got)
	}
	if got := img.GetPixel(1, 1); got != 1 {
		t.Errorf("pixel (1,1) = %d, want 1 (clamped pattern 2)", got)
	}
	if got := img.GetPixel(0, 1); got != 0 {
		t.Errorf("pixel (0,1) = %d, want 0", got)
	}
	if got := img.GetPixel(0, 0); got != 0 {
		t.Errorf("pixel (0,0) = %d, want 0", got)
	}
}
