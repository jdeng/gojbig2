package jbig2

import (
	"errors"
	"fmt"

	"github.com/jdeng/gojbig2/internal/fax"
)

// CodecStatus mirrors the FXCODEC_STATUS enum used by the reference decoder.
type CodecStatus int

const (
	CodecStatusReady CodecStatus = iota
	CodecStatusToBeContinued
	CodecStatusFinished
	CodecStatusError
)

// GRDProc implements the JBIG2 generic region decoding parameters.
type GRDProc struct {
	MMR         bool
	TPGDON      bool
	UseSkip     bool
	GBTemplate  uint8
	GBWidth     uint32
	GBHeight    uint32
	Skip        *Image
	GBAt        [8]int32
	replaceRect Rect

	progressiveStatus CodecStatus
	decodeType        int
	loopIndex         int
	ltp               int
}

// NewGRDProc constructs a generic region decoder configuration.
func NewGRDProc() *GRDProc { return &GRDProc{} }

// ReplaceRect returns the rectangle within the page that should be replaced by the decoded image.
func (p *GRDProc) ReplaceRect() Rect { return p.replaceRect }

// DecodeArith decodes a generic region using arithmetic coding (non-progressive path).
func (p *GRDProc) DecodeArith(decoder *ArithDecoder, contexts []ArithContext) (*Image, error) {
	if decoder == nil {
		return nil, errors.New("jbig2: GRDProc requires a non-nil arithmetic decoder")
	}
	if !IsValidImageSize(int32(p.GBWidth), int32(p.GBHeight)) {
		return NewImage(int32(p.GBWidth), int32(p.GBHeight)), nil
	}

	switch p.GBTemplate {
	case 0, 1, 2:
		if p.useOptimisedTemplate(int(p.GBTemplate)) {
			return p.decodeArithOpt3(decoder, contexts, int(p.GBTemplate))
		}
		return p.decodeArithTemplateUnopt(decoder, contexts, int(p.GBTemplate))
	default:
		if p.useOptimisedTemplate3() {
			return p.decodeArithTemplate3Opt(decoder, contexts)
		}
		return p.decodeArithTemplate3Unopt(decoder, contexts)
	}
}

// StartDecodeArith sets up a progressive arithmetic decode.
func (p *GRDProc) StartDecodeArith(state *GRDProgressiveState) (CodecStatus, error) {
	if state == nil {
		return CodecStatusError, errors.New("jbig2: progressive state is nil")
	}
	if !IsValidImageSize(int32(p.GBWidth), int32(p.GBHeight)) {
		p.progressiveStatus = CodecStatusFinished
		p.replaceRect = Rect{}
		return CodecStatusFinished, nil
	}
	if state.Image == nil {
		return CodecStatusError, errors.New("jbig2: progressive state missing image handle")
	}
	if *state.Image == nil {
		*state.Image = NewImage(int32(p.GBWidth), int32(p.GBHeight))
	}
	img := *state.Image
	if img == nil || img.data == nil {
		return CodecStatusError, errors.New("jbig2: unable to allocate destination image")
	}
	img.Fill(false)
	p.decodeType = 1
	p.loopIndex = 0
	p.ltp = 0
	p.progressiveStatus = CodecStatusReady
	return p.continueArithmetic(state)
}

// StartDecodeMMR sets up an MMR-based decode.
func (p *GRDProc) StartDecodeMMR(img **Image, stream *BitStream) (CodecStatus, error) {
	if img == nil || *img != nil {
		return CodecStatusError, errors.New("jbig2: invalid image pointer for MMR decode")
	}

	image := NewImage(int32(p.GBWidth), int32(p.GBHeight))
	if image == nil || image.data == nil {
		return CodecStatusError, errors.New("jbig2: failed to allocate MMR image")
	}

	// Use FaxModule for MMR (G4) decoding
	bitPos := int(stream.BitPos())
	endBitPos := fax.FaxG4Decode(stream.Buf(), bitPos, int(p.GBWidth), int(p.GBHeight), image.stride, image.data)

	// Update stream position
	stream.SetBitPos(uint32(endBitPos))

	// Invert bits as per JBIG2 spec (FAX uses opposite polarity)
	for i := 0; i < len(image.data); i++ {
		image.data[i] = ^image.data[i]
	}

	p.replaceRect = Rect{Left: 0, Top: 0, Right: image.Width(), Bottom: image.Height()}

	*img = image
	return CodecStatusFinished, nil
}

// ContinueDecode advances a progressive decode.
func (p *GRDProc) ContinueDecode(state *GRDProgressiveState) (CodecStatus, error) {
	if p.decodeType != 1 {
		return CodecStatusError, errors.New("jbig2: unsupported progressive decode type")
	}
	return p.continueArithmetic(state)
}

