package jbig2

import "errors"

// DecoderOptions configures JBIG2 decoding behavior.
type DecoderOptions struct {
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
	ctx *Context
}

// NewDecoder creates a new JBIG2 decoder with the provided options.
func NewDecoder(opts DecoderOptions) (*Decoder, error) {
	if len(opts.SrcData) == 0 {
		return nil, errors.New("jbig2: empty source data")
	}

	docCtx := NewDocumentContext()
	ctx, err := CreateContext(opts.GlobalData, opts.GlobalKey, opts.SrcData, opts.SrcKey, docCtx)
	if err != nil {
		return nil, err
	}

	return &Decoder{ctx: ctx}, nil
}

// DecodeAll processes all segments in the JBIG2 stream.
func (d *Decoder) DecodeAll() error {
	if err := d.ctx.decodeGlobals(nil); err != nil {
		return err
	}
	_, err := d.ctx.DecodeSequential(nil)
	return err
}

// GetFirstPage prepares the first page for rendering.
func (d *Decoder) GetFirstPage(buf []byte, width, height, stride int) (bool, error) {
	return d.ctx.GetFirstPage(buf, width, height, stride, nil)
}

// Continue resumes decoding after a pause.
func (d *Decoder) Continue() (bool, error) {
	return d.ctx.Continue(nil)
}

// GetPageImage returns the current decoded page image.
func (d *Decoder) GetPageImage() *Image {
	return d.ctx.PageImage()
}

// GetProcessingStatus returns the current codec processing status.
func (d *Decoder) GetProcessingStatus() CodecStatus {
	return d.ctx.ProcessingStatus()
}

// GetSegments returns all decoded segments.
func (d *Decoder) GetSegments() []*Segment {
	return d.ctx.Segments()
}
