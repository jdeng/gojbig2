package jbig2

// SymbolDict mirrors CJBig2_SymbolDict, storing exported symbol bitmaps and
// arithmetic decoder context state.
type SymbolDict struct {
	symbols    []*Image
	gbContexts []ArithContext
	grContexts []ArithContext
}

// NewSymbolDict creates an empty symbol dictionary.
func NewSymbolDict() *SymbolDict {
	return &SymbolDict{}
}

// DeepCopy produces a clone of the dictionary, duplicating image data so the
// caller can mutate the copy without affecting the original.
func (sd *SymbolDict) DeepCopy() *SymbolDict {
	if sd == nil {
		return nil
	}
	copyDict := &SymbolDict{
		symbols:    make([]*Image, len(sd.symbols)),
		gbContexts: append([]ArithContext(nil), sd.gbContexts...),
		grContexts: append([]ArithContext(nil), sd.grContexts...),
	}
	for i, img := range sd.symbols {
		copyDict.symbols[i] = cloneImage(img)
	}
	return copyDict
}

// AddImage appends an image to the dictionary.
func (sd *SymbolDict) AddImage(img *Image) {
	if sd == nil {
		return
	}
	sd.symbols = append(sd.symbols, img)
}

// NumImages returns the number of stored symbols.
func (sd *SymbolDict) NumImages() int { return len(sd.symbols) }

// GetImage returns the symbol image at the specified index.
func (sd *SymbolDict) GetImage(index int) *Image {
	if sd == nil || index < 0 || index >= len(sd.symbols) {
		return nil
	}
	return sd.symbols[index]
}

// SetGbContexts stores the generic region arithmetic contexts.
func (sd *SymbolDict) SetGbContexts(ctx []ArithContext) {
	sd.gbContexts = append([]ArithContext(nil), ctx...)
}

// SetGrContexts stores the refinement arithmetic contexts.
func (sd *SymbolDict) SetGrContexts(ctx []ArithContext) {
	sd.grContexts = append([]ArithContext(nil), ctx...)
}

// GbContexts returns the stored generic region contexts.
func (sd *SymbolDict) GbContexts() []ArithContext { return sd.gbContexts }

// GrContexts returns the stored refinement contexts.
func (sd *SymbolDict) GrContexts() []ArithContext { return sd.grContexts }

func cloneImage(img *Image) *Image {
	if img == nil {
		return nil
	}
	clone := &Image{
		width:  img.width,
		height: img.height,
		stride: img.stride,
		owned:  true,
	}
	if img.data != nil {
		clone.data = append([]byte(nil), img.data...)
	}
	return clone
}