// GRDProgressiveState tracks state required for progressive generic region decoding.
type GRDProgressiveState struct {
	Image    **Image
	Decoder  *ArithDecoder
	Contexts []ArithContext
	Pause    PauseIndicator
}

// PauseIndicator mimics the PauseIndicatorIface used by PDFium.
type PauseIndicator interface {
	ShouldPause() bool
}

var (
	optConstant1  = [...]uint16{0x9b25, 0x0795, 0x00e5}
	optConstant9  = [...]uint32{0x000c, 0x0009, 0x0007}
	optConstant10 = [...]uint32{0x0007, 0x000f, 0x0007}
	optConstant11 = [...]uint32{0x001f, 0x001f, 0x000f}
	optConstant12 = [...]uint32{0x000f, 0x0007, 0x0003}
)

var (
	optConstant2 = [...]uint32{0x0006, 0x0004, 0x0001}
	optConstant3 = [...]uint32{0xf800, 0x1e00, 0x0380}
	optConstant4 = [...]uint32{0x0000, 0x0001, 0x0003}
	optConstant5 = [...]uint32{0x07f0, 0x01f8, 0x007c}
	optConstant6 = [...]uint32{0x7bf7, 0x0efb, 0x01bd}
	optConstant7 = [...]uint32{0x0800, 0x0200, 0x0080}
	optConstant8 = [...]uint32{0x0010, 0x0008, 0x0004}
)

func (p *GRDProc) decodeArithTemplateUnopt(decoder *ArithDecoder, contexts []ArithContext, unopt int) (*Image, error) {
	img := NewImage(int32(p.GBWidth), int32(p.GBHeight))
	if img == nil || img.data == nil {
		return nil, errors.New("jbig2: failed to allocate generic region image")
	}

	img.Fill(false)
	ltp := 0
	mod2 := unopt % 2
	div2 := unopt / 2
	shift := 4 - unopt
	skipImage := p.Skip
	useSkip := p.UseSkip && skipImage != nil

	for h := uint32(0); h < p.GBHeight; h++ {
		if p.TPGDON {
			if decoder.IsComplete() {
				return nil, errArithDecoderComplete
			}
			ctx, err := getArithContext(contexts, optConstant1[unopt])
			if err != nil {
				return nil, err
			}
			val, err := decoder.Decode(ctx)
			if err != nil {
				return nil, err
			}
			ltp ^= val
		}

		if ltp != 0 {
			img.CopyLine(int32(h), int32(h-1))
			continue
		}

		line1 := uint32(img.GetPixel(int32(1+mod2), int32(h-2)))
		line1 |= uint32(img.GetPixel(int32(mod2), int32(h-2))) << 1
		if unopt == 1 {
			line1 |= uint32(img.GetPixel(0, int32(h-2))) << 2
		}
		line2 := uint32(img.GetPixel(int32(2-div2), int32(h-1)))
		line2 |= uint32(img.GetPixel(int32(1-div2), int32(h-1))) << 1
		if unopt < 2 {
			line2 |= uint32(img.GetPixel(0, int32(h-1))) << 2
		}
		line3 := uint32(0)

		for w := uint32(0); w < p.GBWidth; w++ {
			bVal := 0
			if !(useSkip && skipImage.GetPixel(int32(w), int32(h)) != 0) {
				if decoder.IsComplete() {
					return nil, errArithDecoderComplete
				}
				ctxVal := line3
				ctxVal |= uint32(img.GetPixel(int32(int32(w)+int32(p.GBAt[0])), int32(int32(h)+int32(p.GBAt[1])))) << uint(shift)
				ctxVal |= line2 << uint(shift+1)
				ctxVal |= line1 << uint(optConstant9[unopt])
				if unopt == 0 {
					ctxVal |= uint32(img.GetPixel(int32(int32(w)+int32(p.GBAt[2])), int32(int32(h)+int32(p.GBAt[3])))) << 10
					ctxVal |= uint32(img.GetPixel(int32(int32(w)+int32(p.GBAt[4])), int32(int32(h)+int32(p.GBAt[5])))) << 11
					ctxVal |= uint32(img.GetPixel(int32(int32(w)+int32(p.GBAt[6])), int32(int32(h)+int32(p.GBAt[7])))) << 15
				}
				if int(ctxVal) >= len(contexts) {
					return nil, fmt.Errorf("jbig2: context index %d exceeds available contexts (%d)", ctxVal, len(contexts))
				}
				val, err := decoder.Decode(&contexts[ctxVal])
				if err != nil {
					return nil, err
				}
				bVal = val
			}

			if bVal != 0 {
				img.SetPixel(int32(w), int32(h), bVal)
			}
			line1 = ((line1 << 1) | uint32(img.GetPixel(int32(int32(w)+2+int32(mod2)), int32(h-2)))) & optConstant10[unopt]
			line2 = ((line2 << 1) | uint32(img.GetPixel(int32(int32(w)+3-int32(div2)), int32(h-1)))) & optConstant11[unopt]
			line3 = ((line3 << 1) | uint32(bVal)) & optConstant12[unopt]
		}
	}

	return img, nil
}

