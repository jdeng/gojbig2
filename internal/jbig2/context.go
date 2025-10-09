package jbig2

import (
	"errors"
	"fmt"
	"math"
)

// DecodeResult mirrors the JBig2_Result enum from the reference implementation.
type DecodeResult int

const (
	DecodeResultSuccess DecodeResult = iota
	DecodeResultFailure
	DecodeResultEndReached
)

// Segment type tags.
const (
	segmentTypeSymbolDict                        = 0
	segmentTypeTextRegionImmediate               = 4
	segmentTypeTextRegionImmediateLossless       = 6
	segmentTypeTextRegionRefine                  = 7
	segmentTypePatternDict                       = 16
	segmentTypeHalftoneRegion                    = 20
	segmentTypeHalftoneRegionImmediate           = 22
	segmentTypeHalftoneRegionImmediateLossless   = 23
	segmentTypeGenericRegion                     = 36
	segmentTypeGenericRegionImmediate            = 38
	segmentTypeGenericRegionImmediateLossless    = 39
	segmentTypeRefinementRegion                  = 40
	segmentTypeRefinementRegionImmediate         = 42
	segmentTypeRefinementRegionImmediateLossless = 43
	segmentTypePageInfo                          = 48
	segmentTypeEndOfPage                         = 49
	segmentTypeEndOfStripe                       = 50
	segmentTypeEndOfFile                         = 51
	segmentTypeTables                            = 53
)

var errNotImplemented = errors.New("jbig2: context decode not yet implemented")

const JBIG2MinSegmentSize = 11

const symbolDictCacheMaxSize = 2

func huffContextSize(template uint8) int {
	switch template {
	case 0:
		return 65536
	case 1:
		return 8192
	default:
		return 1024
	}
}

func refAggContextSize(template bool) int {
	if template {
		return 1024
	}
	return 8192
}

// CompoundKey identifies cached symbol dictionaries using the stream key and index.
type CompoundKey struct {
	StreamKey uint64
	Segment   uint32
}

// CachePair pairs a compound key with a decoded symbol dictionary instance.
type CachePair struct {
	Key  CompoundKey
	Dict *SymbolDict
}

// Context represents the JBIG2 decoding context responsible for orchestrating
// segment parsing, dictionary reuse, and page assembly.
type Context struct {
	stream         *BitStream
	globalContext  *Context
	segments       []*Segment
	pageInfos      []*PageInfo
	page           *Image
	fileHeader     *FileHeader
	huffmanTables  []*HuffmanTable
	isGlobal       bool
	inPage         bool
	bufSpecified   bool
	pauseStep      int
	processing     CodecStatus
	gbContexts     []ArithContext
	grContexts     []ArithContext
	arithDecoder   *ArithDecoder
	grdProc        *GRDProc
	currentSegment *Segment
	offset         uint32
	ri             RegionInfo
	cache          *[]CachePair
}

// CreateContext instantiates a new context and optional global context tree.
func CreateContext(globalData []byte, globalKey uint64, srcData []byte, srcKey uint64, docCtx *DocumentContext) (*Context, error) {
	trimmedSrc, srcHeader, err := stripJBIG2FileHeader(srcData)
	if err != nil {
		return nil, err
	}
	ctx := newContext(trimmedSrc, srcKey, docCtx, false)
	if ctx.stream == nil {
		return nil, errors.New("jbig2: failed to initialise bitstream")
	}
	ctx.fileHeader = srcHeader
	if len(globalData) > 0 {
		trimmedGlobal, globalHeader, err := stripJBIG2FileHeader(globalData)
		if err != nil {
			return nil, err
		}
		ctx.globalContext = newContext(trimmedGlobal, globalKey, docCtx, true)
		if ctx.globalContext != nil {
			ctx.globalContext.fileHeader = globalHeader
		}
	}
	return ctx, nil
}

func newContext(data []byte, key uint64, docCtx *DocumentContext, isGlobal bool) *Context {
	var cache *[]CachePair
	if docCtx != nil {
		cache = docCtx.SymbolDictCache()
	}
	return &Context{
		stream:        NewBitStream(data, key),
		huffmanTables: make([]*HuffmanTable, len(builtinHuffmanTables)),
		isGlobal:      isGlobal,
		cache:         cache,
	}
}

// HuffmanAssignCode runs canonical Huffman code assignment on the provided slice.
func HuffmanAssignCode(codes []HuffmanCode) error {
	return assignHuffmanCodes(codes)
}

func (c *Context) DecodeSequential(pause PauseIndicator) (DecodeResult, error) {
	if c.stream == nil || c.stream.BytesLeft() == 0 {
		return DecodeResultEndReached, nil
	}

	for c.stream.BytesLeft() >= JBIG2MinSegmentSize {
		if c.currentSegment == nil {
			c.currentSegment = NewSegment()
			if err := c.parseSegmentHeader(c.currentSegment); err != nil {
				c.currentSegment = nil
				return DecodeResultFailure, err
			}
			c.offset = c.stream.Offset()
		}

		res, err := c.parseSegmentData(c.currentSegment, pause)
		if err != nil {
			c.currentSegment = nil
			return DecodeResultFailure, err
		}
		if res == DecodeResultEndReached {
			c.currentSegment = nil
			return DecodeResultSuccess, nil
		}
		if c.processing == CodecStatusToBeContinued {
			return DecodeResultSuccess, nil
		}

		if c.currentSegment.DataLength != 0xffffffff {
			if c.currentSegment.DataLength > math.MaxUint32-c.offset {
				c.currentSegment = nil
				return DecodeResultFailure, errors.New("jbig2: segment offset overflow")
			}
			c.offset += c.currentSegment.DataLength
			c.stream.SetOffset(c.offset)
		} else {
			c.stream.AddOffset(4)
		}
		c.segments = append(c.segments, c.currentSegment)
		c.currentSegment = nil
		if c.stream.BytesLeft() > 0 && c.page != nil && pause != nil && pause.ShouldPause() {
			c.processing = CodecStatusToBeContinued
			return DecodeResultSuccess, nil
		}
	}

	return DecodeResultSuccess, nil
}

func (c *Context) decodeGlobals(pause PauseIndicator) error {
	if c.globalContext == nil {
		return nil
	}
	if _, err := c.globalContext.DecodeSequential(pause); err != nil {
		c.processing = CodecStatusError
		return err
	}
	return nil
}

