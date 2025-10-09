package jbig2

import (
	"testing"
)

func TestImageEmpty(t *testing.T) {
	img := NewImage(0, 0)

	// Check dimensions
	if img.Width() != 0 {
		t.Errorf("Expected width 0, got %d", img.Width())
	}
	if img.Height() != 0 {
		t.Errorf("Expected height 0, got %d", img.Height())
	}

	// Out-of-bounds SetPixel should be no-op
	img.SetPixel(0, 0, 1)
	img.SetPixel(1, 1, 1)

	// Out-of-bounds GetPixel should return 0
	if pixel := img.GetPixel(0, 0); pixel != 0 {
		t.Errorf("Expected pixel (0,0) to be 0, got %d", pixel)
	}
	if pixel := img.GetPixel(1, 1); pixel != 0 {
		t.Errorf("Expected pixel (1,1) to be 0, got %d", pixel)
	}

	// For empty image, any access should be safe (no-op for SetPixel, return 0 for GetPixel)
}

func TestImageCreate(t *testing.T) {
	width := 80
	height := 20
	img := NewImage(int32(width), int32(height))

	// Check dimensions
	if img.Width() != width {
		t.Errorf("Expected width %d, got %d", width, img.Width())
	}
	if img.Height() != height {
		t.Errorf("Expected height %d, got %d", height, img.Height())
	}

	// Check initial pixel values
	if pixel := img.GetPixel(0, 0); pixel != 0 {
		t.Errorf("Expected pixel (0,0) to be 0, got %d", pixel)
	}
	if pixel := img.GetPixel(int32(width-1), int32(height-1)); pixel != 0 {
		t.Errorf("Expected pixel (%d,%d) to be 0, got %d", width-1, height-1, pixel)
	}

	// Set some pixels
	img.SetPixel(0, 0, 1)
	img.SetPixel(int32(width-1), int32(height-1), 1)

	// Check pixel values
	if pixel := img.GetPixel(0, 0); pixel != 1 {
		t.Errorf("Expected pixel (0,0) to be 1, got %d", pixel)
	}
	if pixel := img.GetPixel(int32(width-1), int32(height-1)); pixel != 1 {
		t.Errorf("Expected pixel (%d,%d) to be 1, got %d", width-1, height-1, pixel)
	}

	// Out-of-bounds SetPixel should be no-op
	img.SetPixel(-1, 1, 1)
	img.SetPixel(int32(width), int32(height), 1)

	// Out-of-bounds GetPixel should return 0
	if pixel := img.GetPixel(-1, -1); pixel != 0 {
		t.Errorf("Expected out-of-bounds pixel to be 0, got %d", pixel)
	}
	if pixel := img.GetPixel(int32(width), int32(height)); pixel != 0 {
		t.Errorf("Expected out-of-bounds pixel to be 0, got %d", pixel)
	}

	// Out-of-bounds access should be safe
}

func TestImageCreateTooBig(t *testing.T) {
	width := 80
	height := 40000000 // Too large
	img := NewImage(int32(width), int32(height))

	// Should fail to allocate
	if img.Width() != 0 {
		t.Errorf("Expected width 0 for too-large image, got %d", img.Width())
	}
	if img.Height() != 0 {
		t.Errorf("Expected height 0 for too-large image, got %d", img.Height())
	}
}

func TestImageCreateExternal(t *testing.T) {
	width := 80
	height := 20
	stride := 12 // Different from width
	buf := make([]byte, height*stride)
	img, err := NewImageFromBuffer(int32(width), int32(height), int32(stride), buf)
	if err != nil {
		t.Fatalf("NewImageFromBuffer failed: %v", err)
	}

	// Check dimensions
	if img.Width() != width {
		t.Errorf("Expected width %d, got %d", width, img.Width())
	}
	if img.Height() != height {
		t.Errorf("Expected height %d, got %d", height, img.Height())
	}

	// Set and check pixels
	img.SetPixel(0, 0, 1)
	img.SetPixel(int32(width-1), int32(height-1), 0)

	if pixel := img.GetPixel(0, 0); pixel != 1 {
		t.Errorf("Expected pixel (0,0) to be 1, got %d", pixel)
	}
	if pixel := img.GetPixel(int32(width-1), int32(height-1)); pixel != 0 {
		t.Errorf("Expected pixel (%d,%d) to be 0, got %d", width-1, height-1, pixel)
	}
}