func (p *GRDProc) decodeArithTemplate3Unopt(decoder *ArithDecoder, contexts []ArithContext) (*Image, error) {
	img := NewImage(int32(p.GBWidth), int32(p.GBHeight))
	if img == nil || img.data == nil {
		return nil, errors.New("jbig2: failed to allocate generic region image")
	}

	img.Fill(false)
	ltp := 0
	skipImage := p.Skip
	useSkip := p.UseSkip && skipImage != nil

	for h := uint32(0); h < p.GBHeight; h++ {
		if p.TPGDON {
			if decoder.IsComplete() {
				return nil, errArithDecoderComplete
			}
			ctx, err := getArithContext(contexts, 0x0195)
			if err != nil {
				return nil, err
			}
			val, err := decoder.Decode(ctx)
			if err != nil {
				return nil, err
			}
			ltp ^= val
		}

		if ltp != 0 {
			img.CopyLine(int32(h), int32(h-1))
			continue
		}

		line1 := uint32(img.GetPixel(1, int32(h-1)))
		line1 |= uint32(img.GetPixel(0, int32(h-1))) << 1
		line2 := uint32(0)

		for w := uint32(0); w < p.GBWidth; w++ {
			bVal := 0
			if !(useSkip && skipImage.GetPixel(int32(w), int32(h)) != 0) {
				ctxVal := line2
				ctxVal |= uint32(img.GetPixel(int32(int32(w)+int32(p.GBAt[0])), int32(int32(h)+int32(p.GBAt[1])))) << 4
				ctxVal |= line1 << 5
				if decoder.IsComplete() {
					return nil, errArithDecoderComplete
				}
				if int(ctxVal) >= len(contexts) {
					return nil, fmt.Errorf("jbig2: context index %d exceeds available contexts (%d)", ctxVal, len(contexts))
				}
				val, err := decoder.Decode(&contexts[ctxVal])
				if err != nil {
					return nil, err
				}
				bVal = val
			}
			if bVal != 0 {
				img.SetPixel(int32(w), int32(h), bVal)
			}
			line1 = ((line1 << 1) | uint32(img.GetPixel(int32(int32(w)+2), int32(h-1)))) & 0x1f
			line2 = ((line2 << 1) | uint32(bVal)) & 0x0f
		}
	}

	return img, nil
}

func getArithContext(contexts []ArithContext, idx uint16) (*ArithContext, error) {
	if int(idx) >= len(contexts) {
		return nil, fmt.Errorf("jbig2: context index %d exceeds available contexts (%d)", idx, len(contexts))
	}
	return &contexts[idx], nil
}

func (p *GRDProc) processTemplateOptLine(img *Image, decoder *ArithDecoder, contexts []ArithContext, h int, opt int) error {
	line := img.data
	stride := img.stride
	lineBytes := int((p.GBWidth + 7) >> 3)
	if lineBytes == 0 {
		lineBytes = 1
	}
	bitsLeft := int(p.GBWidth) - (lineBytes-1)*8
	offset := h * stride
	current := line[offset:]
	if h <= 1 {
		return p.processTemplateUnoptLine(img, decoder, contexts, h, opt)
	}
	line1 := line[offset-(stride<<1):]
	line2 := line[offset-stride:]

	if p.TPGDON {
		ctx, err := getArithContext(contexts, optConstant1[opt])
		if err != nil {
			return err
		}
		if decoder.IsComplete() {
			return errArithDecoderComplete
		}
		val, err := decoder.Decode(ctx)
		if err != nil {
			return err
		}
		p.ltp ^= val
	}

	if p.ltp != 0 {
		copy(current[:lineBytes], line[offset-stride:offset-stride+lineBytes])
		return nil
	}

	var l1, l2 uint32
	l1 = uint32(line1[0]) << optConstant2[opt]
	l2 = uint32(line2[0])
	contextVal := (l1 & optConstant3[opt]) | ((l2 >> optConstant4[opt]) & optConstant5[opt])
	lastByte := lineBytes - 1
	for cc := 0; cc < lastByte; cc++ {
		l1 = (l1 << 8) | (uint32(line1[cc+1]) << optConstant2[opt])
		l2 = (l2 << 8) | uint32(line2[cc+1])
		var cVal byte
		for k := 7; k >= 0; k-- {
			if decoder.IsComplete() {
				return errArithDecoderComplete
			}
			idx := contextVal
			if int(idx) >= len(contexts) {
				return fmt.Errorf("jbig2: context index %d exceeds available contexts (%d)", idx, len(contexts))
			}
			val, err := decoder.Decode(&contexts[idx])
			if err != nil {
				return err
			}
			cVal |= byte(val << k)
			contextVal = (((contextVal & optConstant6[opt]) << 1) | uint32(val) | ((l1 >> uint(k)) & optConstant7[opt]) | ((l2 >> uint(k+int(optConstant4[opt]))) & optConstant8[opt]))
		}
		current[cc] = cVal
	}

	l1 <<= 8
	l2 <<= 8
	var cVal byte
	for k := 0; k < bitsLeft; k++ {
		if decoder.IsComplete() {
			return errArithDecoderComplete
		}
		idx := contextVal
		if int(idx) >= len(contexts) {
			return fmt.Errorf("jbig2: context index %d exceeds available contexts (%d)", idx, len(contexts))
		}
		val, err := decoder.Decode(&contexts[idx])
		if err != nil {
			return err
		}
		cVal |= byte(val << (7 - k))
		contextVal = (((contextVal & optConstant6[opt]) << 1) | uint32(val) | ((l1 >> uint(7-k)) & optConstant7[opt]) | ((l2 >> uint(7+int(optConstant4[opt])-k)) & optConstant8[opt]))
	}
	current[lastByte] = cVal
	return nil
}

