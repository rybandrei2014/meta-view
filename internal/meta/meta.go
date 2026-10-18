package meta

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

type Property struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Section struct {
	Name  string     `json:"name"`
	Props []Property `json:"props"`
}

type Result struct {
	FileName  string    `json:"fileName"`
	Ext       string    `json:"ext"`
	Mime      string    `json:"mime"`
	Size      int64     `json:"size"`
	SizeHuman string    `json:"sizeHuman"`
	Modified  string    `json:"modified"`
	Format    string    `json:"format"`
	Width     int       `json:"width,omitempty"`
	Height    int       `json:"height,omitempty"`
	Sections  []Section `json:"sections"`
	Count     int       `json:"count"`
}

func (r *Result) add(section string, props ...Property) {
	for i := range r.Sections {
		if r.Sections[i].Name == section {
			r.Sections[i].Props = append(r.Sections[i].Props, props...)
			return
		}
	}
	r.Sections = append(r.Sections, Section{Name: section, Props: props})
}

func (r *Result) dims(w, h int) {
	if r.Width == 0 && w > 0 {
		r.Width = w
	}
	if r.Height == 0 && h > 0 {
		r.Height = h
	}
}

// Extract inspects data (the full file content) and returns all metadata found.
func Extract(name string, data []byte, mod time.Time) *Result {
	r := &Result{FileName: name, Size: int64(len(data)), SizeHuman: humanSize(int64(len(data)))}
	if !mod.IsZero() {
		r.Modified = mod.Format("2006-01-02 15:04:05")
	}
	r.Ext = strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")

	format, mime := detect(data, r.Ext)
	r.Format = format
	r.Mime = mime

	fileProps := []Property{
		{"file name", name},
		{"file size", r.SizeHuman},
		{"file type", format},
		{"mime type", mime},
	}
	if r.Ext != "" {
		fileProps = append(fileProps, Property{"file extension", r.Ext})
	}
	if r.Modified != "" {
		fileProps = append(fileProps, Property{"last modified", r.Modified})
	}
	r.add("File Properties", fileProps...)

	switch format {
	case "JPEG":
		parseJPEG(data, r)
	case "PNG":
		parsePNG(data, r)
	case "GIF":
		parseGIF(data, r)
	case "BMP":
		parseBMP(data, r)
	case "WebP":
		parseWebP(data, r)
	case "TIFF":
		parseTIFFFile(data, r)
	case "SVG":
		parseSVG(data, r)
	case "MP3":
		parseMP3(data, r)
	case "FLAC":
		parseFLAC(data, r)
	case "WAV":
		parseWAV(data, r)
	case "MP4", "QuickTime", "M4A":
		parseMP4(data, r)
	case "AVI":
		parseAVI(data, r)
	case "PDF":
		parsePDF(data, r)
	}

	r.add("Raw Data", Property{"first 256 bytes", hexDump(data, 256)})

	for _, s := range r.Sections {
		r.Count += len(s.Props)
	}
	return r
}

func detect(data []byte, ext string) (string, string) {
	has := func(sig ...byte) bool {
		if len(data) < len(sig) {
			return false
		}
		for i := range sig {
			if data[i] != sig[i] {
				return false
			}
		}
		return true
	}
	switch {
	case has(0xFF, 0xD8, 0xFF):
		return "JPEG", "image/jpeg"
	case has(0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A):
		return "PNG", "image/png"
	case has('G', 'I', 'F', '8'):
		return "GIF", "image/gif"
	case has('B', 'M'):
		return "BMP", "image/bmp"
	case has('f', 'L', 'a', 'C'):
		return "FLAC", "audio/flac"
	case has('I', 'D', '3'):
		return "MP3", "audio/mpeg"
	case len(data) > 1 && data[0] == 0xFF && data[1]&0xE0 == 0xE0 && data[1]&0x18 != 0:
		return "MP3", "audio/mpeg"
	case has('R', 'I', 'F', 'F') && len(data) > 11 && string(data[8:12]) == "WEBP":
		return "WebP", "image/webp"
	case has('R', 'I', 'F', 'F') && len(data) > 11 && string(data[8:12]) == "WAVE":
		return "WAV", "audio/wav"
	case has('R', 'I', 'F', 'F') && len(data) > 11 && string(data[8:12]) == "AVI ":
		return "AVI", "video/x-msvideo"
	case len(data) > 11 && string(data[4:8]) == "ftyp":
		switch {
		case strings.Contains(ext, "m4a") || strings.Contains(ext, "m4b"):
			return "M4A", "audio/mp4"
		case strings.Contains(ext, "mov"):
			return "QuickTime", "video/quicktime"
		default:
			return "MP4", "video/mp4"
		}
	case has('I', 'I', '*', 0x00), has('M', 'M', 0x00, '*'):
		return "TIFF", "image/tiff"
	case has('%', 'P', 'D', 'F', '-'):
		return "PDF", "application/pdf"
	}
	head := data
	if len(head) > 512 {
		head = head[:512]
	}
	s := strings.ToLower(strings.TrimSpace(string(head)))
	if strings.HasPrefix(s, "<svg") || (strings.HasPrefix(s, "<?xml") && strings.Contains(s, "<svg")) {
		return "SVG", "image/svg+xml"
	}
	switch ext {
	case "jpg", "jpeg":
		return "JPEG", "image/jpeg"
	case "png":
		return "PNG", "image/png"
	case "gif":
		return "GIF", "image/gif"
	case "webp":
		return "WebP", "image/webp"
	case "bmp":
		return "BMP", "image/bmp"
	case "tif", "tiff":
		return "TIFF", "image/tiff"
	case "svg":
		return "SVG", "image/svg+xml"
	case "mp3":
		return "MP3", "audio/mpeg"
	case "flac":
		return "FLAC", "audio/flac"
	case "wav":
		return "WAV", "audio/wav"
	case "mp4", "m4v":
		return "MP4", "video/mp4"
	case "mov":
		return "QuickTime", "video/quicktime"
	case "m4a":
		return "M4A", "audio/mp4"
	case "avi":
		return "AVI", "video/x-msvideo"
	case "pdf":
		return "PDF", "application/pdf"
	}
	return "Unknown", "application/octet-stream"
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %s", float64(n)/float64(div), []string{"kB", "MB", "GB", "TB"}[exp])
}

func hexDump(data []byte, n int) string {
	if len(data) > n {
		data = data[:n]
	}
	var b strings.Builder
	for off := 0; off < len(data); off += 16 {
		end := off + 16
		if end > len(data) {
			end = len(data)
		}
		row := data[off:end]
		fmt.Fprintf(&b, "%08X  ", off)
		for i := 0; i < 16; i++ {
			if i < len(row) {
				fmt.Fprintf(&b, "%02X ", row[i])
			} else {
				b.WriteString("   ")
			}
		}
		b.WriteString(" |")
		for _, c := range row {
			if c >= 0x20 && c < 0x7F {
				b.WriteByte(c)
			} else {
				b.WriteByte('.')
			}
		}
		b.WriteString("|")
		if off+16 < len(data) {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func jsonSafe(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}
