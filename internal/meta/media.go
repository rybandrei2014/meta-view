package meta

import (
	"encoding/binary"
	"fmt"
	"strings"
	"time"
)

var id3Frames = map[string]string{
	"TIT2": "title", "TPE1": "artist", "TPE2": "album artist", "TALB": "album",
	"TCON": "genre", "TYER": "year", "TDRC": "recording time", "TRCK": "track",
	"TPOS": "disc", "TPUB": "publisher", "TSST": "set subtitle", "TENC": "encoded by",
	"TLEN": "length (ms)", "TBPM": "bpm", "TKEY": "initial key", "TLAN": "language",
	"COMM": "comment", "USLT": "lyrics", "WXXX": "url",
}

func parseMP3(data []byte, r *Result) {
	pos := 0
	if len(data) >= 10 && string(data[0:3]) == "ID3" {
		ver := data[3]
		size := int(data[6]&0x7F)<<21 | int(data[7]&0x7F)<<14 | int(data[8]&0x7F)<<7 | int(data[9]&0x7F)
		body := data[10:]
		if size < len(body) {
			body = body[:size]
		}
		r.add("ID3v2", Property{"id3 version", fmt.Sprintf("2.%d.%d", ver, data[4])})
		parseID3Frames(body, ver, r)
		pos = 10 + size
	}
	if pos+4 <= len(data) && data[pos] == 0xFF && data[pos+1]&0xE0 == 0xE0 {
		parseMPEGFrame(data[pos:], r)
	}
	if len(data) >= 128 {
		tail := data[len(data)-128:]
		if string(tail[0:3]) == "TAG" {
			r.add("ID3v1",
				Property{"title", trimNUL(string(tail[3:33]))},
				Property{"artist", trimNUL(string(tail[33:63]))},
				Property{"album", trimNUL(string(tail[63:93]))},
				Property{"year", trimNUL(string(tail[93:97]))},
				Property{"comment", trimNUL(string(tail[97:127]))},
			)
		}
	}
}

func parseID3Frames(b []byte, ver byte, r *Result) {
	for len(b) >= 10 {
		id := string(b[0:4])
		if id[0] == 0 {
			break
		}
		var size int
		if ver >= 4 {
			size = int(b[4]&0x7F)<<21 | int(b[5]&0x7F)<<14 | int(b[6]&0x7F)<<7 | int(b[7]&0x7F)
		} else {
			size = int(b[4])<<24 | int(b[5])<<16 | int(b[6])<<8 | int(b[7])
		}
		if size <= 0 || 10+size > len(b) {
			break
		}
		payload := b[10 : 10+size]
		name, ok := id3Frames[id]
		if !ok {
			name = strings.ToLower(id)
		}
		switch {
		case strings.HasPrefix(id, "T"):
			r.add("ID3v2", Property{name, decodeID3Text(payload)})
		case id == "COMM" || id == "USLT":
			if len(payload) > 4 {
				rest := payload[4:] // encoding + 3-byte lang
				if i := indexOfNUL(rest); i >= 0 {
					desc := decodeID3Text(append([]byte{payload[0]}, rest[:i]...))
					text := decodeID3Text(append([]byte{payload[0]}, rest[i+1:]...))
					label := name
					if desc != "" {
						label = name + " (" + desc + ")"
					}
					r.add("ID3v2", Property{label, text})
				}
			}
		case id == "APIC":
			r.add("ID3v2", Property{"embedded image", fmt.Sprintf("%d bytes", size)})
		}
		b = b[10+size:]
	}
}

func decodeID3Text(p []byte) string {
	if len(p) == 0 {
		return ""
	}
	enc := p[0]
	body := p[1:]
	switch enc {
	case 1: // UTF-16 with BOM
		if len(body) >= 2 {
			le := body[0] == 0xFF && body[1] == 0xFE
			body = body[2:]
			var out []uint16
			for i := 0; i+1 < len(body); i += 2 {
				if le {
					out = append(out, binary.LittleEndian.Uint16(body[i:]))
				} else {
					out = append(out, binary.BigEndian.Uint16(body[i:]))
				}
			}
			return strings.TrimRight(string(utf16Decode(out)), "\x00")
		}
	case 2: // UTF-16BE
		var out []uint16
		for i := 0; i+1 < len(body); i += 2 {
			out = append(out, binary.BigEndian.Uint16(body[i:]))
		}
		return strings.TrimRight(string(utf16Decode(out)), "\x00")
	case 3: // UTF-8
		return trimNUL(string(body))
	}
	// 0 = ISO-8859-1
	var sb strings.Builder
	for _, c := range body {
		if c == 0 {
			break
		}
		sb.WriteRune(rune(c))
	}
	return sb.String()
}

