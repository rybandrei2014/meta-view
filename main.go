package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"meta-view/internal/meta"
)

//go:embed web
var webFS embed.FS

const maxUpload = 512 << 20

func main() {
	port := flag.Int("port", 0, "TCP port to listen on (0 = automatic)")
	noBrowser := flag.Bool("no-browser", false, "do not open the browser automatically")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: meta-view [flags] [file]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "  Starts a local web UI for inspecting file metadata.")
		fmt.Fprintln(os.Stderr, "  If a file path is given, its metadata is shown immediately.")
		fmt.Fprintln(os.Stderr, "")
		flag.PrintDefaults()
	}
	flag.Parse()

	var prefill *meta.Result
	if flag.NArg() > 0 {
		path := flag.Arg(0)
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		var mod time.Time
		if st, err := os.Stat(path); err == nil {
			mod = st.ModTime()
		}
		prefill = meta.Extract(filepath.Base(path), data, mod)
	}

	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	url := fmt.Sprintf("http://%s/", ln.Addr().String())

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(sub)))
	mux.HandleFunc("/api/extract", handleExtract(prefill))

	fmt.Printf("Meta View running at %s  (Ctrl+C to stop)\n", url)
	if !*noBrowser {
		go func() {
			time.Sleep(300 * time.Millisecond)
			openBrowser(url)
		}()
	}
	if err := http.Serve(ln, mux); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func handleExtract(prefill *meta.Result) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			if prefill == nil {
				http.Error(w, "no preloaded file", http.StatusNotFound)
				return
			}
			writeJSON(w, prefill)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxUpload)
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			http.Error(w, "upload failed or file too large (max 512 MB)", http.StatusBadRequest)
			return
		}
		f, hdr, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "missing file field", http.StatusBadRequest)
			return
		}
		defer f.Close()
		data, err := readAllMax(f, maxUpload)
		if err != nil {
			http.Error(w, "failed to read file", http.StatusBadRequest)
			return
		}
		writeJSON(w, meta.Extract(hdr.Filename, data, time.Time{}))
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := jsonEncoder(w)
	_ = enc.Encode(v)
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
