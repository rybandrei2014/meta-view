package meta

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
)

type tiffReader struct {
	b  []byte
	le bool
}

func (t *tiffReader) u16(o int) uint16 {
	if o+2 > len(t.b) {
		return 0
	}
	if t.le {
		return binary.LittleEndian.Uint16(t.b[o:])
	}
	return binary.BigEndian.Uint16(t.b[o:])
}

func (t *tiffReader) u32(o int) uint32 {
	if o+4 > len(t.b) {
		return 0
	}
	if t.le {
		return binary.LittleEndian.Uint32(t.b[o:])
	}
	return binary.BigEndian.Uint32(t.b[o:])
}

func (t *tiffReader) u64(o int) uint64 {
	if o+8 > len(t.b) {
		return 0
	}
	if t.le {
		return binary.LittleEndian.Uint64(t.b[o:])
	}
	return binary.BigEndian.Uint64(t.b[o:])
}

type tiffEntry struct {
	tag   uint16
	typ   uint16
	count uint32
	off   int // where the value data lives (inline or at offset)
}

func typeSize(typ uint16) int {
	switch typ {
	case 1, 2, 6, 7: // BYTE, ASCII, SBYTE, UNDEFINED
		return 1
	case 3, 8: // SHORT, SSHORT
		return 2
	case 4, 9, 11: // LONG, SLONG, FLOAT
		return 4
	case 5, 10, 12: // RATIONAL, SRATIONAL, DOUBLE
		return 8
	}
	return 0
}

// parseTIFF parses a TIFF/EXIF block and appends sections to r.
// base is the offset of the TIFF header within t.b.
func (t *tiffReader) parseTIFF(base int, r *Result) {
	if base+8 > len(t.b) {
		return
	}
	h := t.b[base : base+2]
	if h[0] == 'I' && h[1] == 'I' {
		t.le = true
	} else if h[0] == 'M' && h[1] == 'M' {
		t.le = false
	} else {
		return
	}
	if t.u16(base+2) != 42 {
		return
	}
	ifd0Off := base + int(t.u32(base+4))
	ifd0, exifPtr, gpsPtr, interopPtr := t.readIFD(ifd0Off, base)
	t.emit(ifd0, "EXIF", r, exifPtr, gpsPtr)

	if exifPtr > 0 {
		entries, _, _, _ := t.readIFD(base+int(exifPtr), base)
		t.emit(entries, "EXIF Photo", r, 0, 0)
	}
	if gpsPtr > 0 {
		entries, _, _, _ := t.readIFD(base+int(gpsPtr), base)
		t.emitGPS(entries, base, r)
	}
	if interopPtr > 0 {
		entries, _, _, _ := t.readIFD(base+int(interopPtr), base)
		t.emit(entries, "EXIF Interoperability", r, 0, 0)
	}
}

func (t *tiffReader) readIFD(off, base int) (entries []tiffEntry, exifPtr, gpsPtr, interopPtr uint32) {
	if off < 0 || off+2 > len(t.b) {
		return
	}
	n := int(t.u16(off))
	if n > 1000 {
		return
	}
	for i := 0; i < n; i++ {
		e := off + 2 + i*12
		if e+12 > len(t.b) {
			break
		}
		ent := tiffEntry{
			tag:   t.u16(e),
			typ:   t.u16(e + 2),
			count: t.u32(e + 4),
		}
		size := int(ent.count) * typeSize(ent.typ)
		if size <= 4 {
			ent.off = e + 8
		} else {
			ent.off = base + int(t.u32(e+8))
		}
		switch ent.tag {
		case 0x8769:
			if size == 4 {
				exifPtr = t.u32(e + 8)
			}
			continue
		case 0x8825:
			if size == 4 {
				gpsPtr = t.u32(e + 8)
			}
			continue
		case 0xA005:
			if size == 4 {
				interopPtr = t.u32(e + 8)
			}
			continue
		}
		entries = append(entries, ent)
	}
	return
}

