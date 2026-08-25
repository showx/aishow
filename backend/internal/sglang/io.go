package sglang

import (
	"io"
	"os"
	"path/filepath"
)

func writeFile(dest string, r io.Reader) (int64, error) {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return 0, err
	}
	f, err := os.Create(dest)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return io.Copy(f, r)
}
