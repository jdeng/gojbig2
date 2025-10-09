package jbig2

import (
	"errors"
	"fmt"
)

// SDDProc orchestrates symbol dictionary decoding for JBIG2 segments.
type SDDProc struct {
	SDHUFF        bool
	SDREFAGG      bool
	SDRTEMPLATE   bool
	SDTEMPLATE    uint8
	SDNUMINSYMS   uint32
	SDNUMNEWSYMS  uint32
	SDNUMEXSYMS   uint32
	SDINSYMS      []*Image
	SDHUFFDH      *HuffmanTable
	SDHUFFDW      *HuffmanTable
	SDHUFFBMSIZE  *HuffmanTable
	SDHUFFAGGINST *HuffmanTable
	SDAT          [8]int8
	SDRAT         [4]int8
}

// NewSDDProc constructs an empty symbol dictionary decoder configuration.
func NewSDDProc() *SDDProc { return &SDDProc{} }

// DecodeArith decodes the dictionary using arithmetic coding.
func (p *SDDProc) DecodeArith(decoder *ArithDecoder, gbContexts, grContexts []ArithContext) (*SymbolDict, error) {
	if decoder == nil {
		return nil, errors.New("jbig2: nil arithmetic decoder for symbol dictionary")
	}

	totalSymbols := p.SDNUMINSYMS + p.SDNUMNEWSYMS
	newSymbols := make([]*Image, p.SDNUMNEWSYMS)

	iadH := NewArithIntDecoder()
	iadW := NewArithIntDecoder()
	iaai := NewArithIntDecoder()
	iardx := NewArithIntDecoder()
	iardy := NewArithIntDecoder()
	iaex := NewArithIntDecoder()
	iadt := NewArithIntDecoder()
	iafs := NewArithIntDecoder()
	iads := NewArithIntDecoder()
	iait := NewArithIntDecoder()
	iari := NewArithIntDecoder()
	iardw := NewArithIntDecoder()
	iardh := NewArithIntDecoder()

	symCodeLen := uint8(0)
	for (uint32(1) << symCodeLen) < totalSymbols {
		symCodeLen++
	}
	iaid := NewArithIaidDecoder(symCodeLen)

	var hcHeight uint32
	var decoded uint32
	for decoded < p.SDNUMNEWSYMS {
		hDelta, inBand, err := iadH.Decode(decoder)
		if err != nil {
			return nil, err
		}
		if !inBand {
			return nil, errors.New("jbig2: unexpected OOB while decoding symbol height delta")
		}
		hcHeightVal := int64(hcHeight) + int64(hDelta)
		if hcHeightVal < 0 || hcHeightVal > int64(JBig2MaxImageSize) {
			return nil, errors.New("jbig2: symbol height out of range")
		}
		hcHeight = uint32(hcHeightVal)

		var symWidth uint32
		for {
			delta, inBand, err := iadW.Decode(decoder)
			if err != nil {
				return nil, err
			}
			if !inBand {
				break
			}
			if decoded >= p.SDNUMNEWSYMS {
				return nil, errors.New("jbig2: decoded more symbols than declared")
			}
			widthVal := int64(symWidth) + int64(delta)
			if widthVal < 0 || widthVal > int64(JBig2MaxImageSize) {
				return nil, errors.New("jbig2: symbol width out of range")
			}
			symWidth = uint32(widthVal)
			var symbol *Image
			if hcHeight == 0 || symWidth == 0 {
				symbol = nil
			} else if !p.SDREFAGG {
				var decImgErr error
				symbol, decImgErr = p.decodeGenericSymbolArith(decoder, gbContexts, symWidth, hcHeight)
				if decImgErr != nil {
					return nil, decImgErr
				}
			} else {
				refAggInst, ok, err := iaai.Decode(decoder)
				if err != nil {
					return nil, err
				}
				if !ok {
					return nil, errors.New("jbig2: unexpected OOB while decoding refinement instances")
				}
				if refAggInst < 0 {
					return nil, errors.New("jbig2: negative refinement instance count")
				}
				if refAggInst > 1 {
					trd, ids, err := p.configureTRDProcArithmetic(symWidth, hcHeight, uint32(refAggInst), decoded, newSymbols, iadt, iafs, iads, iait, iari, iardw, iardh, iardx, iardy, iaid)
					if err != nil {
						return nil, err
					}
					var imgErr error
					symbol, imgErr = trd.DecodeArith(decoder, grContexts, ids)
					if imgErr != nil {
						return nil, imgErr
					}
				} else {
					var imgErr error
					symbol, imgErr = p.decodeRefinedSymbolArith(decoder, grContexts, symWidth, hcHeight, decoded, newSymbols, iardx, iardy, iaid)
					if imgErr != nil {
						return nil, imgErr
					}
				}
			}
			newSymbols[decoded] = symbol
			decoded++
		}
	}

	exportFlags, err := p.decodeExportFlagsArith(iaex, decoder, totalSymbols)
	if err != nil {
		return nil, err
	}
	return p.buildExportedDictionary(newSymbols, exportFlags)
}