func utf16Decode(u []uint16) []rune {
	var out []rune
	for i := 0; i < len(u); i++ {
		c := u[i]
		if c >= 0xD800 && c < 0xDC00 && i+1 < len(u) && u[i+1] >= 0xDC00 && u[i+1] < 0xE000 {
			r := rune(0x10000) + rune(c-0xD800)<<10 + rune(u[i+1]-0xDC00)
			out = append(out, r)
			i++
			continue
		}
		out = append(out, rune(c))
	}
	return out
}

func indexOfNUL(b []byte) int {
	for i, c := range b {
		if c == 0 {
			return i
		}
	}
	return -1
}

var (
	mpegVersions = []string{"MPEG-2.5", "reserved", "MPEG-2", "MPEG-1"}
	mpegLayers   = []string{"reserved", "Layer III", "Layer II", "Layer I"}
	bitrateV1L3  = []int{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 0}
	bitrateV2L3  = []int{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160, 0}
	sampleRates  = []int{11025, 12000, 8000, 0, 0, 0, 0, 0, 22050, 24000, 16000, 0, 44100, 48000, 32000, 0}
	channelModes = []string{"stereo", "joint stereo", "dual channel", "mono"}
)

func parseMPEGFrame(p []byte, r *Result) {
	if len(p) < 4 {
		return
	}
	h := binary.BigEndian.Uint32(p)
	verIdx := int((h >> 19) & 3)
	layerIdx := int((h >> 17) & 3)
	bitrateIdx := int((h >> 12) & 0xF)
	srIdx := int((h >> 10) & 3)
	chIdx := int((h >> 6) & 3)
	if verIdx == 1 || layerIdx == 0 || bitrateIdx == 0 || bitrateIdx == 15 || srIdx == 3 {
		return
	}
	var bitrate int
	if verIdx == 3 { // MPEG-1
		bitrate = bitrateV1L3[bitrateIdx]
	} else {
		bitrate = bitrateV2L3[bitrateIdx]
	}
	sampleRate := sampleRates[verIdx*4+srIdx]
	r.add("MPEG Audio",
		Property{"version", mpegVersions[verIdx]},
		Property{"layer", mpegLayers[layerIdx]},
		Property{"bitrate", fmt.Sprintf("%d kbps", bitrate)},
		Property{"sample rate", fmt.Sprintf("%d Hz", sampleRate)},
		Property{"channel mode", channelModes[chIdx]},
	)
	if bitrate > 0 && r.Size > 0 {
		secs := float64(r.Size*8) / float64(bitrate*1000)
		r.add("MPEG Audio", Property{"approx. duration", fmtDuration(time.Duration(secs * float64(time.Second)))})
	}
}

func fmtDuration(d time.Duration) string {
	if d >= time.Hour {
		return fmt.Sprintf("%d:%02d:%02d", int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60)
	}
	return fmt.Sprintf("%d:%02d", int(d.Minutes()), int(d.Seconds())%60)
}

