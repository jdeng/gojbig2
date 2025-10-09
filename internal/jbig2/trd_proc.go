package jbig2

import (
	"errors"
	"fmt"
)

// IntDecoderState bundles the arithmetic integer decoders used during text region decoding.
type IntDecoderState struct {
	IADT  *ArithIntDecoder
	IAFS  *ArithIntDecoder
	IADS  *ArithIntDecoder
	IAIT  *ArithIntDecoder
	IARI  *ArithIntDecoder
	IARDW *ArithIntDecoder
	IARDH *ArithIntDecoder
	IARDX *ArithIntDecoder
	IARDY *ArithIntDecoder
	IAID  *ArithIaidDecoder
}

// TRDProc stores the configuration for a JBIG2 text region decoder.
type TRDProc struct {
	SBHUFF         bool
	SBREFINE       bool
	SBRTEMPLATE    bool
	Transposed     bool
	SBDefPixel     bool
	SBDSOffset     int8
	SBSymCodeLen   uint8
	SBWidth        uint32
	SBHeight       uint32
	SBNumInstances uint32
	SBStrips       uint32
	SBNumSyms      uint32
	SBSymCodes     []HuffmanCode
	SBSyms         []*Image
	SBCombOp       ComposeOp
	RefCorner      JBig2Corner
	SBHUFFFS       *HuffmanTable
	SBHUFFDS       *HuffmanTable
	SBHUFFDT       *HuffmanTable
	SBHUFFRDW      *HuffmanTable
	SBHUFFRDH      *HuffmanTable
	SBHUFFRDX      *HuffmanTable
	SBHUFFRDY      *HuffmanTable
	SBHUFFRSize    *HuffmanTable
	SBRAT          [4]int8
}

// NewTRDProc constructs a text region decoder configuration.
func NewTRDProc() *TRDProc { return &TRDProc{} }

type composeData struct {
	x         int64
	y         int64
	increment int64
}