func (c *Context) parseSegmentHeader(seg *Segment) error {
	if c.stream == nil {
		return errors.New("jbig2: nil bitstream")
	}
	number, err := c.stream.ReadUint32()
	if err != nil {
		return err
	}
	seg.Number = number
	flagByte, err := c.stream.ReadByte()
	if err != nil {
		return err
	}
	seg.Flags = SegmentFlags(flagByte)
	count, err := c.readReferredSegmentCount(seg)
	if err != nil {
		return err
	}
	seg.ReferredToSegmentCount = int32(count)
	seg.ReferredToSegmentNumbers = make([]uint32, count)
	sizeBytes := c.segmentNumberSize(seg.Number)
	for i := 0; i < count; i++ {
		var ref uint32
		switch sizeBytes {
		case 1:
			b, err := c.stream.ReadByte()
			if err != nil {
				return err
			}
			ref = uint32(b)
		case 2:
			val, err := c.stream.ReadUint16()
			if err != nil {
				return err
			}
			ref = uint32(val)
		default:
			val, err := c.stream.ReadUint32()
			if err != nil {
				return err
			}
			ref = val
		}
		if ref >= seg.Number {
			return errors.New("jbig2: invalid referred segment number")
		}
		seg.ReferredToSegmentNumbers[i] = ref
	}
	var page uint32
	if seg.Flags.HasLongPageAssociation() {
		page, err = c.stream.ReadUint32()
	} else {
		var b byte
		b, err = c.stream.ReadByte()
		page = uint32(b)
	}
	if err != nil {
		return err
	}
	seg.PageAssociation = page
	length, err := c.stream.ReadUint32()
	if err != nil {
		return err
	}
	seg.DataLength = length
	seg.Key = c.stream.Key()
	seg.DataOffset = c.stream.Offset()
	seg.State = SegmentStateDataUnparsed
	return nil
}

func (c *Context) readReferredSegmentCount(seg *Segment) (int, error) {
	cur := c.stream.CurByte()
	if cur>>5 == 7 {
		val, err := c.stream.ReadUint32()
		if err != nil {
			return 0, err
		}
		count := int(val & 0x1fffffff)
		if count > int(JBig2MaxReferredSegmentCount) {
			return 0, errors.New("jbig2: referred segment count out of range")
		}
		return count, nil
	}
	_, err := c.stream.ReadByte()
	if err != nil {
		return 0, err
	}
	return int(cur >> 5), nil
}

func (c *Context) segmentNumberSize(number uint32) int {
	if number > 65536 {
		return 4
	}
	if number > 256 {
		return 2
	}
	return 1
}

func (c *Context) parseSegmentData(seg *Segment, pause PauseIndicator) (DecodeResult, error) {
	switch seg.Flags.Type() {
	case segmentTypeSymbolDict:
		return c.parseSymbolDictSegment(seg, pause)
	case segmentTypePatternDict:
		return c.parsePatternDictSegment(seg, pause)
	case segmentTypeTextRegionImmediate, segmentTypeTextRegionImmediateLossless, segmentTypeTextRegionRefine:
		if !c.inPage {
			return DecodeResultFailure, errors.New("jbig2: text region outside page context")
		}
		return c.parseTextRegionSegment(seg, pause)
	case segmentTypeRefinementRegion, segmentTypeRefinementRegionImmediate, segmentTypeRefinementRegionImmediateLossless:
		if !c.inPage {
			return DecodeResultFailure, errors.New("jbig2: refinement region outside page context")
		}
		return c.parseRefinementRegionSegment(seg, pause)
	case segmentTypeGenericRegion, segmentTypeGenericRegionImmediate, segmentTypeGenericRegionImmediateLossless:
		if !c.inPage {
			return DecodeResultFailure, errors.New("jbig2: generic region outside page context")
		}
		return c.parseGenericRegionSegment(seg, pause)
	case segmentTypePageInfo:
		return c.parsePageInfoSegment()
	case segmentTypeEndOfPage:
		c.inPage = false
		return DecodeResultEndReached, nil
	case segmentTypeEndOfStripe:
		if seg.DataLength != 0 {
			c.stream.AddOffset(seg.DataLength)
		}
		return DecodeResultSuccess, nil
	case segmentTypeEndOfFile:
		return DecodeResultEndReached, nil
	case segmentTypeHalftoneRegion, segmentTypeHalftoneRegionImmediate, segmentTypeHalftoneRegionImmediateLossless:
		if !c.inPage {
			return DecodeResultFailure, errors.New("jbig2: halftone region outside page context")
		}
		return c.parseHalftoneRegionSegment(seg, pause)
	case segmentTypeTables:
		return c.parseTablesSegment(seg)
	default:
		// For unknown segment types, log and skip the segment data
		segmentType := seg.Flags.Type()
		fmt.Printf("Warning: Unknown segment type %d encountered, skipping\n", segmentType)
		if seg.DataLength > 0 {
			// Skip the segment data
			skipBytes := int(seg.DataLength)
			if c.stream.BytesLeft() < uint32(skipBytes) {
				skipBytes = int(c.stream.BytesLeft())
			}
			for i := 0; i < skipBytes; i++ {
				c.stream.ReadByte()
			}
		}
		return DecodeResultSuccess, nil
	}
}

func (c *Context) parsePageInfoSegment() (DecodeResult, error) {
	width, err := c.stream.ReadUint32()
	if err != nil {
		return DecodeResultFailure, err
	}
	height, err := c.stream.ReadUint32()
	if err != nil {
		return DecodeResultFailure, err
	}
	resX, err := c.stream.ReadUint32()
	if err != nil {
		return DecodeResultFailure, err
	}
	resY, err := c.stream.ReadUint32()
	if err != nil {
		return DecodeResultFailure, err
	}
	flags, err := c.stream.ReadByte()
	if err != nil {
		return DecodeResultFailure, err
	}
	strip, err := c.stream.ReadUint16()
	if err != nil {
		return DecodeResultFailure, err
	}
	info := &PageInfo{
		Width:             width,
		Height:            height,
		ResolutionX:       resX,
		ResolutionY:       resY,
		DefaultPixelValue: flags&4 != 0,
		Striped:           strip&0x8000 != 0,
		MaxStripeSize:     strip & 0x7fff,
	}
	c.pageInfos = append(c.pageInfos, info)
	if !c.bufSpecified {
		heightToAlloc := info.Height
		if info.Height == 0xffffffff {
			heightToAlloc = uint32(info.MaxStripeSize)
		}
		c.page = NewImage(int32(info.Width), int32(heightToAlloc))
	}
	if c.page == nil || c.page.data == nil {
		c.processing = CodecStatusError
		return DecodeResultFailure, errors.New("jbig2: failed to allocate page image")
	}
	c.page.Fill(info.DefaultPixelValue)
	c.inPage = true
	return DecodeResultSuccess, nil
}

