// Package release writes reproducible release archives.
package release

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"
)

var (
	epoch      = time.Unix(0, 0).UTC()
	releaseTag = regexp.MustCompile(`^v(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)(?:-(?:(?:0|[1-9][0-9]*|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*)(?:\.(?:0|[1-9][0-9]*|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*))*))?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)
)

// Archive writes a deterministic gzip-compressed tar archive. source must be
// immutable or snapshotted: generic mutable fs.FS values cannot close symlink
// TOCTOU races. ArchiveRoot provides rooted live-filesystem confinement.
func Archive(output io.Writer, source fs.FS, paths []string) error {
	return TarGZ(output, source, paths)
}

// ArchiveRoot writes an archive from a live, rooted source. os.Root confines
// symlink resolution beneath source during concurrent filesystem changes.
func ArchiveRoot(output io.Writer, source *os.Root, paths []string) error {
	if source == nil {
		return errors.New("release: source root is nil")
	}
	return Archive(output, source.FS(), paths)
}

// TarGZ writes a deterministic gzip-compressed tar archive. source has the
// same immutable-or-snapshotted contract as Archive.
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

// ZIP writes a deterministic ZIP archive. source must be immutable or
// snapshotted; use ZIPRoot for a live disk-backed source.
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

// ZIPRoot writes a ZIP archive from a live, rooted source.
func ZIPRoot(output io.Writer, source *os.Root, paths []string) error {
	if source == nil {
		return errors.New("release: source root is nil")
	}
	return ZIP(output, source.FS(), paths)
}

// PublicBundle copies one immutable release beneath releases/<releaseID>.
// Existing equal bytes are retained; differing bytes are a collision.
func PublicBundle(ctx context.Context, destination *os.Root, releaseID string, source fs.FS, paths []string) error {
	if ctx == nil {
		return errors.New("release: context is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if destination == nil {
		return errors.New("release: destination root is nil")
	}
	if !validReleaseID(releaseID) {
		return fmt.Errorf("release: invalid release ID %q", releaseID)
	}
	entries, err := load(source, paths)
	if err != nil {
		return err
	}
	base := "releases/" + releaseID
	if err := ensureDirectory(destination, base); err != nil {
		return fmt.Errorf("release: create public bundle root: %w", err)
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		target := base + "/" + entry.name
		if err := ensureDirectory(destination, strings.TrimSuffix(target, "/"+pathBase(entry.name))); err != nil {
			return fmt.Errorf("release: create directory for %s: %w", entry.name, err)
		}
		info, err := destination.Lstat(target)
		if errors.Is(err, fs.ErrNotExist) {
			if err := destination.WriteFile(target, entry.data, 0o644); err != nil {
				return fmt.Errorf("release: write public member %s: %w", entry.name, err)
			}
			continue
		}
		if err != nil {
			return fmt.Errorf("release: inspect public member %s: %w", entry.name, err)
		}
		if info.Mode()&fs.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("release: non-regular collision %s", entry.name)
		}
		existing, err := destination.ReadFile(target)
		if err != nil {
			return fmt.Errorf("release: read public member %s: %w", entry.name, err)
		}
		if !slices.Equal(existing, entry.data) {
			return fmt.Errorf("release: collision at %s", target)
		}
	}
	return nil
}

func ensureDirectory(root *os.Root, name string) error {
	if name == "." || name == "" {
		return nil
	}
	current := ""
	for _, component := range strings.Split(name, "/") {
		if current == "" {
			current = component
		} else {
			current += "/" + component
		}
		info, err := root.Lstat(current)
		if errors.Is(err, fs.ErrNotExist) {
			if err := root.Mkdir(current, 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
				return err
			}
			info, err = root.Lstat(current)
		}
		if err != nil {
			return err
		}
		if info.Mode()&fs.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("non-directory path %s", current)
		}
	}
	return nil
}

func pathBase(name string) string {
	parts := strings.Split(name, "/")
	return parts[len(parts)-1]
}

func validReleaseID(releaseID string) bool {
	return releaseTag.MatchString(releaseID)
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
	return name != "." && fs.ValidPath(name) && !strings.Contains(name, `\`) && !strings.Contains(strings.Split(name, "/")[0], ":")
}

func sourceSymlink(source fs.FS, name string) (bool, error) {
	parent := "."
	for _, component := range strings.Split(name, "/") {
		entries, err := fs.ReadDir(source, parent)
		if err != nil {
			return false, err
		}
		found := false
		for _, candidate := range entries {
			if candidate.Name() == component {
				if candidate.Type()&fs.ModeSymlink != 0 {
					return true, nil
				}
				found = true
				break
			}
		}
		if !found {
			return false, fs.ErrNotExist
		}
		if parent == "." {
			parent = component
		} else {
			parent += "/" + component
		}
	}
	return false, nil
}
