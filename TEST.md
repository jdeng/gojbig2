# Test Inventory

Status captured from running `GOCACHE=$(pwd)/.gocache go test` on 2025-10-08 inside the workspace sandbox.

## Quick Summary
- ✅ `go test ./internal/jbig2`
- ✅ `go test ./pkg/jbig2`
- ✅ `go test ./...`

## Unit Tests by Package
| Package | Test File | Focus | Status | Notes |
| --- | --- | --- | --- | --- |
| `internal/jbig2` | `bitstream_test.go` | Bit-level reader and signed/unsigned helpers | ✅ Pass | Exercises `ReadNBits`, `Read1Bit`, error paths. |
| `internal/jbig2` | `context_test.go` | Page composition & segment caches | ✅ Pass | Verifies striped page growth and symbol dictionary cache isolation. |
| `internal/jbig2` | `file_header_test.go` | Header parsing helpers | ✅ Pass | Ensures default header values and magic detection. |
| `internal/jbig2` | `image_test.go` | Image buffer utilities | ✅ Pass | Checks pixel set/get and resizing helpers. |
| `internal/jbig2` | `pdd_proc_test.go` | Pattern dict decode stubs | ✅ Pass | Validates placeholder arithmetic paths. |
| `internal/jbig2` | `htrd_proc_test.go` | Halftone region routines | ✅ Pass | Confirms image composition boundaries. |
| `pkg/jbig2` | `decoder_test.go` | Public API surface | ✅ Pass | Covers decoder construction, options, status enums. |

## Gaps & Follow-Ups
- Port the remaining PDFium gtests for arithmetic decoders, symbol dictionaries, and text regions to close coverage gaps.
- Add integration tests that decode sample JBIG2 streams end-to-end for regression coverage beyond unit tests.
