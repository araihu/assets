// Package release writes reproducible release archives.
package release

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"slices"
	"strings"
	"time"
)

var epoch = time.Unix(0, 0).UTC()

// Archive writes a deterministic gzip-compressed tar archive.
func Archive(output io.Writer, source fs.FS, paths []string) error {
	return TarGZ(output, source, paths)
}

// TarGZ writes a deterministic gzip-compressed tar archive.
func TarGZ(output io.Writer, source fs.FS, paths []string) error {
	entries, err := load(source, paths)
	if err != nil {
		return err
	}
	gzipWriter, err := gzip.NewWriterLevel(output, gzip.BestCompression)
	if err != nil {
		return fmt.Errorf("release: create gzip: %w", err)
	}
	gzipWriter.ModTime = epoch
	gzipWriter.OS = 255
	tarWriter := tar.NewWriter(gzipWriter)
	for _, entry := range entries {
		// USTAR carries only ModTime. Leave unsupported access/change timestamps
		// at their zero values instead of triggering PAX extensions.
		header := &tar.Header{Name: entry.name, Mode: 0o644, Size: int64(len(entry.data)), ModTime: epoch, Format: tar.FormatUSTAR}
		if err := tarWriter.WriteHeader(header); err != nil {
			return closeTarGZ(tarWriter, gzipWriter, fmt.Errorf("release: write tar header %s: %w", entry.name, err))
		}
		if _, err := tarWriter.Write(entry.data); err != nil {
			return closeTarGZ(tarWriter, gzipWriter, fmt.Errorf("release: write tar data %s: %w", entry.name, err))
		}
	}
	return closeTarGZ(tarWriter, gzipWriter, nil)
}

func closeTarGZ(tarWriter *tar.Writer, gzipWriter *gzip.Writer, prior error) error {
	if err := tarWriter.Close(); prior == nil && err != nil {
		prior = fmt.Errorf("release: close tar: %w", err)
	}
	if err := gzipWriter.Close(); prior == nil && err != nil {
		prior = fmt.Errorf("release: close gzip: %w", err)
	}
	return prior
}

// ZIP writes a deterministic ZIP archive.
func ZIP(output io.Writer, source fs.FS, paths []string) error {
	entries, err := load(source, paths)
	if err != nil {
		return err
	}
	writer := zip.NewWriter(output)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Deflate, Flags: 0}
		header.SetModTime(epoch)
		header.SetMode(0o644)
		file, err := writer.CreateHeader(header)
		if err != nil {
			_ = writer.Close()
			return fmt.Errorf("release: write ZIP header %s: %w", entry.name, err)
		}
		if _, err := file.Write(entry.data); err != nil {
			_ = writer.Close()
			return fmt.Errorf("release: write ZIP data %s: %w", entry.name, err)
		}
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("release: close ZIP: %w", err)
	}
	return nil
}

type entry struct {
	name string
	data []byte
}

func load(source fs.FS, paths []string) ([]entry, error) {
	if source == nil {
		return nil, errors.New("release: source is nil")
	}
	ordered := slices.Clone(paths)
	slices.Sort(ordered)
	entries := make([]entry, 0, len(ordered))
	for index, name := range ordered {
		if !validPath(name) {
			return nil, fmt.Errorf("release: invalid path %q", name)
		}
		if index != 0 && ordered[index-1] == name {
			return nil, fmt.Errorf("release: duplicate path %q", name)
		}
		if linked, err := sourceSymlink(source, name); err != nil {
			return nil, fmt.Errorf("release: inspect %s: %w", name, err)
		} else if linked {
			return nil, fmt.Errorf("release: symbolic link %s", name)
		}
		info, err := fs.Stat(source, name)
		if err != nil {
			return nil, fmt.Errorf("release: stat %s: %w", name, err)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("release: non-regular member %s", name)
		}
		data, err := fs.ReadFile(source, name)
		if err != nil {
			return nil, fmt.Errorf("release: read %s: %w", name, err)
		}
		entries = append(entries, entry{name: name, data: data})
	}
	return entries, nil
}

func validPath(name string) bool {
	return name != "." && fs.ValidPath(name) && !strings.Contains(name, `\`)
}

func sourceSymlink(source fs.FS, name string) (bool, error) {
	parent, base := strings.TrimSuffix(name, "/"), name
	if slash := strings.LastIndex(parent, "/"); slash >= 0 {
		parent, base = parent[:slash], parent[slash+1:]
	} else {
		parent = "."
	}
	entries, err := fs.ReadDir(source, parent)
	if err != nil {
		return false, err
	}
	for _, candidate := range entries {
		if candidate.Name() == base {
			return candidate.Type()&fs.ModeSymlink != 0, nil
		}
	}
	return false, fs.ErrNotExist
}