// DecodeHuffman decodes the dictionary using Huffman coding.
func (p *SDDProc) DecodeHuffman(stream *BitStream, gbContexts, grContexts []ArithContext) (*SymbolDict, error) {
	if stream == nil {
		return nil, errors.New("jbig2: nil bitstream for symbol dictionary")
	}
	if p.SDHUFFDH == nil || p.SDHUFFDW == nil || p.SDHUFFBMSIZE == nil {
		return nil, errors.New("jbig2: missing Huffman tables for symbol dictionary")
	}

	decoder := NewHuffmanDecoder(stream)
	totalSymbols := p.SDNUMINSYMS + p.SDNUMNEWSYMS
	newSymbols := make([]*Image, p.SDNUMNEWSYMS)
	widths := make([]uint32, p.SDNUMNEWSYMS)

	var currentHeight uint32
	var decoded uint32
	for decoded < p.SDNUMNEWSYMS {
		hDelta, err := decoder.Decode(p.SDHUFFDH)
		if err != nil {
			return nil, err
		}
		if hDelta == int(JBig2OOB) {
			return nil, errors.New("jbig2: unexpected OOB while decoding symbol height delta")
		}
		newHeight := int64(currentHeight) + int64(hDelta)
		if newHeight < 0 || newHeight > int64(JBig2MaxImageSize) {
			return nil, errors.New("jbig2: symbol height out of range")
		}
		currentHeight = uint32(newHeight)

		var currentWidth uint32
		var totalWidth uint32
		firstIndex := decoded
		for {
			wDelta, err := decoder.Decode(p.SDHUFFDW)
			if err != nil {
				return nil, err
			}
			if wDelta == int(JBig2OOB) {
				break
			}
			if decoded >= p.SDNUMNEWSYMS {
				return nil, errors.New("jbig2: decoded more symbols than declared")
			}
			newWidth := int64(currentWidth) + int64(wDelta)
			if newWidth < 0 || newWidth > int64(JBig2MaxImageSize) {
				return nil, errors.New("jbig2: symbol width out of range")
			}
			currentWidth = uint32(newWidth)
			totalWidth += currentWidth
			if totalWidth > uint32(JBig2MaxImageSize) {
				return nil, errors.New("jbig2: aggregate symbol width out of range")
			}
			widths[decoded] = currentWidth
			decoded++
		}

		// Handle refinement aggregation
		if p.SDREFAGG {
			refAggInst, err := decoder.Decode(p.SDHUFFAGGINST)
			if err != nil {
				return nil, err
			}
			if refAggInst == int(JBig2OOB) {
				return nil, errors.New("jbig2: unexpected OOB while decoding refinement instances")
			}
			if refAggInst < 0 {
				return nil, errors.New("jbig2: negative refinement instance count")
			}

			if refAggInst > 1 {
				// Create Huffman tables for text region decoder
				sbHuffFS, err := NewStandardHuffmanTable(6)
				if err != nil {
					return nil, err
				}
				sbHuffDS, err := NewStandardHuffmanTable(8)
				if err != nil {
					return nil, err
				}
				sbHuffDT, err := NewStandardHuffmanTable(11)
				if err != nil {
					return nil, err
				}
				sbHuffRDW, err := NewStandardHuffmanTable(15)
				if err != nil {
					return nil, err
				}
				sbHuffRDH, err := NewStandardHuffmanTable(15)
				if err != nil {
					return nil, err
				}
				sbHuffRDX, err := NewStandardHuffmanTable(15)
				if err != nil {
					return nil, err
				}
				sbHuffRDY, err := NewStandardHuffmanTable(15)
				if err != nil {
					return nil, err
				}
				sbHuffRSize, err := NewStandardHuffmanTable(1)
				if err != nil {
					return nil, err
				}

				trd := NewTRDProc()
				trd.SBHUFF = p.SDHUFF
				trd.SBREFINE = true
				trd.SBWidth = currentWidth
				trd.SBHeight = currentHeight
				trd.SBNumInstances = uint32(refAggInst)
				trd.SBStrips = 1
				trd.SBNumSyms = p.SDNUMINSYMS + decoded
				symCodeLen := uint8(0)
				for (uint32(1) << symCodeLen) < trd.SBNumSyms {
					symCodeLen++
				}
				trd.SBSymCodeLen = symCodeLen

				trd.SBSyms = make([]*Image, int(trd.SBNumSyms))
				for i := uint32(0); i < p.SDNUMINSYMS && i < trd.SBNumSyms; i++ {
					trd.SBSyms[int(i)] = p.SDINSYMS[i]
				}
				for i := p.SDNUMINSYMS; i < trd.SBNumSyms; i++ {
					idx := i - p.SDNUMINSYMS
					if int(idx) < len(newSymbols) {
						trd.SBSyms[int(i)] = newSymbols[idx]
					}
				}

				trd.SBDefPixel = false
				trd.SBCombOp = ComposeOR
				trd.Transposed = false
				trd.RefCorner = CornerTopLeft
				trd.SBDSOffset = 0
				trd.SBHUFFFS = sbHuffFS
				trd.SBHUFFDS = sbHuffDS
				trd.SBHUFFDT = sbHuffDT
				trd.SBHUFFRDW = sbHuffRDW
				trd.SBHUFFRDH = sbHuffRDH
				trd.SBHUFFRDX = sbHuffRDX
				trd.SBHUFFRDY = sbHuffRDY
				trd.SBHUFFRSize = sbHuffRSize
				trd.SBRTEMPLATE = p.SDRTEMPLATE
				copy(trd.SBRAT[:], p.SDRAT[:])

				symbol, err := trd.DecodeHuffman(stream, grContexts)
				if err != nil {
					return nil, err
				}

				// Store the decoded symbol
				newSymbols[decoded-1] = symbol
				continue
			} else if refAggInst == 1 {
				// Handle single refinement case
				sbNumSyms := p.SDNUMINSYMS + decoded
				symCodeLen := uint8(0)
				for (uint32(1) << symCodeLen) < sbNumSyms {
					symCodeLen++
				}

				// Read symbol ID directly from bitstream (similar to C++ implementation)
				var symID uint32
				for i := uint8(0); i < symCodeLen; i++ {
					bit, err := stream.Read1Bit()
					if err != nil {
						return nil, err
					}
					symID = (symID << 1) | bit
				}
				if symID >= sbNumSyms {
					return nil, errors.New("jbig2: refinement symbol id out of range")
				}

				refImg, err := p.lookupSymbol(symID, decoded, newSymbols)
				if err != nil {
					return nil, err
				}
				if refImg == nil {
					return nil, errors.New("jbig2: refinement source symbol is nil")
				}

				// Create Huffman tables for refinement
				sbHuffRDX, err := NewStandardHuffmanTable(15)
				if err != nil {
					return nil, err
				}
				sbHuffRSize, err := NewStandardHuffmanTable(1)
				if err != nil {
					return nil, err
				}

				// Decode refinement deltas
				rdx, err := decoder.Decode(sbHuffRDX)
				if err != nil {
					return nil, err
				}
				rdy, err := decoder.Decode(sbHuffRDX)
				if err != nil {
					return nil, err
				}
				rsize, err := decoder.Decode(sbHuffRSize)
				if err != nil {
					return nil, err
				}

				stream.AlignByte()
				startOffset := stream.Offset()

				// Create refinement region processor
				grrd := NewGRRDProc()
				grrd.Template = p.SDRTEMPLATE
				grrd.TPGRON = false
				grrd.Width = currentWidth
				grrd.Height = currentHeight
				grrd.Reference = refImg
				grrd.ReferenceDX = int32(rdx)
				grrd.ReferenceDY = int32(rdy)
				copy(grrd.GRAT[:], p.SDRAT[:])

				// Create arithmetic decoder for refinement
				refArithDecoder := NewArithDecoder(stream)
				symbol, err := grrd.Decode(refArithDecoder, grContexts)
				if err != nil {
					return nil, err
				}

				stream.AlignByte()
				stream.AddOffset(2)
				if uint32(rsize) != stream.Offset()-startOffset {
					return nil, errors.New("jbig2: refinement size mismatch")
				}

				// Store the decoded symbol
				newSymbols[decoded-1] = symbol
				continue
			}
		}

		bmsize, err := decoder.Decode(p.SDHUFFBMSIZE)
		if err != nil {
			return nil, err
		}
		if bmsize == int(JBig2OOB) {
			return nil, errors.New("jbig2: unexpected OOB in bitmap size")
		}
		if bmsize < 0 {
			return nil, errors.New("jbig2: negative bitmap size")
		}

		stream.AlignByte()
		if currentHeight == 0 || totalWidth == 0 {
			continue
		}

		var bhc *Image
		if bmsize != 0 {
			// Use MMR decoding for compressed bitmaps
			grd := NewGRDProc()
			grd.MMR = true
			grd.GBWidth = totalWidth
			grd.GBHeight = currentHeight
			var decodedImg *Image
			status, err := grd.StartDecodeMMR(&decodedImg, stream)
			if err != nil || status != CodecStatusFinished {
				return nil, errors.New("jbig2: failed to decode MMR symbol bitmap")
			}
			bhc = decodedImg
		} else {
			stride := (totalWidth + 7) >> 3
			if stride == 0 {
				continue
			}
			needed := stride * currentHeight
			if needed > stream.BytesLeft() {
				return nil, errors.New("jbig2: insufficient data for symbol bitmaps")
			}

			bhc = NewImage(int32(totalWidth), int32(currentHeight))
			if bhc == nil || bhc.data == nil {
				return nil, errors.New("jbig2: failed to allocate symbol bitmap")
			}
			strideBytes := int(stride)
			for row := uint32(0); row < currentHeight; row++ {
				line := bhc.lineUnsafe(int(row))
				src := stream.Pointer()
				copy(line[:strideBytes], src[:strideBytes])
				stream.AddOffset(stride)
			}
		}

		var offset uint32
		for i := firstIndex; i < decoded; i++ {
			width := widths[i]
			if width == 0 {
				newSymbols[i] = nil
				continue
			}
			newSymbols[i] = bhc.SubImage(int32(offset), 0, int32(width), int32(currentHeight))
			offset += width
		}
	}

	lengthTable, err := NewStandardHuffmanTable(1)
	if err != nil {
		return nil, err
	}
	exportFlags, err := p.decodeExportFlagsHuffman(decoder, lengthTable, totalSymbols)
	if err != nil {
		return nil, err
	}
	return p.buildExportedDictionary(newSymbols, exportFlags)
}