func (c *Context) parseSymbolDictSegment(seg *Segment, pause PauseIndicator) (DecodeResult, error) {
	flags, err := c.stream.ReadUint16()
	if err != nil {
		return DecodeResultFailure, err
	}
	proc := NewSDDProc()
	proc.SDHUFF = flags&0x0001 != 0
	proc.SDREFAGG = flags>>1&0x0001 != 0
	proc.SDTEMPLATE = uint8((flags >> 10) & 0x0003)
	proc.SDRTEMPLATE = flags>>12&0x0003 != 0

	if !proc.SDHUFF {
		iLimit := uint32(2)
		if proc.SDTEMPLATE == 0 {
			iLimit = 8
		}
		for i := uint32(0); i < iLimit; i++ {
			b, err := c.stream.ReadByte()
			if err != nil {
				return DecodeResultFailure, err
			}
			proc.SDAT[i] = int8(b)
		}
	}
	if proc.SDREFAGG && !proc.SDRTEMPLATE {
		for i := 0; i < 4; i++ {
			b, err := c.stream.ReadByte()
			if err != nil {
				return DecodeResultFailure, err
			}
			proc.SDRAT[i] = int8(b)
		}
	}
	var numEx, numNew uint32
	if numEx, err = c.stream.ReadUint32(); err != nil {
		return DecodeResultFailure, err
	}
	if numNew, err = c.stream.ReadUint32(); err != nil {
		return DecodeResultFailure, err
	}
	if numEx > uint32(JBig2MaxExportSymbols) || numNew > uint32(JBig2MaxNewSymbols) {
		return DecodeResultFailure, errors.New("jbig2: symbol dictionary limits exceeded")
	}
	proc.SDNUMEXSYMS = numEx
	proc.SDNUMNEWSYMS = numNew

	existing, lastDict, err := c.collectReferencedSymbols(seg)
	if err != nil {
		return DecodeResultFailure, err
	}
	proc.SDINSYMS = existing
	proc.SDNUMINSYMS = uint32(len(existing))

	if err := c.configureSymbolDictHuffman(flags, proc, seg); err != nil {
		return DecodeResultFailure, err
	}

	useGbContext := !proc.SDHUFF
	useGrContext := proc.SDREFAGG
	var gbContexts, grContexts []ArithContext

	key := CompoundKey{StreamKey: seg.Key, Segment: seg.DataOffset}
	seg.ResultType = ResultTypeSymbolDict
	if c.isGlobal && key.StreamKey != 0 {
		if cached, ok := c.LookupSymbolDict(key); ok && cached != nil {
			seg.SymbolDict = cached.DeepCopy()
			return DecodeResultSuccess, nil
		}
	}

	if flags&0x0100 != 0 {
		if lastDict == nil {
			return DecodeResultFailure, errors.New("jbig2: missing last symbol dictionary for context reuse")
		}
		if useGbContext {
			gbContexts = append([]ArithContext(nil), lastDict.GbContexts()...)
			expected := huffContextSize(proc.SDTEMPLATE)
			if len(gbContexts) != expected {
				return DecodeResultFailure, fmt.Errorf("jbig2: unexpected generic context size %d, want %d", len(gbContexts), expected)
			}
		}
		if useGrContext {
			grContexts = append([]ArithContext(nil), lastDict.GrContexts()...)
			expected := refAggContextSize(proc.SDRTEMPLATE)
			if len(grContexts) != expected {
				return DecodeResultFailure, fmt.Errorf("jbig2: unexpected refinement context size %d, want %d", len(grContexts), expected)
			}
		}
	} else {
		if useGbContext {
			gbContexts = make([]ArithContext, huffContextSize(proc.SDTEMPLATE))
		}
		if useGrContext {
			grContexts = make([]ArithContext, refAggContextSize(proc.SDRTEMPLATE))
		}
	}

	var dict *SymbolDict
	if proc.SDHUFF {
		dict, err = proc.DecodeHuffman(c.stream, gbContexts, grContexts)
		if err == nil {
			c.stream.AlignByte()
		}
	} else {
		procCtx := c.ensureArithDecoder()
		dict, err = proc.DecodeArith(procCtx, gbContexts, grContexts)
		if err == nil {
			c.stream.AlignByte()
			c.stream.AddOffset(2)
		}
	}
	if err != nil || dict == nil {
		return DecodeResultFailure, errors.New("jbig2: failed to decode symbol dictionary")
	}
	if flags&0x0200 != 0 {
		if useGbContext {
			dict.SetGbContexts(gbContexts)
		}
		if useGrContext {
			dict.SetGrContexts(grContexts)
		}
	}
	seg.SymbolDict = dict
	if c.isGlobal && key.StreamKey != 0 {
		c.StoreSymbolDict(key, dict.DeepCopy())
	}
	return DecodeResultSuccess, nil
}

func (c *Context) parsePatternDictSegment(seg *Segment, pause PauseIndicator) (DecodeResult, error) {
	flagByte, err := c.stream.ReadByte()
	if err != nil {
		return DecodeResultFailure, err
	}
	widthByte, err := c.stream.ReadByte()
	if err != nil {
		return DecodeResultFailure, err
	}
	heightByte, err := c.stream.ReadByte()
	if err != nil {
		return DecodeResultFailure, err
	}
	grayMax, err := c.stream.ReadUint32()
	if err != nil {
		return DecodeResultFailure, err
	}
	if grayMax > JBig2MaxPatternIndex {
		return DecodeResultFailure, errors.New("jbig2: pattern dictionary size too large")
	}

	proc := NewPDDProc()
	proc.HDMMR = flagByte&0x01 != 0
	proc.HDTemplate = uint8((flagByte >> 1) & 0x03)
	proc.HDPW = widthByte
	proc.HDPH = heightByte
	proc.GrayMax = grayMax

	var dict *PatternDict
	if proc.HDMMR {
		dict, err = proc.DecodeMMR(c.stream)
		if err == nil {
			c.stream.AlignByte()
		}
	} else {
		contexts := make([]ArithContext, huffContextSize(proc.HDTemplate))
		decoder := c.ensureArithDecoder()
		dict, err = proc.DecodeArith(decoder, contexts, pause)
		if err == nil {
			c.stream.AlignByte()
			c.stream.AddOffset(2)
		}
	}

	if err != nil || dict == nil {
		if err == nil {
			err = errors.New("jbig2: failed to decode pattern dictionary")
		}
		return DecodeResultFailure, err
	}

	seg.ResultType = ResultTypePatternDict
	seg.PatternDict = dict
	return DecodeResultSuccess, nil
}

