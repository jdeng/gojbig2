package jbig2

// DocumentContext holds per-document JBIG2 state, primarily the LRU cache of
// symbol dictionaries shared across page contexts.
type DocumentContext struct {
	symbolDictCache []CachePair
}

// NewDocumentContext creates an empty document context.
func NewDocumentContext() *DocumentContext {
	return &DocumentContext{symbolDictCache: make([]CachePair, 0, symbolDictCacheMaxSize)}
}

// SymbolDictCache returns a pointer to the LRU cache slice used by decoding contexts.
func (dc *DocumentContext) SymbolDictCache() *[]CachePair {
	return &dc.symbolDictCache
}
