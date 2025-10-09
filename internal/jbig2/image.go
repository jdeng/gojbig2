package jbig2

import "errors"

const (
	maxImagePixels = int(^uint32(0)>>1) - 31 // INT_MAX - 31 on 32-bit C++
	maxImageBytes  = maxImagePixels / 8
)

// ComposeOp duplicates the JBIG2 composition operators used for image blending.
type ComposeOp int

const (
	ComposeOR ComposeOp = iota
	ComposeAND
	ComposeXOR
	ComposeXNOR
	ComposeReplace
)

// Rect is a lightweight equivalent of PDFium's FX_RECT.
type Rect struct {
	Left, Top, Right, Bottom int
}

// Width returns the span of the rectangle on the X axis.
func (r Rect) Width() int { return r.Right - r.Left }

// Height returns the span of the rectangle on the Y axis.
func (r Rect) Height() int { return r.Bottom - r.Top }

// Image is the Go translation of CJBig2_Image.
type Image struct {
	width  int
	height int
	stride int // bytes per line, always word aligned
	data   []byte
	owned  bool
}

// NewImage constructs an owned buffer with dimensions aligned to 32 bits.
func NewImage(w, h int32) *Image {
	img := &Image{}
	if w <= 0 || h <= 0 || int64(w) > int64(maxImagePixels) {
		return img
	}

	stridePixels := alignTo32(int(w))
	if int(h) > maxImagePixels/stridePixels {
		return img
	}

	img.width = int(w)
	img.height = int(h)
	img.stride = stridePixels / 8
	size := img.stride * img.height
	if size <= 0 {
		return img
	}
	img.data = make([]byte, size)
	img.owned = true
	return img
}

// NewImageFromBuffer references an external buffer without taking ownership.
func NewImageFromBuffer(w, h, stride int32, buf []byte) (*Image, error) {
	if w < 0 || h < 0 {
		return nil, errors.New("negative dimensions")
	}
	if stride < 0 || int(stride) > maxImageBytes || stride%4 != 0 {
		return nil, errors.New("invalid stride")
	}
	stridePixels := int(stride) * 8
	if stridePixels < int(w) {
		return nil, errors.New("stride smaller than width")
	}
	if int(h) > 0 && int(h) > maxImagePixels/stridePixels {
		return nil, errors.New("image too large")
	}
	required := int(stride) * int(h)
	if required > len(buf) {
		return nil, errors.New("buffer too small")
	}
	return &Image{
		width:  int(w),
		height: int(h),
		stride: int(stride),
		data:   buf[:required],
		owned:  false,
	}, nil
}

// Width returns the image width in pixels.
func (img *Image) Width() int { return img.width }

// Height returns the image height in pixels.
func (img *Image) Height() int { return img.height }

// Stride returns the number of bytes per scanline.
func (img *Image) Stride() int { return img.stride }

// Data exposes the underlying backing buffer.
func (img *Image) Data() []byte { return img.data }

// IsValidImageSize matches the C++ bounds guard used before image creation.
func IsValidImageSize(w, h int32) bool {
	return w > 0 && w <= JBig2MaxImageSize && h > 0 && h <= JBig2MaxImageSize
}

// GetPixel returns the bit value at the requested coordinate.
func (img *Image) GetPixel(x, y int32) int {
	if img == nil || img.data == nil {
		return 0
	}
	if x < 0 || int(x) >= img.width {
		return 0
	}
	line := img.line(int(y))
	if line == nil {
		return 0
	}
	m := bitIndexToByte(int(x))
	n := int(x) & 7
	return int((line[m] >> (7 - n)) & 1)
}

// SetPixel mutates the pixel at the requested coordinate.
func (img *Image) SetPixel(x, y int32, v int) {
	if img == nil || img.data == nil {
		return
	}
	if x < 0 || int(x) >= img.width {
		return
	}
	line := img.line(int(y))
	if line == nil {
		return
	}
	m := bitIndexToByte(int(x))
	mask := byte(1 << (7 - (int(x) & 7)))
	if v != 0 {
		line[m] |= mask
	} else {
		line[m] &^= mask
	}
}

