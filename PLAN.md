# JBIG2 Translation Plan

This plan mirrors the backlog in `TODO.md` and tracks the sequence we will follow while finishing the Go port.

## Status Snapshot (derived from `TODO.md`)
- ✅ **Phase 1 – Core data definitions:** complete (`define.go`, `segment.go`, `image.go`, `page.go`).
- ✅ **Phase 2 – I/O primitives:** bitstream, arithmetic, and Huffman decoders are translated.
- ✅ **Phase 3 – Dictionaries & regions:** symbol, pattern, generic, refinement, text, and halftone procedures compiled.
- ✅ **Phase 4 – Context & API:** document context, segment handlers, and the public decoder facade shipped.
- ✅ **Phase 5 – FAX support:** baseline module in `internal/fax` is present.
- 🔄 **Testing & validation:** unit and integration test coverage still needs to be built out (open items in `TODO.md`).

## Near-Term Priorities
1. **Unit test coverage (in progress):**
   - Port PDFium gtests and craft Go-first scenarios for bitstream, arithmetic, region procedures, and context orchestration.
   - Stabilize fixtures so `go test ./internal/jbig2` covers all core primitives without depending on command binaries.
2. **Integration tests:**
   - Add golden-image decoding tests that run whole-segment samples through `pkg/jbig2.Decoder`.
   - Capture regression cases discovered during manual runs of `cmd/jbig2jpg`.
3. **API hardening:**
   - Refine `pkg/jbig2` types and error messages once coverage is in place.
   - Document public API expectations and retention policy before tagging a release candidate.

## Reference Mapping (unchanged)
| Domain | Source (PDFium) | Go Translation | Notes |
| --- | --- | --- | --- |
| Bitstream & entropy | `JBig2_BitStream`, `JBig2_ArithDecoder`, `JBig2_HuffmanDecoder` | `internal/jbig2/bitstream.go`, `arith_decoder.go`, `huffman_decoder.go` | Foundational primitives shared by all regions. |
| Structures & metadata | `JBig2_Define`, `JBig2_Segment`, `JBig2_Image`, `JBig2_Page` | `internal/jbig2/define.go`, `segment.go`, `image.go`, `page.go` | Model JBIG2 pages, segments, and images. |
| Region procedures | `JBig2_GrdProc`, `JBig2_GrrdProc`, `JBig2_TrdProc`, `JBig2_PddProc`, `JBig2_HtrdProc`, `JBig2_SddProc` | `internal/jbig2/grd_proc.go`, `grrd_proc.go`, `trd_proc.go`, `pdd_proc.go`, `htrd_proc.go`, `sdd_proc.go` | Decode individual segment payloads; reuse shared primitives. |
| Dictionaries & caches | `JBig2_PatternDict`, `JBig2_SymbolDict`, `JBig2_Context`, `JBig2_DocumentContext` | `internal/jbig2/pattern_dict.go`, `symbol_dict.go`, `context.go`, `document_context.go` | Manage decoded artifacts and lifecycle. |
| Decoder surface | `jbig2_decoder.cpp` | `internal/jbig2/decoder.go`, `pkg/jbig2/decoder.go` | Bridge to callers; ensures `internal` package remains private. |
| FAX module | `faxmodule.cpp` | `internal/fax/faxmodule.go` | Supports MMR paths used by halftone/symbol segments. |

## Working Agreements
- Keep translation work under `internal/` until the API and coverage solidify.
- Use `go test ./internal/jbig2` and targeted `-run` filters for fast feedback; full `go test ./...` should pass once the `cmd/` binaries build cleanly.
- Update `TODO.md` alongside implementation changes so this plan continues to reflect reality.
