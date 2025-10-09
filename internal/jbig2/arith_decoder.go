package jbig2

import (
	"errors"
)

const defaultAValue = 0x8000

var errArithDecoderComplete = errors.New("jbig2: arithmetic decoder exhausted")

// arithQe matches the probability table entries used by the JBIG2 arithmetic decoder.
type arithQe struct {
	qe      uint16
	nmps    uint8
	nlps    uint8
	switchM bool
}

var arithQeTable = [...]arithQe{
	{0x5601, 1, 1, true}, {0x3401, 2, 6, false}, {0x1801, 3, 9, false},
	{0x0AC1, 4, 12, false}, {0x0521, 5, 29, false}, {0x0221, 38, 33, false},
	{0x5601, 7, 6, true}, {0x5401, 8, 14, false}, {0x4801, 9, 14, false},
	{0x3801, 10, 14, false}, {0x3001, 11, 17, false}, {0x2401, 12, 18, false},
	{0x1C01, 13, 20, false}, {0x1601, 29, 21, false}, {0x5601, 15, 14, true},
	{0x5401, 16, 14, false}, {0x5101, 17, 15, false}, {0x4801, 18, 16, false},
	{0x3801, 19, 17, false}, {0x3401, 20, 18, false}, {0x3001, 21, 19, false},
	{0x2801, 22, 19, false}, {0x2401, 23, 20, false}, {0x2201, 24, 21, false},
	{0x1C01, 25, 22, false}, {0x1801, 26, 23, false}, {0x1601, 27, 24, false},
	{0x1401, 28, 25, false}, {0x1201, 29, 26, false}, {0x1101, 30, 27, false},
	{0x0AC1, 31, 28, false}, {0x09C1, 32, 29, false}, {0x08A1, 33, 30, false},
	{0x0521, 34, 31, false}, {0x0441, 35, 32, false}, {0x02A1, 36, 33, false},
	{0x0221, 37, 34, false}, {0x0141, 38, 35, false}, {0x0111, 39, 36, false},
	{0x0085, 40, 37, false}, {0x0049, 41, 38, false}, {0x0025, 42, 39, false},
	{0x0015, 43, 40, false}, {0x0009, 44, 41, false}, {0x0005, 45, 42, false},
	{0x0001, 45, 43, false}, {0x5601, 46, 46, false},
}

// ArithContext stores the probability state for the arithmetic decoder.
type ArithContext struct {
	mps bool
	i   uint8
}

// NewArithContext constructs a context initialised to the default state.
func NewArithContext() ArithContext { return ArithContext{} }

// Index returns the current state index.
func (ctx *ArithContext) Index() uint8 { return ctx.i }

// SetIndex allows tests or higher-level code to seed the state machine.
func (ctx *ArithContext) SetIndex(i uint8) { ctx.i = i }

// MPS returns the most probable symbol for the context.
func (ctx *ArithContext) MPS() int {
	if ctx.mps {
		return 1
	}
	return 0
}

func (ctx *ArithContext) decodeNLPS(qe arithQe) int {
	D := 0
	if ctx.mps {
		D = 0
	} else {
		D = 1
	}
	if qe.switchM {
		ctx.mps = !ctx.mps
	}
	ctx.i = qe.nlps
	return D
}

func (ctx *ArithContext) decodeNMPS(qe arithQe) int {
	ctx.i = qe.nmps
	return ctx.MPS()
}

// ArithDecoder mirrors CJBig2_ArithDecoder and performs binary arithmetic decoding.
type ArithDecoder struct {
	stream   *BitStream
	complete bool
	state    arithStreamState
	b        uint8
	c        uint32
	a        uint32
	ct       uint32
}

type arithStreamState uint8

const (
	streamDataAvailable arithStreamState = iota
	streamDecodingFinished
	streamLooping
)

// NewArithDecoder prepares a decoder over the supplied bitstream.
func NewArithDecoder(stream *BitStream) *ArithDecoder {
	dec := &ArithDecoder{stream: stream}
	dec.b = stream.CurByteArith()
	dec.c = uint32(dec.b^0xFF) << 16
	dec.byteIn()
	dec.c <<= 7
	if dec.ct >= 7 {
		dec.ct -= 7
	} else {
		dec.ct = 0
	}
	dec.a = defaultAValue
	return dec
}

// Decode consumes the next bit using the supplied probability context.
func (dec *ArithDecoder) Decode(ctx *ArithContext) (int, error) {
	if dec.complete {
		return 0, errArithDecoderComplete
	}

	index := ctx.Index()
	if int(index) >= len(arithQeTable) {
		return 0, errors.New("jbig2: arithmetic context index out of range")
	}

	qe := arithQeTable[index]
	dec.a -= uint32(qe.qe)
	if (dec.c >> 16) < dec.a {
		if dec.a&defaultAValue != 0 {
			return ctx.MPS(), nil
		}

		var D int
		if dec.a < uint32(qe.qe) {
			D = ctx.decodeNLPS(qe)
		} else {
			D = ctx.decodeNMPS(qe)
		}
		dec.readValueA()
		return D, nil
	}

	dec.c -= dec.a << 16
	var D int
	if dec.a < uint32(qe.qe) {
		D = ctx.decodeNMPS(qe)
	} else {
		D = ctx.decodeNLPS(qe)
	}
	dec.a = uint32(qe.qe)
	dec.readValueA()
	return D, nil
}

// IsComplete reports when the decoder has reached an end state.
func (dec *ArithDecoder) IsComplete() bool { return dec.complete }

func (dec *ArithDecoder) byteIn() {
	if dec.b == 0xFF {
		b1 := dec.stream.NextByteArith()
		if b1 > 0x8F {
			dec.ct = 8
			switch dec.state {
			case streamDataAvailable:
				dec.state = streamDecodingFinished
			case streamDecodingFinished:
				dec.state = streamLooping
			case streamLooping:
				dec.complete = true
			}
		} else {
			dec.stream.IncByte()
			dec.b = b1
			dec.c = dec.c + 0xFE00 - (uint32(dec.b) << 9)
			dec.ct = 7
		}
	} else {
		dec.stream.IncByte()
		dec.b = dec.stream.CurByteArith()
		dec.c = dec.c + 0xFF00 - (uint32(dec.b) << 8)
		dec.ct = 8
	}

	if !dec.stream.InBounds() {
		dec.complete = true
	}
}

func (dec *ArithDecoder) readValueA() {
	for {
		if dec.ct == 0 {
			dec.byteIn()
		}
		dec.a <<= 1
		dec.c <<= 1
		dec.ct--
		if dec.a&defaultAValue != 0 {
			return
		}
	}
}