func parseFLAC(data []byte, r *Result) {
	if len(data) < 4 || string(data[0:4]) != "fLaC" {
		return
	}
	off := 4
	for off+4 <= len(data) {
		last := data[off]&0x80 != 0
		typ := data[off] & 0x7F
		size := int(data[off+1])<<16 | int(data[off+2])<<8 | int(data[off+3])
		off += 4
		if size < 0 || off+size > len(data) {
			break
		}
		p := data[off : off+size]
		switch typ {
		case 0: // STREAMINFO
			if len(p) >= 34 {
				minBlock := int(p[0])<<8 | int(p[1])
				maxBlock := int(p[2])<<8 | int(p[3])
				minFrame := int(p[4])<<16 | int(p[5])<<8 | int(p[6])
				maxFrame := int(p[7])<<16 | int(p[8])<<8 | int(p[9])
				sampleRate := uint32(p[10])<<12 | uint32(p[11])<<4 | uint32(p[12])>>4
				channels := int((p[12]>>1)&7) + 1
				bits := int((p[12]&1)<<4|p[13]>>4) + 1
				total := uint64(p[13]&0x0F)<<32 | uint64(p[14])<<24 |
					uint64(p[15])<<16 | uint64(p[16])<<8 | uint64(p[17])
				r.add("FLAC",
					Property{"codec", "FLAC (Free Lossless Audio Codec)"},
					Property{"block size min", fmt.Sprint(minBlock)},
					Property{"block size max", fmt.Sprint(maxBlock)},
					Property{"frame size min", fmt.Sprint(minFrame)},
					Property{"frame size max", fmt.Sprint(maxFrame)},
					Property{"sample rate", fmt.Sprintf("%d Hz", sampleRate)},
					Property{"channels", fmt.Sprint(channels)},
					Property{"bits per sample", fmt.Sprint(bits)},
					Property{"total samples", fmt.Sprint(total)},
					Property{"md5 signature", fmt.Sprintf("%X", p[18:34])},
				)
				if sampleRate > 0 && total > 0 {
					secs := float64(total) / float64(sampleRate)
					r.add("FLAC", Property{"duration", fmtDuration(time.Duration(secs * float64(time.Second)))})
				}
			}
		case 1:
			if size > 0 {
				r.add("FLAC", Property{"padding", humanSize(int64(size))})
			}
		case 4: // VORBIS_COMMENT
			parseVorbisComments(p, "FLAC Tags", r)
		case 6: // PICTURE
			r.add("FLAC", Property{"embedded picture", fmt.Sprintf("%d bytes", size)})
		}
		off += size
		if last {
			break
		}
	}
}

func parseVorbisComments(p []byte, section string, r *Result) {
	if len(p) < 4 {
		return
	}
	vlen := int(binary.LittleEndian.Uint32(p[0:]))
	if vlen < 0 || 4+vlen > len(p) {
		return
	}
	if vendor := trimNUL(string(p[4 : 4+vlen])); vendor != "" {
		r.add(section, Property{"vendor", vendor})
	}
	rest := p[4+vlen:]
	if len(rest) < 4 {
		return
	}
	n := int(binary.LittleEndian.Uint32(rest[0:]))
	rest = rest[4:]
	for i := 0; i < n && len(rest) >= 4; i++ {
		clen := int(binary.LittleEndian.Uint32(rest[0:]))
		rest = rest[4:]
		if clen < 0 || clen > len(rest) {
			break
		}
		comment := string(rest[:clen])
		rest = rest[clen:]
		if k, v, ok := strings.Cut(comment, "="); ok {
			r.add(section, Property{strings.ToLower(k), v})
		}
	}
}

var mp4TagNames = map[string]string{
	"\xa9nam": "title", "\xa9ART": "artist", "\xa9alb": "album", "\xa9day": "year",
	"\xa9gen": "genre", "\xa9grp": "grouping", "\xa9lyr": "lyrics", "\xa9cmt": "comment",
	"\xa9too": "encoder", "\xa9enc": "encoded by", "\xa9wrt": "composer",
	"aART": "album artist", "trkn": "track", "disk": "disc", "covr": "cover art",
	"desc": "description", "sonm": "sort title", "sart": "sort artist",
	"hdvd": "high definition video", "pgap": "gapless playback",
}

func parseMP4Tags(data []byte, start, end int, r *Result) {
	off := start
	for off+8 <= end {
		size := int(binary.BigEndian.Uint32(data[off:]))
		typ := string(data[off+4 : off+8])
		if size < 8 || off+size > end {
			break
		}
		inner := data[off+8 : off+size]
		name, ok := mp4TagNames[typ]
		if !ok {
			name = trim(typ)
		}
		if len(inner) >= 16 && string(inner[4:8]) == "data" {
			dsize := int(binary.BigEndian.Uint32(inner))
			if dsize >= 16 && dsize <= len(inner) {
				payload := inner[16:dsize]
				var val string
				switch {
				case typ == "trkn" || typ == "disk":
					if len(payload) >= 6 {
						a := int(binary.BigEndian.Uint16(payload[2:]))
						b := int(binary.BigEndian.Uint16(payload[4:]))
						val = fmt.Sprintf("%d of %d", a, b)
					}
				case typ == "covr":
					kind := "image"
					switch {
					case len(payload) > 2 && payload[0] == 0xFF && payload[1] == 0xD8:
						kind = "JPEG"
					case len(payload) > 3 && string(payload[1:4]) == "PNG":
						kind = "PNG"
					}
					val = fmt.Sprintf("%s, %d bytes", kind, len(payload))
				default:
					val = trimNUL(string(payload))
				}
				if val != "" {
					r.add("MP4 Tags", Property{name, val})
				}
			}
		}
		off += size
	}
}

