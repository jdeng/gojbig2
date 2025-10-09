package jbig2

import (
	"errors"
	"fmt"
)

// HTRDProc stores halftone region decoding parameters.
type HTRDProc struct {
	HBWidth     uint32
	HBHeight    uint32
	HMMR        bool
	HTemplate   uint8
	HNumPats    uint32
	HPats       []*Image
	HDefPixel   bool
	HCombOp     ComposeOp
	HEnableSkip bool
	HGWidth     uint32
	HGHeight    uint32
	HGX         int32
	HGY         int32
	HRX         uint16
	HRY         uint16
	HPW         uint8
	HPH         uint8
}

// NewHTRDProc constructs a halftone region decoder configuration.
func NewHTRDProc() *HTRDProc { return &HTRDProc{} }

// DecodeArith decodes a halftone region using arithmetic coding.
func (p *HTRDProc) DecodeArith(decoder *ArithDecoder, contexts []ArithContext, pause PauseIndicator) (*Image, error) {
	if decoder == nil {
		return nil, errors.New("jbig2: nil arithmetic decoder for halftone region")
	}
	if len(contexts) == 0 {
		return nil, errors.New("jbig2: missing arithmetic contexts for halftone region")
	}
	if expected := huffContextSize(p.HTemplate); expected != len(contexts) {
		return nil, fmt.Errorf("jbig2: unexpected halftone context size %d, want %d", len(contexts), expected)
	}

	var hskip *Image
	if p.HEnableSkip {
		hskip = NewImage(int32(p.HGWidth), int32(p.HGHeight))
		if hskip == nil || hskip.data == nil {
			return nil, errors.New("jbig2: failed to allocate halftone skip image")
		}
		for mg := uint32(0); mg < p.HGHeight; mg++ {
			for ng := uint32(0); ng < p.HGWidth; ng++ {
				mgInt := int64(mg)
				ngInt := int64(ng)
				x := (int64(p.HGX) + mgInt*int64(p.HRY) + ngInt*int64(p.HRX)) >> 8
				y := (int64(p.HGY) + mgInt*int64(p.HRX) - ngInt*int64(p.HRY)) >> 8
				skip := 0
				if (x+int64(p.HPW) <= 0) || (x >= int64(p.HBWidth)) || (y+int64(p.HPH) <= 0) || (y >= int64(p.HBHeight)) {
					skip = 1
				}
				hskip.SetPixel(int32(ng), int32(mg), skip)
			}
		}
	}

	hbpp := uint32(1)
	for (uint32(1) << hbpp) < p.HNumPats {
		hbpp++
	}
	gsbpp := uint8(hbpp)
	if gsbpp == 0 {
		return nil, errors.New("jbig2: invalid halftone plane count")
	}

	grd := NewGRDProc()
	grd.GBTemplate = p.HTemplate
	grd.TPGDON = false
	grd.UseSkip = p.HEnableSkip
	grd.Skip = hskip
	grd.GBWidth = p.HGWidth
	grd.GBHeight = p.HGHeight
	grd.MMR = false
	if p.HTemplate <= 1 {
		grd.GBAt[0] = 3
	} else {
		grd.GBAt[0] = 2
	}
	grd.GBAt[1] = -1
	if grd.GBTemplate == 0 {
		grd.GBAt[2] = -3
		grd.GBAt[3] = -1
		grd.GBAt[4] = 2
		grd.GBAt[5] = -2
		grd.GBAt[6] = -2
		grd.GBAt[7] = -2
	}

	gsplanes := make([]*Image, gsbpp)
	for idx := int(gsbpp) - 1; idx >= 0; idx-- {
		var plane *Image
		state := &GRDProgressiveState{
			Image:    &plane,
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
		if status != CodecStatusFinished || plane == nil || plane.data == nil {
			return nil, errors.New("jbig2: failed to decode halftone plane")
		}
		gsplanes[idx] = plane
		if idx < int(gsbpp)-1 {
			if !gsplanes[idx].ComposeFrom(0, 0, gsplanes[idx+1], ComposeXOR) {
				return nil, errors.New("jbig2: failed to combine halftone planes")
			}
		}
	}

	return p.decodeImage(gsplanes)
}

// DecodeMMR decodes a halftone region using MMR compression.
func (p *HTRDProc) DecodeMMR(stream *BitStream) (*Image, error) {
	if stream == nil {
		return nil, errors.New("jbig2: nil bitstream for halftone region")
	}

	// Calculate bits per pattern
	hbpp := uint32(1)
	for (uint32(1) << hbpp) < p.HNumPats {
		hbpp++
	}

	gsbpp := uint8(hbpp)

	// Create GRD processor for halftone region
	grd := NewGRDProc()
	grd.MMR = p.HMMR
	grd.GBWidth = p.HGWidth
	grd.GBHeight = p.HGHeight

	// Create planes
	gsplanes := make([]*Image, gsbpp)

	// Decode first plane
	status, err := grd.StartDecodeMMR(&gsplanes[gsbpp-1], stream)
	if err != nil || status != CodecStatusFinished {
		return nil, errors.New("jbig2: failed to decode MMR halftone plane")
	}
	if gsplanes[gsbpp-1] == nil {
		return nil, errors.New("jbig2: failed to decode MMR halftone plane")
	}

	stream.AlignByte()
	stream.AddOffset(3)

	// Decode remaining planes
	for j := int(gsbpp) - 2; j >= 0; j-- {
		status, err := grd.StartDecodeMMR(&gsplanes[j], stream)
		if err != nil || status != CodecStatusFinished {
			return nil, errors.New("jbig2: failed to decode MMR halftone plane")
		}
		if gsplanes[j] == nil {
			return nil, errors.New("jbig2: failed to decode MMR halftone plane")
		}

		stream.AlignByte()
		stream.AddOffset(3)

		if !gsplanes[j].ComposeFrom(0, 0, gsplanes[j+1], ComposeXOR) {
			return nil, errors.New("jbig2: failed to combine MMR halftone planes")
		}
	}

	return p.decodeImage(gsplanes)
}

func (p *HTRDProc) decodeImage(gsplanes []*Image) (*Image, error) {
	if p.HNumPats == 0 {
		return nil, errors.New("jbig2: halftone pattern dictionary is empty")
	}
	htreg := NewImage(int32(p.HBWidth), int32(p.HBHeight))
	if htreg == nil || htreg.data == nil {
		return nil, errors.New("jbig2: failed to allocate halftone region image")
	}
	htreg.Fill(p.HDefPixel)

	for mg := uint32(0); mg < p.HGHeight; mg++ {
		for ng := uint32(0); ng < p.HGWidth; ng++ {
			patternIndex := uint32(0)
			for plane := 0; plane < len(gsplanes); plane++ {
				if gsplanes[plane] == nil {
					return nil, errors.New("jbig2: missing halftone plane")
				}
				bit := gsplanes[plane].GetPixel(int32(ng), int32(mg))
				patternIndex |= uint32(bit) << uint(plane)
			}
			if patternIndex >= p.HNumPats {
				patternIndex = p.HNumPats - 1
			}
			pattern := p.HPats[patternIndex]
			if pattern == nil {
				continue
			}
			mgInt := int64(mg)
			ngInt := int64(ng)
			x := (int64(p.HGX) + mgInt*int64(p.HRY) + ngInt*int64(p.HRX)) >> 8
			y := (int64(p.HGY) + mgInt*int64(p.HRX) - ngInt*int64(p.HRY)) >> 8
			pattern.ComposeTo(htreg, x, y, p.HCombOp)
		}
	}

	return htreg, nil
}