func (p *SDDProc) decodeGenericSymbolArith(decoder *ArithDecoder, gbContexts []ArithContext, width, height uint32) (*Image, error) {
	proc := NewGRDProc()
	proc.MMR = false
	proc.TPGDON = false
	proc.UseSkip = false
	proc.GBTemplate = p.SDTEMPLATE
	proc.GBWidth = width
	proc.GBHeight = height
	for i := range p.SDAT {
		proc.GBAt[i] = int32(p.SDAT[i])
	}
	return proc.DecodeArith(decoder, gbContexts)
}

func (p *SDDProc) configureTRDProcArithmetic(width, height, instances uint32, decoded uint32, newSymbols []*Image, iadt, iafs, iads, iait, iari, iardw, iardh, iardx, iardy *ArithIntDecoder, iaid *ArithIaidDecoder) (*TRDProc, *IntDecoderState, error) {
	trd := NewTRDProc()
	trd.SBHUFF = p.SDHUFF
	trd.SBREFINE = true
	trd.SBWidth = width
	trd.SBHeight = height
	trd.SBNumInstances = instances
	trd.SBStrips = 1
	trd.SBNumSyms = p.SDNUMINSYMS + decoded
	symCodeLen := uint8(0)
	for (uint32(1) << symCodeLen) < trd.SBNumSyms {
		symCodeLen++
	}
	trd.SBSymCodeLen = symCodeLen

	if trd.SBNumSyms > uint32(len(p.SDINSYMS))+decoded {
		return nil, nil, errors.New("jbig2: inconsistent symbol dictionary state")
	}
	trd.SBSyms = make([]*Image, int(trd.SBNumSyms))
	for i := uint32(0); i < p.SDNUMINSYMS && i < trd.SBNumSyms; i++ {
		trd.SBSyms[int(i)] = p.SDINSYMS[i]
	}
	for i := p.SDNUMINSYMS; i < trd.SBNumSyms; i++ {
		idx := i - p.SDNUMINSYMS
		if int(idx) < len(newSymbols) {
			trd.SBSyms[int(i)] = newSymbols[idx]
		}
	}

	trd.SBDefPixel = false
	trd.SBCombOp = ComposeOR
	trd.Transposed = false
	trd.RefCorner = CornerTopLeft
	trd.SBDSOffset = 0
	trd.SBRTEMPLATE = p.SDRTEMPLATE
	trd.SBRAT = p.SDRAT

	ids := &IntDecoderState{
		IADT:  iadt,
		IAFS:  iafs,
		IADS:  iads,
		IAIT:  iait,
		IARI:  iari,
		IARDW: iardw,
		IARDH: iardh,
		IARDX: iardx,
		IARDY: iardy,
		IAID:  iaid,
	}
	return trd, ids, nil
}

