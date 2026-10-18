package meta

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
)

var pngColorTypes = map[byte]string{
	0: "Grayscale", 2: "RGB", 3: "Palette", 4: "Grayscale + alpha", 6: "RGBA",
}

func parsePNG(data []byte, r *Result) {
	var idat int64
	for off := 8; off+8 <= len(data); {
		length := int(binary.BigEndian.Uint32(data[off:]))
		typ := string(data[off+4 : off+8])
		if length < 0 || off+8+length > len(data) {
			break
		}
		p := data[off+8 : off+8+length]
		switch typ {
		case "IHDR":
			if len(p) >= 13 {
				w := int(binary.BigEndian.Uint32(p[0:]))
				h := int(binary.BigEndian.Uint32(p[4:]))
				r.dims(w, h)
				interlace := "none"
				if p[12] == 1 {
					interlace = "Adam7"
				}
				r.add("PNG",
					Property{"image width", fmt.Sprint(w)},
					Property{"image height", fmt.Sprint(h)},
					Property{"bit depth", fmt.Sprint(p[8])},
					Property{"color type", fmt.Sprintf("%s (%d)", pngColorTypes[p[9]], p[9])},
					Property{"interlace", interlace},
					Property{"image size", fmt.Sprintf("%dx%d", w, h)},
				)
			}
		case "PLTE":
			r.add("PNG", Property{"palette entries", fmt.Sprint(len(p) / 3)})
		case "IDAT":
			idat += int64(length)
		case "pHYs":
			if len(p) >= 9 {
				x := int(binary.BigEndian.Uint32(p[0:]))
				y := int(binary.BigEndian.Uint32(p[4:]))
				unit := p[8]
				if unit == 1 {
					r.add("PNG",
						Property{"x resolution", fmt.Sprintf("%.0f px/m (%.0f dpi)", float64(x), float64(x)/39.3701)},
						Property{"y resolution", fmt.Sprintf("%.0f px/m (%.0f dpi)", float64(y), float64(y)/39.3701)},
						Property{"resolution unit", "meter"},
					)
				}
			}
		case "gAMA":
			if len(p) >= 4 {
				r.add("PNG", Property{"gamma", fmt.Sprintf("%.5f", float64(binary.BigEndian.Uint32(p))/100000)})
			}
		case "sRGB":
			if len(p) >= 1 {
				names := map[byte]string{0: "Perceptual", 1: "Relative colorimetric", 2: "Saturation", 3: "Absolute colorimetric"}
				r.add("PNG", Property{"srgb rendering intent", names[p[0]]})
			}
		case "bKGD":
			r.add("PNG", Property{"background", fmt.Sprintf("%d bytes", len(p))})
		case "tEXt":
			if i := bytes.IndexByte(p, 0); i >= 0 {
				r.add("PNG Text", Property{string(p[:i]), string(p[i+1:])})
			}
		case "zTXt":
			if i := bytes.IndexByte(p, 0); i >= 0 && i+2 <= len(p) {
				key := string(p[:i])
				val := inflateAll(p[i+2:])
				r.add("PNG Text", Property{key, val})
			}
		case "iTXt":
			if i := bytes.IndexByte(p, 0); i >= 0 && i+3 <= len(p) {
				key := string(p[:i])
				rest := p[i+1:]
				compressed := rest[0] == 1
				rest = rest[1:] // compression flag
				rest = rest[1:] // compression method
				j := bytes.IndexByte(rest, 0)
				if j >= 0 {
					rest = rest[j+1:] // language tag
					j = bytes.IndexByte(rest, 0)
					if j >= 0 {
						rest = rest[j+1:] // translated keyword
						if compressed {
							rest = []byte(inflateAll(rest))
						}
						r.add("PNG Text", Property{key, string(rest)})
					}
				}
			}
		case "tIME":
			if len(p) >= 7 {
				r.add("PNG", Property{"last modification time", fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d",
					binary.BigEndian.Uint16(p[0:]), p[2], p[3], p[4], p[5], p[6])})
			}
		case "iCCP":
			if i := bytes.IndexByte(p, 0); i >= 0 {
				r.add("PNG", Property{"icc profile", string(p[:i])})
			}
		case "cHRM":
			if len(p) >= 32 {
				r.add("PNG", Property{"chromaticities", fmt.Sprintf("white (%.4f, %.4f)",
					float64(binary.BigEndian.Uint32(p[0:]))/100000, float64(binary.BigEndian.Uint32(p[4:]))/100000)})
			}
		}
		off += 12 + length
		if typ == "IEND" {
			break
		}
	}
	if idat > 0 {
		r.add("PNG", Property{"compressed pixel data", humanSize(idat)})
	}
}

func inflateAll(p []byte) string {
	rd := flate.NewReader(bytes.NewReader(p))
	defer rd.Close()
	out, err := io.ReadAll(io.LimitReader(rd, 1<<20))
	if err != nil && len(out) == 0 {
		return ""
	}
	return strings.ToValidUTF8(string(out), "\uFFFD")
}