func (c *Context) parseGenericRegionSegment(seg *Segment, pause PauseIndicator) (DecodeResult, error) {
	if c.grdProc == nil {
		if err := c.parseRegionInfo(&c.ri); err != nil {
			return DecodeResultFailure, err
		}
		if !IsValidImageSize(c.ri.Width, c.ri.Height) {
			return DecodeResultFailure, errors.New("jbig2: invalid generic region dimensions")
		}

		flagByte, err := c.stream.ReadByte()
		if err != nil {
			return DecodeResultFailure, err
		}

		proc := NewGRDProc()
		proc.MMR = flagByte&0x01 != 0
		proc.GBTemplate = uint8((flagByte >> 1) & 0x03)
		proc.TPGDON = flagByte&0x08 != 0
		proc.UseSkip = flagByte&0x10 != 0
		proc.GBWidth = uint32(c.ri.Width)
		proc.GBHeight = uint32(c.ri.Height)

		if !proc.MMR {
			iLimit := uint32(2)
			if proc.GBTemplate == 0 {
				iLimit = 8
			}
			for i := uint32(0); i < iLimit; i++ {
				b, err := c.stream.ReadByte()
				if err != nil {
					return DecodeResultFailure, err
				}
				proc.GBAt[i] = int32(int8(b))
			}
		}

		if proc.UseSkip {
			skipWidth, err := c.stream.ReadUint32()
			if err != nil {
				return DecodeResultFailure, err
			}
			skipHeight, err := c.stream.ReadUint32()
			if err != nil {
				return DecodeResultFailure, err
			}
			if !IsValidImageSize(int32(skipWidth), int32(skipHeight)) {
				return DecodeResultFailure, errors.New("jbig2: invalid skip image dimensions")
			}
			proc.Skip = NewImage(int32(skipWidth), int32(skipHeight))
			if proc.Skip == nil || proc.Skip.data == nil {
				return DecodeResultFailure, errors.New("jbig2: failed to allocate skip image")
			}
			stride := (skipWidth + 7) >> 3
			for row := uint32(0); row < skipHeight; row++ {
				line := proc.Skip.lineUnsafe(int(row))
				src := c.stream.Pointer()
				copy(line[:stride], src[:stride])
				c.stream.AddOffset(stride)
			}
		}

		c.grdProc = proc
		seg.ResultType = ResultTypeImage
	}

	proc := c.grdProc
	if proc == nil {
		return DecodeResultFailure, errors.New("jbig2: missing generic region state")
	}

	seg.ResultType = ResultTypeImage

	if proc.MMR {
		if seg.Image == nil {
			status, err := proc.StartDecodeMMR(&seg.Image, c.stream)
			if err != nil || status != CodecStatusFinished {
				return DecodeResultFailure, errors.New("jbig2: failed to decode MMR generic region")
			}
			c.stream.AlignByte()
		}
		if seg.Image == nil {
			return DecodeResultFailure, errors.New("jbig2: failed to decode generic region")
		}
		rect := proc.ReplaceRect()
		if rect.Width() <= 0 || rect.Height() <= 0 {
			rect = Rect{Left: 0, Top: 0, Right: seg.Image.Width(), Bottom: seg.Image.Height()}
		}
		if seg.Flags.Type() != segmentTypeGenericRegion {
			if err := c.composeRegion(c.ri, seg.Image, &rect); err != nil {
				c.processing = CodecStatusError
				return DecodeResultFailure, err
			}
			seg.Image = nil
		}
		c.grdProc = nil
		return DecodeResultSuccess, nil
	}

	if len(c.gbContexts) == 0 {
		c.gbContexts = make([]ArithContext, huffContextSize(proc.GBTemplate))
	}
	if c.arithDecoder == nil {
		c.arithDecoder = c.ensureArithDecoder()
	}

	state := &GRDProgressiveState{
		Image:    &seg.Image,
		Decoder:  c.arithDecoder,
		Contexts: c.gbContexts,
		Pause:    pause,
	}

	var (
		status CodecStatus
		err    error
	)
	if seg.Image == nil {
		status, err = proc.StartDecodeArith(state)
	} else {
		status, err = proc.ContinueDecode(state)
	}
	if err != nil {
		c.processing = CodecStatusError
		return DecodeResultFailure, err
	}
	if seg.Image == nil {
		return DecodeResultFailure, errors.New("jbig2: failed to decode generic region")
	}

	rect := proc.ReplaceRect()
	if rect.Width() <= 0 || rect.Height() <= 0 {
		rect = Rect{Left: 0, Top: 0, Right: seg.Image.Width(), Bottom: seg.Image.Height()}
	}

	switch status {
	case CodecStatusToBeContinued:
		c.processing = CodecStatusToBeContinued
		if seg.Flags.Type() != segmentTypeGenericRegion {
			if err := c.composeRegion(c.ri, seg.Image, &rect); err != nil {
				c.processing = CodecStatusError
				return DecodeResultFailure, err
			}
		}
		return DecodeResultSuccess, nil
	case CodecStatusFinished:
		c.stream.AlignByte()
		c.stream.AddOffset(2)
		if seg.Flags.Type() != segmentTypeGenericRegion {
			if err := c.composeRegion(c.ri, seg.Image, &rect); err != nil {
				c.processing = CodecStatusError
				return DecodeResultFailure, err
			}
			seg.Image = nil
		}
		c.grdProc = nil
		c.gbContexts = nil
		c.arithDecoder = nil
		return DecodeResultSuccess, nil
	case CodecStatusError:
		c.processing = CodecStatusError
		return DecodeResultFailure, errors.New("jbig2: generic region decode error")
	default:
		return DecodeResultSuccess, nil
	}
}

func (c *Context) parseRefinementRegionSegment(seg *Segment, pause PauseIndicator) (DecodeResult, error) {
	var ri RegionInfo
	if err := c.parseRegionInfo(&ri); err != nil {
		return DecodeResultFailure, err
	}
	if !IsValidImageSize(ri.Width, ri.Height) {
		return DecodeResultFailure, errors.New("jbig2: invalid refinement region dimensions")
	}

	flags, err := c.stream.ReadUint16()
	if err != nil {
		return DecodeResultFailure, err
	}

	proc := NewGRRDProc()
	proc.Template = flags&0x0001 != 0
	proc.TPGRON = flags&0x0002 != 0
	proc.Width = uint32(ri.Width)
	proc.Height = uint32(ri.Height)

	if !proc.Template {
		for i := 0; i < 4; i++ {
			b, err := c.stream.ReadByte()
			if err != nil {
				return DecodeResultFailure, err
			}
			proc.GRAT[i] = int8(b)
		}
	}

	var referenceSeg *Segment
	if seg.ReferredToSegmentCount > 0 {
		for _, ref := range seg.ReferredToSegmentNumbers {
			candidate := c.findSegmentByNumber(ref)
			if candidate == nil {
				return DecodeResultFailure, fmt.Errorf("jbig2: missing referenced segment %d", ref)
			}
			switch candidate.Flags.Type() {
			case segmentTypeTextRegionImmediate,
				segmentTypeHalftoneRegion,
				segmentTypeGenericRegion,
				segmentTypeRefinementRegion:
				referenceSeg = candidate
			}
			if referenceSeg != nil {
				break
			}
		}
		if referenceSeg == nil {
			return DecodeResultFailure, errors.New("jbig2: refinement region missing suitable referenced segment")
		}
		if referenceSeg.Image == nil {
			return DecodeResultFailure, errors.New("jbig2: referenced segment lacks image data for refinement region")
		}
		proc.Reference = referenceSeg.Image
	} else {
		if c.page == nil {
			return DecodeResultFailure, errors.New("jbig2: refinement region missing page image")
		}
		proc.Reference = c.page
	}
	proc.ReferenceDX = 0
	proc.ReferenceDY = 0

	var grContexts []ArithContext
	if c.grContexts != nil {
		grContexts = c.grContexts
	} else {
		grContexts = make([]ArithContext, refAggContextSize(proc.Template))
	}

	decoder := c.ensureArithDecoder()
	img, err := proc.Decode(decoder, grContexts)
	if err != nil {
		return DecodeResultFailure, err
	}
	if img == nil {
		return DecodeResultFailure, errors.New("jbig2: failed to decode refinement region")
	}

	c.stream.AlignByte()
	c.stream.AddOffset(2)

	seg.ResultType = ResultTypeImage
	seg.Image = img
	if seg.Flags.Type() != segmentTypeRefinementRegion {
		if err := c.composeRegion(ri, img, nil); err != nil {
			return DecodeResultFailure, err
		}
		seg.Image = nil
	}
	return DecodeResultSuccess, nil
}