func (t *tiffReader) value(e tiffEntry) string {
	if e.off < 0 || e.off > len(t.b) {
		return ""
	}
	switch e.typ {
	case 2: // ASCII
		n := int(e.count)
		if e.off+n > len(t.b) {
			n = len(t.b) - e.off
		}
		return strings.TrimSpace(strings.TrimRight(string(t.b[e.off:e.off+n]), "\x00"))
	case 1, 7: // BYTE / UNDEFINED
		n := int(e.count)
		if e.off+n > len(t.b) {
			n = len(t.b) - e.off
		}
		raw := t.b[e.off : e.off+n]
		printable := true
		for _, c := range raw {
			if c < 0x20 || c > 0x7E {
				printable = false
				break
			}
		}
		if printable && n > 0 {
			return string(raw)
		}
		if n > 48 {
			return fmt.Sprintf("%d bytes (binary)", n)
		}
		var sb strings.Builder
		for _, c := range raw {
			fmt.Fprintf(&sb, "%02X ", c)
		}
		return strings.TrimSpace(sb.String())
	case 3: // SHORT
		return joinNums(e.count, func(i uint32) string {
			return fmt.Sprint(t.u16(e.off + int(i)*2))
		})
	case 4: // LONG
		return joinNums(e.count, func(i uint32) string {
			return fmt.Sprint(t.u32(e.off + int(i)*4))
		})
	case 8: // SSHORT
		return joinNums(e.count, func(i uint32) string {
			return fmt.Sprint(int16(t.u16(e.off + int(i)*2)))
		})
	case 9: // SLONG
		return joinNums(e.count, func(i uint32) string {
			return fmt.Sprint(int32(t.u32(e.off + int(i)*4)))
		})
	case 5: // RATIONAL
		return joinNums(e.count, func(i uint32) string {
			return fmtRatio(t.u32(e.off+int(i)*8), t.u32(e.off+int(i)*8+4))
		})
	case 10: // SRATIONAL
		return joinNums(e.count, func(i uint32) string {
			return fmtRatioS(int32(t.u32(e.off+int(i)*8)), int32(t.u32(e.off+int(i)*8+4)))
		})
	case 11: // FLOAT
		return joinNums(e.count, func(i uint32) string {
			return fmtFloat(float64(math.Float32frombits(t.u32(e.off + int(i)*4))))
		})
	case 12: // DOUBLE
		return joinNums(e.count, func(i uint32) string {
			return fmtFloat(math.Float64frombits(t.u64(e.off + int(i)*8)))
		})
	}
	return ""
}

func joinNums(count uint32, f func(uint32) string) string {
	if count == 0 {
		return ""
	}
	if count > 10 {
		count = 10
	}
	parts := make([]string, 0, count)
	for i := uint32(0); i < count; i++ {
		parts = append(parts, f(i))
	}
	return strings.Join(parts, ", ")
}

func fmtRatio(n, d uint32) string {
	if d == 0 {
		return fmt.Sprint(n)
	}
	return fmtFloat(float64(n) / float64(d))
}

func fmtRatioS(n, d int32) string {
	if d == 0 {
		return fmt.Sprint(n)
	}
	return fmtFloat(float64(n) / float64(d))
}

func fmtFloat(f float64) string {
	s := fmt.Sprintf("%.6g", f)
	return s
}

var exifTagNames = map[uint16]string{
	0x0100: "image width", 0x0101: "image height", 0x0102: "bits per sample",
	0x0103: "compression", 0x0106: "photometric interpretation",
	0x010F: "make", 0x0110: "model", 0x0111: "strip offsets",
	0x0112: "orientation", 0x0115: "samples per pixel", 0x0116: "rows per strip",
	0x0117: "strip byte counts", 0x011A: "x resolution", 0x011B: "y resolution",
	0x011C: "planar configuration", 0x0128: "resolution unit",
	0x012D: "transfer function", 0x0131: "software", 0x0132: "date time",
	0x013B: "artist", 0x013E: "white point", 0x013F: "primary chromaticities",
	0x0142: "transfer range", 0x0143: "image source data", 0x014C: "icc profile descriptor",
	0x014D: "make note", 0x0150: "text", 0x0151: "description",
	0x0152: "copyright", 0x0201: "thumbnail offset", 0x0202: "thumbnail length",
	0x010E: "image description",
	0x829A: "exposure time", 0x829D: "f number", 0x8773: "color space",
	0x8822: "exposure program", 0x8824: "spectral sensitivity", 0x8827: "iso speed ratings",
	0x8830: "sensitivity type", 0x9000: "exif version", 0x9003: "date time original",
	0x9004: "date time digitized", 0x9101: "components configuration",
	0x9102: "compressed bits per pixel", 0x9201: "shutter speed value",
	0x9202: "aperture value", 0x9203: "brightness value", 0x9204: "exposure bias value",
	0x9205: "max aperture value", 0x9206: "subject distance",
	0x9207: "metering mode", 0x9208: "light source", 0x9209: "flash",
	0x920A: "focal length", 0x9214: "subject area", 0x927C: "maker note",
	0x9286: "user comment", 0x9290: "subsec time", 0x9291: "subsec time original",
	0x9292: "subsec time digitized", 0x940B: "flash energy", 0x940C: "spatial light engine",
	0xA000: "flash pix version", 0xA001: "color space", 0xA002: "pixel x dimension",
	0xA003: "pixel y dimension", 0xA004: "related sound file",
	0xA20E: "spatial resolution", 0xA20F: "spatial resolution unit",
	0xA215: "interoperability index", 0xA401: "custom rendered",
	0xA402: "exposure mode", 0xA403: "white balance", 0xA404: "digital zoom ratio",
	0xA405: "focal length in 35 mm film", 0xA406: "scene capture type",
	0xA407: "gain control", 0xA408: "contrast", 0xA409: "saturation",
	0xA40A: "sharpness", 0xA40B: "device settings description",
	0xA40C: "subject distance range", 0xA420: "image unique id",
	0xA430: "camera owner name", 0xA431: "body serial number",
	0xA432: "lens make", 0xA433: "lens model", 0xA434: "lens serial number",
	0xA435: "cfa", 0xA500: "gamma",
}

