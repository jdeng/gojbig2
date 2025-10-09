package jbig2

// SegmentState enumerates the parsing lifecycle for a JBIG2 segment.
type SegmentState int

const (
	SegmentStateHeaderUnparsed SegmentState = iota
	SegmentStateDataUnparsed
	SegmentStateParseComplete
	SegmentStatePaused
	SegmentStateError
)

// ResultType identifies what kind of result payload a segment produced.
type ResultType int

const (
	ResultTypeVoid ResultType = iota
	ResultTypeImage
	ResultTypeSymbolDict
	ResultTypePatternDict
	ResultTypeHuffmanTable
)

// Deprecated C++ names retained for ease of comparison with reference code.
const (
	JBIG2SegmentStateHeaderUnparsed = SegmentStateHeaderUnparsed
	JBIG2SegmentStateDataUnparsed   = SegmentStateDataUnparsed
	JBIG2SegmentStateParseComplete  = SegmentStateParseComplete
	JBIG2SegmentStatePaused         = SegmentStatePaused
	JBIG2SegmentStateError          = SegmentStateError

	JBIG2ResultTypeVoid         = ResultTypeVoid
	JBIG2ResultTypeImage        = ResultTypeImage
	JBIG2ResultTypeSymbolDict   = ResultTypeSymbolDict
	JBIG2ResultTypePatternDict  = ResultTypePatternDict
	JBIG2ResultTypeHuffmanTable = ResultTypeHuffmanTable
)

// SegmentFlags mirrors the bit-level semantics of the PDFium segment flag byte.
type SegmentFlags uint8

const (
	segmentFlagTypeMask              = 0x3f
	segmentFlagPageAssociationSize   = 0x40
	segmentFlagDeferredNonRetainMask = 0x80
)

// Raw exposes the underlying flag byte.
func (f SegmentFlags) Raw() uint8 { return uint8(f) }

// Type returns the 6-bit segment type identifier.
func (f SegmentFlags) Type() uint8 { return uint8(f) & segmentFlagTypeMask }

// HasLongPageAssociation indicates whether the page association field is 4 bytes instead of 1.
func (f SegmentFlags) HasLongPageAssociation() bool {
	return f&segmentFlagPageAssociationSize != 0
}

// DeferredNonRetain matches the C++ deferred non-retain bit.
func (f SegmentFlags) DeferredNonRetain() bool {
	return f&segmentFlagDeferredNonRetainMask != 0
}

// WithType returns a copy of f with the type bits replaced.
func (f SegmentFlags) WithType(t uint8) SegmentFlags {
	return (f &^ segmentFlagTypeMask) | SegmentFlags(t&segmentFlagTypeMask)
}

// WithLongPageAssociation toggles the long page association bit.
func (f SegmentFlags) WithLongPageAssociation(long bool) SegmentFlags {
	if long {
		return f | segmentFlagPageAssociationSize
	}
	return f &^ segmentFlagPageAssociationSize
}

// WithDeferredNonRetain toggles the deferred non-retain bit.
func (f SegmentFlags) WithDeferredNonRetain(deferred bool) SegmentFlags {
	if deferred {
		return f | segmentFlagDeferredNonRetainMask
	}
	return f &^ segmentFlagDeferredNonRetainMask
}

// Segment is the Go translation of CJBig2_Segment.
type Segment struct {
	Number                   uint32
	Flags                    SegmentFlags
	ReferredToSegmentCount   int32
	ReferredToSegmentNumbers []uint32
	PageAssociation          uint32
	DataLength               uint32
	HeaderLength             uint32
	DataOffset               uint32
	Key                      uint64
	State                    SegmentState
	ResultType               ResultType
	SymbolDict               *SymbolDict
	PatternDict              *PatternDict
	Image                    *Image
	HuffmanTable             *HuffmanTable
}

// NewSegment mirrors the default construction semantics from the C++ implementation.
func NewSegment() *Segment {
	return &Segment{}
}