// DecodeHuffman decodes a text region using Huffman coding.
func (p *TRDProc) DecodeHuffman(stream *BitStream, contexts []ArithContext) (*Image, error) {
	if stream == nil {
		return nil, errors.New("jbig2: nil bitstream for text region")
	}
	if p.SBHUFFFS == nil || p.SBHUFFDS == nil || p.SBHUFFDT == nil ||
		p.SBHUFFRDW == nil || p.SBHUFFRDH == nil || p.SBHUFFRDX == nil ||
		p.SBHUFFRDY == nil || p.SBHUFFRSize == nil {
		return nil, errors.New("jbig2: missing Huffman tables for text region")
	}

	img := NewImage(int32(p.SBWidth), int32(p.SBHeight))
	if img == nil || img.data == nil {
		return nil, errors.New("jbig2: failed to allocate text region image")
	}
	img.Fill(p.SBDefPixel)

	decoder := NewHuffmanDecoder(stream)

	stript, ok, err := p.decodeHuffmanStrip(decoder, img)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("jbig2: text region initial DT OOB")
	}
	stripPosition := int64(-stript * int(p.SBStrips))
	firstS := int64(0)
	instances := uint32(0)

	for instances < p.SBNumInstances {
		dt, ok, err := p.decodeHuffmanStrip(decoder, img)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, errors.New("jbig2: text region DT OOB")
		}
		stripPosition += int64(dt * int(p.SBStrips))
		curs := int64(0)
		first := true
		for {
			if first {
				dfs, ok, err := p.decodeHuffmanFirstS(decoder)
				if err != nil {
					return nil, err
				}
				if !ok {
					return nil, errors.New("jbig2: text region DFS OOB")
				}
				firstS += int64(dfs)
				curs = firstS
				first = false
			} else {
				idsVal, ok, err := p.decodeHuffmanDS(decoder)
				if err != nil {
					return nil, err
				}
				if !ok {
					break
				}
				curs += int64(idsVal) + int64(p.SBDSOffset)
			}
			if instances >= p.SBNumInstances {
				break
			}

			curt := 0
			if p.SBStrips != 1 {
				val, ok, err := p.decodeHuffmanIT(decoder)
				if err != nil {
					return nil, err
				}
				if !ok {
					return nil, errors.New("jbig2: text region strip value OOB")
				}
				curt = val
			}
			ti := stripPosition + int64(curt)
			if ti < -1<<30 || ti > 1<<30 {
				return nil, errors.New("jbig2: text region vertical position overflow")
			}

			symID, err := p.decodeHuffmanSymID(stream, decoder)
			if err != nil {
				return nil, err
			}
			if symID >= p.SBNumSyms {
				return nil, fmt.Errorf("jbig2: text region symbol id %d out of range", symID)
			}

			ri := 0
			if p.SBREFINE {
				val, ok, err := p.decodeHuffmanRI(decoder)
				if err != nil {
					return nil, err
				}
				if !ok {
					return nil, errors.New("jbig2: text region refinement flag OOB")
				}
				ri = val
			}

			var glyph *Image
			if symID < uint32(len(p.SBSyms)) {
				glyph = p.SBSyms[symID]
			}
			if glyph == nil {
				return nil, fmt.Errorf("jbig2: text region missing symbol %d", symID)
			}

			if ri != 0 {
				rdw, ok, err := p.decodeHuffmanRDW(decoder)
				if err != nil {
					return nil, err
				}
				if !ok {
					return nil, errors.New("jbig2: text region RDW OOB")
				}
				rdh, ok, err := p.decodeHuffmanRDH(decoder)
				if err != nil {
					return nil, err
				}
				if !ok {
					return nil, errors.New("jbig2: text region RDH OOB")
				}
				rdx, ok, err := p.decodeHuffmanRDX(decoder)
				if err != nil {
					return nil, err
				}
				if !ok {
					return nil, errors.New("jbig2: text region RDX OOB")
				}
				rdy, ok, err := p.decodeHuffmanRDY(decoder)
				if err != nil {
					return nil, err
				}
				if !ok {
					return nil, errors.New("jbig2: text region RDY OOB")
				}

				newWidth, err := checkTRDDimension(glyph.Width(), rdw)
				if err != nil {
					return nil, err
				}
				newHeight, err := checkTRDDimension(glyph.Height(), rdh)
				if err != nil {
					return nil, err
				}
				referenceDX, err := checkTRDReferenceDimension(rdw, 1, rdx)
				if err != nil {
					return nil, err
				}
				referenceDY, err := checkTRDReferenceDimension(rdh, 1, rdy)
				if err != nil {
					return nil, err
				}

				grrd := NewGRRDProc()
				grrd.Template = p.SBRTEMPLATE
				grrd.TPGRON = false
				grrd.Width = uint32(newWidth)
				grrd.Height = uint32(newHeight)
				grrd.Reference = glyph
				grrd.ReferenceDX = int32(referenceDX)
				grrd.ReferenceDY = int32(referenceDY)
				copy(grrd.GRAT[:], p.SBRAT[:])

				// For Huffman refinement, we need to use arithmetic decoding
				refArithDecoder := NewArithDecoder(stream)
				refined, err := grrd.Decode(refArithDecoder, contexts)
				if err != nil {
					return nil, err
				}
				glyph = refined
			}

			wi := uint32(glyph.Width())
			hi := uint32(glyph.Height())
			if !p.Transposed && (p.RefCorner == CornerTopRight || p.RefCorner == CornerBottomRight) {
				curs += int64(wi) - 1
			} else if p.Transposed && (p.RefCorner == CornerBottomLeft || p.RefCorner == CornerBottomRight) {
				curs += int64(hi) - 1
			}

			compose := p.getComposeData(curs, ti, wi, hi)
			if !glyph.ComposeTo(img, compose.x, compose.y, p.SBCombOp) {
				return nil, errors.New("jbig2: failed to compose text region glyph")
			}
			if compose.increment != 0 {
				curs += compose.increment
			}
			instances++
		}
	}
	return img, nil
}

// Helper methods for Huffman decoding
func (p *TRDProc) decodeHuffmanStrip(decoder *HuffmanDecoder, img *Image) (int, bool, error) {
	if p.SBHUFFDT == nil {
		return 0, false, errors.New("jbig2: missing DT Huffman table")
	}
	val, err := decoder.Decode(p.SBHUFFDT)
	if err != nil {
		return 0, false, err
	}
	if val == int(JBig2OOB) {
		return 0, false, nil
	}
	return val, true, nil
}

