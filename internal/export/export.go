// Package export copies selected release files into a confined destination.
package export

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"
)

// Copy writes paths from source beneath destination. Existing identical files
// are retained; existing different files are never overwritten.
func Copy(source fs.FS, paths []string, destination *os.Root) error {
	if source == nil {
		return errors.New("export: source is nil")
	}
	if destination == nil {
		return errors.New("export: destination root is nil")
	}
	ordered, err := safePaths(paths)
	if err != nil {
		return err
	}
	for _, name := range ordered {
		if err := copyOne(source, name, destination); err != nil {
			return err
		}
	}
	return nil
}

func safePaths(paths []string) ([]string, error) {
	ordered := slices.Clone(paths)
	slices.Sort(ordered)
	for index, name := range ordered {
		if !validPath(name) {
			return nil, fmt.Errorf("export: invalid path %q", name)
		}
		if index != 0 && ordered[index-1] == name {
			return nil, fmt.Errorf("export: duplicate path %q", name)
		}
	}
	return ordered, nil
}

func validPath(name string) bool {
	return name != "." && fs.ValidPath(name) && !strings.Contains(name, `\`)
}

func copyOne(source fs.FS, name string, destination *os.Root) error {
	if linked, err := sourceSymlink(source, name); err != nil {
		return fmt.Errorf("export: inspect source %s: %w", name, err)
	} else if linked {
		return fmt.Errorf("export: source %s is a symbolic link", name)
	}
	info, err := fs.Stat(source, name)
	if err != nil {
		return fmt.Errorf("export: stat source %s: %w", name, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("export: source %s is not a regular file", name)
	}
	data, err := fs.ReadFile(source, name)
	if err != nil {
		return fmt.Errorf("export: read source %s: %w", name, err)
	}
	if existing, err := destination.Lstat(name); err == nil {
		if !existing.Mode().IsRegular() {
			return fmt.Errorf("export: destination %s is not a regular file", name)
		}
		current, readErr := destination.ReadFile(name)
		if readErr != nil {
			return fmt.Errorf("export: read destination %s: %w", name, readErr)
		}
		if !bytes.Equal(current, data) {
			return fmt.Errorf("export: destination collision at %s", name)
		}
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("export: inspect destination %s: %w", name, err)
	}

	parent := path.Dir(name)
	if parent != "." {
		if err := destination.MkdirAll(parent, 0o755); err != nil {
			return fmt.Errorf("export: create destination directory for %s: %w", name, err)
		}
	}
	temporary := path.Join(parent, ".export-"+randomToken())
	file, err := destination.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("export: create temporary %s: %w", name, err)
	}
	published := false
	defer func() {
		if !published {
			_ = destination.Remove(temporary)
		}
	}()
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return fmt.Errorf("export: write temporary %s: %w", name, err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("export: sync temporary %s: %w", name, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("export: close temporary %s: %w", name, err)
	}
	if err := destination.Rename(temporary, name); err != nil {
		return fmt.Errorf("export: publish %s: %w", name, err)
	}
	published = true
	return nil
}

func sourceSymlink(source fs.FS, name string) (bool, error) {
	parent, base := path.Dir(name), path.Base(name)
	entries, err := fs.ReadDir(source, parent)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if entry.Name() == base {
			return entry.Type()&fs.ModeSymlink != 0, nil
		}
	}
	return false, fs.ErrNotExist
}

func randomToken() string {
	var data [12]byte
	if _, err := rand.Read(data[:]); err != nil {
		panic("export: crypto/rand unavailable: " + err.Error())
	}
	return fmt.Sprintf("%x", data[:])
}
