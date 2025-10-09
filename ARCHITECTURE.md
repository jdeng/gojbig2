# Architecture

## Tech Stack
- **Language:** Go 1.x using the standard library only.
- **Reference Spec:** PDFium's C++ JBIG2 decoder (`reference/jbig2`) and FAX module (`reference/fax`).
- **Build/Test Tooling:** Go toolchain commands (`go build`, `go test`) with per-package unit tests.
- **CLI Utilities:** Small Go entry points under `cmd/` provide binaries for exercising the decoder.

## Layered Layout
1. **Reference sources (`reference/…`)** mirror the upstream C++ implementation and serve as the behavioral specification while porting.
2. **Core translation (`internal/jbig2`)** contains private Go equivalents for the decoder primitives:
   - Bitstream reader, arithmetic/Huffman decoders, and shared data structures (`bitstream.go`, `arith_decoder.go`, `define.go`, `segment.go`, `image.go`, `page.go`).
   - Region and dictionary procedures (`sdd_proc.go`, `grd_proc.go`, `grrd_proc.go`, `trd_proc.go`, `pdd_proc.go`, `htrd_proc.go`, `pattern_dict.go`, `symbol_dict.go`).
   - Context orchestration and document-wide state (`context.go`, `document_context.go`, `decoder.go`).
   - Unit tests (`*_test.go`) keep coverage co-located with the code they exercise.
3. **FAX support (`internal/fax`)** contains the initial translation of the PDFium module; wiring it into the JBIG2 pipeline waits on validation.
4. **Public facade (`pkg/jbig2`)** exposes an idiomatic Go API that wraps the internal decoder and presents stable types to callers (`Decoder`, `Segment`, `Image`).
5. **Command-line tools (`cmd/…`)** assemble thin binaries around the public API for manual testing (e.g. `cmd/jbig2jpg`).

Only `pkg/jbig2` is intended for external consumption; everything else remains under `internal/` while behavior is validated against the reference.

## Data Flow Overview
1. Callers instantiate `pkg/jbig2.Decoder`, which populates an `internal/jbig2.Decoder` with the provided streams (`DecoderOptions`).
2. The decoder parses segment headers via `file_header.go` and `segment.go`, then dispatches to the relevant region/dictionary procedures based on segment type.
3. Each procedure consumes the shared primitives:
   - `BitStream` supplies bit-level reads from the JBIG2 payload.
   - `ArithmeticDecoder` and `HuffmanDecoder` perform entropy decoding, aided by `HuffmanTable` construction helpers.
   - Region implementations (generic, refinement, text, halftone) reconstruct bitmaps and dictionaries, reusing helpers in `image.go` and `pattern_dict.go`.
4. Decoded artifacts are cached inside the `DocumentContext` and exposed to the public layer as `Image`, `SymbolDict`, `PatternDict`, or `HuffmanTable` handles.

## Concurrency & Error Handling
- The current implementation assumes single-threaded decoding, matching the reference behavior. Shared state (segment caches, document context) is not guarded for concurrent access.
- Errors propagate using Go `error` returns instead of PDFium status enums; callers should treat any non-nil error as fatal for the current decode session.

## Future Extensions
- Finalize FAX integration and wire the module into the segment dispatcher once JBIG2 parity is validated.
- Grow integration tests (golden files or PDF fixtures) under `test/` or dedicated `cmd/` tools as the public API stabilizes.