func (p *GRDProc) processTemplate3OptLine(img *Image, decoder *ArithDecoder, contexts []ArithContext, h int) error {
	line := img.data
	stride := img.stride
	lineBytes := int((p.GBWidth + 7) >> 3)
	if lineBytes == 0 {
		lineBytes = 1
	}
	bitsLeft := int(p.GBWidth) - (lineBytes-1)*8
	offset := h * stride
	current := line[offset:]

	if p.TPGDON {
		ctx, err := getArithContext(contexts, 0x0195)
		if err != nil {
			return err
		}
		if decoder.IsComplete() {
			return errArithDecoderComplete
		}
		val, err := decoder.Decode(ctx)
		if err != nil {
			return err
		}
		p.ltp ^= val
	}

	if p.ltp != 0 {
		copy(current[:lineBytes], line[offset-stride:offset-stride+lineBytes])
		return nil
	}

	if h > 0 {
		prev := line[offset-stride:]
		line1 := uint32(prev[0])
		contextVal := (line1 >> 1) & 0x03f0
		lastByte := lineBytes - 1
		for cc := 0; cc < lastByte; cc++ {
			line1 = (line1 << 8) | uint32(prev[cc+1])
			var cVal byte
			for k := 7; k >= 0; k-- {
				if decoder.IsComplete() {
					return errArithDecoderComplete
				}
				idx := contextVal
				if int(idx) >= len(contexts) {
					return fmt.Errorf("jbig2: context index %d exceeds available contexts (%d)", idx, len(contexts))
				}
				val, err := decoder.Decode(&contexts[idx])
				if err != nil {
					return err
				}
				cVal |= byte(val << k)
				contextVal = ((contextVal & 0x01f7) << 1) | uint32(val) | ((line1 >> uint(k+1)) & 0x0010)
			}
			current[cc] = cVal
		}
		line1 <<= 8
		var cVal byte
		for k := 0; k < bitsLeft; k++ {
			if decoder.IsComplete() {
				return errArithDecoderComplete
			}
			idx := contextVal
			if int(idx) >= len(contexts) {
				return fmt.Errorf("jbig2: context index %d exceeds available contexts (%d)", idx, len(contexts))
			}
			val, err := decoder.Decode(&contexts[idx])
			if err != nil {
				return err
			}
			cVal |= byte(val << (7 - k))
			contextVal = ((contextVal & 0x01f7) << 1) | uint32(val) | ((line1 >> uint(8-k)) & 0x0010)
		}
		current[lineBytes-1] = cVal
		return nil
	}

	contextVal := uint32(0)
	lastByte := lineBytes - 1
	for cc := 0; cc < lastByte; cc++ {
		var cVal byte
		for k := 7; k >= 0; k-- {
			if decoder.IsComplete() {
				return errArithDecoderComplete
			}
			idx := contextVal
			if int(idx) >= len(contexts) {
				return fmt.Errorf("jbig2: context index %d exceeds available contexts (%d)", idx, len(contexts))
			}
			val, err := decoder.Decode(&contexts[idx])
			if err != nil {
				return err
			}
			cVal |= byte(val << k)
			contextVal = ((contextVal & 0x01f7) << 1) | uint32(val)
		}
		current[cc] = cVal
	}
	var cVal byte
	for k := 0; k < bitsLeft; k++ {
		if decoder.IsComplete() {
			return errArithDecoderComplete
		}
		idx := contextVal
		if int(idx) >= len(contexts) {
			return fmt.Errorf("jbig2: context index %d exceeds available contexts (%d)", idx, len(contexts))
		}
		val, err := decoder.Decode(&contexts[idx])
		if err != nil {
			return err
		}
		cVal |= byte(val << (7 - k))
		contextVal = ((contextVal & 0x01f7) << 1) | uint32(val)
	}
	current[lineBytes-1] = cVal
	return nil
}

