package jbig2

import "errors"

// GRRDProc holds parameters for generic refinement region decoding.
type GRRDProc struct {
	Template    bool
	TPGRON      bool
	Width       uint32
	Height      uint32
	ReferenceDX int32
	ReferenceDY int32
	Reference   *Image
	GRAT        [4]int8
}

// NewGRRDProc constructs an empty refinement region descriptor.
func NewGRRDProc() *GRRDProc { return &GRRDProc{} }

// Decode executes the refinement region decoding algorithm.
func (p *GRRDProc) Decode(decoder *ArithDecoder, contexts []ArithContext) (*Image, error) {
	if decoder == nil {
		return nil, errors.New("jbig2: nil arithmetic decoder for refinement region")
	}
	if p.Reference == nil {
		return nil, errors.New("jbig2: refinement region missing reference image")
	}
	if !IsValidImageSize(int32(p.Width), int32(p.Height)) {
		return NewImage(int32(p.Width), int32(p.Height)), nil
	}
	img := NewImage(int32(p.Width), int32(p.Height))
	if img == nil || img.data == nil {
		return nil, errors.New("jbig2: failed to allocate refinement image")
	}
	img.Fill(false)

	var err error
	if !p.Template {
		err = p.decodeTemplate0(decoder, contexts, img)
	} else {
		err = p.decodeTemplate1(decoder, contexts, img)
	}
	if err != nil {
		return nil, err
	}
	return img, nil
}

func (p *GRRDProc) decodeTemplate0(decoder *ArithDecoder, contexts []ArithContext, img *Image) error {
	refDX := p.ReferenceDX
	refDY := p.ReferenceDY
	for y := uint32(0); y < p.Height; y++ {
		if p.TPGRON {
			if _, err := p.decodeContextBit(decoder, contexts, 0x0010); err != nil {
				return err
			}
		}

		lines := [5]uint32{}
		lines[0] = uint32(img.GetPixel(1, int32(y)-1))
		lines[0] |= uint32(img.GetPixel(0, int32(y)-1)) << 1
		lines[1] = 0
		lines[2] = uint32(p.Reference.GetPixel(1-refDX, int32(y)-refDY-1))
		lines[2] |= uint32(p.Reference.GetPixel(-refDX, int32(y)-refDY-1)) << 1
		lines[3] = uint32(p.Reference.GetPixel(1-refDX, int32(y)-refDY))
		lines[3] |= uint32(p.Reference.GetPixel(-refDX, int32(y)-refDY)) << 1
		lines[3] |= uint32(p.Reference.GetPixel(-refDX-1, int32(y)-refDY)) << 2
		lines[4] = uint32(p.Reference.GetPixel(1-refDX, int32(y)-refDY+1))
		lines[4] |= uint32(p.Reference.GetPixel(-refDX, int32(y)-refDY+1)) << 1
		lines[4] |= uint32(p.Reference.GetPixel(-refDX-1, int32(y)-refDY+1)) << 2

		for x := uint32(0); x < p.Width; x++ {
			context := lines[4]
			context |= lines[3] << 3
			context |= lines[2] << 6
			referencePixel := p.Reference.GetPixel(int32(int32(x)+int32(p.GRAT[2])-refDX), int32(int32(y)+int32(p.GRAT[3])-refDY))
			context |= uint32(referencePixel) << 8
			context |= lines[1] << 9
			context |= lines[0] << 10
			context |= uint32(img.GetPixel(int32(int32(x)+int32(p.GRAT[0])), int32(int32(y)+int32(p.GRAT[1])))) << 12

			bit, err := p.decodeContextBit(decoder, contexts, context)
			if err != nil {
				return err
			}
			img.SetPixel(int32(x), int32(y), bit)

			lines[0] = ((lines[0] << 1) | uint32(img.GetPixel(int32(x)+2, int32(y)-1))) & 0x03
			lines[1] = ((lines[1] << 1) | uint32(bit)) & 0x01
			lines[2] = ((lines[2] << 1) | uint32(p.Reference.GetPixel(int32(x)-refDX+2, int32(y)-refDY-1))) & 0x03
			lines[3] = ((lines[3] << 1) | uint32(p.Reference.GetPixel(int32(x)-refDX+2, int32(y)-refDY))) & 0x07
			lines[4] = ((lines[4] << 1) | uint32(p.Reference.GetPixel(int32(x)-refDX+2, int32(y)-refDY+1))) & 0x07
		}
	}
	return nil
}

func (p *GRRDProc) decodeTemplate1(decoder *ArithDecoder, contexts []ArithContext, img *Image) error {
	refDX := p.ReferenceDX
	refDY := p.ReferenceDY
	for y := uint32(0); y < p.Height; y++ {
		if p.TPGRON {
			if _, err := p.decodeContextBit(decoder, contexts, 0x0004); err != nil {
				return err
			}
		}

		line1 := uint32(img.GetPixel(1, int32(y)-1))
		line1 |= uint32(img.GetPixel(0, int32(y)-1)) << 1
		line2 := uint32(p.Reference.GetPixel(1-refDX, int32(y)-refDY))
		line2 |= uint32(p.Reference.GetPixel(-refDX, int32(y)-refDY)) << 1

		for x := uint32(0); x < p.Width; x++ {
			context := line2
			context |= line1 << 2
			referencePixel := p.Reference.GetPixel(int32(int32(x)+int32(p.GRAT[2])-refDX), int32(int32(y)+int32(p.GRAT[3])-refDY))
			context |= uint32(referencePixel) << 4
			context |= uint32(img.GetPixel(int32(int32(x)+int32(p.GRAT[0])), int32(int32(y)+int32(p.GRAT[1])))) << 5

			bit, err := p.decodeContextBit(decoder, contexts, context)
			if err != nil {
				return err
			}
			img.SetPixel(int32(x), int32(y), bit)

			line1 = ((line1 << 1) | uint32(img.GetPixel(int32(x)+2, int32(y)-1))) & 0x07
			line2 = ((line2 << 1) | uint32(p.Reference.GetPixel(int32(x)-refDX+2, int32(y)-refDY))) & 0x03
		}
	}
	return nil
}

func (p *GRRDProc) decodeContextBit(decoder *ArithDecoder, contexts []ArithContext, idx uint32) (int, error) {
	if int(idx) >= len(contexts) {
		return 0, errors.New("jbig2: refinement context index out of range")
	}
	val, err := decoder.Decode(&contexts[idx])
	if err != nil {
		return 0, err
	}
	return val, nil
}
