# AGENTS.md — Meta View

Guidance for AI agents working in this repository.

## What this is

A single-binary, cross-platform file-metadata inspector (inspired by metadata2go).
Go backend + embedded vanilla-JS web UI. Files are parsed locally; nothing is uploaded.
Design mockups live in `design/` (dark UI, teal accents, drop zone, summary cards, property tables).

## Toolchain

- Go >= 1.24 (installed on this machine at `C:\Program Files\Go\bin` — if `go` is not found, prepend it to PATH for the session).
- Python 3 only for generating test fixtures. Node only for `node --check web/app.js`.
- **Zero Go dependencies** (pure stdlib). Keep it that way — portability is the core requirement.

## Commands

```sh
go build -o meta-view.exe .        # current platform (meta-view on unix)
go vet ./... && gofmt -l .         # lint (run gofmt -w . to fix)
go test ./...                      # smoke tests (need testfiles/ generated first)
python gen_testfiles.py            # regenerate testfiles/ fixtures
.\build.ps1                        # all 6 release binaries -> dist/ (also build.sh on unix)
.\meta-view.exe -no-browser -port 8765   # dev server without opening browser
```

Always run `gofmt -w .`, `go vet ./...`, `go test ./...` and a build before declaring work done.

## Layout

```
main.go            HTTP server, flags, browser launch, //go:embed web
helpers.go         jsonEncoder / readAllMax
internal/meta/     all extraction logic (no external deps)
  meta.go          Property/Section/Result types, detect() magic-byte dispatcher, hexDump
  exif.go          TIFF/EXIF parser (IFD0, Exif, GPS, Interop sub-IFDs)
  jpeg.go          JPEG markers: JFIF, APP1 Exif/XMP, COM, SOF
  png.go  images.go (GIF/BMP/WebP/SVG)  media.go (MP3/FLAC/WAV/MP4-MOV-M4A/AVI/PDF/Opus-Ogg)
  extract_test.go  smoke test over testfiles/
web/               index.html, style.css, app.js (no framework, no build step)
design/            original UI mockups — match this look
dist/              cross-compiled release binaries (gitignored)
```

## Data model

`Extract(name, data, modTime) *Result` → `{fileName, ext, mime, size, format, width, height, sections[], count}`.
A `Section` is a named list of `Property{Key, Value}`; values are always pre-formatted strings.
`Result.add(section, props...)` appends or merges into a section by name.
Frontend renders sections as collapsible tables; `Raw Data` section starts collapsed.

## Conventions

- Property keys: lowercase, human-readable (`"image width"`, `"date time original"`).
- Enum values formatted as `"Label (N)"` (e.g. `Rotate 90 CW (6)`).
- Unknown EXIF tags surface as `tag 0xXXXX`; unknown formats still return File Properties + Raw Data.
- Parsers must be bounds-checked — never panic on truncated/hostile files (server must survive any upload).
- Frontend: no framework, no CDN, keep everything inside `web/`; palette via CSS variables, dark default, light via `prefers-color-scheme`.
- UI must stay user-friendly: drag/drop/paste, live search, click-to-copy, JSON/CSV export.

## Gotchas (learned the hard way)

- TIFF `typeSize`: SHORT=2, LONG=4, RATIONAL=8 — wrong sizes silently break sub-IFD pointers.
- `sampleRates` in media.go is indexed `verIdx*4 + srIdx` (16 entries, reserved slots kept).
- JFIF densities are at payload offsets 8–11 (after `JFIF\0` + version + units).
- `multipart.FileHeader` has no `ModTime`/`ModifiedAt` in this Go version — pass `time.Time{}`.
- `go:embed web` requires the `web/` dir at module root.
- Windows PowerShell 5.1: no `&&`; chain with `; if ($?) { ... }`.

## Testing strategy

`gen_testfiles.py` builds minimal valid files for every supported format (plus a hand-built
TIFF/JPEG with EXIF+GPS). `extract_test.go` asserts format detection, minimum property count,
and no NUL bytes leaking into values. For manual e2e: start dev server, then
`curl.exe -F "file=@testfiles/test.jpg" http://127.0.0.1:8765/api/extract`.
The design PNG screenshots in `design/` double as real-world PNG fixtures.

## Release

`build.ps1` / `build.sh` cross-compile windows/darwin/linux × amd64/arm64 with
`CGO_ENABLED=0 -trimpath -ldflags "-s -w"`. No C toolchain needed — do not add cgo.
