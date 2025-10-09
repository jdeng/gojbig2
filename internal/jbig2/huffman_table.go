package jbig2

import (
	"errors"
)

const (
	int32Max = int32(1<<31 - 1)
	int32Min = -1 << 31
)

type tableLine struct {
	preflen  uint8
	rangeLen int
	rangeLow int
}

// HuffmanTable captures the canonical JBIG2 Huffman tables, including
// custom tables parsed from the bitstream.
type HuffmanTable struct {
	hasOOB   bool
	codes    []HuffmanCode
	rangeLen []int
	rangeLow []int
}

// NewStandardHuffmanTable constructs one of the predefined tables (indices 1-15).
func NewStandardHuffmanTable(idx int) (*HuffmanTable, error) {
	if idx <= 0 || idx >= len(builtinHuffmanTables) {
		return nil, errors.New("jbig2: invalid standard Huffman table index")
	}
	lineData := builtinHuffmanTables[idx]
	ht := &HuffmanTable{
		hasOOB:   lineData.htoob,
		codes:    make([]HuffmanCode, len(lineData.lines)),
		rangeLen: make([]int, len(lineData.lines)),
		rangeLow: make([]int, len(lineData.lines)),
	}
	for i, line := range lineData.lines {
		ht.codes[i] = HuffmanCode{CodeLength: int32(line.preflen)}
		ht.rangeLen[i] = line.rangeLen
		ht.rangeLow[i] = line.rangeLow
	}
	if err := assignHuffmanCodes(ht.codes); err != nil {
		return nil, err
	}
	return ht, nil
}

// NewHuffmanTableFromStream builds a Huffman table defined in the bitstream per Annex B.3.
func NewHuffmanTableFromStream(stream *BitStream) (*HuffmanTable, error) {
	if stream == nil {
		return nil, errors.New("jbig2: nil bitstream for Huffman table")
	}

	flag, err := stream.ReadByte()
	if err != nil {
		return nil, err
	}

	ht := &HuffmanTable{hasOOB: flag&0x01 != 0}
	htps := ((flag >> 1) & 0x07) + 1
	htrs := ((flag >> 4) & 0x07) + 1
	lowVal, err := stream.ReadUint32()
	if err != nil {
		return nil, err
	}
	highVal, err := stream.ReadUint32()
	if err != nil {
		return nil, err
	}
	low := int32(lowVal)
	high := int32(highVal)
	if low > high {
		return nil, errors.New("jbig2: invalid Huffman range bounds")
	}

	curLow := int64(low)
	upper := int64(high)
	for {
		codeLen, err := stream.ReadNBits(uint32(htps))
		if err != nil {
			return nil, err
		}
		rangeLen, err := stream.ReadNBits(uint32(htrs))
		if err != nil {
			return nil, err
		}
		if rangeLen >= 32 {
			return nil, errors.New("jbig2: Huffman range length overflow")
		}
		ht.appendEntry(int32(codeLen), int(rangeLen), int(curLow))
		span := int64(1) << rangeLen
		curLow += span
		if curLow < int64(int32Min) || curLow > int64(int32Max) {
			return nil, errors.New("jbig2: Huffman range exceeds int32 bounds")
		}
		if curLow >= upper {
			break
		}
	}

	codeLen, err := stream.ReadNBits(uint32(htps))
	if err != nil {
		return nil, err
	}
	if low == int32Min {
		return nil, errors.New("jbig2: Huffman low bound underflow")
	}
	ht.appendEntry(int32(codeLen), 32, int(low-1))

	codeLen, err = stream.ReadNBits(uint32(htps))
	if err != nil {
		return nil, err
	}
	ht.appendEntry(int32(codeLen), 32, int(high))

	if ht.hasOOB {
		codeLen, err = stream.ReadNBits(uint32(htps))
		if err != nil {
			return nil, err
		}
		ht.appendEntry(int32(codeLen), 0, 0)
	}

	if err := assignHuffmanCodes(ht.codes); err != nil {
		return nil, err
	}
	return ht, nil
}

// Size returns the number of entries in the table.
func (ht *HuffmanTable) Size() int { return len(ht.codes) }