func (p *TRDProc) decodeHuffmanFirstS(decoder *HuffmanDecoder) (int, bool, error) {
	if p.SBHUFFFS == nil {
		return 0, false, errors.New("jbig2: missing FS Huffman table")
	}
	val, err := decoder.Decode(p.SBHUFFFS)
	if err != nil {
		return 0, false, err
	}
	if val == int(JBig2OOB) {
		return 0, false, nil
	}
	return val, true, nil
}

func (p *TRDProc) decodeHuffmanDS(decoder *HuffmanDecoder) (int, bool, error) {
	if p.SBHUFFDS == nil {
		return 0, false, errors.New("jbig2: missing DS Huffman table")
	}
	val, err := decoder.Decode(p.SBHUFFDS)
	if err != nil {
		return 0, false, err
	}
	if val == int(JBig2OOB) {
		return 0, false, nil
	}
	return val, true, nil
}

func (p *TRDProc) decodeHuffmanIT(decoder *HuffmanDecoder) (int, bool, error) {
	if p.SBHUFFDT == nil {
		return 0, false, errors.New("jbig2: missing IT Huffman table")
	}
	val, err := decoder.Decode(p.SBHUFFDT)
	if err != nil {
		return 0, false, err
	}
	if val == int(JBig2OOB) {
		return 0, false, nil
	}
	return val, true, nil
}

func (p *TRDProc) decodeHuffmanSymID(stream *BitStream, decoder *HuffmanDecoder) (uint32, error) {
	// Read symbol ID directly from bitstream
	var symID uint32
	for i := uint8(0); i < p.SBSymCodeLen; i++ {
		bit, err := stream.Read1Bit()
		if err != nil {
			return 0, err
		}
		symID = (symID << 1) | bit
	}
	return symID, nil
}

func (p *TRDProc) decodeHuffmanRI(decoder *HuffmanDecoder) (int, bool, error) {
	if p.SBHUFFRSize == nil {
		return 0, false, errors.New("jbig2: missing RI Huffman table")
	}
	val, err := decoder.Decode(p.SBHUFFRSize)
	if err != nil {
		return 0, false, err
	}
	if val == int(JBig2OOB) {
		return 0, false, nil
	}
	return val, true, nil
}

func (p *TRDProc) decodeHuffmanRDW(decoder *HuffmanDecoder) (int, bool, error) {
	if p.SBHUFFRDW == nil {
		return 0, false, errors.New("jbig2: missing RDW Huffman table")
	}
	val, err := decoder.Decode(p.SBHUFFRDW)
	if err != nil {
		return 0, false, err
	}
	if val == int(JBig2OOB) {
		return 0, false, nil
	}
	return val, true, nil
}

func (p *TRDProc) decodeHuffmanRDH(decoder *HuffmanDecoder) (int, bool, error) {
	if p.SBHUFFRDH == nil {
		return 0, false, errors.New("jbig2: missing RDH Huffman table")
	}
	val, err := decoder.Decode(p.SBHUFFRDH)
	if err != nil {
		return 0, false, err
	}
	if val == int(JBig2OOB) {
		return 0, false, nil
	}
	return val, true, nil
}

func (p *TRDProc) decodeHuffmanRDX(decoder *HuffmanDecoder) (int, bool, error) {
	if p.SBHUFFRDX == nil {
		return 0, false, errors.New("jbig2: missing RDX Huffman table")
	}
	val, err := decoder.Decode(p.SBHUFFRDX)
	if err != nil {
		return 0, false, err
	}
	if val == int(JBig2OOB) {
		return 0, false, nil
	}
	return val, true, nil
}

func (p *TRDProc) decodeHuffmanRDY(decoder *HuffmanDecoder) (int, bool, error) {
	if p.SBHUFFRDY == nil {
		return 0, false, errors.New("jbig2: missing RDY Huffman table")
	}
	val, err := decoder.Decode(p.SBHUFFRDY)
	if err != nil {
		return 0, false, err
	}
	if val == int(JBig2OOB) {
		return 0, false, nil
	}
	return val, true, nil
}

// DecodeArith decodes a text region using arithmetic coding.
func (p *TRDProc) DecodeArith(decoder *ArithDecoder, contexts []ArithContext, ids *IntDecoderState) (*Image, error) {
	if decoder == nil {
		return nil, errors.New("jbig2: nil arithmetic decoder for text region")
	}
	return p.decodeTextRegionArith(decoder, contexts, ids)
}

// JBig2Corner enumerates the four reference corners used in text region placement.
type JBig2Corner int

