package fax

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// max returns the maximum of two integers.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// FaxModule provides FAX decoding functionality for JBIG2 MMR compression.
type FaxModule struct{}

// FaxG4Decode decodes a G4 (MMR) compressed FAX image.
// Returns the ending bit position.
func FaxG4Decode(srcBuf []byte, startingBitPos, width, height, pitch int, destBuf []byte) int {
	if pitch == 0 {
		return startingBitPos
	}

	refBuf := make([]byte, pitch)
	for i := range refBuf {
		refBuf[i] = 0xFF
	}

	bitPos := startingBitPos
	for iRow := 0; iRow < height; iRow++ {
		lineBuf := destBuf[iRow*pitch : (iRow+1)*pitch]
		for i := range lineBuf {
			lineBuf[i] = 0xFF
		}
		bitPos = faxG4GetRow(srcBuf, len(srcBuf)<<3, &bitPos, lineBuf, refBuf, width)
		copy(refBuf, lineBuf)
	}
	return bitPos
}

// faxG4GetRow decodes one row of G4 compressed data.
func faxG4GetRow(srcBuf []byte, bitsize int, bitpos *int, destBuf, refBuf []byte, columns int) int {
	a0 := -1
	a0color := true

	for {
		if *bitpos >= bitsize {
			return *bitpos
		}

		b1, b2 := faxG4FindB1B2(refBuf, columns, a0, a0color)

		var vDelta int
		if !nextBit(srcBuf, bitpos) {
			if *bitpos >= bitsize {
				return *bitpos
			}

			bit1 := nextBit(srcBuf, bitpos)
			if *bitpos >= bitsize {
				return *bitpos
			}

			bit2 := nextBit(srcBuf, bitpos)
			if bit1 {
				// Mode "Vertical", VR(1), VL(1)
				vDelta = 1
				if bit2 {
					vDelta = 1
				} else {
					vDelta = -1
				}
			} else if bit2 {
				// Mode "Horizontal"
				runLen1 := 0
				for {
					var runTable []byte
					if a0color {
						runTable = faxWhiteRunIns
					} else {
						runTable = faxBlackRunIns
					}
					run := faxGetRun(srcBuf, bitpos, bitsize, runTable)
					runLen1 += run
					if run < 64 {
						break
					}
				}
				if a0 < 0 {
					runLen1++
				}
				if runLen1 < 0 {
					return *bitpos
				}

				a1 := a0 + runLen1
				if !a0color {
					faxFillBits(destBuf, columns, a0, a1)
				}

				runLen2 := 0
				for {
					var runTable []byte
					if !a0color {
						runTable = faxWhiteRunIns
					} else {
						runTable = faxBlackRunIns
					}
					run := faxGetRun(srcBuf, bitpos, bitsize, runTable)
					runLen2 += run
					if run < 64 {
						break
					}
				}
				if runLen2 < 0 {
					return *bitpos
				}
				a2 := a1 + runLen2
				if a0color {
					faxFillBits(destBuf, columns, a1, a2)
				}

				a0 = a2
				if a0 < columns {
					continue
				}
				return *bitpos
			} else {
				if *bitpos >= bitsize {
					return *bitpos
				}

				if nextBit(srcBuf, bitpos) {
					// Mode "Pass"
					if !a0color {
						faxFillBits(destBuf, columns, a0, b2)
					}

					if b2 >= columns {
						return *bitpos
					}

					a0 = b2
					continue
				}

				if *bitpos >= bitsize {
					return *bitpos
				}

				nextBit1 := nextBit(srcBuf, bitpos)
				if *bitpos >= bitsize {
					return *bitpos
				}

				nextBit2 := nextBit(srcBuf, bitpos)
				if nextBit1 {
					// Mode "Vertical", VR(2), VL(2)
					if nextBit2 {
						vDelta = 2
					} else {
						vDelta = -2
					}
				} else if nextBit2 {
					if *bitpos >= bitsize {
						return *bitpos
					}
					// Mode "Vertical", VR(3), VL(3)
					if nextBit(srcBuf, bitpos) {
						vDelta = 3
					} else {
						vDelta = -3
					}
				} else {
					if *bitpos >= bitsize {
						return *bitpos
					}
					// Extension
					if nextBit(srcBuf, bitpos) {
						*bitpos += 3
						continue
					}
					*bitpos += 5
					return *bitpos
				}
			}
		} else {
			// Mode "Vertical", V(0)
		}

		a1 := b1 + vDelta
		if !a0color {
			faxFillBits(destBuf, columns, a0, a1)
		}

		if a1 >= columns {
			return *bitpos
		}

		// The position of picture element must be monotonic increasing
		if a0 >= a1 {
			return *bitpos
		}

		a0 = a1
		a0color = !a0color
	}
}

