package jbig2

import "testing"

func TestComposeRegionExpandsStripedPage(t *testing.T) {
	ctx := &Context{
		page: NewImage(4, 4),
		pageInfos: []*PageInfo{{
			Width:             4,
			Height:            4,
			DefaultPixelValue: false,
			Striped:           true,
		}},
		inPage: true,
	}

	src := NewImage(2, 2)
	src.SetPixel(0, 0, 1)

	ri := RegionInfo{Width: 2, Height: 2, X: 0, Y: 3, Flags: 0}
	if err := ctx.composeRegion(ri, src, nil); err != nil {
		t.Fatalf("composeRegion returned error: %v", err)
	}

	if got := ctx.page.Height(); got != 5 {
		t.Fatalf("expected expanded height 5, got %d", got)
	}
	if pixel := ctx.page.GetPixel(0, 3); pixel != 1 {
		t.Fatalf("expected composed pixel at (0,3) to be 1, got %d", pixel)
	}
}

func TestParseSymbolDictSegmentUsesCache(t *testing.T) {
	doc := NewDocumentContext()
	cacheKey := CompoundKey{StreamKey: 42, Segment: 0}

	baseDict := NewSymbolDict()
	img := NewImage(1, 1)
	img.SetPixel(0, 0, 1)
	baseDict.AddImage(img)
	slice := doc.SymbolDictCache()
	*slice = append(*slice, CachePair{Key: cacheKey, Dict: baseDict})

	streamData := []byte{
		0x00, 0x01, // flags: SDHUFF only
		0x00, 0x00, 0x00, 0x00, // SDNUMEXSYMS
		0x00, 0x00, 0x00, 0x00, // SDNUMNEWSYMS
	}
	ctx := newContext(streamData, cacheKey.StreamKey, doc, true)

	seg := NewSegment()
	seg.Key = cacheKey.StreamKey
	seg.DataOffset = cacheKey.Segment
	seg.Flags = SegmentFlags(segmentTypeSymbolDict)
	seg.ReferredToSegmentCount = 0

	res, err := ctx.parseSymbolDictSegment(seg, nil)
	if err != nil {
		t.Fatalf("parseSymbolDictSegment returned error: %v", err)
	}
	if res != DecodeResultSuccess {
		t.Fatalf("expected DecodeResultSuccess, got %v", res)
	}
	if seg.SymbolDict == nil {
		t.Fatal("expected symbol dictionary on segment")
	}
	if seg.SymbolDict == baseDict {
		t.Fatal("expected deep copy of cached dictionary")
	}
	if seg.SymbolDict.NumImages() != 1 {
		t.Fatalf("expected 1 symbol, got %d", seg.SymbolDict.NumImages())
	}
	if pix := seg.SymbolDict.GetImage(0).GetPixel(0, 0); pix != 1 {
		t.Fatalf("expected copied image pixel to be 1, got %d", pix)
	}

	seg.SymbolDict.GetImage(0).SetPixel(0, 0, 0)
	cached := (*doc.SymbolDictCache())[0].Dict
	if pix := cached.GetImage(0).GetPixel(0, 0); pix != 1 {
		t.Fatal("cache dictionary mutated by consumer")
	}

	expectedOffset := uint32(len(streamData))
	if got := ctx.stream.Offset(); got != expectedOffset {
		t.Fatalf("expected stream offset %d, got %d", expectedOffset, got)
	}
}

func TestParseTablesSegmentMinimal(t *testing.T) {
	data := []byte{
		0x00,                   // flag
		0x00, 0x00, 0x00, 0x00, // low
		0x00, 0x00, 0x00, 0x00, // high
		0x00, // code bits
	}
	ctx := newContext(data, 0, nil, false)

	seg := NewSegment()
	seg.Flags = SegmentFlags(segmentTypeTables)
	seg.DataLength = uint32(len(data))

	res, err := ctx.parseTablesSegment(seg)
	if err != nil {
		t.Fatalf("parseTablesSegment returned error: %v", err)
	}
	if res != DecodeResultSuccess {
		t.Fatalf("expected DecodeResultSuccess, got %v", res)
	}
	if seg.HuffmanTable == nil {
		t.Fatal("expected Huffman table on segment")
	}
}