// CopyLine clones one scanline into another, zero-filling when the source is absent.
func (img *Image) CopyLine(dstY, srcY int32) {
	if img == nil || img.data == nil {
		return
	}
	dst := img.line(int(dstY))
	if dst == nil {
		return
	}
	src := img.line(int(srcY))
	if src == nil {
		for i := range dst {
			dst[i] = 0
		}
		return
	}
	copy(dst, src)
}

// Fill writes the same bit across the whole buffer.
func (img *Image) Fill(v bool) {
	if img == nil || img.data == nil {
		return
	}
	value := byte(0)
	if v {
		value = 0xff
	}
	for i := range img.data {
		img.data[i] = value
	}
}

// ComposeFrom matches the semantics of CJBig2_Image::ComposeFrom.
func (img *Image) ComposeFrom(x, y int64, src *Image, op ComposeOp) bool {
	if img == nil || img.data == nil || src == nil {
		return false
	}
	return src.ComposeTo(img, x, y, op)
}

// ComposeFromWithRect mirrors the overload that restricts the source rectangle.
func (img *Image) ComposeFromWithRect(x, y int64, src *Image, rect Rect, op ComposeOp) bool {
	if img == nil || img.data == nil || src == nil {
		return false
	}
	return src.ComposeToWithRect(img, x, y, rect, op)
}

// ComposeTo projects this image onto the destination at the desired offset.
func (img *Image) ComposeTo(dst *Image, x, y int64, op ComposeOp) bool {
	if img == nil || img.data == nil {
		return false
	}
	return img.composeToInternal(dst, x, y, op, Rect{Left: 0, Top: 0, Right: img.width, Bottom: img.height})
}

// ComposeToWithRect projects a source rectangle onto the destination.
func (img *Image) ComposeToWithRect(dst *Image, x, y int64, rect Rect, op ComposeOp) bool {
	if img == nil || img.data == nil {
		return false
	}
	return img.composeToInternal(dst, x, y, op, rect)
}

// SubImage returns a newly allocated Image cropped to the requested rectangle.
func (img *Image) SubImage(x, y, w, h int32) *Image {
	result := NewImage(w, h)
	if result.data == nil || img == nil || img.data == nil {
		return result
	}
	if int(x) < 0 || int(x) >= img.width || int(y) < 0 || int(y) >= img.height {
		return result
	}
	if (int(x) & 7) == 0 {
		img.subImageFast(int(x), int(y), int(w), int(h), result)
	} else {
		img.subImageSlow(int(x), int(y), int(w), int(h), result)
	}
	return result
}

// Expand increases the image height, allocating new storage when required.
func (img *Image) Expand(h int32, v bool) {
	if img == nil || img.data == nil {
		return
	}
	if int(h) <= img.height || int(h) > maxImageBytes/img.stride {
		return
	}
	currentSize := img.stride * img.height
	desiredSize := img.stride * int(h)
	if img.owned {
		newBuf := make([]byte, desiredSize)
		copy(newBuf, img.data)
		img.data = newBuf
	} else {
		newBuf := make([]byte, desiredSize)
		copy(newBuf, img.data)
		img.data = newBuf
		img.owned = true
	}
	fill := byte(0)
	if v {
		fill = 0xff
	}
	for i := currentSize; i < desiredSize; i++ {
		img.data[i] = fill
	}
	img.height = int(h)
}

// composeToInternal projects a source rectangle onto the destination image.
// This implementation favors clarity over bit-twiddling optimizations; revisit
// once the decoder pipeline is complete if performance becomes a concern.
func (img *Image) composeToInternal(dst *Image, xIn, yIn int64, op ComposeOp, rect Rect) bool {
	if img == nil || img.data == nil || dst == nil || dst.data == nil {
		return false
	}
	if xIn < -1048576 || xIn > 1048576 || yIn < -1048576 || yIn > 1048576 {
		return false
	}
	if rect.Left < 0 || rect.Top < 0 || rect.Right > img.width || rect.Bottom > img.height {
		return false
	}
	x := int(xIn)
	y := int(yIn)

	sw := rect.Width()
	sh := rect.Height()

	xs0 := 0
	if x < 0 {
		xs0 = -x
	}
	xs1 := sw
	if tmp := dst.width - x; tmp < xs1 {
		xs1 = tmp
	}
	ys0 := 0
	if y < 0 {
		ys0 = -y
	}
	ys1 := sh
	if tmp := dst.height - y; tmp < ys1 {
		ys1 = tmp
	}

	if ys0 >= ys1 || xs0 >= xs1 {
		return false
	}

	xd0 := max(x, 0)
	yd0 := max(y, 0)
	w := xs1 - xs0
	h := ys1 - ys0

	for yy := 0; yy < h; yy++ {
		srcY := rect.Top + ys0 + yy
		dstY := yd0 + yy
		srcLine := img.line(srcY)
		dstLine := dst.line(dstY)
		if srcLine == nil || dstLine == nil {
			return false
		}
		for xx := 0; xx < w; xx++ {
			srcX := rect.Left + xs0 + xx
			dstX := xd0 + xx
			srcBit := readBit(srcLine, srcX)
			dstBit := readBit(dstLine, dstX)
			writeBit(dstLine, dstX, applyCompose(op, dstBit, srcBit))
		}
	}

	return true
}

