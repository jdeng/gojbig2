package jbig2

import (
	"errors"
	"math"
)

type arithIntDecodeDatum struct {
	needBits int
	base     int
}

var arithIntDecodeData = []arithIntDecodeDatum{
	{2, 0},
	{4, 4},
	{6, 20},
	{8, 84},
	{12, 340},
	{32, 4436},
}

// ArithIntDecoder implements the JBIG2 arithmetic integer decoding procedure.
type ArithIntDecoder struct {
	ctx []ArithContext
}

// NewArithIntDecoder constructs a decoder with 512 probability contexts.
func NewArithIntDecoder() *ArithIntDecoder {
	dec := &ArithIntDecoder{ctx: make([]ArithContext, 512)}
	return dec
}

// Decode returns the decoded integer along with a boolean indicating whether
// the value is in-band (true) or represents the JBIG2 out-of-band condition.
func (dec *ArithIntDecoder) Decode(arith *ArithDecoder) (int, bool, error) {
	if arith == nil {
		return 0, false, errors.New("jbig2: arithmetic decoder is nil")
	}

	prev := 1
	s, err := arith.Decode(&dec.ctx[prev])
	if err != nil {
		return 0, false, err
	}
	prev = shiftOr(prev, s)

	depth, err := recursiveDecode(arith, dec.ctx, &prev, 0)
	if err != nil {
		return 0, false, err
	}

	temp := 0
	needBits := arithIntDecodeData[depth].needBits
	for i := 0; i < needBits; i++ {
		bit, err := arith.Decode(&dec.ctx[prev])
		if err != nil {
			return 0, false, err
		}
		prev = shiftOr(prev, bit)
		if prev >= 256 {
			prev = (prev & 0x1FF) | 0x100
		}
		temp = shiftOr(temp, bit)
	}

	value := int64(arithIntDecodeData[depth].base) + int64(temp)
	if value < math.MinInt32 || value > math.MaxInt32 {
		return 0, false, nil
	}

	result := int(value)
	if s == 1 && result > 0 {
		result = -result
	}

	return result, !(s == 1 && result == 0), nil
}

// ArithIaidDecoder handles arithmetic decoding of IAID codewords.
type ArithIaidDecoder struct {
	ctx []ArithContext
	len uint8
}

// NewArithIaidDecoder builds a decoder for codewords of length SBSYMCODELEN.
func NewArithIaidDecoder(symCodeLen uint8) *ArithIaidDecoder {
	size := 1 << symCodeLen
	return &ArithIaidDecoder{
		ctx: make([]ArithContext, size),
		len: symCodeLen,
	}
}

// Decode reads a value using the IAID procedure.
func (dec *ArithIaidDecoder) Decode(arith *ArithDecoder) (uint32, error) {
	if arith == nil {
		return 0, errors.New("jbig2: arithmetic decoder is nil")
	}

	prev := 1
	for i := uint8(0); i < dec.len; i++ {
		bit, err := arith.Decode(&dec.ctx[prev])
		if err != nil {
			return 0, err
		}
		prev = shiftOr(prev, bit)
	}

	return uint32(prev - (1 << dec.len)), nil
}

func shiftOr(val, bit int) int {
	return (val << 1) | bit
}

func recursiveDecode(arith *ArithDecoder, ctxs []ArithContext, prev *int, depth int) (int, error) {
	maxDepth := len(arithIntDecodeData) - 1
	if depth == maxDepth {
		return maxDepth, nil
	}

	bit, err := arith.Decode(&ctxs[*prev])
	if err != nil {
		return 0, err
	}
	*prev = shiftOr(*prev, bit)
	if bit == 0 {
		return depth, nil
	}
	return recursiveDecode(arith, ctxs, prev, depth+1)
}