func parseOGG(data []byte, r *Result) {
	var preskip int
	var lastGranule int64 = -1
	serials := map[uint32]bool{}
	off := 0
	for off+27 <= len(data) && string(data[off:off+4]) == "OggS" {
		headerType := data[off+5]
		granule := int64(binary.LittleEndian.Uint64(data[off+6:]))
		serial := binary.LittleEndian.Uint32(data[off+14:])
		nseg := int(data[off+26])
		if off+27+nseg > len(data) {
			break
		}
		seg := data[off+27 : off+27+nseg]
		bodyLen := 0
		for _, s := range seg {
			bodyLen += int(s)
		}
		bodyStart := off + 27 + nseg
		if bodyStart+bodyLen > len(data) {
			break
		}
		body := data[bodyStart : bodyStart+bodyLen]
		serials[serial] = true
		if headerType&0x02 != 0 {
			switch {
			case len(body) >= 19 && string(body[0:8]) == "OpusHead":
				channels := int(body[9])
				preskip = int(le16(body[10:]))
				inputRate := binary.LittleEndian.Uint32(body[12:])
				r.add("Opus",
					Property{"codec", "Opus"},
					Property{"channels", fmt.Sprint(channels)},
					Property{"pre-skip", fmt.Sprintf("%d samples", preskip)},
					Property{"input sample rate", fmt.Sprintf("%d Hz", inputRate)},
					Property{"output gain", fmt.Sprintf("%.2f dB", float64(int16(le16(body[16:])))/256)},
				)
			case len(body) >= 8 && body[0] == 0x7F && string(body[1:5]) == "flac":
				r.add("Ogg", Property{"codec", "FLAC in Ogg"})
			}
		}
		switch {
		case len(body) >= 8 && string(body[0:8]) == "OpusTags":
			parseVorbisComments(body[8:], "Ogg Tags", r)
		case len(body) >= 7 && string(body[1:7]) == "vorbis":
			parseVorbisComments(body[7:], "Ogg Tags", r)
		}
		if granule >= 0 {
			lastGranule = granule
		}
		off = bodyStart + bodyLen
	}
	if len(serials) > 1 {
		r.add("Ogg", Property{"logical streams", fmt.Sprint(len(serials))})
	}
	if lastGranule > 0 && r.Format == "Opus" {
		secs := float64(lastGranule-int64(preskip)) / 48000.0
		if secs > 0 {
			r.add("Opus", Property{"duration", fmtDuration(time.Duration(secs * float64(time.Second)))})
		}
	}
}

func parseWAV(data []byte, r *Result) {
	for off := 12; off+8 <= len(data); {
		id := string(data[off : off+4])
		size := le32(data[off+4:])
		end := off + 8 + size
		if end > len(data) {
			end = len(data)
		}
		p := data[off+8 : end]
		switch id {
		case "fmt ":
			if len(p) >= 16 {
				format := le16(p[0:])
				ch := le16(p[2:])
				rate := le32(p[4:])
				bits := le16(p[14:])
				formats := map[int]string{1: "PCM", 3: "IEEE float", 6: "A-law", 7: "mu-law", 0xFFFE: "extensible"}
				fname, ok := formats[format]
				if !ok {
					fname = fmt.Sprintf("code %d", format)
				}
				r.add("WAV",
					Property{"format", fname},
					Property{"channels", fmt.Sprint(ch)},
					Property{"sample rate", fmt.Sprintf("%d Hz", rate)},
					Property{"bits per sample", fmt.Sprint(bits)},
				)
				if ch > 0 && rate > 0 && bits > 0 {
					if ds := wavDataSize(data); ds > 0 {
						secs := float64(ds) / float64(rate*ch*bits/8)
						r.add("WAV", Property{"duration", fmtDuration(time.Duration(secs * float64(time.Second)))})
					}
				}
			}
		case "LIST":
			if len(p) >= 8 && string(p[4:8]) == "INFO" {
				for q := 8; q+8 <= len(p); {
					fid := string(p[q : q+4])
					fsize := le32(p[q+4:])
					if fsize < 0 || q+8+fsize > len(p) {
						break
					}
					val := trimNUL(string(p[q+8 : q+8+fsize]))
					if val != "" {
						r.add("WAV INFO", Property{strings.ToLower(fid), val})
					}
					q += 8 + fsize
					if fsize%2 == 1 {
						q++
					}
				}
			}
		}
		off = end
		if size%2 == 1 {
			off++
		}
	}
}