const (
	CornerBottomLeft JBig2Corner = iota
	CornerTopLeft
	CornerBottomRight
	CornerTopRight
)

type arithDecodeContext struct {
	decoder *ArithDecoder
	ids     *IntDecoderState
}

func (p *TRDProc) decodeTextRegionArith(decoder *ArithDecoder, contexts []ArithContext, ids *IntDecoderState) (*Image, error) {
	if !IsValidImageSize(int32(p.SBWidth), int32(p.SBHeight)) {
		return NewImage(int32(p.SBWidth), int32(p.SBHeight)), nil
	}
	img := NewImage(int32(p.SBWidth), int32(p.SBHeight))
	if img == nil || img.data == nil {
		return nil, errors.New("jbig2: failed to allocate text region image")
	}
	img.Fill(p.SBDefPixel)

	var iadt, iafs, iads, iait, iari, iardw, iardh, iardx, iardy *ArithIntDecoder
	if ids != nil {
		iadt = ensureDecoder(&ids.IADT)
		iafs = ensureDecoder(&ids.IAFS)
		iads = ensureDecoder(&ids.IADS)
		iait = ensureDecoder(&ids.IAIT)
		iari = ensureDecoder(&ids.IARI)
		iardw = ensureDecoder(&ids.IARDW)
		iardh = ensureDecoder(&ids.IARDH)
		iardx = ensureDecoder(&ids.IARDX)
		iardy = ensureDecoder(&ids.IARDY)
	} else {
		iadt = NewArithIntDecoder()
		iafs = NewArithIntDecoder()
		iads = NewArithIntDecoder()
		iait = NewArithIntDecoder()
		iari = NewArithIntDecoder()
		iardw = NewArithIntDecoder()
		iardh = NewArithIntDecoder()
		iardx = NewArithIntDecoder()
		iardy = NewArithIntDecoder()
	}

	symCodeLen := p.SBSymCodeLen
	if symCodeLen == 0 {
		tmp := uint8(0)
		for (uint32(1) << tmp) < p.SBNumSyms {
			tmp++
		}
		symCodeLen = tmp
	}
	var iaid *ArithIaidDecoder
	if ids != nil {
		iaid = ensureIAID(ids, symCodeLen)
	} else {
		iaid = NewArithIaidDecoder(symCodeLen)
	}

	stript, ok, err := iadt.Decode(decoder)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("jbig2: text region initial DT OOB")
	}
	stripPosition := int64(-stript * int(p.SBStrips))
	firstS := int64(0)
	instances := uint32(0)

	for instances < p.SBNumInstances {
		dt, ok, err := iadt.Decode(decoder)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, errors.New("jbig2: text region DT OOB")
		}
		stripPosition += int64(dt * int(p.SBStrips))
		curs := int64(0)
		first := true
		for {
			if first {
				dfs, ok, err := iafs.Decode(decoder)
				if err != nil {
					return nil, err
				}
				if !ok {
					return nil, errors.New("jbig2: text region DFS OOB")
				}
				firstS += int64(dfs)
				curs = firstS
				first = false
			} else {
				idsVal, ok, err := iads.Decode(decoder)
				if err != nil {
					return nil, err
				}
				if !ok {
					break
				}
				curs += int64(idsVal) + int64(p.SBDSOffset)
			}
			if instances >= p.SBNumInstances {
				break
			}

			curt := 0
			if p.SBStrips != 1 {
				val, ok, err := iait.Decode(decoder)
				if err != nil {
					return nil, err
				}
				if !ok {
					return nil, errors.New("jbig2: text region strip value OOB")
				}
				curt = val
			}
			ti := stripPosition + int64(curt)
			if ti < -1<<30 || ti > 1<<30 {
				return nil, errors.New("jbig2: text region vertical position overflow")
			}

			symID, err := iaid.Decode(decoder)
			if err != nil {
				return nil, err
			}
			if symID >= p.SBNumSyms {
				return nil, fmt.Errorf("jbig2: text region symbol id %d out of range", symID)
			}

			ri := 0
			if p.SBREFINE {
				val, ok, err := iari.Decode(decoder)
				if err != nil {
					return nil, err
				}
				if !ok {
					return nil, errors.New("jbig2: text region refinement flag OOB")
				}
				ri = val
			}

			var glyph *Image
			if symID < uint32(len(p.SBSyms)) {
				glyph = p.SBSyms[symID]
			}
			if glyph == nil {
				return nil, fmt.Errorf("jbig2: text region missing symbol %d", symID)
			}

			if ri != 0 {
				rdw, ok, err := iardw.Decode(decoder)
				if err != nil {
					return nil, err
				}
				if !ok {
					return nil, errors.New("jbig2: text region RDW OOB")
				}
				rdh, ok, err := iardh.Decode(decoder)
				if err != nil {
					return nil, err
				}
				if !ok {
					return nil, errors.New("jbig2: text region RDH OOB")
				}
				rdx, ok, err := iardx.Decode(decoder)
				if err != nil {
					return nil, err
				}
				if !ok {
					return nil, errors.New("jbig2: text region RDX OOB")
				}
				rdy, ok, err := iardy.Decode(decoder)
				if err != nil {
					return nil, err
				}
				if !ok {
					return nil, errors.New("jbig2: text region RDY OOB")
				}

				newWidth, err := checkTRDDimension(glyph.Width(), rdw)
				if err != nil {
					return nil, err
				}
				newHeight, err := checkTRDDimension(glyph.Height(), rdh)
				if err != nil {
					return nil, err
				}
				referenceDX, err := checkTRDReferenceDimension(rdw, 1, rdx)
				if err != nil {
					return nil, err
				}
				referenceDY, err := checkTRDReferenceDimension(rdh, 1, rdy)
				if err != nil {
					return nil, err
				}

				grrd := NewGRRDProc()
				grrd.Template = p.SBRTEMPLATE
				grrd.TPGRON = false
				grrd.Width = uint32(newWidth)
				grrd.Height = uint32(newHeight)
				grrd.Reference = glyph
				grrd.ReferenceDX = int32(referenceDX)
				grrd.ReferenceDY = int32(referenceDY)
				copy(grrd.GRAT[:], p.SBRAT[:])
				refined, err := grrd.Decode(decoder, contexts)
				if err != nil {
					return nil, err
				}
				glyph = refined
			}

			wi := uint32(glyph.Width())
			hi := uint32(glyph.Height())
			if !p.Transposed && (p.RefCorner == CornerTopRight || p.RefCorner == CornerBottomRight) {
				curs += int64(wi) - 1
			} else if p.Transposed && (p.RefCorner == CornerBottomLeft || p.RefCorner == CornerBottomRight) {
				curs += int64(hi) - 1
			}

			compose := p.getComposeData(curs, ti, wi, hi)
			if !glyph.ComposeTo(img, compose.x, compose.y, p.SBCombOp) {
				return nil, errors.New("jbig2: failed to compose text region glyph")
			}
			if compose.increment != 0 {
				curs += compose.increment
			}
			instances++
		}
	}
	return img, nil
}

