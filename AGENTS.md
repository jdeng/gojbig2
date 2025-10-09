# Repository Guidelines

## Project Structure & Module Organization
The repository currently mirrors the PDFium JBIG2 decoder in `reference/jbig2` and the FAX module in `reference/fax`; treat these as the authoritative behavior spec while we port features to Go. The Go module (`go.mod`) already declares `gojbig2`; add new translation code under `internal/jbig2` (decoder, bitstream, arithmetic) and `internal/fax` to keep the surface area private until the API stabilizes. Place integration entry points and public types in `pkg/jbig2` once they are production-ready, and co-locate tests with the package they exercise.

## Build, Test, and Development Commands
Run `go test ./...` to compile and execute every Go package; this also confirms the module graph is intact. Use `go test ./internal/jbig2 -run TestBitStream` (or similar) for focused work. When wiring binaries or examples, prefer lightweight Go programs under `cmd/` and build them with `go build ./cmd/<tool>`.

## Coding Style & Naming Conventions
Follow canonical Go style: tabs for indentation, `gofmt` for formatting, and `goimports` to manage imports. Use mixedCaps for exported Go identifiers (`Context`, `DecodeOptions`) and keep unexported helpers lowerCamel. Mirror C++ entity names where parity helps (`bitstream.go` for `JBig2_BitStream`), but evolve toward idiomatic Go naming (`BitStream` → `Bitstream` methods, error returns instead of status enums).

## Testing Guidelines
Adopt Go’s `testing` package with table-driven tests. Name files `*_test.go`, functions `Test<Entity>` to align with the originating C++ coverage. Start by porting gtest scenarios once the corresponding entity is translated, and add regression tests for edge cases uncovered during translation. Target full coverage of decoding primitives before layering document-level flows.

## Commit & Pull Request Guidelines
Non-scoped Conventional Commits work well here (`feat: add pattern dictionary translator`). Reference the C++ source you mirrored in the description and call out any intentional divergence. Pull requests should summarize validation (`go test ./...`) and highlight follow-up items so reviewers can gauge remaining parity gaps.