func wavDataSize(data []byte) int {
	for off := 12; off+8 <= len(data); {
		id := string(data[off : off+4])
		size := le32(data[off+4:])
		if id == "data" {
			return size
		}
		end := off + 8 + size
		if end > len(data) || size < 0 {
			return 0
		}
		off = end
		if size%2 == 1 {
			off++
		}
	}
	return 0
}

var macEpoch = time.Date(1904, 1, 1, 0, 0, 0, 0, time.UTC)

func parseMP4(data []byte, r *Result) {
	parseMP4Boxes(data, 0, len(data), 0, r)
}

func parseMP4Boxes(data []byte, start, end, depth int, r *Result) {
	if depth > 4 {
		return
	}
	off := start
	for off+8 <= end {
		size := int(binary.BigEndian.Uint32(data[off:]))
		typ := string(data[off+4 : off+8])
		hdr := 8
		if size == 1 {
			if off+16 > end {
				break
			}
			size = int(binary.BigEndian.Uint64(data[off+8:]))
			hdr = 16
		}
		if size < hdr || off+size > end {
			break
		}
		body := data[off+hdr : off+size]
		switch typ {
		case "ftyp":
			if len(body) >= 8 {
				brands := []string{brandName(body[0:4])}
				for q := 8; q+4 <= len(body); q += 4 {
					brands = append(brands, brandName(body[q:q+4]))
				}
				r.add("MP4",
					Property{"major brand", brands[0]},
					Property{"compatible brands", strings.Join(brands[1:], ", ")},
				)
			}
		case "moov", "trak", "mdia", "minf", "stbl", "udta":
			parseMP4Boxes(data, off+hdr, off+size, depth+1, r)
		case "ilst":
			parseMP4Tags(data, off+hdr, off+size, r)
		case "meta":
			if len(body) > 4 {
				parseMP4Boxes(data, off+hdr+4, off+size, depth+1, r)
			}
		case "mvhd":
			parseMvhd(body, r)
		case "tkhd":
			parseTkhd(body, r)
		case "free", "skip":
			r.add("MP4", Property{"free space", humanSize(int64(size))})
		}
		off += size
	}
}

func brandName(b []byte) string {
	s := strings.TrimRight(string(b), "\x00")
	if trim(s) == "" {
		return fmt.Sprintf("%X", b)
	}
	return s
}

func parseMvhd(p []byte, r *Result) {
	if len(p) < 4 {
		return
	}
	var created, modified int64
	var timescale, duration uint64
	if p[0] == 1 {
		if len(p) < 32 {
			return
		}
		created = int64(binary.BigEndian.Uint64(p[4:]))
		modified = int64(binary.BigEndian.Uint64(p[12:]))
		timescale = uint64(binary.BigEndian.Uint32(p[20:]))
		duration = binary.BigEndian.Uint64(p[24:])
	} else {
		if len(p) < 20 {
			return
		}
		created = int64(binary.BigEndian.Uint32(p[4:]))
		modified = int64(binary.BigEndian.Uint32(p[8:]))
		timescale = uint64(binary.BigEndian.Uint32(p[12:]))
		duration = uint64(binary.BigEndian.Uint32(p[16:]))
	}
	if created > 0 {
		r.add("MP4", Property{"creation time", macEpoch.Add(time.Duration(created) * time.Second).UTC().Format("2006-01-02 15:04:05 UTC")})
	}
	if modified > 0 {
		r.add("MP4", Property{"modification time", macEpoch.Add(time.Duration(modified) * time.Second).UTC().Format("2006-01-02 15:04:05 UTC")})
	}
	if timescale > 0 && duration > 0 {
		r.add("MP4", Property{"duration", fmtDuration(time.Duration(float64(duration) / float64(timescale) * float64(time.Second)))})
	}
}