func TestImageCreateExternalTooBig(t *testing.T) {
	width := 80
	height := 40000000 // Too large
	stride := 82
	buf := make([]byte, height*stride)
	img, err := NewImageFromBuffer(int32(width), int32(height), int32(stride), buf)

	// Should fail to allocate
	if err == nil {
		t.Error("Expected error for too-large image")
	}
	if img != nil {
		t.Error("Expected nil image for too-large image")
	}
}

func TestImageExpand(t *testing.T) {
	width := 80
	height := 20
	largerHeight := 100
	img := NewImage(int32(width), int32(height))

	// Set some pixels
	img.SetPixel(0, 0, 1)
	img.SetPixel(int32(width-1), int32(height-1), 0)

	// Expand
	img.Expand(int32(largerHeight), true)

	// Check dimensions after expand
	if img.Width() != width {
		t.Errorf("Expected width %d after expand, got %d", width, img.Width())
	}
	if img.Height() != largerHeight {
		t.Errorf("Expected height %d after expand, got %d", largerHeight, img.Height())
	}

	// Check preserved pixels
	if pixel := img.GetPixel(0, 0); pixel != 1 {
		t.Errorf("Expected pixel (0,0) to still be 1 after expand, got %d", pixel)
	}
	if pixel := img.GetPixel(int32(width-1), int32(height-1)); pixel != 0 {
		t.Errorf("Expected pixel (%d,%d) to still be 0 after expand, got %d", width-1, height-1, pixel)
	}

	// Check new pixels are filled with default value (true)
	if pixel := img.GetPixel(int32(width-1), int32(largerHeight-1)); pixel != 1 {
		t.Errorf("Expected new pixel (%d,%d) to be 1 (default), got %d", width-1, largerHeight-1, pixel)
	}
}

func TestImageExpandTooBig(t *testing.T) {
	width := 80
	height := 20
	tooLargeHeight := 40000000
	img := NewImage(int32(width), int32(height))

	// Set some pixels
	img.SetPixel(0, 0, 1)
	img.SetPixel(int32(width-1), int32(height-1), 0)

	// Try to expand to too large size
	img.Expand(int32(tooLargeHeight), true)

	// Should not change dimensions
	if img.Width() != width {
		t.Errorf("Expected width %d after failed expand, got %d", width, img.Width())
	}
	if img.Height() != height {
		t.Errorf("Expected height %d after failed expand, got %d", height, img.Height())
	}

	// Pixels should be preserved
	if pixel := img.GetPixel(0, 0); pixel != 1 {
		t.Errorf("Expected pixel (0,0) to still be 1 after failed expand, got %d", pixel)
	}
	if pixel := img.GetPixel(int32(width-1), int32(height-1)); pixel != 0 {
		t.Errorf("Expected pixel (%d,%d) to still be 0 after failed expand, got %d", width-1, height-1, pixel)
	}
}

func TestImageSubImage(t *testing.T) {
	// Create test pattern
	pattern := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x01, 0xff, 0x80, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x01, 0x00, 0x80, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x01, 0xff, 0x80, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}

	img, err := NewImageFromBuffer(32, 5, 8, pattern)
	if err != nil {
		t.Fatalf("NewImageFromBuffer failed: %v", err)
	}

	// Test empty subimage
	sub := img.SubImage(0, 0, 0, 0)
	if sub.Width() != 0 || sub.Height() != 0 {
		t.Errorf("Expected empty subimage to have size 0x0, got %dx%d", sub.Width(), sub.Height())
	}

	// Test full subimage
	sub = img.SubImage(0, 0, 32, 5)
	if sub.Width() != 32 || sub.Height() != 5 {
		t.Errorf("Expected full subimage to have size 32x5, got %dx%d", sub.Width(), sub.Height())
	}

	// Test subimage with offset
	sub = img.SubImage(2, 0, 30, 5)
	if sub.Width() != 30 || sub.Height() != 5 {
		t.Errorf("Expected offset subimage to have size 30x5, got %dx%d", sub.Width(), sub.Height())
	}

	// Test subimage with negative offset (should be zero-padded)
	sub = img.SubImage(-1, 0, 32, 5)
	if sub.Width() != 32 || sub.Height() != 5 {
		t.Errorf("Expected negative offset subimage to have size 32x5, got %dx%d", sub.Width(), sub.Height())
	}

	// Test subimage with negative dimensions (should be empty)
	sub = img.SubImage(0, 0, -1, -1)
	if sub.Width() != 0 || sub.Height() != 0 {
		t.Errorf("Expected negative dimension subimage to be empty, got %dx%d", sub.Width(), sub.Height())
	}
}

