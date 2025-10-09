package jbig2

import "errors"

// HuffmanDecoder wraps a bitstream to decode canonical JBIG2 Huffman codes.
type HuffmanDecoder struct {
	stream *BitStream
}

// NewHuffmanDecoder constructs a Huffman decoder bound to the provided stream.
func NewHuffmanDecoder(stream *BitStream) *HuffmanDecoder {
	return &HuffmanDecoder{stream: stream}
}

// Decode returns the next value using the supplied table. The JBIG2 out-of-band
// condition is reported via the returned error being nil and the value equalling
// `JBig2OOB`.
func (hd *HuffmanDecoder) Decode(table *HuffmanTable) (int, error) {
	if hd == nil || hd.stream == nil {
		return 0, errors.New("jbig2: decoder requires a stream")
	}
	if table == nil {
		return 0, errors.New("jbig2: missing Huffman table")
	}

	var code uint32
	bits := 0

	tableCodes := table.Codes()
	rangeLens := table.RangeLengths()
	rangeLows := table.RangeLows()

	for {
		bit, err := hd.stream.Read1Bit()
		if err != nil {
			return 0, err
		}
		code = (code << 1) | bit
		bits++

		for i, entry := range tableCodes {
			if int(entry.CodeLength) != bits || uint32(entry.Code) != code {
				continue
			}

			if table.HasOOB() && i == len(tableCodes)-1 {
				return int(JBig2OOB), nil
			}

			extraBits := rangeLens[i]
			var extra uint32
			if extraBits > 0 {
				var readErr error
				extra, readErr = hd.stream.ReadNBits(uint32(extraBits))
				if readErr != nil {
					return 0, readErr
				}
			}

			offset := 2
			if table.HasOOB() {
				offset = 3
			}

			var value int
			if i == len(tableCodes)-offset {
				value = rangeLows[i] - int(extra)
			} else {
				value = rangeLows[i] + int(extra)
			}
			return value, nil
		}
	}
}