func (p *GRDProc) continueArithmetic(state *GRDProgressiveState) (CodecStatus, error) {
	if state == nil {
		return CodecStatusError, errors.New("jbig2: progressive state is nil")
	}
	if state.Decoder == nil {
		return CodecStatusError, errors.New("jbig2: progressive decode requires an arithmetic decoder")
	}
	if state.Image == nil || *state.Image == nil {
		return CodecStatusError, errors.New("jbig2: progressive decode missing destination image")
	}

	img := *state.Image
	startLine := p.loopIndex
	for p.loopIndex < int(p.GBHeight) {
		var err error
		switch p.GBTemplate {
		case 0, 1, 2:
			if p.useOptimisedTemplate(int(p.GBTemplate)) {
				err = p.processTemplateOptLine(img, state.Decoder, state.Contexts, p.loopIndex, int(p.GBTemplate))
			} else {
				err = p.processTemplateUnoptLine(img, state.Decoder, state.Contexts, p.loopIndex, int(p.GBTemplate))
			}
		case 3:
			if p.useOptimisedTemplate3() {
				err = p.processTemplate3OptLine(img, state.Decoder, state.Contexts, p.loopIndex)
			} else {
				err = p.processTemplate3UnoptLine(img, state.Decoder, state.Contexts, p.loopIndex)
			}
		default:
			err = p.processTemplate3UnoptLine(img, state.Decoder, state.Contexts, p.loopIndex)
		}
		if err != nil {
			p.progressiveStatus = CodecStatusError
			return CodecStatusError, err
		}
		p.loopIndex++
		if state.Pause != nil && state.Pause.ShouldPause() {
			p.progressiveStatus = CodecStatusToBeContinued
			p.replaceRect = Rect{Left: 0, Top: startLine, Right: img.Width(), Bottom: p.loopIndex}
			return CodecStatusToBeContinued, nil
		}
	}

	p.progressiveStatus = CodecStatusFinished
	p.replaceRect = Rect{Left: 0, Top: 0, Right: img.Width(), Bottom: img.Height()}
	return CodecStatusFinished, nil
}

func (p *GRDProc) processTemplateUnoptLine(img *Image, decoder *ArithDecoder, contexts []ArithContext, h int, unopt int) error {
	if p.TPGDON {
		ctx, err := getArithContext(contexts, optConstant1[unopt])
		if err != nil {
			return err
		}
		if decoder.IsComplete() {
			return errArithDecoderComplete
		}
		val, err := decoder.Decode(ctx)
		if err != nil {
			return err
		}
		p.ltp ^= val
	}

	if p.ltp != 0 {
		if h > 0 {
			img.CopyLine(int32(h), int32(h-1))
		}
		return nil
	}

	mod2 := unopt % 2
	div2 := unopt / 2
	shift := 4 - unopt
	skipImage := p.Skip
	useSkip := p.UseSkip && skipImage != nil

	line1 := uint32(img.GetPixel(int32(1+mod2), int32(h-2)))
	line1 |= uint32(img.GetPixel(int32(mod2), int32(h-2))) << 1
	if unopt == 1 {
		line1 |= uint32(img.GetPixel(0, int32(h-2))) << 2
	}
	line2 := uint32(img.GetPixel(int32(2-div2), int32(h-1)))
	line2 |= uint32(img.GetPixel(int32(1-div2), int32(h-1))) << 1
	if unopt < 2 {
		line2 |= uint32(img.GetPixel(0, int32(h-1))) << 2
	}
	line3 := uint32(0)

	for w := uint32(0); w < p.GBWidth; w++ {
		bVal := 0
		if !(useSkip && skipImage.GetPixel(int32(w), int32(h)) != 0) {
			if decoder.IsComplete() {
				return errArithDecoderComplete
			}
			ctxVal := line3
			ctxVal |= uint32(img.GetPixel(int32(int(w)+int(p.GBAt[0])), int32(h+int(p.GBAt[1])))) << uint(shift)
			ctxVal |= line2 << uint(shift+1)
			ctxVal |= line1 << uint(optConstant9[unopt])
			if unopt == 0 {
				ctxVal |= uint32(img.GetPixel(int32(int(w)+int(p.GBAt[2])), int32(h+int(p.GBAt[3])))) << 10
				ctxVal |= uint32(img.GetPixel(int32(int(w)+int(p.GBAt[4])), int32(h+int(p.GBAt[5])))) << 11
				ctxVal |= uint32(img.GetPixel(int32(int(w)+int(p.GBAt[6])), int32(h+int(p.GBAt[7])))) << 15
			}
			if int(ctxVal) >= len(contexts) {
				return fmt.Errorf("jbig2: context index %d exceeds available contexts (%d)", ctxVal, len(contexts))
			}
			val, err := decoder.Decode(&contexts[ctxVal])
			if err != nil {
				return err
			}
			bVal = val
		}
		if bVal != 0 {
			img.SetPixel(int32(w), int32(h), bVal)
		}
		line1 = ((line1 << 1) | uint32(img.GetPixel(int32(int(w)+2+int(mod2)), int32(h-2)))) & optConstant10[unopt]
		line2 = ((line2 << 1) | uint32(img.GetPixel(int32(int(w)+3-int(div2)), int32(h-1)))) & optConstant11[unopt]
		line3 = ((line3 << 1) | uint32(bVal)) & optConstant12[unopt]
	}

	return nil
}