func (p *SDDProc) decodeRefinedSymbolArith(decoder *ArithDecoder, grContexts []ArithContext, width, height, decoded uint32, newSymbols []*Image, iardx, iardy *ArithIntDecoder, iaid *ArithIaidDecoder) (*Image, error) {
	sbNumSyms := p.SDNUMINSYMS + decoded
	idi, err := iaid.Decode(decoder)
	if err != nil {
		return nil, err
	}
	if idi >= sbNumSyms {
		return nil, fmt.Errorf("jbig2: refinement symbol id %d out of range", idi)
	}
	refImg, err := p.lookupSymbol(idi, decoded, newSymbols)
	if err != nil {
		return nil, err
	}
	if refImg == nil {
		return nil, fmt.Errorf("jbig2: refinement source symbol %d is nil", idi)
	}

	rdx, ok, err := iardx.Decode(decoder)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("jbig2: unexpected OOB while decoding refinement dx")
	}
	rdy, ok, err := iardy.Decode(decoder)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("jbig2: unexpected OOB while decoding refinement dy")
	}

	grrd := NewGRRDProc()
	grrd.Template = p.SDRTEMPLATE
	grrd.TPGRON = false
	grrd.Width = width
	grrd.Height = height
	grrd.Reference = refImg
	grrd.ReferenceDX = int32(rdx)
	grrd.ReferenceDY = int32(rdy)
	copy(grrd.GRAT[:], p.SDRAT[:])
	return grrd.Decode(decoder, grContexts)
}