// faxG4FindB1B2 finds the B1 and B2 positions for G4 decoding.
func faxG4FindB1B2(refBuf []byte, columns, a0 int, a0color bool) (int, int) {
	firstBit := a0 < 0 || (refBuf[a0/8]&(1<<(7-a0%8))) != 0
	b1 := findBit(refBuf, columns, a0+1, !firstBit)
	if b1 >= columns {
		return columns, columns
	}
	if firstBit == !a0color {
		b1 = findBit(refBuf, columns, b1+1, firstBit)
		firstBit = !firstBit
	}
	if b1 >= columns {
		return columns, columns
	}
	b2 := findBit(refBuf, columns, b1+1, firstBit)
	return b1, b2
}

// findBit finds the next bit of the specified color.
func findBit(dataBuf []byte, maxPos, startPos int, bit bool) int {
	if startPos >= maxPos {
		return maxPos
	}

	bitXor := byte(0x00)
	if bit {
		bitXor = 0xFF
	}

	bitOffset := startPos % 8
	if bitOffset != 0 {
		bytePos := startPos / 8
		data := (dataBuf[bytePos] ^ bitXor) & (0xFF >> bitOffset)
		if data != 0 {
			return bytePos*8 + int(oneLeadPos[data])
		}
		startPos += 7
	}

	maxByte := (maxPos + 7) / 8
	bytePos := startPos / 8

	// Try reading in bigger chunks in case there are long runs to be skipped
	const bulkReadSize = 8
	if maxByte >= bulkReadSize && bytePos < maxByte-bulkReadSize {
		skipBlock := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
		if bit {
			skipBlock = []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
		}
		for bytePos < maxByte-bulkReadSize {
			match := true
			for i := 0; i < bulkReadSize; i++ {
				if dataBuf[bytePos+i] != skipBlock[i] {
					match = false
					break
				}
			}
			if !match {
				break
			}
			bytePos += bulkReadSize
		}
	}

	for bytePos < maxByte {
		data := dataBuf[bytePos] ^ bitXor
		if data != 0 {
			return min(bytePos*8+int(oneLeadPos[data]), maxPos)
		}
		bytePos++
	}
	return maxPos
}

// faxFillBits fills bits in the destination buffer.
func faxFillBits(destBuf []byte, columns, startPos, endPos int) {
	startPos = max(startPos, 0)
	endPos = min(endPos, columns)
	if startPos >= endPos {
		return
	}

	firstByte := startPos / 8
	lastByte := (endPos - 1) / 8

	if firstByte == lastByte {
		for i := startPos % 8; i <= (endPos-1)%8; i++ {
			destBuf[firstByte] &^= 1 << (7 - i)
		}
		return
	}

	for i := startPos % 8; i < 8; i++ {
		destBuf[firstByte] &^= 1 << (7 - i)
	}

	for i := 0; i <= (endPos-1)%8; i++ {
		destBuf[lastByte] &^= 1 << (7 - i)
	}

	if lastByte > firstByte+1 {
		for i := firstByte + 1; i < lastByte; i++ {
			destBuf[i] = 0x00
		}
	}
}

// nextBit reads the next bit from the source buffer.
func nextBit(srcBuf []byte, bitpos *int) bool {
	pos := *bitpos
	*bitpos++
	return (srcBuf[pos/8] & (1 << (7 - pos%8))) != 0
}

// faxGetRun gets a run length from the FAX stream.
func faxGetRun(srcBuf []byte, bitpos *int, bitsize int, insArray []byte) int {
	code := uint32(0)
	insOff := 0

	for {
		if *bitpos >= bitsize {
			return -1
		}

		bit := nextBit(srcBuf, bitpos)
		if bit {
			code = (code << 1) | 1
		} else {
			code = (code << 1) | 0
		}

		ins := insArray[insOff]
		if ins == 0xFF {
			return -1
		}
		insOff++

		nextOff := insOff + int(ins)*3
		for ; insOff < nextOff; insOff += 3 {
			if insArray[insOff] == byte(code) {
				return int(insArray[insOff+1]) + int(insArray[insOff+2])*256
			}
		}
	}
}

// faxSkipEOL skips end-of-line markers.
func faxSkipEOL(srcBuf []byte, bitsize int, bitpos *int) {
	startbit := *bitpos
	for *bitpos < bitsize {
		if nextBit(srcBuf, bitpos) {
			if *bitpos-startbit <= 11 {
				*bitpos = startbit
			}
			return
		}
	}
}