var gpsTagNames = map[uint16]string{
	0x0000: "gps version id", 0x0001: "latitude ref", 0x0002: "latitude",
	0x0003: "longitude ref", 0x0004: "longitude", 0x0005: "altitude ref",
	0x0006: "altitude", 0x0007: "time stamp", 0x0008: "satellites",
	0x0009: "status", 0x000A: "measure mode", 0x000B: "dop",
	0x000C: "speed ref", 0x000D: "speed", 0x000E: "track ref", 0x000F: "track",
	0x0010: "img direction ref", 0x0011: "img direction", 0x0012: "map datum",
	0x0013: "dest bearing ref", 0x0014: "dest bearing", 0x0015: "dest distance ref",
	0x0016: "dest distance", 0x0017: "processing method", 0x0018: "area information",
	0x0019: "date stamp", 0x001A: "differential",
	0x001B: "h positioning error", 0x001C: "g ps measure mode",
	0x001D: "d positioning error", 0x001E: "v positioning error",
	0x001F: "gps pdop", 0x0020: "gps hdop", 0x0021: "gps vdop",
	0x0022: "gps date stamp", 0x0023: "gps time stamp",
	0x0024: "gps latitude", 0x0025: "gps longitude",
}

var enumNames = map[uint16]map[uint32]string{
	0x0103: {1: "JPEG compression", 6: "JPEG (old-style)", 8: "Deflate"},
	0x0106: {0: "WhiteIsZero", 1: "BlackIsZero", 2: "RGB", 3: "Palette", 4: "Transparency mask", 5: "CMYK", 6: "YCbCr", 8: "CieLab"},
	0x0112: {1: "Normal", 2: "Mirror horizontal", 3: "Rotate 180", 4: "Mirror vertical", 5: "Mirror horizontal and rotate 270 CW", 6: "Rotate 90 CW", 7: "Mirror horizontal and rotate 90 CW", 8: "Rotate 270 CW"},
	0x011C: {1: "Contiguous", 2: "Separate"},
	0x0128: {1: "none", 2: "inches", 3: "cm"},
	0x8822: {0: "Not defined", 1: "Manual", 2: "Normal program", 3: "Aperture priority", 4: "Shutter priority", 6: "Creative program", 7: "Action program", 8: "Portrait mode", 9: "Landscape mode"},
	0x9207: {0: "Undefined", 1: "Average", 2: "CenterWeightedAverage", 3: "Spot", 4: "MultiSpot", 5: "Pattern", 6: "Partial", 255: "Other"},
	0x9208: {1: "Daylight", 2: "Fluorescent", 3: "Tungsten", 4: "Flash", 9: "Fine weather", 10: "Cloudy weather", 11: "Shade", 12: "Daylight fluorescent", 13: "Day white fluorescent", 14: "Cool white fluorescent", 15: "White fluorescent", 17: "Standard light A", 18: "Standard light B", 19: "Standard light C", 20: "D55", 21: "D65", 22: "D75", 23: "ISO studio tungsten", 255: "Other"},
	0x9209: {0: "Flash did not fire", 1: "Flash fired", 5: "Strobe return light not detected", 7: "Strobe return light detected", 9: "Flash did not fire, compulsory", 13: "Flash fired, compulsory", 25: "Flash fired, auto mode", 29: "Flash fired, auto mode, return light detected", 65: "Flash did not fire, open", 73: "Flash fired, open"},
	0xA001: {1: "sRGB", 2: "Adobe RGB", 0xFFFF: "Uncalibrated", 4: "Camera (owner space)"},
	0xA402: {0: "Auto", 1: "Manual", 2: "Aperture priority", 3: "Shutter priority"},
	0xA403: {0: "Auto", 1: "Manual"},
	0xA406: {0: "Standard", 1: "Portrait", 2: "Landscape", 3: "Night"},
}