func (c *Context) parseHalftoneRegionSegment(seg *Segment, pause PauseIndicator) (DecodeResult, error) {
	var ri RegionInfo
	if err := c.parseRegionInfo(&ri); err != nil {
		return DecodeResultFailure, err
	}
	if !IsValidImageSize(ri.Width, ri.Height) {
		return DecodeResultFailure, errors.New("jbig2: invalid halftone region dimensions")
	}

	flags, err := c.stream.ReadUint16()
	if err != nil {
		return DecodeResultFailure, err
	}

	proc := NewHTRDProc()
	proc.HMMR = flags&0x0001 != 0
	proc.HTemplate = uint8((flags >> 1) & 0x0003)
	proc.HEnableSkip = flags&0x0008 != 0
	combOp := (flags >> 4) & 0x0007
	if combOp > uint16(ComposeReplace) {
		return DecodeResultFailure, fmt.Errorf("jbig2: unsupported halftone compose op %d", combOp)
	}
	proc.HCombOp = ComposeOp(combOp)
	proc.HDefPixel = flags&0x0080 != 0
	proc.HBWidth = uint32(ri.Width)
	proc.HBHeight = uint32(ri.Height)

	// Read halftone parameters
	proc.HGWidth, err = c.stream.ReadUint32()
	if err != nil {
		return DecodeResultFailure, err
	}
	proc.HGHeight, err = c.stream.ReadUint32()
	if err != nil {
		return DecodeResultFailure, err
	}
	if !IsValidImageSize(int32(proc.HGWidth), int32(proc.HGHeight)) {
		return DecodeResultFailure, errors.New("jbig2: invalid halftone grid dimensions")
	}

	hgx, err := c.stream.ReadUint32()
	if err != nil {
		return DecodeResultFailure, err
	}
	hgy, err := c.stream.ReadUint32()
	if err != nil {
		return DecodeResultFailure, err
	}
	proc.HGX = int32(hgx)
	proc.HGY = int32(hgy)

	proc.HRX, err = c.stream.ReadUint16()
	if err != nil {
		return DecodeResultFailure, err
	}
	proc.HRY, err = c.stream.ReadUint16()
	if err != nil {
		return DecodeResultFailure, err
	}

	if seg.ReferredToSegmentCount != 1 {
		return DecodeResultFailure, errors.New("jbig2: halftone region requires one pattern dictionary reference")
	}

	patternSeg := c.findSegmentByNumber(seg.ReferredToSegmentNumbers[0])
	if patternSeg == nil {
		return DecodeResultFailure, fmt.Errorf("jbig2: missing pattern dictionary segment %d", seg.ReferredToSegmentNumbers[0])
	}
	if patternSeg.PatternDict == nil {
		return DecodeResultFailure, fmt.Errorf("jbig2: pattern dictionary segment %d lacks pattern dictionary", seg.ReferredToSegmentNumbers[0])
	}

	if patternSeg.PatternDict.NumPatterns == 0 {
		return DecodeResultFailure, errors.New("jbig2: halftone region pattern dictionary is empty")
	}
	proc.HNumPats = patternSeg.PatternDict.NumPatterns
	proc.HPats = make([]*Image, proc.HNumPats)
	for i := uint32(0); i < proc.HNumPats; i++ {
		proc.HPats[i] = patternSeg.PatternDict.GetPattern(i)
	}
	firstPattern := proc.HPats[0]
	if firstPattern == nil {
		return DecodeResultFailure, errors.New("jbig2: halftone pattern dictionary missing base pattern")
	}
	if firstPattern.Width() > 255 || firstPattern.Height() > 255 {
		return DecodeResultFailure, errors.New("jbig2: halftone pattern dimensions exceed 8-bit limits")
	}
	proc.HPW = uint8(firstPattern.Width())
	proc.HPH = uint8(firstPattern.Height())

	var img *Image
	if proc.HMMR {
		img, err = proc.DecodeMMR(c.stream)
		if err == nil {
			c.stream.AlignByte()
		}
	} else {
		decoder := c.ensureArithDecoder()
		contexts := make([]ArithContext, huffContextSize(proc.HTemplate))
		img, err = proc.DecodeArith(decoder, contexts, pause)
		if err == nil {
			c.stream.AlignByte()
			c.stream.AddOffset(2)
		}
	}

	if err != nil || img == nil {
		return DecodeResultFailure, errors.New("jbig2: failed to decode halftone region")
	}

	seg.ResultType = ResultTypeImage
	seg.Image = img
	if seg.Flags.Type() != segmentTypeHalftoneRegion {
		if err := c.composeRegion(ri, img, nil); err != nil {
			return DecodeResultFailure, err
		}
		seg.Image = nil
	}
	return DecodeResultSuccess, nil
}

