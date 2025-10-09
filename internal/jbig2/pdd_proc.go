package jbig2

import (
	"errors"
	"fmt"
)

// PDDProc manages halftone pattern dictionary decoding.
type PDDProc struct {
	HDMMR      bool
	HDPW       uint8
	HDPH       uint8
	GrayMax    uint32
	HDTemplate uint8
}

// NewPDDProc constructs a halftone pattern decoder configuration.
func NewPDDProc() *PDDProc { return &PDDProc{} }

// DecodeArith decodes a pattern dictionary using arithmetic coding.
func (p *PDDProc) DecodeArith(decoder *ArithDecoder, contexts []ArithContext, pause PauseIndicator) (*PatternDict, error) {
	if decoder == nil {
		return nil, errors.New("jbig2: nil arithmetic decoder for pattern dictionary")
	}
	if len(contexts) == 0 {
		return nil, errors.New("jbig2: missing arithmetic contexts for pattern dictionary")
	}

	expected := huffContextSize(p.HDTemplate)
	if expected != len(contexts) {
		return nil, fmt.Errorf("jbig2: unexpected pattern dictionary context size %d, want %d", len(contexts), expected)
	}

	grd, err := p.createGRDProc()
	if err != nil {
		return nil, err
	}
	grd.GBTemplate = p.HDTemplate
	grd.TPGDON = false
	grd.UseSkip = false
	grd.GBAt[0] = -int32(p.HDPW)
	grd.GBAt[1] = 0
	if grd.GBTemplate == 0 {
		grd.GBAt[2] = -3
		grd.GBAt[3] = -1
		grd.GBAt[4] = 2
		grd.GBAt[5] = -2
		grd.GBAt[6] = -2
		grd.GBAt[7] = -2
	}

	var bhdc *Image
	state := &GRDProgressiveState{
		Image:    &bhdc,
		Decoder:  decoder,
		Contexts: contexts,
	}

	status, err := grd.StartDecodeArith(state)
	if err != nil {
		return nil, err
	}
	state.Pause = pause
	for status == CodecStatusToBeContinued {
		status, err = grd.ContinueDecode(state)
		if err != nil {
			return nil, err
		}
	}
	if status != CodecStatusFinished {
		return nil, errors.New("jbig2: pattern dictionary arithmetic decode incomplete")
	}

	if bhdc == nil || bhdc.data == nil {
		return nil, errors.New("jbig2: failed to decode pattern dictionary image")
	}

	return p.buildPatternDictFromImage(bhdc), nil
}

// DecodeMMR decodes a pattern dictionary using MMR compression.
func (p *PDDProc) DecodeMMR(stream *BitStream) (*PatternDict, error) {
	if stream == nil {
		return nil, errors.New("jbig2: nil bitstream for pattern dictionary")
	}

	grd, err := p.createGRDProc()
	if err != nil {
		return nil, err
	}
	var bhdc *Image
	status, err := grd.StartDecodeMMR(&bhdc, stream)
	if err != nil || status != CodecStatusFinished {
		return nil, errors.New("jbig2: failed to decode MMR pattern dictionary")
	}
	if bhdc == nil || bhdc.data == nil {
		return nil, errors.New("jbig2: failed to decode pattern dictionary image")
	}

	return p.buildPatternDictFromImage(bhdc), nil
}

func (p *PDDProc) createGRDProc() (*GRDProc, error) {
	if p.HDPW == 0 || p.HDPH == 0 {
		return nil, errors.New("jbig2: pattern dictionary dimensions must be non-zero")
	}
	count := uint64(p.GrayMax) + 1
	width := count * uint64(p.HDPW)
	height := uint64(p.HDPH)
	if width == 0 || height == 0 {
		return nil, errors.New("jbig2: pattern dictionary dimensions invalid")
	}
	if width > uint64(JBig2MaxImageSize) || height > uint64(JBig2MaxImageSize) {
		return nil, fmt.Errorf("jbig2: pattern dictionary dimensions %dx%d exceed limits", width, height)
	}

	grd := NewGRDProc()
	grd.MMR = p.HDMMR
	grd.GBWidth = uint32(width)
	grd.GBHeight = uint32(height)
	return grd, nil
}

func (p *PDDProc) buildPatternDictFromImage(img *Image) *PatternDict {
	count := p.GrayMax + 1
	dict := NewPatternDict(count)
	if img == nil {
		return dict
	}
	for gray := uint32(0); gray < count; gray++ {
		x := int32(gray) * int32(p.HDPW)
		pattern := img.SubImage(x, 0, int32(p.HDPW), int32(p.HDPH))
		dict.SetPattern(gray, pattern)
	}
	return dict
}