// FaxGet1DLine decodes a 1D FAX line.
func FaxGet1DLine(srcBuf []byte, bitsize int, bitpos *int, destBuf []byte, columns int) {
	color := true
	startpos := 0
	for {
		if *bitpos >= bitsize {
			return
		}

		runLen := 0
		for {
			var runTable []byte
			if color {
				runTable = faxWhiteRunIns
			} else {
				runTable = faxBlackRunIns
			}
			run := faxGetRun(srcBuf, bitpos, bitsize, runTable)
			if run < 0 {
				for *bitpos < bitsize {
					if nextBit(srcBuf, bitpos) {
						return
					}
				}
				return
			}
			runLen += run
			if run < 64 {
				break
			}
		}
		if !color {
			faxFillBits(destBuf, columns, startpos, startpos+runLen)
		}

		startpos += runLen
		if startpos >= columns {
			break
		}

		color = !color
	}
}

// Fax constants and tables
const (
	faxMaxImageDimension = 65535
	faxBpc               = 1
	faxComps             = 1
)

// oneLeadPos maps byte values to the position of the leading 1 bit.
var oneLeadPos = [256]uint8{
	8, 7, 6, 6, 5, 5, 5, 5, 4, 4, 4, 4, 4, 4, 4, 4, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2,
	2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
}

// faxBlackRunIns contains the black run length codes for FAX.
var faxBlackRunIns = []byte{
	0, 2, 0x02, 3, 0, 0x03, 2, 0, 2, 0x02, 1, 0, 0x03, 4, 0, 2, 0x02, 6, 0, 0x03, 5, 0, 1, 0x03, 7, 0, 2, 0x04, 9, 0, 0x05, 8, 0, 3, 0x04, 10, 0, 0x05, 11, 0, 0x07, 12, 0, 2, 0x04, 13, 0, 0x07, 14, 0, 1, 0x18, 15, 0, 5, 0x08, 18, 0, 0x0f, 64, 0, 0x17, 16, 0, 0x18, 17, 0, 0x37, 0, 0, 10, 0x08, 0x00, 0x07, 0x0c, 0x40, 0x07, 0x0d, 0x80, 0x07, 0x17, 24, 0, 0x18, 25, 0, 0x28, 23, 0, 0x37, 22, 0, 0x67, 19, 0, 0x68, 20, 0, 0x6c, 21, 0, 54, 0x12, 1984 % 256, 1984 / 256, 0x13, 2048 % 256, 2048 / 256, 0x14, 2112 % 256, 2112 / 256, 0x15, 2176 % 256, 2176 / 256, 0x16, 2240 % 256, 2240 / 256, 0x17, 2304 % 256, 2304 / 256, 0x1c, 2368 % 256, 2368 / 256, 0x1d, 2432 % 256, 2432 / 256, 0x1e, 2496 % 256, 2496 / 256, 0x1f, 2560 % 256, 2560 / 256, 0x24, 52, 0, 0x27, 55, 0, 0x28, 56, 0, 0x2b, 59, 0, 0x2c, 60, 0, 0x33, 320 % 256, 320 / 256, 0x34, 384 % 256, 384 / 256, 0x35, 448 % 256, 448 / 256, 0x37, 53, 0, 0x38, 54, 0, 0x52, 50, 0, 0x53, 51, 0, 0x54, 44, 0, 0x55, 45, 0, 0x56, 46, 0, 0x57, 47, 0, 0x58, 57, 0, 0x59, 58, 0, 0x5a, 61, 0, 0x5b, 256 % 256, 256 / 256, 0x64, 48, 0, 0x65, 49, 0, 0x66, 62, 0, 0x67, 63, 0, 0x68, 30, 0, 0x69, 31, 0, 0x6a, 32, 0, 0x6b, 33, 0, 0x6c, 40, 0, 0x6d, 41, 0, 0xc8, 128, 0, 0xc9, 192, 0, 0xca, 26, 0, 0xcb, 27, 0, 0xcc, 28, 0, 0xcd, 29, 0, 0xd2, 34, 0, 0xd3, 35, 0, 0xd4, 36, 0, 0xd5, 37, 0, 0xd6, 38, 0, 0xd7, 39, 0, 0xda, 42, 0, 0xdb, 43, 0, 20, 0x4a, 640 % 256, 640 / 256, 0x4b, 704 % 256, 704 / 256, 0x4c, 768 % 256, 768 / 256, 0x4d, 832 % 256, 832 / 256, 0x52, 1280 % 256, 1280 / 256, 0x53, 1344 % 256, 1344 / 256, 0x54, 1408 % 256, 1408 / 256, 0x55, 1472 % 256, 1472 / 256, 0x5a, 1536 % 256, 1536 / 256, 0x5b, 1600 % 256, 1600 / 256, 0x64, 1664 % 256, 1664 / 256, 0x65, 1728 % 256, 1728 / 256, 0x6c, 512 % 256, 512 / 256, 0x6d, 576 % 256, 576 / 256, 0x72, 896 % 256, 896 / 256, 0x73, 960 % 256, 960 / 256, 0x74, 1024 % 256, 1024 / 256, 0x75, 1088 % 256, 1088 / 256, 0x76, 1152 % 256, 1152 / 256, 0x77, 1216 % 256, 1216 / 256, 0xff,
}