func (c *Context) parseTextRegionSegment(seg *Segment, pause PauseIndicator) (DecodeResult, error) {
	var ri RegionInfo
	if err := c.parseRegionInfo(&ri); err != nil {
		return DecodeResultFailure, err
	}
	if !IsValidImageSize(ri.Width, ri.Height) {
		return DecodeResultFailure, errors.New("jbig2: invalid text region dimensions")
	}

	flags, err := c.stream.ReadUint16()
	if err != nil {
		return DecodeResultFailure, err
	}

	proc := NewTRDProc()
	proc.SBWidth = uint32(ri.Width)
	proc.SBHeight = uint32(ri.Height)
	proc.SBHUFF = flags&0x0001 != 0
	proc.SBREFINE = flags&0x0002 != 0
	proc.SBStrips = 1 << ((flags >> 2) & 0x0003)
	if proc.SBStrips == 0 {
		proc.SBStrips = 1
	}
	proc.RefCorner = JBig2Corner((flags >> 4) & 0x0003)
	proc.Transposed = flags&0x0040 != 0
	proc.SBCombOp = ComposeOp((flags >> 7) & 0x0003)
	proc.SBDefPixel = flags&0x0200 != 0
	dsOffset := int8((flags >> 10) & 0x001f)
	if dsOffset >= 16 {
		dsOffset -= 32
	}
	proc.SBDSOffset = dsOffset
	proc.SBRTEMPLATE = flags&0x8000 != 0

	var huffFlags uint16
	if proc.SBHUFF {
		huffFlags, err = c.stream.ReadUint16()
		if err != nil {
			return DecodeResultFailure, err
		}
	}
	if proc.SBREFINE && !proc.SBRTEMPLATE {
		for i := 0; i < 4; i++ {
			b, err := c.stream.ReadByte()
			if err != nil {
				return DecodeResultFailure, err
			}
			proc.SBRAT[i] = int8(b)
		}
	}

	instances, err := c.stream.ReadUint32()
	if err != nil {
		return DecodeResultFailure, err
	}
	proc.SBNumInstances = instances

	symbols, _, err := c.collectReferencedSymbols(seg)
	if err != nil {
		return DecodeResultFailure, err
	}
	proc.SBSyms = symbols
	proc.SBNumSyms = uint32(len(symbols))

	if proc.SBHUFF {
		codes, err := c.decodeSymbolIDHuffmanTable(proc.SBNumSyms)
		if err != nil {
			return DecodeResultFailure, err
		}
		proc.SBSymCodes = codes
		c.stream.AlignByte()

		cSBHUFFFS := huffFlags & 0x0003
		cSBHUFFDS := (huffFlags >> 2) & 0x0003
		cSBHUFFDT := (huffFlags >> 4) & 0x0003
		cSBHUFFRDW := (huffFlags >> 6) & 0x0003
		cSBHUFFRDH := (huffFlags >> 8) & 0x0003
		cSBHUFFRDX := (huffFlags >> 10) & 0x0003
		cSBHUFFRDY := (huffFlags >> 12) & 0x0003
		cSBHUFFRSIZE := (huffFlags >> 14) & 0x0001
		if cSBHUFFFS == 2 || cSBHUFFRDW == 2 || cSBHUFFRDH == 2 || cSBHUFFRDX == 2 || cSBHUFFRDY == 2 {
			return DecodeResultFailure, errors.New("jbig2: unsupported text region Huffman selector")
		}

		index := 0
		getTable := func(idx int) (*HuffmanTable, error) {
			return c.getHuffmanTable(idx)
		}

		switch cSBHUFFFS {
		case 0:
			table, err := getTable(6)
			if err != nil {
				return DecodeResultFailure, err
			}
			proc.SBHUFFFS = table
		case 1:
			table, err := getTable(7)
			if err != nil {
				return DecodeResultFailure, err
			}
			proc.SBHUFFFS = table
		default:
			ref := c.findReferredTableSegmentByIndex(seg, index)
			if ref == nil || ref.HuffmanTable == nil {
				return DecodeResultFailure, errors.New("jbig2: missing referenced Huffman table for SBHUFFFS")
			}
			proc.SBHUFFFS = ref.HuffmanTable
			index++
		}

		switch cSBHUFFDS {
		case 0:
			table, err := getTable(8)
			if err != nil {
				return DecodeResultFailure, err
			}
			proc.SBHUFFDS = table
		case 1:
			table, err := getTable(9)
			if err != nil {
				return DecodeResultFailure, err
			}
			proc.SBHUFFDS = table
		case 2:
			table, err := getTable(10)
			if err != nil {
				return DecodeResultFailure, err
			}
			proc.SBHUFFDS = table
		default:
			ref := c.findReferredTableSegmentByIndex(seg, index)
			if ref == nil || ref.HuffmanTable == nil {
				return DecodeResultFailure, errors.New("jbig2: missing referenced Huffman table for SBHUFFDS")
			}
			proc.SBHUFFDS = ref.HuffmanTable
			index++
		}

		switch cSBHUFFDT {
		case 0:
			table, err := getTable(11)
			if err != nil {
				return DecodeResultFailure, err
			}
			proc.SBHUFFDT = table
		case 1:
			table, err := getTable(12)
			if err != nil {
				return DecodeResultFailure, err
			}
			proc.SBHUFFDT = table
		case 2:
			table, err := getTable(13)
			if err != nil {
				return DecodeResultFailure, err
			}
			proc.SBHUFFDT = table
		default:
			ref := c.findReferredTableSegmentByIndex(seg, index)
			if ref == nil || ref.HuffmanTable == nil {
				return DecodeResultFailure, errors.New("jbig2: missing referenced Huffman table for SBHUFFDT")
			}
			proc.SBHUFFDT = ref.HuffmanTable
			index++
		}

		switch cSBHUFFRDW {
		case 0:
			table, err := getTable(14)
			if err != nil {
				return DecodeResultFailure, err
			}
			proc.SBHUFFRDW = table
		case 1:
			table, err := getTable(15)
			if err != nil {
				return DecodeResultFailure, err
			}
			proc.SBHUFFRDW = table
		default:
			ref := c.findReferredTableSegmentByIndex(seg, index)
			if ref == nil || ref.HuffmanTable == nil {
				return DecodeResultFailure, errors.New("jbig2: missing referenced Huffman table for SBHUFFRDW")
			}
			proc.SBHUFFRDW = ref.HuffmanTable
			index++
		}

		switch cSBHUFFRDH {
		case 0:
			table, err := getTable(14)
			if err != nil {
				return DecodeResultFailure, err
			}
			proc.SBHUFFRDH = table
		case 1:
			table, err := getTable(15)
			if err != nil {
				return DecodeResultFailure, err
			}
			proc.SBHUFFRDH = table
		default:
			ref := c.findReferredTableSegmentByIndex(seg, index)
			if ref == nil || ref.HuffmanTable == nil {
				return DecodeResultFailure, errors.New("jbig2: missing referenced Huffman table for SBHUFFRDH")
			}
			proc.SBHUFFRDH = ref.HuffmanTable
			index++
		}

		switch cSBHUFFRDX {
		case 0:
			table, err := getTable(14)
			if err != nil {
				return DecodeResultFailure, err
			}
			proc.SBHUFFRDX = table
		case 1:
			table, err := getTable(15)
			if err != nil {
				return DecodeResultFailure, err
			}
			proc.SBHUFFRDX = table
		default:
			ref := c.findReferredTableSegmentByIndex(seg, index)
			if ref == nil || ref.HuffmanTable == nil {
				return DecodeResultFailure, errors.New("jbig2: missing referenced Huffman table for SBHUFFRDX")
			}
			proc.SBHUFFRDX = ref.HuffmanTable
			index++
		}

		switch cSBHUFFRDY {
		case 0:
			table, err := getTable(14)
			if err != nil {
				return DecodeResultFailure, err
			}
			proc.SBHUFFRDY = table
		case 1:
			table, err := getTable(15)
			if err != nil {
				return DecodeResultFailure, err
			}
			proc.SBHUFFRDY = table
		default:
			ref := c.findReferredTableSegmentByIndex(seg, index)
			if ref == nil || ref.HuffmanTable == nil {
				return DecodeResultFailure, errors.New("jbig2: missing referenced Huffman table for SBHUFFRDY")
			}
			proc.SBHUFFRDY = ref.HuffmanTable
			index++
		}

		if cSBHUFFRSIZE == 0 {
			table, err := getTable(1)
			if err != nil {
				return DecodeResultFailure, err
			}
			proc.SBHUFFRSize = table
		} else {
			ref := c.findReferredTableSegmentByIndex(seg, index)
			if ref == nil || ref.HuffmanTable == nil {
				return DecodeResultFailure, errors.New("jbig2: missing referenced Huffman table for SBHUFFRSIZE")
			}
			proc.SBHUFFRSize = ref.HuffmanTable
		}
	} else {
		var codeLen uint8
		for (uint32(1) << codeLen) < proc.SBNumSyms {
			codeLen++
		}
		proc.SBSymCodeLen = codeLen
	}

	if proc.SBNumInstances > uint32(len(c.stream.Buf()))*32 {
		return DecodeResultFailure, errors.New("jbig2: text region instances exceed stream limit")
	}

	var grContexts []ArithContext
	if proc.SBREFINE {
		grContexts = make([]ArithContext, refAggContextSize(proc.SBRTEMPLATE))
	}

	var img *Image
	if proc.SBHUFF {
		img, err = proc.DecodeHuffman(c.stream, grContexts)
		if err == nil {
			c.stream.AlignByte()
		}
	} else {
		decoder := c.ensureArithDecoder()
		var ids *IntDecoderState
		if proc.SBREFINE {
			ids = &IntDecoderState{
				IADT:  NewArithIntDecoder(),
				IAFS:  NewArithIntDecoder(),
				IADS:  NewArithIntDecoder(),
				IAIT:  NewArithIntDecoder(),
				IARI:  NewArithIntDecoder(),
				IARDW: NewArithIntDecoder(),
				IARDH: NewArithIntDecoder(),
				IARDX: NewArithIntDecoder(),
				IARDY: NewArithIntDecoder(),
				IAID:  NewArithIaidDecoder(proc.SBSymCodeLen),
			}
		}
		img, err = proc.DecodeArith(decoder, grContexts, ids)
		if err == nil {
			c.stream.AlignByte()
			c.stream.AddOffset(2)
		}
	}

	if err != nil || img == nil {
		return DecodeResultFailure, errors.New("jbig2: failed to decode text region")
	}

	seg.ResultType = ResultTypeImage
	seg.Image = img
	if seg.Flags.Type() != segmentTypeTextRegionImmediate {
		if err := c.composeRegion(ri, img, nil); err != nil {
			return DecodeResultFailure, err
		}
		seg.Image = nil
	}
	return DecodeResultSuccess, nil
}

