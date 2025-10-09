package jbig2

import (
	"errors"
	"math"
)

const maxSpanSize = 256 * 1024 * 1024

// BitStream is the Go translation of CJBig2_BitStream.
type BitStream struct {
	buf    []byte
	key    uint64
	byteIx uint32
	bitIx  uint32
}

// NewBitStream constructs a bit stream over the provided data. The constructor
// enforces the same 256 MB limit used by the PDFium reference; larger inputs
// are ignored and result in an empty buffer.
func NewBitStream(data []byte, key uint64) *BitStream {
	if len(data) > maxSpanSize {
		data = nil
	}
	return &BitStream{
		buf: data,
		key: key,
	}
}

// Key returns the JBIG2 stream key associated with the buffer.
func (bs *BitStream) Key() uint64 { return bs.key }

// ReadNBits reads up to dwBits bits and returns them as an unsigned integer.
// If insufficient data remain, the read stops at the end of the stream and an
// error is returned.
func (bs *BitStream) ReadNBits(count uint32) (uint32, error) {
	if !bs.InBounds() {
		return 0, errors.New("bitstream: out of bounds")
	}

	bitPos := bs.BitPos()
	if bitPos > bs.lengthInBits() {
		return 0, errors.New("bitstream: beyond length")
	}

	var bitsToRead uint32
	if bitPos+count <= bs.lengthInBits() {
		bitsToRead = count
	} else {
		bitsToRead = bs.lengthInBits() - bitPos
	}

	var result uint32
	for bitsToRead > 0 {
		result = (result << 1) | uint32((bs.buf[bs.byteIx]>>(7-bs.bitIx))&0x01)
		bs.advanceBit()
		bitsToRead--
	}
	return result, nil
}

// ReadNBitsSigned behaves like ReadNBits but returns the value as a signed int.
func (bs *BitStream) ReadNBitsSigned(count uint32) (int32, error) {
	value, err := bs.ReadNBits(count)
	if err != nil {
		return 0, err
	}
	return int32(value), nil
}

// Read1Bit returns the next single bit as a uint32.
func (bs *BitStream) Read1Bit() (uint32, error) {
	if !bs.InBounds() {
		return 0, errors.New("bitstream: out of bounds")
	}
	value := uint32((bs.buf[bs.byteIx] >> (7 - bs.bitIx)) & 0x01)
	bs.advanceBit()
	return value, nil
}

// Read1BitBool returns the next single bit as a boolean.
func (bs *BitStream) Read1BitBool() (bool, error) {
	bit, err := bs.Read1Bit()
	if err != nil {
		return false, err
	}
	return bit != 0, nil
}

// ReadByte returns the next raw byte.
func (bs *BitStream) ReadByte() (uint8, error) {
	if !bs.InBounds() {
		return 0, errors.New("bitstream: out of bounds")
	}
	value := bs.buf[bs.byteIx]
	bs.byteIx++
	return value, nil
}

// ReadUint32 reads a big-endian 32-bit value.
func (bs *BitStream) ReadUint32() (uint32, error) {
	if bs.byteIx+3 >= uint32(len(bs.buf)) {
		return 0, errors.New("bitstream: underflow reading uint32")
	}
	v := uint32(bs.buf[bs.byteIx])<<24 |
		uint32(bs.buf[bs.byteIx+1])<<16 |
		uint32(bs.buf[bs.byteIx+2])<<8 |
		uint32(bs.buf[bs.byteIx+3])
	bs.byteIx += 4
	return v, nil
}

// ReadUint16 reads a big-endian 16-bit value.
func (bs *BitStream) ReadUint16() (uint16, error) {
	if bs.byteIx+1 >= uint32(len(bs.buf)) {
		return 0, errors.New("bitstream: underflow reading uint16")
	}
	v := uint16(bs.buf[bs.byteIx])<<8 | uint16(bs.buf[bs.byteIx+1])
	bs.byteIx += 2
	return v, nil
}

// AlignByte advances the stream to the next byte boundary.
func (bs *BitStream) AlignByte() {
	if bs.bitIx != 0 {
		bs.AddOffset(1)
		bs.bitIx = 0
	}
}

// CurByte returns the current byte position, or zero when out of bounds.
func (bs *BitStream) CurByte() uint8 {
	if bs.InBounds() {
		return bs.buf[bs.byteIx]
	}
	return 0
}

// IncByte increments the underlying byte index.
func (bs *BitStream) IncByte() {
	bs.AddOffset(1)
}

// CurByteArith mirrors getCurByte_arith by returning 0xFF when out of bounds.
func (bs *BitStream) CurByteArith() uint8 {
	if bs.InBounds() {
		return bs.buf[bs.byteIx]
	}
	return 0xFF
}

// NextByteArith returns the next byte for arithmetic decoding or 0xFF if none.
func (bs *BitStream) NextByteArith() uint8 {
	next := bs.byteIx + 1
	if next < uint32(len(bs.buf)) {
		return bs.buf[next]
	}
	return 0xFF
}

// Offset returns the current byte index.
func (bs *BitStream) Offset() uint32 { return bs.byteIx }

// SetOffset moves the stream to the provided byte offset.
func (bs *BitStream) SetOffset(offset uint32) {
	if offset > uint32(len(bs.buf)) {
		offset = uint32(len(bs.buf))
	}
	bs.byteIx = offset
}

// AddOffset advances the current byte index while clamping to the buffer size.
func (bs *BitStream) AddOffset(delta uint32) {
	if delta > math.MaxUint32-bs.byteIx {
		delta = math.MaxUint32 - bs.byteIx
	}
	newOffset := bs.byteIx + delta
	if newOffset < bs.byteIx {
		newOffset = math.MaxUint32
	}
	bs.SetOffset(newOffset)
}

// BitPos returns the absolute bit position from the start of the stream.
func (bs *BitStream) BitPos() uint32 {
	return (bs.byteIx << 3) + bs.bitIx
}

// SetBitPos positions the stream at the provided bit offset.
func (bs *BitStream) SetBitPos(bitPos uint32) {
	bs.byteIx = bitPos >> 3
	bs.bitIx = bitPos & 7
}

// Buf returns the underlying slice (read-only view).
func (bs *BitStream) Buf() []byte { return bs.buf }

// Pointer returns a subslice starting at the current position.
func (bs *BitStream) Pointer() []byte {
	if int(bs.byteIx) >= len(bs.buf) {
		return nil
	}
	return bs.buf[bs.byteIx:]
}

// BytesLeft returns the number of remaining bytes in the stream.
func (bs *BitStream) BytesLeft() uint32 {
	if int(bs.byteIx) >= len(bs.buf) {
		return 0
	}
	return uint32(len(bs.buf) - int(bs.byteIx))
}

// InBounds reports whether the current byte index is within the buffer.
func (bs *BitStream) InBounds() bool {
	return bs.byteIx < uint32(len(bs.buf))
}

func (bs *BitStream) lengthInBits() uint32 {
	return uint32(len(bs.buf)) * 8
}

func (bs *BitStream) advanceBit() {
	if bs.bitIx == 7 {
		bs.byteIx++
		bs.bitIx = 0
	} else {
		bs.bitIx++
	}
}
