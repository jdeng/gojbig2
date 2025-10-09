# gojbig2

`gojbig2` is a work-in-progress translation of the PDFium JBIG2 decoder into Go. The C++ reference implementation lives under `reference/jbig2` (and `reference/fax` for the supporting FAX paths); the Go sources under `internal/` mirror feature parity, while `pkg/jbig2` hosts the draft public API until the surface stabilizes.

## Repository Layout
- `internal/jbig2`: private Go port of the JBIG2 primitives (bitstream, arithmetic/Huffman decoders, region procedures, context orchestration).
- `internal/fax`: Go translation of the PDFium FAX module used by JBIG2 MMR paths.
- `pkg/jbig2`: draft public API surface wrapping the internal decoder while the interface settles.
- `cmd/`: developer binaries (`jbig2jpg`, `create-test-jbig2`) used for smoke testing and fixture generation.
- `reference/jbig2`: authoritative PDFium JBIG2 decoder used as the behavioral spec.
- `reference/fax`: authoritative PDFium FAX module mirrored by `internal/fax`.
- `PLAN.md`: per-entity translation plan aligned with the active backlog.
- `TODO.md`: current implementation backlog derived from `PLAN.md`.
- `ARCHITECTURE.md`: tech stack overview and the layered layout of the port.
- `TEST.md`: unit-test inventory with pass/fail status and follow-up actions.

## Translation Status
The table below summarizes the porting progress against the plan in `PLAN.md`.

| Area | Status | Notes |
| ---- | ------ | ----- |
| Core data definitions (`define.go`, `segment.go`, `image.go`, `page.go`) | ✅ Done | Region structs, segment metadata, and page/image helpers are in place. |
| File header parsing (`file_header.go`) | ✅ Done | JBIG2 signature stripping and metadata extraction covered by unit tests. |
| Bitstream & arithmetic primitives (`bitstream.go`, `arith_decoder.go`, `arith_int_decoder.go`) | ✅ Done | Bit-level reader and arithmetic decoders match the reference behavior. |
| Huffman tables & decoder (`huffman_table.go`, `huffman_decoder.go`) | ✅ Done | Canonical table assignment and decode loop implemented. |
| Symbol dictionary (`symbol_dict.go`, `sdd_proc.go`) | ✅ Done | Arithmetic and Huffman paths, including MMR bitmap support, are translated. |
| Generic region (`grd_proc.go`) | ✅ Done | Arithmetic, progressive, and MMR decode paths mirror the reference implementation. |
| Refinement region (`grrd_proc.go`) | ✅ Done | Arithmetic refinement decode (templates 0/1) implemented. |
| Text region (`trd_proc.go`) | ✅ Done | Arithmetic and Huffman decode pipelines are in place. |
| Halftone dictionary/region (`pdd_proc.go`, `htrd_proc.go`) | ✅ Done | Arithmetic + MMR paths ported, including bit-plane assembly and pattern composition. |
| Context orchestration (`context.go`, `document_context.go`) | ✅ Done | Segment handlers cover pattern, generic, refinement, halftone, text, and table segments. |
| Decoder entry points (`internal/jbig2/decoder.go`, `pkg/jbig2/decoder.go`) | ✅ Done | Internal decoder scaffold and public API wrapper are available. |
| FAX support (`internal/fax`) | ✅ Done | Initial module translation landed; integration validation tracked separately. |
| Testing (`*_test.go`) | 🟡 In progress | Core unit tests exist; broader coverage and e2e fixtures tracked in `TEST.md`. |

## Working Notes
- In sandboxed environments, run `GOCACHE=$(pwd)/.gocache go test ./...` to keep build artifacts writable; focused commands such as `GOCACHE=$(pwd)/.gocache go test ./internal/jbig2 -run TestBitStream` remain useful during translation.
- Refer to `PLAN.md` for the expected implementation order and entity mapping when picking up new work.
- Keep new code under `internal/` until the decoder API is production-ready.
- Review `ARCHITECTURE.md` when onboarding to the codebase, and `TEST.md` before modifying decoder primitives or public APIs.
- `go build ./cmd/jbig2jpg` provides a quick smoke test path; `./cmd/create-test-jbig2` helps mint fixture streams while expanding coverage.
