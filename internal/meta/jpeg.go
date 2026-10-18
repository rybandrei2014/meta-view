package meta

import (
	"bytes"
	"fmt"
	"strings"
)

var sofNames = map[byte]string{
	0xC0: "Baseline DCT, Huffman coding",
	0xC1: "Extended sequential DCT, Huffman coding",
	0xC2: "Progressive DCT, Huffman coding",
	0xC3: "Lossless sequential DCT, Huffman coding",
	0xC5: "Differential sequential DCT, Huffman coding",
	0xC6: "Differential progressive DCT, Huffman coding",
	0xC7: "Differential lossless DCT, Huffman coding",
	0xC9: "Extended sequential DCT, arithmetic coding",
	0xCA: "Progressive DCT, arithmetic coding",
	0xCB: "Lossless sequential DCT, arithmetic coding",
	0xCD: "Differential sequential DCT, arithmetic coding",
	0xCE: "Differential progressive DCT, arithmetic coding",
	0xCF: "Differential lossless DCT, arithmetic coding",
}

func parseJPEG(data []byte, r *Result) {
	r.add("JPEG", Property{"marker", "SOI (start of image)"})
	i := 2
	for i+4 <= len(data) {
		if data[i] != 0xFF {
			i++
			continue
		}
		m := data[i+1]
		if m == 0x01 || m == 0xD8 || (m >= 0xD0 && m <= 0xD9) {
			i += 2
			if m == 0xD9 {
				break
			}
			continue
		}
		segLen := int(data[i+2])<<8 | int(data[i+3])
		if segLen < 2 || i+2+segLen > len(data) {
			break
		}
		payload := data[i+4 : i+2+segLen]
		switch {
		case m == 0xE0 && bytes.HasPrefix(payload, []byte("JFIF")):
			parseJFIF(payload, r)
		case m == 0xE1 && bytes.HasPrefix(payload, []byte("Exif")):
			t := &tiffReader{b: data}
			t.parseTIFF(i+4+6, r)
		case m == 0xE1 && bytes.HasPrefix(payload, []byte("http://ns.adobe.com/xap/1.0/")):
			addXMP(string(payload[29:]), r)
		case m == 0xFE:
			r.add("JPEG", Property{"comment", trimNUL(string(payload))})
		case m >= 0xC0 && m <= 0xCF && m != 0xC4 && m != 0xC8 && m != 0xCC:
			if len(payload) >= 6 {
				bits := payload[0]
				h := int(payload[1])<<8 | int(payload[2])
				w := int(payload[3])<<8 | int(payload[4])
				ncomp := int(payload[5])
				r.dims(w, h)
				r.add("JPEG",
					Property{"encoding process", sofNames[m]},
					Property{"image width", fmt.Sprint(w)},
					Property{"image height", fmt.Sprint(h)},
					Property{"bits per sample", fmt.Sprint(bits)},
					Property{"color components", fmt.Sprint(ncomp)},
					Property{"image size", fmt.Sprintf("%dx%d", w, h)},
				)
			}
		}
		i += 2 + segLen
	}
}

func parseJFIF(p []byte, r *Result) {
	if len(p) < 14 {
		return
	}
	units := map[byte]string{0: "none", 1: "inches", 2: "cm"}
	x := int(p[8])<<8 | int(p[9])
	y := int(p[10])<<8 | int(p[11])
	r.add("JPEG",
		Property{"jfif version", fmt.Sprintf("%d.%02d", p[5], p[6])},
		Property{"resolution unit", units[p[7]]},
		Property{"x resolution", fmt.Sprint(x)},
		Property{"y resolution", fmt.Sprint(y)},
	)
	if thumb := int(p[8]) * int(p[9]); thumb > 0 {
		r.add("JPEG", Property{"embedded thumbnail", fmt.Sprintf("%dx%d pixels", p[8], p[9])})
	}
}

func addXMP(xmp string, r *Result) {
	xmp = trimNUL(xmp)
	if xmp == "" {
		return
	}
	for _, tag := range []struct{ xml, label string }{
		{"dc:title", "xmp title"},
		{"dc:creator", "xmp creator"},
		{"dc:description", "xmp description"},
		{"dc:rights", "xmp rights"},
		{"xmp:CreatorTool", "xmp creator tool"},
		{"photoshop:Authors", "xmp authors"},
	} {
		if v := xmlValue(xmp, tag.xml); v != "" {
			r.add("XMP", Property{tag.label, v})
		}
	}
	if len(r.Sections) == 0 || r.Sections[len(r.Sections)-1].Name != "XMP" {
		r.add("XMP", Property{"xmp", "present"})
	}
}

func xmlValue(s, tag string) string {
	for {
		k := strings.Index(s, "<"+tag)
		if k < 0 {
			return ""
		}
		s = s[k+len(tag)+1:]
		if i := strings.Index(s, ">"); i >= 0 {
			s = s[i+1:]
		}
		end := strings.Index(s, "</"+tag)
		if end < 0 {
			return ""
		}
		if v := trimNUL(s[:end]); v != "" {
			return v
		}
	}
}

func trimNUL(s string) string {
	return trim(strings.TrimRight(s, "\x00"))
}

func trim(s string) string {
	for len(s) > 0 && (s[0] <= ' ') {
		s = s[1:]
	}
	for len(s) > 0 && s[len(s)-1] <= ' ' {
		s = s[:len(s)-1]
	}
	return s
}