func ensureDecoder(slot **ArithIntDecoder) *ArithIntDecoder {
	if *slot == nil {
		*slot = NewArithIntDecoder()
	}
	return *slot
}

func ensureIAID(ids *IntDecoderState, codeLen uint8) *ArithIaidDecoder {
	if ids.IAID == nil || ids.IAID.len != codeLen {
		ids.IAID = NewArithIaidDecoder(codeLen)
	}
	return ids.IAID
}

func (p *TRDProc) getComposeData(si int64, ti int64, wi, hi uint32) composeData {
	data := composeData{}
	if !p.Transposed {
		switch p.RefCorner {
		case CornerTopLeft:
			data.x = si
			data.y = ti
			data.increment = int64(wi) - 1
		case CornerTopRight:
			data.x = si - int64(wi) + 1
			data.y = ti
		case CornerBottomLeft:
			data.x = si
			data.y = ti - int64(hi) + 1
			data.increment = int64(wi) - 1
		case CornerBottomRight:
			data.x = si - int64(wi) + 1
			data.y = ti - int64(hi) + 1
		}
	} else {
		switch p.RefCorner {
		case CornerTopLeft:
			data.x = ti
			data.y = si
			data.increment = int64(hi) - 1
		case CornerTopRight:
			data.x = ti - int64(wi) + 1
			data.y = si
			data.increment = int64(hi) - 1
		case CornerBottomLeft:
			data.x = ti
			data.y = si - int64(hi) + 1
		case CornerBottomRight:
			data.x = ti - int64(wi) + 1
			data.y = si - int64(hi) + 1
		}
	}
	return data
}
