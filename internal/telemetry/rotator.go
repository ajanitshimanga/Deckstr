package telemetry

import (
	"fmt"
	"os"
	"path/filepath"
)

// rotatingWriter writes to <dir>/<base>, rotating to <base>.1, <base>.2, ...
// once the active file exceeds maxSize. backups is the number of historical
// files to keep; the oldest is dropped on rotation. Not concurrency-safe on
// its own — Logger serializes access under its mutex.
type rotatingWriter struct {
	dir     string
	base    string
	maxSize int64
	backups int

	f    *os.File
	size int64
}

func newRotatingWriter(dir, base string, maxSize int64, backups int) (*rotatingWriter, error) {
	w := &rotatingWriter{dir: dir, base: base, maxSize: maxSize, backups: backups}
	if err := w.openCurrent(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *rotatingWriter) openCurrent() error {
	path := filepath.Join(w.dir, w.base)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("telemetry: open %s: %w", path, err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return fmt.Errorf("telemetry: stat %s: %w", path, err)
	}
	w.f = f
	w.size = info.Size()
	return nil
}

func (w *rotatingWriter) Write(p []byte) (int, error) {
	if w.size+int64(len(p)) > w.maxSize && w.size > 0 {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}
	n, err := w.f.Write(p)
	w.size += int64(n)
	return n, err
}

// rotate closes the current file, shifts the backup chain down by one
// (dropping the oldest), and opens a fresh active file.
func (w *rotatingWriter) rotate() error {
	if err := w.f.Close(); err != nil {
		return fmt.Errorf("telemetry: close before rotate: %w", err)
	}

	// Drop the oldest backup if we're at capacity.
	oldest := w.backupPath(w.backups)
	if _, err := os.Stat(oldest); err == nil {
		if err := os.Remove(oldest); err != nil {
			return fmt.Errorf("telemetry: drop oldest backup: %w", err)
		}
	}

	// Shift .N-1 → .N, .N-2 → .N-1, ..., .1 → .2
	for i := w.backups - 1; i >= 1; i-- {
		src := w.backupPath(i)
		dst := w.backupPath(i + 1)
		if _, err := os.Stat(src); err == nil {
			if err := os.Rename(src, dst); err != nil {
				return fmt.Errorf("telemetry: shift %s: %w", src, err)
			}
		}
	}

	// Current → .1
	if err := os.Rename(w.activePath(), w.backupPath(1)); err != nil {
		return fmt.Errorf("telemetry: rotate active: %w", err)
	}

	return w.openCurrent()
}

func (w *rotatingWriter) activePath() string {
	return filepath.Join(w.dir, w.base)
}

func (w *rotatingWriter) backupPath(n int) string {
	return filepath.Join(w.dir, fmt.Sprintf("%s.%d", w.base, n))
}

func (w *rotatingWriter) Sync() error {
	if w.f == nil {
		return nil
	}
	return w.f.Sync()
}

func (w *rotatingWriter) Close() error {
	if w.f == nil {
		return nil
	}
	return w.f.Close()
}
