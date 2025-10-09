package jbig2

// RegionInfo describes a rectangular region within a JBIG2 bitmap frame.
// The fields map 1:1 with the original PDFium JBig2RegionInfo struct.
type RegionInfo struct {
	Width  int32
	Height int32
	X      int32
	Y      int32
	Flags  uint8
}

// HuffmanCode represents a single code entry in a JBIG2 Huffman table.
// CodeLength is the number of bits assigned to Code; negative values indicate unused entries.
type HuffmanCode struct {
	CodeLength int32
	Code       int32
}

const (
	// JBig2OOB marks the "out of band" Huffman symbol used by the decoder.
	JBig2OOB int32 = 1

	// JBig2MaxReferredSegmentCount is the largest allowed count of referenced segments.
	JBig2MaxReferredSegmentCount int32 = 64
	// JBig2MaxExportSymbols is the maximum number of symbols exported from a dictionary.
	JBig2MaxExportSymbols uint32 = 65535
	// JBig2MaxNewSymbols is the maximum number of newly decoded symbols.
	JBig2MaxNewSymbols uint32 = 65535
	// JBig2MaxPatternIndex is the upper bound for pattern dictionary indices.
	JBig2MaxPatternIndex uint32 = 65535
	// JBig2MaxImageSize is an upper limit on image dimensions expressed as a 16-bit value.
	JBig2MaxImageSize int32 = 65535

	// JBig2MaxPatternDictSize is the maximum number of patterns in a pattern dictionary.
	JBig2MaxPatternDictSize uint32 = 65535
)