func (p *GRDProc) processTemplate3UnoptLine(img *Image, decoder *ArithDecoder, contexts []ArithContext, h int) error {
	if p.TPGDON {
		ctx, err := getArithContext(contexts, 0x0195)
		if err != nil {
			return err
		}
		if decoder.IsComplete() {
			return errArithDecoderComplete
		}
		val, err := decoder.Decode(ctx)
		if err != nil {
			return err
		}
		p.ltp ^= val
	}

	if p.ltp != 0 {
		if h > 0 {
			img.CopyLine(int32(h), int32(h-1))
		}
		return nil
	}

	skipImage := p.Skip
	useSkip := p.UseSkip && skipImage != nil

	line1 := uint32(img.GetPixel(1, int32(h-1)))
	line1 |= uint32(img.GetPixel(0, int32(h-1))) << 1
	line2 := uint32(0)

	for w := uint32(0); w < p.GBWidth; w++ {
		bVal := 0
		if !(useSkip && skipImage.GetPixel(int32(w), int32(h)) != 0) {
			if decoder.IsComplete() {
				return errArithDecoderComplete
			}
			ctxVal := line2
			ctxVal |= uint32(img.GetPixel(int32(int(w)+int(p.GBAt[0])), int32(h+int(p.GBAt[1])))) << 4
			ctxVal |= line1 << 5
			if int(ctxVal) >= len(contexts) {
				return fmt.Errorf("jbig2: context index %d exceeds available contexts (%d)", ctxVal, len(contexts))
			}
			val, err := decoder.Decode(&contexts[ctxVal])
			if err != nil {
				return err
			}
			bVal = val
		}
		if bVal != 0 {
			img.SetPixel(int32(w), int32(h), bVal)
		}
		line1 = ((line1 << 1) | uint32(img.GetPixel(int32(int(w)+2), int32(h-1)))) & 0x1f
		line2 = ((line2 << 1) | uint32(bVal)) & 0x0f
	}
	return nil
}

func (p *GRDProc) useOptimisedTemplate(template int) bool {
	switch template {
	case 0:
		return p.GBAt == [8]int32{3, -1, -3, -1, 2, -2, -2, -2} && !p.UseSkip
	case 1:
		return p.GBAt[0] == 3 && p.GBAt[1] == -1 && !p.UseSkip
	case 2:
		return p.GBAt[0] == 2 && p.GBAt[1] == -1 && !p.UseSkip
	default:
		return false
	}
}

func (p *GRDProc) useOptimisedTemplate3() bool {
	return p.GBAt[0] == 2 && p.GBAt[1] == -1 && !p.UseSkip
}

func (p *GRDProc) decodeArithOpt3(decoder *ArithDecoder, contexts []ArithContext, opt int) (*Image, error) {
	img := NewImage(int32(p.GBWidth), int32(p.GBHeight))
	if img == nil || img.data == nil {
		return nil, errors.New("jbig2: failed to allocate generic region image")
	}

	ltp := 0
	line := img.data
	stride := img.stride
	lineBytes := int((p.GBWidth + 7) >> 3)
	if lineBytes == 0 {
		lineBytes = 1
	}
	lastByte := lineBytes - 1
	bitsLeft := int(p.GBWidth) - lastByte*8

	if opt == 0 {
		// OPT 0 adjusts the height to handle LTP behaviour.
		height := int(p.GBHeight & 0x7fffffff)
		if height < 0 || height > int(p.GBHeight) {
			height = int(p.GBHeight)
		}
		if err := p.decodeOptLoops(decoder, contexts, img, line, stride, lineBytes, lastByte, bitsLeft, opt, height, &ltp); err != nil {
			return nil, err
		}
		return img, nil
	}

	if err := p.decodeOptLoops(decoder, contexts, img, line, stride, lineBytes, lastByte, bitsLeft, opt, int(p.GBHeight), &ltp); err != nil {
		return nil, err
	}
	return img, nil
}

