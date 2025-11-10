package jbig2

import (
	"errors"
	"fmt"

	"github.com/jdeng/gojbig2/internal/jbig2"
)

// Options configures JBIG2 decoding behavior.
type Options struct {
	// GlobalData provides optional global segment data for the decoding context.
	GlobalData []byte
	// GlobalKey identifies the global data stream.
	GlobalKey uint64
	// SrcData contains the main JBIG2 data to decode.
	SrcData []byte
	// SrcKey identifies the source data stream.
	SrcKey uint64
}

// Decoder manages the JBIG2 decoding process.
type Decoder struct {
	decoder *jbig2.Decoder
}

// New creates a new JBIG2 decoder with the provided options.
func New(opts Options) (*Decoder, error) {
	if len(opts.SrcData) == 0 {
		return nil, errors.New("jbig2: empty source data")
	}

	internalDecoder, err := jbig2.NewDecoder(jbig2.DecoderOptions{
		GlobalData: opts.GlobalData,
		GlobalKey:  opts.GlobalKey,
		SrcData:    opts.SrcData,
		SrcKey:     opts.SrcKey,
	})
	if err != nil {
		return nil, err
	}

	return &Decoder{decoder: internalDecoder}, nil
}

// DecodeAll processes all segments in the JBIG2 stream.
func (d *Decoder) DecodeAll() error {
	return d.decoder.DecodeAll()
}

// GetFirstPage prepares the first page for rendering.
func (d *Decoder) GetFirstPage(buf []byte, width, height, stride int) (bool, error) {
	return d.decoder.GetFirstPage(buf, width, height, stride)
}

// Continue resumes decoding after a pause.
func (d *Decoder) Continue() (bool, error) {
	return d.decoder.Continue()
}

// GetPageImage returns the current decoded page image.
func (d *Decoder) GetPageImage() *Image {
	internalImg := d.decoder.GetPageImage()
	if internalImg == nil {
		return nil
	}
	// Convert internal image to public API image
	return &Image{img: internalImg}
}

// GetProcessingStatus returns the current codec processing status.
func (d *Decoder) GetProcessingStatus() CodecStatus {
	return CodecStatus(d.decoder.GetProcessingStatus())
}

// GetSegments returns all decoded segments.
func (d *Decoder) GetSegments() []*Segment {
	internalSegments := d.decoder.GetSegments()
	segments := make([]*Segment, len(internalSegments))
	for i, seg := range internalSegments {
		segments[i] = &Segment{seg: seg}
	}
	return segments
}

// CodecStatus represents the current state of the decoder.
type CodecStatus int

const (
	// CodecStatusReady indicates the decoder is ready to process data.
	CodecStatusReady CodecStatus = iota
	// CodecStatusToBeContinued indicates processing was paused and can be resumed.
	CodecStatusToBeContinued
	// CodecStatusFinished indicates processing has completed successfully.
	CodecStatusFinished
	// CodecStatusError indicates an error occurred during processing.
	CodecStatusError
)

func (status CodecStatus) String() string {
	switch status {
	case CodecStatusReady:
		return "Ready"
	case CodecStatusToBeContinued:
		return "ToBeContinued"
	case CodecStatusFinished:
		return "Finished"
	case CodecStatusError:
		return "Error"
	default:
		return fmt.Sprintf("CodecStatus(%d)", int(status))
	}
}

// Image represents a decoded JBIG2 image.
type Image struct {
	img *jbig2.Image
}

// Width returns the image width in pixels.
func (img *Image) Width() int {
	if img == nil || img.img == nil {
		return 0
	}
	return int(img.img.Width())
}

// Height returns the image height in pixels.
func (img *Image) Height() int {
	if img == nil || img.img == nil {
		return 0
	}
	return int(img.img.Height())
}

// Data returns the raw image pixel data.
func (img *Image) Data() []byte {
	if img == nil || img.img == nil {
		return nil
	}
	return img.img.Data()
}

// Segment represents a decoded JBIG2 segment.
type Segment struct {
	seg *jbig2.Segment
}

// Number returns the segment number.
func (seg *Segment) Number() uint32 {
	if seg == nil || seg.seg == nil {
		return 0
	}
	return seg.seg.Number
}