func (c *Context) parseRegionInfo(ri *RegionInfo) error {
	if ri == nil {
		return errors.New("jbig2: nil region info")
	}
	width, err := c.stream.ReadUint32()
	if err != nil {
		return err
	}
	height, err := c.stream.ReadUint32()
	if err != nil {
		return err
	}
	x, err := c.stream.ReadUint32()
	if err != nil {
		return err
	}
	y, err := c.stream.ReadUint32()
	if err != nil {
		return err
	}
	flags, err := c.stream.ReadByte()
	if err != nil {
		return err
	}

	ri.Width = int32(width)
	ri.Height = int32(height)
	ri.X = int32(x)
	ri.Y = int32(y)
	ri.Flags = flags
	return nil
}

func (c *Context) decodeSymbolIDHuffmanTable(numSyms uint32) ([]HuffmanCode, error) {
	const runCodesSize = 35
	runCodes := make([]HuffmanCode, runCodesSize)
	for i := range runCodes {
		val, err := c.stream.ReadNBits(4)
		if err != nil {
			return nil, err
		}
		runCodes[i].CodeLength = int32(val)
	}
	if err := HuffmanAssignCode(runCodes); err != nil {
		return nil, err
	}

	codes := make([]HuffmanCode, numSyms)
	for i := 0; i < int(numSyms); {
		var value uint32
		var nBits uint32
		var runcode int
		for {
			bit, err := c.stream.Read1Bit()
			if err != nil {
				return nil, err
			}
			value = (value << 1) | bit
			nBits++
			matched := false
			for j := 0; j < runCodesSize; j++ {
				if int32(nBits) == runCodes[j].CodeLength && int32(value) == runCodes[j].Code {
					runcode = j
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}

		run := 0
		switch {
		case runcode < 32:
			codes[i].CodeLength = int32(runcode)
		case runcode == 32:
			val, err := c.stream.ReadNBits(2)
			if err != nil {
				return nil, err
			}
			run = int(val) + 3
		case runcode == 33:
			val, err := c.stream.ReadNBits(3)
			if err != nil {
				return nil, err
			}
			run = int(val) + 3
		case runcode == 34:
			val, err := c.stream.ReadNBits(7)
			if err != nil {
				return nil, err
			}
			run = int(val) + 11
		default:
			return nil, fmt.Errorf("jbig2: invalid symbol ID Huffman run code %d", runcode)
		}

		if run > 0 {
			if i+run > int(numSyms) {
				return nil, errors.New("jbig2: symbol ID run exceeds symbol count")
			}
			for k := 0; k < run; k++ {
				if runcode == 32 && i > 0 {
					codes[i+k].CodeLength = codes[i-1].CodeLength
				} else {
					codes[i+k].CodeLength = 0
				}
			}
			i += run
		} else {
			i++
		}
	}

	if err := HuffmanAssignCode(codes); err != nil {
		return nil, err
	}
	return codes, nil
}

func (c *Context) parseTablesSegment(seg *Segment) (DecodeResult, error) {
	table, err := NewHuffmanTableFromStream(c.stream)
	if err != nil {
		return DecodeResultFailure, err
	}
	c.stream.AlignByte()
	seg.ResultType = ResultTypeHuffmanTable
	seg.HuffmanTable = table
	return DecodeResultSuccess, nil
}

func (c *Context) collectReferencedSymbols(seg *Segment) ([]*Image, *SymbolDict, error) {
	if seg == nil || seg.ReferredToSegmentCount == 0 {
		return nil, nil, nil
	}
	var images []*Image
	var lastDict *SymbolDict
	for _, ref := range seg.ReferredToSegmentNumbers {
		referred := c.findSegmentByNumber(ref)
		if referred == nil {
			return nil, nil, fmt.Errorf("jbig2: missing referenced segment %d (from %d type %d)", ref, seg.Number, seg.Flags.Type())
		}
		if referred.SymbolDict != nil {
			lastDict = referred.SymbolDict
			for i := 0; i < referred.SymbolDict.NumImages(); i++ {
				images = append(images, referred.SymbolDict.GetImage(i))
			}
		}
	}
	return images, lastDict, nil
}

func (c *Context) configureSymbolDictHuffman(flags uint16, proc *SDDProc, seg *Segment) error {
	cSDHUFFDH := (flags >> 2) & 0x0003
	cSDHUFFDW := (flags >> 4) & 0x0003
	cSDHUFFBMSIZE := (flags >> 6) & 0x0001
	cSDHUFFAGGINST := (flags >> 7) & 0x0001
	index := 0

	getTable := func(idx int) (*HuffmanTable, error) {
		table, err := c.getHuffmanTable(idx)
		if err != nil {
			return nil, err
		}
		return table, nil
	}

	switch cSDHUFFDH {
	case 0:
		table, err := getTable(4)
		if err != nil {
			return err
		}
		proc.SDHUFFDH = table
	case 1:
		table, err := getTable(5)
		if err != nil {
			return err
		}
		proc.SDHUFFDH = table
	case 3:
		ref := c.findReferredTableSegmentByIndex(seg, index)
		if ref == nil || ref.HuffmanTable == nil {
			return fmt.Errorf("jbig2: missing referenced Huffman table for SDHUFFDH")
		}
		proc.SDHUFFDH = ref.HuffmanTable
		index++
	default:
		return fmt.Errorf("jbig2: unsupported SDHUFFDH value %d", cSDHUFFDH)
	}

	switch cSDHUFFDW {
	case 0:
		table, err := getTable(2)
		if err != nil {
			return err
		}
		proc.SDHUFFDW = table
	case 1:
		table, err := getTable(3)
		if err != nil {
			return err
		}
		proc.SDHUFFDW = table
	case 3:
		ref := c.findReferredTableSegmentByIndex(seg, index)
		if ref == nil || ref.HuffmanTable == nil {
			return fmt.Errorf("jbig2: missing referenced Huffman table for SDHUFFDW")
		}
		proc.SDHUFFDW = ref.HuffmanTable
		index++
	default:
		return fmt.Errorf("jbig2: unsupported SDHUFFDW value %d", cSDHUFFDW)
	}

	if cSDHUFFBMSIZE == 0 {
		table, err := getTable(1)
		if err != nil {
			return err
		}
		proc.SDHUFFBMSIZE = table
	} else {
		ref := c.findReferredTableSegmentByIndex(seg, index)
		if ref == nil || ref.HuffmanTable == nil {
			return fmt.Errorf("jbig2: missing referenced Huffman table for SDHUFFBMSIZE")
		}
		proc.SDHUFFBMSIZE = ref.HuffmanTable
		index++
	}

	if proc.SDREFAGG {
		if cSDHUFFAGGINST == 0 {
			table, err := getTable(1)
			if err != nil {
				return err
			}
			proc.SDHUFFAGGINST = table
		} else {
			ref := c.findReferredTableSegmentByIndex(seg, index)
			if ref == nil || ref.HuffmanTable == nil {
				return fmt.Errorf("jbig2: missing referenced Huffman table for SDHUFFAGGINST")
			}
			proc.SDHUFFAGGINST = ref.HuffmanTable
		}
	}
	return nil
}

func (c *Context) ensureArithDecoder() *ArithDecoder {
	// Always create a fresh decoder to mirror PDFium behaviour.
	return NewArithDecoder(c.stream)
}

func (c *Context) findSegmentByNumber(number uint32) *Segment {
	if c.globalContext != nil {
		if seg := c.globalContext.findSegmentByNumber(number); seg != nil {
			return seg
		}
	}
	for _, seg := range c.segments {
		if seg.Number == number {
			return seg
		}
	}
	return nil
}

func (c *Context) findReferredTableSegmentByIndex(seg *Segment, target int) *Segment {
	count := 0
	for _, ref := range seg.ReferredToSegmentNumbers {
		candidate := c.findSegmentByNumber(ref)
		if candidate != nil && candidate.Flags.Type() == segmentTypeTables {
			if count == target {
				return candidate
			}
			count++
		}
	}
	return nil
}

func (c *Context) getHuffmanTable(idx int) (*HuffmanTable, error) {
	if idx <= 0 || idx >= len(builtinHuffmanTables) {
		return nil, fmt.Errorf("jbig2: invalid standard Huffman table index %d", idx)
	}
	if c.huffmanTables[idx] == nil {
		tbl, err := NewStandardHuffmanTable(idx)
		if err != nil {
			return nil, err
		}
		c.huffmanTables[idx] = tbl
	}
	return c.huffmanTables[idx], nil
}

// GetFirstPage prepares the first output page, creating a page image backed by
// the provided buffer and invoking the decode loop.
func (c *Context) GetFirstPage(buf []byte, width, height, stride int, pause PauseIndicator) (bool, error) {
	if err := c.decodeGlobals(pause); err != nil {
		return false, err
	}

	c.pauseStep = 0
	img, err := NewImageFromBuffer(int32(width), int32(height), int32(stride), buf)
	if err != nil {
		c.processing = CodecStatusError
		return false, err
	}
	c.page = img
	c.bufSpecified = true

	if pause != nil && pause.ShouldPause() {
		c.pauseStep = 1
		c.processing = CodecStatusToBeContinued
		return true, nil
	}
	return c.Continue(pause)
}

// Continue resumes decoding after a pause request.
func (c *Context) Continue(pause PauseIndicator) (bool, error) {
	c.processing = CodecStatusReady
	_, err := c.DecodeSequential(pause)
	if err != nil {
		c.processing = CodecStatusError
		return false, err
	}
	if c.processing == CodecStatusToBeContinued {
		return true, nil
	}
	c.processing = CodecStatusFinished
	return true, nil
}

// ProcessingStatus reports the current codec status for the context.
func (c *Context) ProcessingStatus() CodecStatus {
	return c.processing
}

// PageImage returns the current page image, if any.
func (c *Context) PageImage() *Image {
	return c.page
}

// LookupSymbolDict attempts to find a cached symbol dictionary using the
// provided key. The cache acts as an LRU with capacity two, matching the
// reference implementation.
func (c *Context) LookupSymbolDict(key CompoundKey) (*SymbolDict, bool) {
	if c.cache == nil {
		return nil, false
	}
	entries := *c.cache
	for i, entry := range entries {
		if entry.Key == key {
			if i > 0 {
				entries = append([]CachePair{entry}, append(entries[:i], entries[i+1:]...)...)
				*c.cache = entries
			}
			return entry.Dict, true
		}
	}
	return nil, false
}

// StoreSymbolDict inserts or updates a symbol dictionary entry in the LRU cache.
func (c *Context) StoreSymbolDict(key CompoundKey, dict *SymbolDict) {
	if c.cache == nil || dict == nil {
		return
	}
	entries := *c.cache
	filtered := entries[:0]
	for _, entry := range entries {
		if entry.Key != key {
			filtered = append(filtered, entry)
		}
	}
	filtered = append([]CachePair{{Key: key, Dict: dict}}, filtered...)
	if len(filtered) > symbolDictCacheMaxSize {
		filtered = filtered[:symbolDictCacheMaxSize]
	}
	*c.cache = filtered
}

// CurrentSegment exposes the segment currently being processed.
func (c *Context) CurrentSegment() *Segment {
	return c.currentSegment
}

// SetCurrentSegment updates the active segment pointer.
func (c *Context) SetCurrentSegment(seg *Segment) {
	c.currentSegment = seg
}

// PushSegment appends a fully decoded segment to the context list.
func (c *Context) PushSegment(seg *Segment) {
	if seg != nil {
		c.segments = append(c.segments, seg)
	}
}

// Segments returns the list of decoded segments.
func (c *Context) Segments() []*Segment {
	return c.segments
}

// AddPageInfo records metadata about a decoded page segment.
func (c *Context) AddPageInfo(info *PageInfo) {
	if info != nil {
		c.pageInfos = append(c.pageInfos, info)
	}
}

// PageInfos exposes collected page information.
func (c *Context) PageInfos() []*PageInfo {
	return c.pageInfos
}

// Offset returns the current byte offset within the bitstream.
func (c *Context) Offset() uint32 {
	return c.offset
}

// SetOffset stores the current stream offset tracker.
func (c *Context) SetOffset(value uint32) {
	c.offset = value
}

// SetProcessingStatus forces a codec status during translation.
func (c *Context) SetProcessingStatus(status CodecStatus) {
	c.processing = status
}

func (c *Context) composeRegion(ri RegionInfo, img *Image, rect *Rect) error {
	if img == nil || img.data == nil {
		return errors.New("jbig2: compose requires a decoded image")
	}
	if c.page == nil || c.page.data == nil {
		return errors.New("jbig2: compose requires an active page image")
	}

	r := rect
	full := Rect{Left: 0, Top: 0, Right: img.Width(), Bottom: img.Height()}
	if r == nil || r.Width() <= 0 || r.Height() <= 0 {
		r = &full
	}
	if r.Width() <= 0 || r.Height() <= 0 {
		return nil
	}

	bottom := int(ri.Y) + r.Bottom
	c.ensurePageHeight(bottom)

	x := int64(ri.X) + int64(r.Left)
	y := int64(ri.Y) + int64(r.Top)
	op := ComposeOp(ri.Flags & 0x03)
	if !img.ComposeToWithRect(c.page, x, y, *r, op) {
		return errors.New("jbig2: failed to compose region")
	}
	return nil
}

func (c *Context) ensurePageHeight(target int) {
	if c.page == nil || target <= c.page.Height() {
		return
	}
	if target <= 0 || c.bufSpecified {
		return
	}
	info := c.latestPageInfo()
	if info == nil || !info.ShouldTreatAsStriped() {
		return
	}
	c.page.Expand(int32(target), info.DefaultPixelValue)
}

func (c *Context) latestPageInfo() *PageInfo {
	for i := len(c.pageInfos) - 1; i >= 0; i-- {
		if info := c.pageInfos[i]; info != nil {
			return info
		}
	}
	return nil
}
