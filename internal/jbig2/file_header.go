package jbig2

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

var jbig2FileSignature = []byte{0x97, 0x4a, 0x42, 0x32, 0x0d, 0x0a, 0x1a, 0x0a}

// FileHeader captures the parsed JBIG2 file header fields.
type FileHeader struct {
	Flags      uint8
	NumPages   uint32
	HasNumPage bool
}

// stripJBIG2FileHeader removes the JBIG2 file header if present. JBIG2 files
// begin with an 8-byte signature followed by a little-endian flags field and a
// little-endian number-of-pages field. The decoder expects to consume raw
// segments, so this helper recognises the signature, parses the header, and
// returns the remaining byte slice.
func stripJBIG2FileHeader(data []byte) ([]byte, *FileHeader, error) {
	if len(data) < len(jbig2FileSignature) {
		return data, nil, nil
	}
	if !bytes.Equal(data[:len(jbig2FileSignature)], jbig2FileSignature) {
		return data, nil, nil
	}
	// Signature (8 bytes) + 1 byte flags + optional 4 byte page count when known.
	if len(data) < len(jbig2FileSignature)+1 {
		return nil, nil, fmt.Errorf("jbig2: truncated file header, need at least %d bytes", len(jbig2FileSignature)+1)
	}
	flags := data[8]
	offset := len(jbig2FileSignature) + 1
	header := &FileHeader{Flags: flags}
	if flags&0x02 == 0 {
		if len(data) < offset+4 {
			return nil, nil, fmt.Errorf("jbig2: truncated file header, missing page count")
		}
		header.NumPages = binary.BigEndian.Uint32(data[offset : offset+4])
		header.HasNumPage = true
		offset += 4
	}
	return data[offset:], header, nil
}