// Codes exposes the canonical Huffman codes.
func (ht *HuffmanTable) Codes() []HuffmanCode { return ht.codes }

// RangeLengths exposes the bit lengths used for value extension.
func (ht *HuffmanTable) RangeLengths() []int { return ht.rangeLen }

// RangeLows exposes the base values for each Huffman entry.
func (ht *HuffmanTable) RangeLows() []int { return ht.rangeLow }

// HasOOB indicates whether the table encodes an out-of-band value.
func (ht *HuffmanTable) HasOOB() bool { return ht.hasOOB }

func (ht *HuffmanTable) appendEntry(codeLen int32, rangeLen int, rangeLow int) {
	ht.codes = append(ht.codes, HuffmanCode{CodeLength: codeLen})
	ht.rangeLen = append(ht.rangeLen, rangeLen)
	ht.rangeLow = append(ht.rangeLow, rangeLow)
}

func assignHuffmanCodes(codes []HuffmanCode) error {
	maxLen := int32(0)
	for _, c := range codes {
		if c.CodeLength > maxLen {
			maxLen = c.CodeLength
		}
	}
	if maxLen == 0 {
		return nil
	}

	lencounts := make([]int, maxLen+1)
	firstcodes := make([]int, maxLen+1)
	for _, c := range codes {
		if c.CodeLength < 0 || int(c.CodeLength) >= len(lencounts) {
			return errors.New("jbig2: invalid Huffman code length")
		}
		lencounts[c.CodeLength]++
	}
	lencounts[0] = 0

	for i := 1; i <= int(maxLen); i++ {
		shifted := int64(firstcodes[i-1]) + int64(lencounts[i-1])
		shifted <<= 1
		if shifted > int64(int32Max) || shifted < 0 {
			return errors.New("jbig2: Huffman code value overflow")
		}
		firstcodes[i] = int(shifted)
		cur := firstcodes[i]
		for idx := range codes {
			if int(codes[idx].CodeLength) == i {
				if cur > int(int32Max) {
					return errors.New("jbig2: Huffman code exceeds int32")
				}
				codes[idx].Code = int32(cur)
				cur++
			}
		}
	}
	return nil
}

