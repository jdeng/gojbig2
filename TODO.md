# Translation TODO

Backlog derived from the per-entity mapping in `PLAN.md`. Each item mirrors a PDFium component targeted for the Go port.

## Core Data Definitions (PLAN step 1)
- [x] Region info & constants (`internal/jbig2/define.go`).
- [x] Segment metadata (`internal/jbig2/segment.go`).
- [x] Image buffers (`internal/jbig2/image.go`).
- [x] Page metadata (`internal/jbig2/page.go`).

## I/O Primitives (PLAN step 2)
- [x] Bitstream reader (`internal/jbig2/bitstream.go`).
- [x] Arithmetic decoder + IAID/Int decoders (`internal/jbig2/arith_decoder.go`, `arith_int_decoder.go`).
- [x] Huffman tables and decoder (`internal/jbig2/huffman_table.go`, `huffman_decoder.go`).

## Dictionary & Region Procedures (PLAN step 3)
- [x] Pattern dictionary container (`internal/jbig2/pattern_dict.go`).
- [x] Symbol dictionary container & arithmetic decode (`internal/jbig2/symbol_dict.go`, `sdd_proc.go`).
- [x] Symbol dictionary Huffman refinement + MMR bitmap support (`internal/jbig2/sdd_proc.go`).
- [x] Generic region progressive + MMR decode paths (`internal/jbig2/grd_proc.go`).
- [x] Generic refinement region arithmetic decode (`internal/jbig2/grrd_proc.go`).
- [x] Text region Huffman decode (`internal/jbig2/trd_proc.go`).
- [x] Text region arithmetic decode (`internal/jbig2/trd_proc.go`).
- [x] Halftone pattern dictionary decode (arith & MMR) (`internal/jbig2/pdd_proc.go`).
- [x] Halftone region decode (arith & MMR) (`internal/jbig2/htrd_proc.go`).

## Context & API Surface (PLAN step 4)
- [x] Document context cache (`internal/jbig2/document_context.go`).
- [x] Context segment handlers for pattern, generic, refinement, halftone, and text regions (`internal/jbig2/context.go`).
- [x] Support table segments & custom Huffman definitions (`internal/jbig2/context.go`).
- [x] Decoder entry points (`internal/jbig2/decoder.go`, `pkg/jbig2/decoder.go`).

## FAX Support (PLAN step 5)
- [x] Translate PDFium FAX module (`internal/fax`).

## Testing & Validation
- [ ] Add unit tests for translated entities (`internal/jbig2/*_test.go`).
- [ ] Add end-to-end decoding/integration tests once pipeline is functional.
