package main

import (
	"fmt"
	"os"
)

// createMinimalJBIG2 creates a minimal JBIG2 file with a simple image
func createMinimalJBIG2(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// JBIG2 file header
	header := []byte{
		0x97, 0x4A, 0x42, 0x32, 0x0D, 0x0A, 0x1A, 0x0A, // JBIG2 magic
		0x00, 0x00, 0x00, 0x04, // File header length (4 bytes)
		0x00, 0x00, 0x00, 0x01, // Number of pages
	}

	// Page info segment
	pageInfo := []byte{
		0x00, 0x00, 0x00, 0x01, // Segment number (1)
		0x30,                   // Segment flags (type 48 = page info)
		0x00,                   // Referred to segment count
		0x00,                   // Page association (short form)
		0x00, 0x00, 0x00, 0x0C, // Data length (12 bytes)
		// Segment data:
		0x00, 0x00, 0x00, 0x64, // Page width (100)
		0x00, 0x00, 0x00, 0x64, // Page height (100)
		0x00, 0x00, // Resolution X
		0x00, 0x00, // Resolution Y
		0x00,                   // Page flags
		0x00, 0x00, 0x00, 0x00, // Striping info
	}

	// Generic region segment (type 36) - creates a simple 10x10 image
	genericRegion := []byte{
		0x00, 0x00, 0x00, 0x02, // Segment number
		0x24,                   // Segment flags (type 36 = generic region)
		0x00,                   // Referred to segment count
		0x00,                   // Page association (short form)
		0x00, 0x00, 0x00, 0x19, // Data length (25 bytes)
		// Segment data:
		0x00, 0x00, 0x00, 0x0A, // X position (10)
		0x00, 0x00, 0x00, 0x0A, // Y position (10)
		0x00, 0x00, 0x00, 0x0A, // Width (10)
		0x00, 0x00, 0x00, 0x0A, // Height (10)
		0x00, // Region flags (MMR = 0, TPGDON = 0)
		// Minimal image data - just a few bytes
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}

	// End of page segment
	endOfPage := []byte{
		0x00, 0x00, 0x00, 0x03, // Segment number (3)
		0x31,                   // Segment flags (type 49 = end of page)
		0x00,                   // Referred to segment count
		0x00,                   // Page association (short form)
		0x00, 0x00, 0x00, 0x00, // Data length (0 bytes)
	}

	// End of file segment
	endOfFile := []byte{
		0x00, 0x00, 0x00, 0x04, // Segment number (4)
		0x33,                   // Segment flags (type 51 = end of file)
		0x00,                   // Referred to segment count
		0x00,                   // Page association (short form)
		0x00, 0x00, 0x00, 0x00, // Data length (0 bytes)
	}

	// Write all segments
	_, err = file.Write(header)
	if err != nil {
		return err
	}

	_, err = file.Write(pageInfo)
	if err != nil {
		return err
	}

	_, err = file.Write(genericRegion)
	if err != nil {
		return err
	}

	_, err = file.Write(endOfPage)
	if err != nil {
		return err
	}

	_, err = file.Write(endOfFile)
	if err != nil {
		return err
	}

	return nil
}

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: create-test-jbig2 <output-file>")
		os.Exit(1)
	}

	filename := os.Args[1]
	err := createMinimalJBIG2(filename)
	if err != nil {
		fmt.Printf("Error creating test JBIG2 file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Created test JBIG2 file: %s\n", filename)
}