func (t *tiffReader) emit(entries []tiffEntry, section string, r *Result, exifPtr, gpsPtr uint32) {
	var props []Property
	for _, e := range entries {
		name, ok := exifTagNames[e.tag]
		if !ok {
			name = fmt.Sprintf("tag 0x%04X", e.tag)
		}
		val := t.value(e)
		if m, ok := enumNames[e.tag]; ok && e.typ == 3 && e.count == 1 {
			v := t.u16(e.off)
			if label, ok := m[uint32(v)]; ok {
				val = fmt.Sprintf("%s (%d)", label, v)
			}
		}
		if e.tag == 0x9286 {
			val = cleanUserComment(val)
		}
		if val == "" {
			continue
		}
		props = append(props, Property{name, val})
		if e.count == 1 && (e.typ == 3 || e.typ == 4) {
			n := int(t.u32(e.off))
			if e.typ == 3 {
				n = int(t.u16(e.off))
			}
			switch section {
			case "EXIF":
				switch e.tag {
				case 0x0100:
					r.dims(n, 0)
				case 0x0101:
					r.dims(0, n)
				}
			case "EXIF Photo":
				switch e.tag {
				case 0xA002:
					r.dims(n, 0)
				case 0xA003:
					r.dims(0, n)
				}
			}
		}
	}
	if len(props) > 0 {
		r.add(section, props...)
	}
}

func cleanUserComment(s string) string {
	for _, enc := range []string{"ASCII\x00\x00\x00", "UNICODE\x00", "JIS\x00\x00\x00\x00\x00"} {
		s = strings.TrimPrefix(s, enc)
	}
	return strings.TrimSpace(s)
}

func (t *tiffReader) emitGPS(entries []tiffEntry, base int, r *Result) {
	var props []Property
	var lat, lon []uint32
	var latRef, lonRef, altRef string
	for _, e := range entries {
		name, ok := gpsTagNames[e.tag]
		if !ok {
			name = fmt.Sprintf("tag 0x%04X", e.tag)
		}
		val := t.value(e)
		switch e.tag {
		case 0x0002:
			lat = t.rationals(e)
		case 0x0004:
			lon = t.rationals(e)
		case 0x0001:
			latRef = val
		case 0x0003:
			lonRef = val
		case 0x0005:
			altRef = val
		}
		if val != "" {
			props = append(props, Property{name, val})
		}
	}
	if len(lat) >= 6 && len(lon) >= 6 {
		la := dmsToDeg(lat)
		lo := dmsToDeg(lon)
		if latRef == "S" {
			la = -la
		}
		if lonRef == "W" {
			lo = -lo
		}
		props = append(props, Property{"coordinates", fmt.Sprintf("%.6f, %.6f", la, lo)})
	}
	if altRef == "\x02" || altRef == "2" {
		props = append(props, Property{"altitude", "below sea level"})
	}
	if len(props) > 0 {
		r.add("GPS", props...)
	}
	_ = base
}

func (t *tiffReader) rationals(e tiffEntry) []uint32 {
	if e.typ != 5 || int(e.count)*8 > len(t.b)-e.off {
		return nil
	}
	out := make([]uint32, 0, e.count*2)
	for i := uint32(0); i < e.count; i++ {
		out = append(out, t.u32(e.off+int(i)*8), t.u32(e.off+int(i)*8+4))
	}
	return out
}

func dmsToDeg(v []uint32) float64 {
	if len(v) < 6 {
		return 0
	}
	d := float64(v[0]) / safeD(v[1])
	m := float64(v[2]) / safeD(v[3])
	s := float64(v[4]) / safeD(v[5])
	return d + m/60 + s/3600
}

func safeD(d uint32) float64 {
	if d == 0 {
		return 1
	}
	return float64(d)
}

func parseTIFFFile(data []byte, r *Result) {
	t := &tiffReader{b: data}
	t.parseTIFF(0, r)
}
