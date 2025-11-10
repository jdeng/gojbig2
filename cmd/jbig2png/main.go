package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"path/filepath"

	jbig2 "github.com/jdeng/gojbig2/pkg/jbig2"
)

func main() {
	var inputFile = flag.String("input", "", "Input JBIG2 file")
	var globalFile = flag.String("global", "", "Optional JBIG2 globals stream extracted from PDF")
	var outputFile = flag.String("output", "", "Output PNG file (optional, defaults to input filename with .png extension)")
	flag.Parse()

	if *inputFile == "" {
		log.Fatal("Input file is required. Use -input flag.")
	}

	// Read input file
	data, err := os.ReadFile(*inputFile)
	if err != nil {
		log.Fatalf("Failed to read input file: %v", err)
	}

	var globals []byte
	if *globalFile != "" {
		globals, err = os.ReadFile(*globalFile)
		if err != nil {
			log.Fatalf("Failed to read global data file: %v", err)
		}
	}

	// Create JBIG2 decoder
	opts := jbig2.Options{
		GlobalData: globals,
		SrcData:    data,
		SrcKey:     0,
	}

	decoder, err := jbig2.New(opts)
	if err != nil {
		log.Fatalf("Failed to create JBIG2 decoder: %v", err)
	}

	// Decode all segments
	err = decoder.DecodeAll()
	if err != nil {
		log.Fatalf("Failed to decode JBIG2: %v", err)
	}

	// Get segments
	segments := decoder.GetSegments()

	// Debug: Print all segments
	fmt.Printf("Found %d segments in JBIG2 file:\n", len(segments))
	for i, seg := range segments {
		fmt.Printf("  Segment %d: Number=%d, Type=%d, ResultType=%s, DataLength=%d\n",
			i, seg.Number(), seg.Type(), seg.ResultType().String(), seg.DataLength())
	}

	// Debug: Print raw file data
	fmt.Printf("Raw file data (first 64 bytes):\n")
	debugData := opts.SrcData
	if len(debugData) > 64 {
		debugData = debugData[:64]
	}
	for i := 0; i < len(debugData); i += 16 {
		fmt.Printf("  %04x: ", i)
		for j := 0; j < 16 && i+j < len(debugData); j++ {
			fmt.Printf("%02x ", debugData[i+j])
		}
		fmt.Println()
	}

	// Find image segments
	var images []*jbig2.Image
	for _, seg := range segments {
		if seg.ResultType() == jbig2.ResultTypeImage {
			if img := seg.Image(); img != nil {
				images = append(images, img)
			}
		}
	}

	if len(images) == 0 {
		if page := decoder.GetPageImage(); page != nil {
			images = append(images, page)
		}
	}

	if len(images) == 0 {
		log.Fatal("No images found in JBIG2 file")
	}

	// Use the first image (or last if multiple)
	img := images[len(images)-1]

	// Validate image dimensions
	if img.Width() <= 0 || img.Height() <= 0 {
		log.Fatalf("Invalid image dimensions: %dx%d", img.Width(), img.Height())
	}

	// Convert to grayscale image
	grayImg := convertToGrayscale(img)

	// Determine output filename
	output := *outputFile
	if output == "" {
		ext := filepath.Ext(*inputFile)
		output = (*inputFile)[:len(*inputFile)-len(ext)] + ".png"
	}

	// Create output file
	file, err := os.Create(output)
	if err != nil {
		log.Fatalf("Failed to create output file: %v", err)
	}
	defer file.Close()

	// Encode as PNG for lossless output
	err = png.Encode(file, grayImg)
	if err != nil {
		log.Fatalf("Failed to encode PNG: %v", err)
	}

	fmt.Printf("Successfully converted %s to %s\n", *inputFile, output)
	fmt.Printf("Image size: %dx%d pixels\n", img.Width(), img.Height())
}

// convertToGrayscale converts a 1-bit JBIG2 image to 8-bit grayscale
func convertToGrayscale(jbig2Img *jbig2.Image) *image.Gray {
	width := jbig2Img.Width()
	height := jbig2Img.Height()

	// Create grayscale image
	rect := image.Rect(0, 0, width, height)
	grayImg := image.NewGray(rect)

	// Get raw pixel data
	data := jbig2Img.Data()
	if len(data) == 0 {
		// Empty image, fill with white
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				grayImg.SetGray(x, y, color.Gray{Y: 255})
			}
		}
		return grayImg
	}

	stride := len(data) / height
	if stride*height != len(data) {
		stride = (width + 7) / 8
	}

	// Convert 1-bit pixels to 8-bit grayscale
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Calculate bit position within the data
			byteIndex := y*stride + x/8
			bitIndex := uint(7 - (x % 8))

			// Extract bit value
			bit := byte(0)
			if byteIndex < len(data) {
				bit = (data[byteIndex] >> bitIndex) & 1
			}

			// Convert to grayscale (0 = black, 1 = white)
			// In JBIG2, 1 typically means "paint" (black), 0 means "no paint" (white)
			// But for display purposes, we invert this for typical image expectations
			pixelValue := byte(255) // White background
			if bit != 0 {
				pixelValue = 0 // Black pixel
			}

			grayImg.SetGray(x, y, color.Gray{Y: pixelValue})
		}
	}

	return grayImg
}
