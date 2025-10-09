package jbig2

const unboundedPageHeight = ^uint32(0)

// PageInfo mirrors the PDFium JBig2PageInfo struct and captures per-page metadata.
type PageInfo struct {
	Width             uint32
	Height            uint32
	ResolutionX       uint32
	ResolutionY       uint32
	DefaultPixelValue bool
	Striped           bool
	MaxStripeSize     uint16
}

// EffectiveHeight reports the height to allocate for striped pages.
func (p PageInfo) EffectiveHeight() uint32 {
	if p.Height == unboundedPageHeight && p.Striped {
		return uint32(p.MaxStripeSize)
	}
	return p.Height
}

// ShouldTreatAsStriped returns true when the decoder should dynamically grow
// the backing image as additional stripes arrive.
func (p PageInfo) ShouldTreatAsStriped() bool {
	return p.Striped || p.Height == unboundedPageHeight
}
