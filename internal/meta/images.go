package meta

import (
	"encoding/binary"
	"fmt"
	"strings"
)

func le16(p []byte) int { return int(binary.LittleEndian.Uint16(p)) }
func le32(p []byte) int { return int(binary.LittleEndian.Uint32(p)) }

func parseGIF(data []byte, r *Result) {
	if len(data) < 13 {
		return
	}
	w, h := le16(data[6:]), le16(data[8:])
	r.dims(w, h)
	packed := data[10]
	r.add("GIF",
		Property{"version", string(data[3:6])},
		Property{"image width", fmt.Sprint(w)},
		Property{"image height", fmt.Sprint(h)},
		Property{"image size", fmt.Sprintf("%dx%d", w, h)},
		Property{"global color table", boolStr(packed&0x80 != 0)},
		Property{"color table size", fmt.Sprint(1 << (int(packed&7) + 1))},
		Property{"background color index", fmt.Sprint(data[11])},
	)
	if n := strings.Count(string(data), "\x21\xF9"); n > 0 {
		r.add("GIF", Property{"animation frames", fmt.Sprint(n)})
	}
}

func parseBMP(data []byte, r *Result) {
	if len(data) < 26 {
		return
	}
	hdrSize := le32(data[14:])
	var w, h, bits, comp int
	switch hdrSize {
	case 12: // BITMAPCOREHEADER
		if len(data) < 26 {
			return
		}
		w, h = le16(data[18:]), le16(data[20:])
		bits = le16(data[24:])
	default: // BITMAPINFOHEADER and later
		if len(data) < 30 {
			return
		}
		w, h = le32(data[18:]), le32(data[22:])
		bits = le16(data[28:])
		comp = le32(data[30:])
	}
	if h < 0 {
		h = -h
	}
	r.dims(w, h)
	comps := map[int]string{0: "none (RGB)", 1: "RLE 8-bit", 2: "RLE 4-bit", 3: "bit fields", 6: "JPEG", 7: "PNG"}
	name, ok := comps[comp]
	if !ok {
		name = fmt.Sprintf("code %d", comp)
	}
	r.add("BMP",
		Property{"header", fmt.Sprintf("%d bytes", hdrSize)},
		Property{"image width", fmt.Sprint(w)},
		Property{"image height", fmt.Sprint(h)},
		Property{"image size", fmt.Sprintf("%dx%d", w, h)},
		Property{"bits per pixel", fmt.Sprint(bits)},
		Property{"compression", name},
		Property{"pixel data offset", fmt.Sprint(le32(data[10:]))},
	)
}

func parseWebP(data []byte, r *Result) {
	if len(data) < 12 {
		return
	}
	r.add("WebP", Property{"riff size", fmt.Sprint(le32(data[4:]) + 8)})
	for off := 12; off+8 <= len(data); {
		fourcc := string(data[off : off+4])
		size := le32(data[off+4:])
		end := off + 8 + size
		if end > len(data) {
			end = len(data)
		}
		p := data[off+8 : end]
		switch fourcc {
		case "VP8X":
			if len(p) >= 10 {
				w := 1 + (int(p[4]) | int(p[5])<<8 | int(p[6])<<16)
				h := 1 + (int(p[7]) | int(p[8])<<8 | int(p[9])<<16)
				r.dims(w, h)
				flags := p[0]
				r.add("WebP",
					Property{"canvas width", fmt.Sprint(w)},
					Property{"canvas height", fmt.Sprint(h)},
					Property{"canvas size", fmt.Sprintf("%dx%d", w, h)},
					Property{"animation", boolStr(flags&0x02 != 0)},
					Property{"xmp metadata", boolStr(flags&0x04 != 0)},
					Property{"exif metadata", boolStr(flags&0x08 != 0)},
					Property{"icc profile", boolStr(flags&0x20 != 0)},
					Property{"alpha", boolStr(flags&0x10 != 0)},
				)
			}
		case "VP8 ":
			if len(p) >= 10 && p[3] == 0x9D && p[4] == 0x01 && p[5] == 0x2A {
				w := le16(p[6:]) & 0x3FFF
				h := le16(p[8:]) & 0x3FFF
				r.dims(w, h)
				r.add("WebP",
					Property{"codec", "VP8 (lossy)"},
					Property{"frame width", fmt.Sprint(w)},
					Property{"frame height", fmt.Sprint(h)},
					Property{"image size", fmt.Sprintf("%dx%d", w, h)},
				)
			}
		case "VP8L":
			if len(p) >= 5 && p[0] == 0x2F {
				bits := uint32(p[1]) | uint32(p[2])<<8 | uint32(p[3])<<16 | uint32(p[4])<<24
				w := int(bits&0x3FFF) + 1
				h := int((bits>>14)&0x3FFF) + 1
				r.dims(w, h)
				r.add("WebP",
					Property{"codec", "VP8L (lossless)"},
					Property{"image width", fmt.Sprint(w)},
					Property{"image height", fmt.Sprint(h)},
					Property{"image size", fmt.Sprintf("%dx%d", w, h)},
				)
			}
		case "EXIF":
			t := &tiffReader{b: data}
			t.parseTIFF(off+8, r)
		case "XMP":
			addXMP(string(p), r)
		case "ICCP":
			r.add("WebP", Property{"icc profile", fmt.Sprintf("%d bytes", size)})
		case "ANIM":
			if len(p) >= 6 {
				r.add("WebP", Property{"background color", fmt.Sprintf("#%06X", uint32(le32(p[2:]))&0xFFFFFF)})
			}
		}
		off = end
		if size%2 == 1 {
			off++
		}
	}
}

func parseSVG(data []byte, r *Result) {
	s := string(data)
	if len(s) > 4096 {
		s = s[:4096]
	}
	i := strings.Index(s, "<svg")
	if i < 0 {
		return
	}
	end := strings.Index(s[i:], ">")
	if end < 0 {
		return
	}
	tag := s[i : i+end]
	attrs := map[string]string{}
	for _, name := range []string{"width", "height", "viewBox", "version", "xmlns"} {
		for _, q := range []string{`"`, "'"} {
			k := strings.Index(tag, name+"="+q)
			if k < 0 {
				continue
			}
			rest := tag[k+len(name)+2:]
			if j := strings.Index(rest, q); j >= 0 {
				attrs[name] = rest[:j]
				break
			}
		}
	}
	var props []Property
	for _, k := range []string{"width", "height", "viewBox", "version", "xmlns"} {
		if v := attrs[k]; v != "" {
			props = append(props, Property{"svg " + strings.ToLower(k), v})
		}
	}
	if vb := attrs["viewBox"]; vb != "" {
		fields := strings.Fields(vb)
		if len(fields) == 4 {
			props = append(props, Property{"image size", fields[2] + " x " + fields[3]})
		}
	}
	if len(props) > 0 {
		r.add("SVG", props...)
	}
}

func boolStr(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