func (p *GRDProc) decodeOptLoops(decoder *ArithDecoder, contexts []ArithContext, img *Image, line []byte, stride, lineBytes, lastByte, bitsLeft, opt int, height int, ltp *int) error {
	for h := 0; h < height; h++ {
		if p.TPGDON {
			ctx, err := getArithContext(contexts, optConstant1[opt])
			if err != nil {
				return err
			}
			if decoder.IsComplete() {
				return errArithDecoderComplete
			}
			val, err := decoder.Decode(ctx)
			if err != nil {
				return err
			}
			*ltp ^= val
		}

		offset := h * stride
		current := line[offset:]

		if *ltp != 0 {
			if h > 0 {
				copy(current[:lineBytes], line[offset-stride:offset-stride+lineBytes])
			}
			continue
		}

		if h > 1 {
			line1 := line[offset-(stride<<1):]
			line2 := line[offset-stride:]
			var l1 = (uint32(line1[0]) << optConstant2[opt])
			var l2 = uint32(line2[0])
			contextVal := (l1 & optConstant3[opt]) | ((l2 >> optConstant4[opt]) & optConstant5[opt])
			for cc := 0; cc < lastByte; cc++ {
				l1 = (l1 << 8) | (uint32(line1[cc+1]) << optConstant2[opt])
				l2 = (l2 << 8) | uint32(line2[cc+1])
				var cVal byte
				for k := 7; k >= 0; k-- {
					if decoder.IsComplete() {
						return errArithDecoderComplete
					}
					idx := contextVal
					if int(idx) >= len(contexts) {
						return fmt.Errorf("jbig2: context index %d exceeds available contexts (%d)", idx, len(contexts))
					}
					val, err := decoder.Decode(&contexts[idx])
					if err != nil {
						return err
					}
					cVal |= byte(val << k)
					contextVal = (((contextVal & optConstant6[opt]) << 1) | uint32(val) | ((l1 >> k) & optConstant7[opt]) | ((l2 >> uint(k+int(optConstant4[opt]))) & optConstant8[opt]))
				}
				current[cc] = cVal
			}
			l1 <<= 8
			l2 <<= 8
			var cVal byte
			for k := 0; k < bitsLeft; k++ {
				if decoder.IsComplete() {
					return errArithDecoderComplete
				}
				idx := contextVal
				if int(idx) >= len(contexts) {
					return fmt.Errorf("jbig2: context index %d exceeds available contexts (%d)", idx, len(contexts))
				}
				val, err := decoder.Decode(&contexts[idx])
				if err != nil {
					return err
				}
				cVal |= byte(val << (7 - k))
				contextVal = (((contextVal & optConstant6[opt]) << 1) | uint32(val) | ((l1 >> uint(7-k)) & optConstant7[opt]) | ((l2 >> uint(7+int(optConstant4[opt])-k)) & optConstant8[opt]))
			}
			current[lastByte] = cVal
		} else {
			var l2 uint32
			if h&1 == 1 {
				l2 = uint32(line[(offset - stride)])
			}
			contextVal := (l2 >> optConstant4[opt]) & optConstant5[opt]
			for cc := 0; cc < lastByte; cc++ {
				if h&1 == 1 {
					l2 = (l2 << 8) | uint32(line[offset-stride+cc+1])
				}
				var cVal byte
				for k := 7; k >= 0; k-- {
					if decoder.IsComplete() {
						return errArithDecoderComplete
					}
					idx := contextVal
					if int(idx) >= len(contexts) {
						return fmt.Errorf("jbig2: context index %d exceeds available contexts (%d)", idx, len(contexts))
					}
					val, err := decoder.Decode(&contexts[idx])
					if err != nil {
						return err
					}
					cVal |= byte(val << k)
					contextVal = (((contextVal & optConstant6[opt]) << 1) | uint32(val) | ((l2 >> uint(k+int(optConstant4[opt]))) & optConstant8[opt]))
				}
				current[cc] = cVal
			}
			l2 <<= 8
			var cVal byte
			for k := 0; k < bitsLeft; k++ {
				if decoder.IsComplete() {
					return errArithDecoderComplete
				}
				idx := contextVal
				if int(idx) >= len(contexts) {
					return fmt.Errorf("jbig2: context index %d exceeds available contexts (%d)", idx, len(contexts))
				}
				val, err := decoder.Decode(&contexts[idx])
				if err != nil {
					return err
				}
				cVal |= byte(val << (7 - k))
				contextVal = (((contextVal & optConstant6[opt]) << 1) | uint32(val) | ((l2 >> uint(7+int(optConstant4[opt])-k)) & optConstant8[opt]))
			}
			current[lastByte] = cVal
		}
	}
	return nil
}