func parseTkhd(p []byte, r *Result) {
	if len(p) < 4 {
		return
	}
	var w, h int
	if p[0] == 1 {
		if len(p) < 96 {
			return
		}
		w = int(binary.BigEndian.Uint32(p[88:]) >> 16)
		h = int(binary.BigEndian.Uint32(p[92:]) >> 16)
	} else {
		if len(p) < 84 {
			return
		}
		w = int(binary.BigEndian.Uint32(p[76:]) >> 16)
		h = int(binary.BigEndian.Uint32(p[80:]) >> 16)
	}
	if w > 0 && h > 0 {
		r.dims(w, h)
		r.add("MP4",
			Property{"track width", fmt.Sprint(w)},
			Property{"track height", fmt.Sprint(h)},
			Property{"image size", fmt.Sprintf("%dx%d", w, h)},
		)
	}
}

func parseAVI(data []byte, r *Result) {
	if len(data) < 12 {
		return
	}
	for off := 12; off+8 <= len(data); {
		id := string(data[off : off+4])
		size := le32(data[off+4:])
		end := off + 8 + size
		if size < 0 || end > len(data) {
			break
		}
		switch id {
		case "avih":
			p := data[off+8 : end]
			if len(p) >= 40 {
				uspf := le32(p[0:])
				frames := le32(p[16:])
				w, h := le32(p[32:]), le32(p[36:])
				r.dims(w, h)
				r.add("AVI",
					Property{"frame width", fmt.Sprint(w)},
					Property{"frame height", fmt.Sprint(h)},
					Property{"image size", fmt.Sprintf("%dx%d", w, h)},
					Property{"total frames", fmt.Sprint(frames)},
				)
				if uspf > 0 && frames > 0 {
					r.add("AVI",
						Property{"frames per second", fmt.Sprintf("%.2f", 1e6/float64(uspf))},
						Property{"duration", fmtDuration(time.Duration(int64(uspf) * int64(frames) * int64(time.Microsecond)))},
					)
				}
			}
		case "strf":
			p := data[off+8 : end]
			if len(p) >= 40 && string(p[0:4]) == "VIDS" {
				r.add("AVI", Property{"codec fourcc", strings.TrimRight(string(p[16:20]), "\x00")})
			}
		}
		off = end
		if size%2 == 1 {
			off++
		}
	}
}

func parsePDF(data []byte, r *Result) {
	head := data
	if len(head) > 1024 {
		head = head[:1024]
	}
	if i := strings.Index(string(head), "%PDF-"); i >= 0 && i+9 <= len(head) {
		r.add("PDF", Property{"pdf version", string(head[i+5 : i+8])})
	}
	s := string(data)
	if n := strings.Count(s, "/Type/Page") + strings.Count(s, "/Type /Page"); n > 0 {
		r.add("PDF", Property{"pages (approx.)", fmt.Sprint(n)})
	}
	for _, key := range []string{"Title", "Author", "Subject", "Keywords", "Creator", "Producer", "CreationDate", "ModDate"} {
		if v := pdfInfoValue(s, key); v != "" {
			r.add("PDF Info", Property{strings.ToLower(key), v})
		}
	}
}

func pdfInfoValue(s, key string) string {
	i := strings.Index(s, "/"+key)
	if i < 0 {
		return ""
	}
	rest := s[i+len(key)+1:]
	if len(rest) < 2 {
		return ""
	}
	if rest[0] == '(' {
		end := strings.IndexByte(rest, ')')
		if end > 0 {
			return rest[1:end]
		}
	}
	if rest[0] == '<' && len(rest) > 1 && rest[1] == '<' {
		if j := strings.Index(rest, "/"); j > 0 {
			return ""
		}
	}
	return ""
}
