package main

import (
	"encoding/json"
	"io"
)

func jsonEncoder(w io.Writer) *json.Encoder {
	return json.NewEncoder(w)
}

func readAllMax(r io.Reader, limit int64) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, limit))
}