func (img *Image) subImageFast(x, y, w, h int, dst *Image) {
	m := bitIndexToByte(x)
	bytesToCopy := min(dst.stride, img.stride-m)
	linesToCopy := min(dst.height, img.height-y)
	for j := 0; j < linesToCopy; j++ {
		srcLine := img.lineUnsafe(y + j)
		dstLine := dst.lineUnsafe(j)
		copy(dstLine[:bytesToCopy], srcLine[m:m+bytesToCopy])
	}
}

func (img *Image) subImageSlow(x, y, w, h int, dst *Image) {
	m := bitIndexToAlignedByte(x)
	n := x & 31
	bytesToCopy := min(dst.stride, img.stride-m)
	linesToCopy := min(dst.height, img.height-y)
	for j := 0; j < linesToCopy; j++ {
		srcLine := img.lineUnsafe(y + j)
		dstLine := dst.lineUnsafe(j)
		src := m
		end := img.stride
		dstIdx := 0
		for dstIdx < bytesToCopy {
			val := getUint32(srcLine, src) << uint(n)
			if src+4 < end {
				val |= getUint32(srcLine, src+4) >> uint(32-n)
			}
			putUint32(dstLine, dstIdx, val)
			src += 4
			dstIdx += 4
		}
	}
}

func (img *Image) line(y int) []byte {
	if img == nil || img.data == nil {
		return nil
	}
	if y < 0 || y >= img.height {
		return nil
	}
	start := y * img.stride
	return img.data[start : start+img.stride]
}

func (img *Image) lineUnsafe(y int) []byte {
	start := y * img.stride
	return img.data[start : start+img.stride]
}

func readBit(line []byte, x int) int {
	if x < 0 {
		return 0
	}
	byteIdx := x / 8
	if byteIdx < 0 || byteIdx >= len(line) {
		return 0
	}
	bit := 7 - (x & 7)
	return int((line[byteIdx] >> bit) & 1)
}

func writeBit(line []byte, x int, value int) {
	if x < 0 {
		return
	}
	byteIdx := x / 8
	if byteIdx < 0 || byteIdx >= len(line) {
		return
	}
	mask := byte(1 << (7 - (x & 7)))
	if value != 0 {
		line[byteIdx] |= mask
	} else {
		line[byteIdx] &^= mask
	}
}

func applyCompose(op ComposeOp, dst, src int) int {
	switch op {
	case ComposeOR:
		return dst | src
	case ComposeAND:
		return dst & src
	case ComposeXOR:
		return dst ^ src
	case ComposeXNOR:
		return 1 - (dst ^ src)
	case ComposeReplace:
		return src
	default:
		return src
	}
}

func bitIndexToByte(index int) int { return index / 8 }

func bitIndexToAlignedByte(index int) int { return index / 32 * 4 }

func alignTo32(v int) int {
	return (v + 31) / 32 * 32
}

func getUint32(buf []byte, offset int) uint32 {
	return uint32(buf[offset])<<24 | uint32(buf[offset+1])<<16 | uint32(buf[offset+2])<<8 | uint32(buf[offset+3])
}

func putUint32(buf []byte, offset int, value uint32) {
	buf[offset] = byte(value >> 24)
	buf[offset+1] = byte(value >> 16)
	buf[offset+2] = byte(value >> 8)
	buf[offset+3] = byte(value)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
