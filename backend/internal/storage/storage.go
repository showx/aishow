package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"aishow/internal/config"
)

type Store struct {
	root string
}

func New(cfg config.Config) (*Store, error) {
	s := &Store{root: cfg.DataDir}
	for _, dir := range []string{
		s.UploadsDir(),
		s.OutputsDir(),
		s.MediaDir(),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *Store) UploadsDir() string { return filepath.Join(s.root, "uploads") }
func (s *Store) OutputsDir() string { return filepath.Join(s.root, "outputs") }
func (s *Store) MediaDir() string   { return filepath.Join(s.root, "media") }

func (s *Store) UploadPath(id, filename string) string {
	ext := filepath.Ext(filename)
	return filepath.Join(s.UploadsDir(), id+strings.ToLower(ext))
}

func (s *Store) OutputPath(jobID string) string {
	return filepath.Join(s.OutputsDir(), jobID+".mp4")
}

func (s *Store) ImageOutputPath(jobID string) string {
	return filepath.Join(s.OutputsDir(), jobID+".png")
}

func (s *Store) RemoveOutputs(jobID string) {
	_ = os.Remove(s.OutputPath(jobID))
	_ = os.Remove(s.ImageOutputPath(jobID))
}

func (s *Store) JobMediaDir(jobID string) string {
	return filepath.Join(s.MediaDir(), jobID)
}

func (s *Store) SaveUpload(id, filename string, r io.Reader) (string, int64, error) {
	path := s.UploadPath(id, filename)
	f, err := os.Create(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	n, err := io.Copy(f, r)
	if err != nil {
		return "", 0, err
	}
	return path, n, nil
}

func (s *Store) Copy(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func (s *Store) ToFileURI(absPath, prefix string) string {
	base := filepath.Base(absPath)
	job := filepath.Base(filepath.Dir(absPath))
	return strings.TrimRight(prefix, "/") + "/" + job + "/" + base
}

func (s *Store) ToHTTPURI(publicBase, jobID, filename string) string {
	return fmt.Sprintf("%s/api/v1/media/%s/%s", strings.TrimRight(publicBase, "/"), jobID, filename)
}