// Type returns the segment type.
func (seg *Segment) Type() uint8 {
	if seg == nil || seg.seg == nil {
		return 0
	}
	return seg.seg.Flags.Type()
}

// DataLength returns the length of the segment data.
func (seg *Segment) DataLength() uint32 {
	if seg == nil || seg.seg == nil {
		return 0
	}
	return seg.seg.DataLength
}

// ResultType returns the type of result this segment produced.
func (seg *Segment) ResultType() ResultType {
	if seg == nil || seg.seg == nil {
		return ResultTypeVoid
	}
	return ResultType(seg.seg.ResultType)
}

// Image returns the decoded image if this segment produced one.
func (seg *Segment) Image() *Image {
	if seg == nil || seg.seg == nil || seg.seg.Image == nil {
		return nil
	}
	return &Image{img: seg.seg.Image}
}

// SymbolDict returns the decoded symbol dictionary if this segment produced one.
func (seg *Segment) SymbolDict() *SymbolDict {
	if seg == nil || seg.seg == nil || seg.seg.SymbolDict == nil {
		return nil
	}
	return &SymbolDict{dict: seg.seg.SymbolDict}
}

// PatternDict returns the decoded pattern dictionary if this segment produced one.
func (seg *Segment) PatternDict() *PatternDict {
	if seg == nil || seg.seg == nil || seg.seg.PatternDict == nil {
		return nil
	}
	return &PatternDict{dict: seg.seg.PatternDict}
}

// HuffmanTable returns the decoded Huffman table if this segment produced one.
func (seg *Segment) HuffmanTable() *HuffmanTable {
	if seg == nil || seg.seg == nil || seg.seg.HuffmanTable == nil {
		return nil
	}
	return &HuffmanTable{table: seg.seg.HuffmanTable}
}

// ResultType identifies what kind of result payload a segment produced.
type ResultType int

const (
	// ResultTypeVoid indicates the segment produced no result.
	ResultTypeVoid ResultType = iota
	// ResultTypeImage indicates the segment produced an image.
	ResultTypeImage
	// ResultTypeSymbolDict indicates the segment produced a symbol dictionary.
	ResultTypeSymbolDict
	// ResultTypePatternDict indicates the segment produced a pattern dictionary.
	ResultTypePatternDict
	// ResultTypeHuffmanTable indicates the segment produced a Huffman table.
	ResultTypeHuffmanTable
)

func (rt ResultType) String() string {
	switch rt {
	case ResultTypeVoid:
		return "Void"
	case ResultTypeImage:
		return "Image"
	case ResultTypeSymbolDict:
		return "SymbolDict"
	case ResultTypePatternDict:
		return "PatternDict"
	case ResultTypeHuffmanTable:
		return "HuffmanTable"
	default:
		return fmt.Sprintf("ResultType(%d)", int(rt))
	}
}

// SymbolDict represents a decoded symbol dictionary.
type SymbolDict struct {
	dict *jbig2.SymbolDict
}

// NumImages returns the number of symbols in the dictionary.
func (sd *SymbolDict) NumImages() int {
	if sd == nil || sd.dict == nil {
		return 0
	}
	return sd.dict.NumImages()
}

// GetImage returns the symbol at the specified index.
func (sd *SymbolDict) GetImage(index int) *Image {
	if sd == nil || sd.dict == nil || index < 0 || index >= sd.dict.NumImages() {
		return nil
	}
	img := sd.dict.GetImage(index)
	if img == nil {
		return nil
	}
	return &Image{img: img}
}

// PatternDict represents a decoded pattern dictionary.
type PatternDict struct {
	dict *jbig2.PatternDict
}

// NumPatterns returns the number of patterns in the dictionary.
func (pd *PatternDict) NumPatterns() uint32 {
	if pd == nil || pd.dict == nil {
		return 0
	}
	return pd.dict.NumPatterns
}

// GetPattern returns the pattern at the specified index.
func (pd *PatternDict) GetPattern(index uint32) *Image {
	if pd == nil || pd.dict == nil || int(index) >= len(pd.dict.Patterns) {
		return nil
	}
	img := pd.dict.GetPattern(index)
	if img == nil {
		return nil
	}
	return &Image{img: img}
}

// HuffmanTable represents a decoded Huffman table.
type HuffmanTable struct {
	table *jbig2.HuffmanTable
}
