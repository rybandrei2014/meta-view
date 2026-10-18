# Meta View

A fast, portable, single-binary tool to inspect file metadata. One small executable contains the whole app (backend + UI).
Files are parsed **locally**; nothing is uploaded anywhere.

![upload](design/file-upload.PNG)
![results](design/extracted%20metadata%20view.PNG)

## Features

- **Single static binary**, no installer, no runtime, no internet needed
- **Cross-platform**: Windows, macOS, Linux (amd64 + arm64)
- Neutral dark/light color palette (follows system theme)
- Drag & drop, paste (Ctrl+V) or file picker
- Summary cards (type, size, dimensions, property count)
- Live search across all properties and values
- Collapsible sections with counts (File, JPEG, EXIF, EXIF Photo, GPS, XMP, …)
- Click any value to copy it
- Export as JSON or CSV
- Raw hex dump of the first 256 bytes
- Works with any file — unknown formats still show file properties + raw data

## Supported formats

| Format | What is extracted |
|---|---|
| JPEG | JFIF, EXIF (IFD0 / Exif / GPS / Interop), XMP, comment, dimensions, encoding process |
| PNG | IHDR, pHYs (dpi), gAMA, sRGB, tEXt / zTXt / iTXt, tIME, iCCP, cHRM |
| GIF | version, dimensions, color table, animation frame count |
| BMP | header, dimensions, bit depth, compression |
| WebP | VP8 / VP8L / VP8X, canvas size, alpha/animation flags, embedded EXIF/XMP/ICC |
| TIFF | full EXIF (IFD0, Exif, GPS, Interop) |
| SVG | width, height, viewBox, version, xmlns |
| MP3 | ID3v2.2/3/4 tags, ID3v1, MPEG version/layer/bitrate/sample rate, duration |
| FLAC | STREAMINFO (sample rate, channels, bit depth, total samples, MD5, block/frame sizes), duration, VORBIS_COMMENT tags (vendor, title, artist, …), picture/padding blocks |
| WAV | PCM format, channels, sample rate, bit depth, duration, LIST INFO tags |
| MP4/MOV/M4A | brands, creation/modification time, duration, track dimensions, audio tags (title, artist, album, track, cover art, …) |
| Opus/Ogg | OpusHead (channels, pre-skip, input rate, gain), Vorbis-style tags, duration, Ogg FLAC detection |
| AVI | frame size, frame rate, frame count, duration, codec fourcc |
| PDF | version, page count, Info dictionary (title, author, …) |

## Usage

```
meta-view [flags] [file]

  -port int        TCP port to listen on (0 = automatic)
  -no-browser      do not open the browser automatically
```

```sh
meta-view                      # open the UI, then drop a file
meta-view photo.jpg            # inspect a file immediately
meta-view -port 8080 a.png b   # fixed port
```

The app serves a local web UI on `127.0.0.1` and opens your browser.
Stop with `Ctrl+C`.

## Build

Only [Go](https://go.dev) is required (no C toolchain, zero dependencies):

```sh
go build -o meta-view .          # build for the current platform
./build.sh                       # (macOS/Linux) all 6 platform binaries -> dist/
.\build.ps1                      # (Windows) all 6 platform binaries -> dist/
```

Cross-compilation works from any OS because the app is pure Go
(`CGO_ENABLED=0`), so one command produces Windows, macOS and Linux binaries.

## Development

```sh
python gen_testfiles.py          # generate sample files for every format
go test ./...                    # extraction smoke tests
go run . -no-browser -port 8765  # dev server without browser launch
```

## Project layout

```
main.go            HTTP server, browser launch, embedded UI
helpers.go         small IO helpers
internal/meta/     pure-Go metadata parsers (no external deps)
  meta.go          format detection + dispatcher
  exif.go          TIFF/EXIF/GPS parser
  jpeg.go  png.go  images.go (GIF/BMP/WebP/SVG)  media.go (MP3/WAV/MP4/AVI/PDF)
web/               frontend (index.html, style.css, app.js)
design/            original design mockups
```