func (p *SDDProc) lookupSymbol(id, decoded uint32, newSymbols []*Image) (*Image, error) {
	if id < p.SDNUMINSYMS {
		if int(id) >= len(p.SDINSYMS) {
			return nil, fmt.Errorf("jbig2: missing input symbol %d", id)
		}
		return p.SDINSYMS[id], nil
	}
	idx := id - p.SDNUMINSYMS
	if idx >= decoded {
		return nil, fmt.Errorf("jbig2: referenced new symbol %d not yet decoded", idx)
	}
	if int(idx) >= len(newSymbols) {
		return nil, fmt.Errorf("jbig2: new symbol index %d out of range", idx)
	}
	return newSymbols[idx], nil
}

func (p *SDDProc) decodeExportFlagsArith(decoder *ArithIntDecoder, arith *ArithDecoder, total uint32) ([]bool, error) {
	flags := make([]bool, total)
	curFlag := false
	var index uint32
	var exported uint32
	for index < total {
		run, inBand, err := decoder.Decode(arith)
		if err != nil {
			return nil, err
		}
		if !inBand {
			return nil, errors.New("jbig2: unexpected OOB while decoding export run length")
		}
		if run < 0 {
			return nil, errors.New("jbig2: negative export run length")
		}
		runLen := uint32(run)
		if runLen > total-index {
			return nil, errors.New("jbig2: export run exceeds symbol count")
		}
		if curFlag {
			exported += runLen
		}
		for i := uint32(0); i < runLen; i++ {
			flags[index+i] = curFlag
		}
		index += runLen
		curFlag = !curFlag
	}
	if index != total {
		return nil, errors.New("jbig2: export run lengths do not cover symbol set")
	}
	if exported > p.SDNUMEXSYMS {
		return nil, errors.New("jbig2: export symbol count exceeds declared limit")
	}
	return flags, nil
}