func TestImageCopyLine(t *testing.T) {
	// Create test pattern
	pattern := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x01, 0xff, 0x80, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}

	img, err := NewImageFromBuffer(37, 3, 8, pattern)
	if err != nil {
		t.Fatalf("NewImageFromBuffer failed: %v", err)
	}

	// Shuffle lines
	img.CopyLine(2, 1) // Copy line 1 to line 2
	img.CopyLine(1, 0) // Copy line 0 to line 1
	img.CopyLine(0, 2) // Copy line 2 to line 0

	// Create expected result
	expectedPattern := []byte{
		0x00, 0x01, 0xff, 0x80, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x01, 0xff, 0x80, 0x00, 0x00, 0x00, 0x00,
	}

	expected, err := NewImageFromBuffer(37, 3, 8, expectedPattern)
	if err != nil {
		t.Fatalf("NewImageFromBuffer failed: %v", err)
	}

	// Compare images
	for y := int32(0); y < 3; y++ {
		for x := int32(0); x < 37; x++ {
			expectedPixel := expected.GetPixel(x, y)
			actualPixel := img.GetPixel(x, y)
			if expectedPixel != actualPixel {
				t.Errorf("Pixel mismatch at (%d,%d): expected %d, got %d", x, y, expectedPixel, actualPixel)
			}
		}
	}
}

func TestImageFill(t *testing.T) {
	img := NewImage(10, 10)

	// Fill with 1s
	img.Fill(true)

	// Check all pixels are 1
	for y := int32(0); y < 10; y++ {
		for x := int32(0); x < 10; x++ {
			if pixel := img.GetPixel(x, y); pixel != 1 {
				t.Errorf("Expected pixel (%d,%d) to be 1 after fill, got %d", x, y, pixel)
			}
		}
	}

	// Fill with 0s
	img.Fill(false)

	// Check all pixels are 0
	for y := int32(0); y < 10; y++ {
		for x := int32(0); x < 10; x++ {
			if pixel := img.GetPixel(x, y); pixel != 0 {
				t.Errorf("Expected pixel (%d,%d) to be 0 after fill, got %d", x, y, pixel)
			}
		}
	}
}

func TestImageComposeTo(t *testing.T) {
	// Create source and destination images
	src := NewImage(4, 4)
	src.SetPixel(0, 0, 1)
	src.SetPixel(1, 1, 1)
	src.SetPixel(2, 2, 1)
	src.SetPixel(3, 3, 1)

	dst := NewImage(8, 8)

	// Compose source onto destination
	if !src.ComposeTo(dst, 2, 2, ComposeOR) {
		t.Error("ComposeTo failed")
	}

	// Check result
	// Pixels outside source should be unchanged (0)
	if pixel := dst.GetPixel(0, 0); pixel != 0 {
		t.Errorf("Expected pixel (0,0) to be 0, got %d", pixel)
	}

	// Pixels from source should be composed with OR
	if pixel := dst.GetPixel(2, 2); pixel != 1 {
		t.Errorf("Expected pixel (2,2) to be 1, got %d", pixel)
	}
	if pixel := dst.GetPixel(3, 3); pixel != 1 {
		t.Errorf("Expected pixel (3,3) to be 1, got %d", pixel)
	}
	if pixel := dst.GetPixel(4, 4); pixel != 1 {
		t.Errorf("Expected pixel (4,4) to be 1, got %d", pixel)
	}
	if pixel := dst.GetPixel(5, 5); pixel != 1 {
		t.Errorf("Expected pixel (5,5) to be 1, got %d", pixel)
	}
}
