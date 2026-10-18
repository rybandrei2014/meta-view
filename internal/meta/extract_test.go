package meta

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestExtractTestfiles(t *testing.T) {
	files, err := os.ReadDir("../../testfiles")
	if err != nil {
		t.Skip("no testfiles dir")
	}
	for _, f := range files {
		data, err := os.ReadFile("../../testfiles/" + f.Name())
		if err != nil {
			t.Fatal(err)
		}
		r := Extract(f.Name(), data, time.Time{})
		if r.Format == "Unknown" {
			t.Errorf("%s: format unknown", f.Name())
		}
		if r.Count < 5 {
			t.Errorf("%s: too few properties (%d)", f.Name(), r.Count)
		}
		for _, s := range r.Sections {
			if s.Name == "Raw Data" {
				continue
			}
			for _, p := range s.Props {
				if strings.ContainsRune(p.Value, 0) {
					t.Errorf("%s: %s/%s contains NUL: %q", f.Name(), s.Name, p.Key, p.Value)
				}
			}
		}
	}
}
