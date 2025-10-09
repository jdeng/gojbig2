package jbig2

// PatternDict stores the set of halftone patterns referenced by halftone regions.
type PatternDict struct {
	NumPatterns uint32
	Patterns    []*Image
}

// NewPatternDict allocates a dictionary sized to the provided number of patterns.
func NewPatternDict(dictSize uint32) *PatternDict {
	return &PatternDict{
		NumPatterns: dictSize,
		Patterns:    make([]*Image, dictSize),
	}
}

// SetPattern installs an image at the requested index.
func (pd *PatternDict) SetPattern(index uint32, img *Image) {
	if pd == nil || int(index) >= len(pd.Patterns) {
		return
	}
	pd.Patterns[index] = img
}

// GetPattern returns the pattern image at the requested index.
func (pd *PatternDict) GetPattern(index uint32) *Image {
	if pd == nil || int(index) >= len(pd.Patterns) {
		return nil
	}
	return pd.Patterns[index]
}
