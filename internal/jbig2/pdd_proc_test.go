package jbig2

import "testing"

func TestPDDProcBuildPatternDictFromImage(t *testing.T) {
	p := NewPDDProc()
	p.HDPW = 2
	p.HDPH = 2
	p.GrayMax = 1

	canvas := NewImage(4, 2)
	if canvas == nil || canvas.data == nil {
		t.Fatal("failed to allocate source image")
	}
	canvas.SetPixel(0, 0, 1)
	canvas.SetPixel(3, 1, 1)

	dict := p.buildPatternDictFromImage(canvas)
	if dict == nil {
		t.Fatalf("expected pattern dictionary")
	}
	if dict.NumPatterns != 2 {
		t.Fatalf("NumPatterns = %d, want 2", dict.NumPatterns)
	}

	pat0 := dict.GetPattern(0)
	if pat0 == nil {
		t.Fatalf("missing pattern 0")
	}
	if pat0.Width() != 2 || pat0.Height() != 2 {
		t.Fatalf("pattern 0 dimensions %dx%d, want 2x2", pat0.Width(), pat0.Height())
	}
	if pat0.GetPixel(0, 0) != 1 {
		t.Errorf("pattern 0 pixel (0,0) = %d, want 1", pat0.GetPixel(0, 0))
	}
	if pat0.GetPixel(1, 1) != 0 {
		t.Errorf("pattern 0 pixel (1,1) = %d, want 0", pat0.GetPixel(1, 1))
	}

	pat1 := dict.GetPattern(1)
	if pat1 == nil {
		t.Fatalf("missing pattern 1")
	}
	if pat1.GetPixel(1, 1) != 1 {
		t.Errorf("pattern 1 pixel (1,1) = %d, want 1", pat1.GetPixel(1, 1))
	}
	if pat1.GetPixel(0, 0) != 0 {
		t.Errorf("pattern 1 pixel (0,0) = %d, want 0", pat1.GetPixel(0, 0))
	}
}

func TestPDDProcCreateGRDProcRejectsZeroDimensions(t *testing.T) {
	p := NewPDDProc()
	p.HDPW = 0
	p.HDPH = 1
	p.GrayMax = 0
	if _, err := p.createGRDProc(); err == nil {
		t.Fatal("expected error for zero width pattern dictionary")
	}
}