func (p *SDDProc) decodeExportFlagsHuffman(decoder *HuffmanDecoder, table *HuffmanTable, total uint32) ([]bool, error) {
	if table == nil {
		return nil, errors.New("jbig2: missing export run-length table")
	}
	flags := make([]bool, total)
	curFlag := false
	var index uint32
	var exported uint32
	for index < total {
		run, err := decoder.Decode(table)
		if err != nil {
			return nil, err
		}
		if run == int(JBig2OOB) {
			return nil, errors.New("jbig2: unexpected OOB in export run-length stream")
		}
		if run < 0 {
			return nil, errors.New("jbig2: negative export run length")
		}
		runLen := uint32(run)
		if runLen > total-index {
			return nil, errors.New("jbig2: export run exceeds symbol count")
		}
		if curFlag {
			exported += runLen
		}
		for i := uint32(0); i < runLen; i++ {
			flags[index+i] = curFlag
		}
		index += runLen
		curFlag = !curFlag
	}
	if index != total {
		return nil, errors.New("jbig2: export run lengths do not cover symbol set")
	}
	if exported > p.SDNUMEXSYMS {
		return nil, errors.New("jbig2: export symbol count exceeds declared limit")
	}
	return flags, nil
}

func (p *SDDProc) buildExportedDictionary(newSymbols []*Image, exportFlags []bool) (*SymbolDict, error) {
	total := p.SDNUMINSYMS + p.SDNUMNEWSYMS
	if len(exportFlags) < int(total) {
		return nil, errors.New("jbig2: insufficient export flags")
	}
	dict := NewSymbolDict()
	var exported uint32
	for i := uint32(0); i < total; i++ {
		if !exportFlags[i] || exported >= p.SDNUMEXSYMS {
			continue
		}
		if i < p.SDNUMINSYMS {
			if int(i) >= len(p.SDINSYMS) {
				return nil, fmt.Errorf("jbig2: missing referenced input symbol %d", i)
			}
			dict.AddImage(cloneImage(p.SDINSYMS[i]))
		} else {
			idx := i - p.SDNUMINSYMS
			if int(idx) >= len(newSymbols) {
				return nil, fmt.Errorf("jbig2: missing decoded symbol %d", idx)
			}
			dict.AddImage(newSymbols[idx])
			newSymbols[idx] = nil
		}
		exported++
	}
	return dict, nil
}

func checkTRDDimension(base int, delta int) (int, error) {
	value := int64(base) + int64(delta)
	if value < 0 || value > int64(JBig2MaxImageSize) {
		return 0, errors.New("jbig2: refinement dimension out of range")
	}
	return int(value), nil
}

func checkTRDReferenceDimension(delta int, shift uint32, offset int) (int, error) {
	adj := offset + (delta >> shift)
	if adj < -1<<30 || adj > 1<<30 {
		return 0, errors.New("jbig2: refinement reference offset out of range")
	}
	return adj, nil
}