// faxWhiteRunIns contains the white run length codes for FAX.
var faxWhiteRunIns = []byte{
	0, 0, 0, 6, 0x07, 2, 0, 0x08, 3, 0, 0x0B, 4, 0, 0x0C, 5, 0, 0x0E, 6, 0, 0x0F, 7, 0, 6, 0x07, 10, 0, 0x08, 11, 0, 0x12, 128, 0, 0x13, 8, 0, 0x14, 9, 0, 0x1b, 64, 0, 9, 0x03, 13, 0, 0x07, 1, 0, 0x08, 12, 0, 0x17, 192, 0, 0x18, 1664 % 256, 1664 / 256, 0x2a, 16, 0, 0x2B, 17, 0, 0x34, 14, 0, 0x35, 15, 0, 12, 0x03, 22, 0, 0x04, 23, 0, 0x08, 20, 0, 0x0c, 19, 0, 0x13, 26, 0, 0x17, 21, 0, 0x18, 28, 0, 0x24, 27, 0, 0x27, 18, 0, 0x28, 24, 0, 0x2B, 25, 0, 0x37, 256 % 256, 256 / 256, 42, 0x02, 29, 0, 0x03, 30, 0, 0x04, 45, 0, 0x05, 46, 0, 0x0a, 47, 0, 0x0b, 48, 0, 0x12, 33, 0, 0x13, 34, 0, 0x14, 35, 0, 0x15, 36, 0, 0x16, 37, 0, 0x17, 38, 0, 0x1a, 31, 0, 0x1b, 32, 0, 0x24, 53, 0, 0x25, 54, 0, 0x28, 39, 0, 0x29, 40, 0, 0x2a, 41, 0, 0x2b, 42, 0, 0x2c, 43, 0, 0x2d, 44, 0, 0x32, 61, 0, 0x33, 62, 0, 0x34, 63, 0, 0x35, 0, 0, 0x36, 320 % 256, 320 / 256, 0x37, 384 % 256, 384 / 256, 0x4a, 59, 0, 0x4b, 60, 0, 0x52, 49, 0, 0x53, 50, 0, 0x54, 51, 0, 0x55, 52, 0, 0x58, 55, 0, 0x59, 56, 0, 0x5a, 57, 0, 0x5b, 58, 0, 0x64, 448 % 256, 448 / 256, 0x65, 512 % 256, 512 / 256, 0x67, 640 % 256, 640 / 256, 0x68, 576 % 256, 576 / 256, 16, 0x98, 1472 % 256, 1472 / 256, 0x99, 1536 % 256, 1536 / 256, 0x9a, 1600 % 256, 1600 / 256, 0x9b, 1728 % 256, 1728 / 256, 0xcc, 704 % 256, 704 / 256, 0xcd, 768 % 256, 768 / 256, 0xd2, 832 % 256, 832 / 256, 0xd3, 896 % 256, 896 / 256, 0xd4, 960 % 256, 960 / 256, 0xd5, 1024 % 256, 1024 / 256, 0xd6, 1088 % 256, 1088 / 256, 0xd7, 1152 % 256, 1152 / 256, 0xd8, 1216 % 256, 1216 / 256, 0xd9, 1280 % 256, 1280 / 256, 0xda, 1344 % 256, 1344 / 256, 0xdb, 1408 % 256, 1408 / 256, 0, 3, 0x08, 1792 % 256, 1792 / 256, 0x0c, 1856 % 256, 1856 / 256, 0x0d, 1920 % 256, 1920 / 256, 10, 0x12, 1984 % 256, 1984 / 256, 0x13, 2048 % 256, 2048 / 256, 0x14, 2112 % 256, 2112 / 256, 0x15, 2176 % 256, 2176 / 256, 0x16, 2240 % 256, 2240 / 256, 0x17, 2304 % 256, 2304 / 256, 0x1c, 2368 % 256, 2368 / 256, 0x1d, 2432 % 256, 2432 / 256, 0x1e, 2496 % 256, 2496 / 256, 0x1f, 2560 % 256, 2560 / 256, 0xff,
}