func (p *GRDProc) decodeArithTemplate3Opt(decoder *ArithDecoder, contexts []ArithContext) (*Image, error) {
	img := NewImage(int32(p.GBWidth), int32(p.GBHeight))
	if img == nil || img.data == nil {
		return nil, errors.New("jbig2: failed to allocate generic region image")
	}

	ltp := 0
	line := img.data
	stride := img.stride
	lineBytes := int((p.GBWidth + 7) >> 3)
	if lineBytes == 0 {
		lineBytes = 1
	}
	lastByte := lineBytes - 1
	bitsLeft := int(p.GBWidth) - lastByte*8

	for h := uint32(0); h < p.GBHeight; h++ {
		if p.TPGDON {
			ctx, err := getArithContext(contexts, 0x0195)
			if err != nil {
				return nil, err
			}
			if decoder.IsComplete() {
				return nil, errArithDecoderComplete
			}
			val, err := decoder.Decode(ctx)
			if err != nil {
				return nil, err
			}
			ltp ^= val
		}

		offset := int(h) * stride
		current := line[offset:]

		if ltp != 0 {
			if h > 0 {
				copy(current[:lineBytes], line[offset-stride:offset-stride+lineBytes])
			}
			continue
		}

		if h > 0 {
			prev := line[offset-stride:]
			line1 := uint32(prev[0])
			contextVal := (line1 >> 1) & 0x03f0
			for cc := 0; cc < lastByte; cc++ {
				line1 = (line1 << 8) | uint32(prev[cc+1])
				var cVal byte
				for k := 7; k >= 0; k-- {
					if decoder.IsComplete() {
						return nil, errArithDecoderComplete
					}
					idx := contextVal
					if int(idx) >= len(contexts) {
						return nil, fmt.Errorf("jbig2: context index %d exceeds available contexts (%d)", idx, len(contexts))
					}
					val, err := decoder.Decode(&contexts[idx])
					if err != nil {
						return nil, err
					}
					cVal |= byte(val << k)
					contextVal = ((contextVal & 0x01f7) << 1) | uint32(val) | ((line1 >> uint(k+1)) & 0x0010)
				}
				current[cc] = cVal
			}
			line1 <<= 8
			var cVal byte
			for k := 0; k < bitsLeft; k++ {
				if decoder.IsComplete() {
					return nil, errArithDecoderComplete
				}
				idx := contextVal
				if int(idx) >= len(contexts) {
					return nil, fmt.Errorf("jbig2: context index %d exceeds available contexts (%d)", idx, len(contexts))
				}
				val, err := decoder.Decode(&contexts[idx])
				if err != nil {
					return nil, err
				}
				cVal |= byte(val << (7 - k))
				contextVal = ((contextVal & 0x01f7) << 1) | uint32(val) | ((line1 >> uint(8-k)) & 0x0010)
			}
			current[lastByte] = cVal
		} else {
			contextVal := uint32(0)
			for cc := 0; cc < lastByte; cc++ {
				var cVal byte
				for k := 7; k >= 0; k-- {
					if decoder.IsComplete() {
						return nil, errArithDecoderComplete
					}
					idx := contextVal
					if int(idx) >= len(contexts) {
						return nil, fmt.Errorf("jbig2: context index %d exceeds available contexts (%d)", idx, len(contexts))
					}
					val, err := decoder.Decode(&contexts[idx])
					if err != nil {
						return nil, err
					}
					cVal |= byte(val << k)
					contextVal = ((contextVal & 0x01f7) << 1) | uint32(val)
				}
				current[cc] = cVal
			}
			var cVal byte
			for k := 0; k < bitsLeft; k++ {
				if decoder.IsComplete() {
					return nil, errArithDecoderComplete
				}
				idx := contextVal
				if int(idx) >= len(contexts) {
					return nil, fmt.Errorf("jbig2: context index %d exceeds available contexts (%d)", idx, len(contexts))
				}
				val, err := decoder.Decode(&contexts[idx])
				if err != nil {
					return nil, err
				}
				cVal |= byte(val << (7 - k))
				contextVal = ((contextVal & 0x01f7) << 1) | uint32(val)
			}
			current[lastByte] = cVal
		}
	}

	return img, nil
}