var (
	tableLine1  = []tableLine{{1, 4, 0}, {2, 8, 16}, {3, 16, 272}, {0, 32, -1}, {3, 32, 65808}}
	tableLine2  = []tableLine{{1, 0, 0}, {2, 0, 1}, {3, 0, 2}, {4, 3, 3}, {5, 6, 11}, {0, 32, -1}, {6, 32, 75}, {6, 0, 0}}
	tableLine3  = []tableLine{{8, 8, -256}, {1, 0, 0}, {2, 0, 1}, {3, 0, 2}, {4, 3, 3}, {5, 6, 11}, {8, 32, -257}, {7, 32, 75}, {6, 0, 0}}
	tableLine4  = []tableLine{{1, 0, 1}, {2, 0, 2}, {3, 0, 3}, {4, 3, 4}, {5, 6, 12}, {0, 32, -1}, {5, 32, 76}}
	tableLine5  = []tableLine{{7, 8, -255}, {1, 0, 1}, {2, 0, 2}, {3, 0, 3}, {4, 3, 4}, {5, 6, 12}, {7, 32, -256}, {6, 32, 76}}
	tableLine6  = []tableLine{{5, 10, -2048}, {4, 9, -1024}, {4, 8, -512}, {4, 7, -256}, {5, 6, -128}, {5, 5, -64}, {4, 5, -32}, {2, 7, 0}, {3, 7, 128}, {3, 8, 256}, {4, 9, 512}, {4, 10, 1024}, {6, 32, -2049}, {6, 32, 2048}}
	tableLine7  = []tableLine{{4, 9, -1024}, {3, 8, -512}, {4, 7, -256}, {5, 6, -128}, {5, 5, -64}, {4, 5, -32}, {4, 5, 0}, {5, 5, 32}, {5, 6, 64}, {4, 7, 128}, {3, 8, 256}, {3, 9, 512}, {3, 10, 1024}, {5, 32, -1025}, {5, 32, 2048}}
	tableLine8  = []tableLine{{8, 3, -15}, {9, 1, -7}, {8, 1, -5}, {9, 0, -3}, {7, 0, -2}, {4, 0, -1}, {2, 1, 0}, {5, 0, 2}, {6, 0, 3}, {3, 4, 4}, {6, 1, 20}, {4, 4, 22}, {4, 5, 38}, {5, 6, 70}, {5, 7, 134}, {6, 7, 262}, {7, 8, 390}, {6, 10, 646}, {9, 32, -16}, {9, 32, 1670}, {2, 0, 0}}
	tableLine9  = []tableLine{{8, 4, -31}, {9, 2, -15}, {8, 2, -11}, {9, 1, -7}, {7, 1, -5}, {4, 1, -3}, {3, 1, -1}, {3, 1, 1}, {5, 1, 3}, {6, 1, 5}, {3, 5, 7}, {6, 2, 39}, {4, 5, 43}, {4, 6, 75}, {5, 7, 139}, {5, 8, 267}, {6, 8, 523}, {7, 9, 779}, {6, 11, 1291}, {9, 32, -32}, {9, 32, 3339}, {2, 0, 0}}
	tableLine10 = []tableLine{{7, 4, -21}, {8, 0, -5}, {7, 0, -4}, {5, 0, -3}, {2, 2, -2}, {5, 0, 2}, {6, 0, 3}, {7, 0, 4}, {8, 0, 5}, {2, 6, 6}, {5, 5, 70}, {6, 5, 102}, {6, 6, 134}, {6, 7, 198}, {6, 8, 326}, {6, 9, 582}, {6, 10, 1094}, {7, 11, 2118}, {8, 32, -22}, {8, 32, 4166}, {2, 0, 0}}
	tableLine11 = []tableLine{{1, 0, 1}, {2, 1, 2}, {4, 0, 4}, {4, 1, 5}, {5, 1, 7}, {5, 2, 9}, {6, 2, 13}, {7, 2, 17}, {7, 3, 21}, {7, 4, 29}, {7, 5, 45}, {7, 6, 77}, {0, 32, 0}, {7, 32, 141}}
	tableLine12 = []tableLine{{1, 0, 1}, {2, 0, 2}, {3, 1, 3}, {5, 0, 5}, {5, 1, 6}, {6, 1, 8}, {7, 0, 10}, {7, 1, 11}, {7, 2, 13}, {7, 3, 17}, {7, 4, 25}, {8, 5, 41}, {0, 32, 0}, {8, 32, 73}}
	tableLine13 = []tableLine{{1, 0, 1}, {3, 0, 2}, {4, 0, 3}, {5, 0, 4}, {4, 1, 5}, {3, 3, 7}, {6, 1, 15}, {6, 2, 17}, {6, 3, 21}, {6, 4, 29}, {6, 5, 45}, {7, 6, 77}, {0, 32, 0}, {7, 32, 141}}
	tableLine14 = []tableLine{{3, 0, -2}, {3, 0, -1}, {1, 0, 0}, {3, 0, 1}, {3, 0, 2}, {0, 32, -3}, {0, 32, 3}}
	tableLine15 = []tableLine{{7, 4, -24}, {6, 2, -8}, {5, 1, -4}, {4, 0, -2}, {3, 0, -1}, {1, 0, 0}, {3, 0, 1}, {4, 0, 2}, {5, 1, 3}, {6, 2, 5}, {7, 4, 9}, {7, 32, -25}, {7, 32, 25}}
)

var builtinHuffmanTables = [...]struct {
	htoob bool
	lines []tableLine
}{
	{},
	{false, tableLine1},
	{true, tableLine2},
	{true, tableLine3},
	{false, tableLine4},
	{false, tableLine5},
	{false, tableLine6},
	{false, tableLine7},
	{true, tableLine8},
	{true, tableLine9},
	{true, tableLine10},
	{false, tableLine11},
	{false, tableLine12},
	{false, tableLine13},
	{false, tableLine14},
	{false, tableLine15},
}
