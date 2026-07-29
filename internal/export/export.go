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

// publishHook is a test-only fault/race injection seam. Production leaves it nil.
var publishHook func(string, *os.Root) error

type candidate struct {
	name       string
	data       []byte
	temporary  string
	created    bool
	targetInfo fs.FileInfo
}

// Copy writes paths from source beneath destination as one operation. Existing
// identical files are retained; different files are never overwritten.
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
	candidates, err := preflight(source, ordered, destination)
	if err != nil {
		return err
	}
	if err := stage(candidates, destination); err != nil {
		cleanupTemporary(candidates, destination)
		return err
	}
	defer cleanupTemporary(candidates, destination)
	if err := publish(candidates, destination); err != nil {
		return errors.Join(err, rollback(candidates, destination))
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
	return name != "." && fs.ValidPath(name) && !strings.Contains(name, `\`) && !strings.Contains(strings.Split(name, "/")[0], ":")
}

func preflight(source fs.FS, paths []string, destination *os.Root) ([]candidate, error) {
	candidates := make([]candidate, 0, len(paths))
	for _, name := range paths {
		if linked, err := sourceSymlink(source, name); err != nil {
			return nil, fmt.Errorf("export: inspect source %s: %w", name, err)
		} else if linked {
			return nil, fmt.Errorf("export: source %s has a symbolic-link component", name)
		}
		info, err := fs.Stat(source, name)
		if err != nil {
			return nil, fmt.Errorf("export: stat source %s: %w", name, err)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("export: source %s is not a regular file", name)
		}
		data, err := fs.ReadFile(source, name)
		if err != nil {
			return nil, fmt.Errorf("export: read source %s: %w", name, err)
		}
		if existing, err := destination.Lstat(name); err == nil {
			if !existing.Mode().IsRegular() {
				return nil, fmt.Errorf("export: destination %s is not a regular file", name)
			}
			current, readErr := destination.ReadFile(name)
			if readErr != nil {
				return nil, fmt.Errorf("export: read destination %s: %w", name, readErr)
			}
			if !bytes.Equal(current, data) {
				return nil, fmt.Errorf("export: destination collision at %s", name)
			}
			continue
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("export: inspect destination %s: %w", name, err)
		}
		candidates = append(candidates, candidate{name: name, data: data})
	}
	return candidates, nil
}

func stage(candidates []candidate, destination *os.Root) (err error) {
	for index := range candidates {
		parent := path.Dir(candidates[index].name)
		if parent != "." {
			if err := destination.MkdirAll(parent, 0o755); err != nil {
				return fmt.Errorf("export: create destination directory for %s: %w", candidates[index].name, err)
			}
		}
		temporary := path.Join(parent, ".export-"+randomToken())
		file, err := destination.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			return fmt.Errorf("export: create temporary %s: %w", candidates[index].name, err)
		}
		if _, err := file.Write(candidates[index].data); err != nil {
			_ = file.Close()
			return fmt.Errorf("export: write temporary %s: %w", candidates[index].name, err)
		}
		if err := file.Sync(); err != nil {
			_ = file.Close()
			return fmt.Errorf("export: sync temporary %s: %w", candidates[index].name, err)
		}
		if err := file.Close(); err != nil {
			return fmt.Errorf("export: close temporary %s: %w", candidates[index].name, err)
		}
		candidates[index].temporary = temporary
	}
	return nil
}

func publish(candidates []candidate, destination *os.Root) error {
	for index := range candidates {
		candidate := &candidates[index]
		if publishHook != nil {
			if err := publishHook(candidate.name, destination); err != nil {
				return fmt.Errorf("export: publish hook %s: %w", candidate.name, err)
			}
		}
		linkErr := destination.Link(candidate.temporary, candidate.name)
		if linkErr == nil {
			info, statErr := destination.Lstat(candidate.name)
			if statErr != nil {
				return fmt.Errorf("export: inspect published %s: %w", candidate.name, statErr)
			}
			candidate.created, candidate.targetInfo = true, info
			continue
		}
		current, readErr := destination.ReadFile(candidate.name)
		if readErr == nil && bytes.Equal(current, candidate.data) {
			continue
		}
		if readErr != nil && !errors.Is(readErr, fs.ErrNotExist) {
			return fmt.Errorf("export: inspect concurrent destination %s: %w", candidate.name, readErr)
		}
		if errors.Is(readErr, fs.ErrNotExist) {
			return fmt.Errorf("export: publish %s: %w", candidate.name, linkErr)
		}
		return fmt.Errorf("export: destination collision at %s", candidate.name)
	}
	return nil
}

func rollback(candidates []candidate, destination *os.Root) error {
	var cleanup error
	for index := len(candidates) - 1; index >= 0; index-- {
		candidate := candidates[index]
		if !candidate.created {
			continue
		}
		current, err := destination.Lstat(candidate.name)
		if err == nil && os.SameFile(current, candidate.targetInfo) {
			if err := destination.Remove(candidate.name); err != nil {
				cleanup = errors.Join(cleanup, fmt.Errorf("export: rollback %s: %w", candidate.name, err))
			}
		} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
			cleanup = errors.Join(cleanup, fmt.Errorf("export: inspect rollback %s: %w", candidate.name, err))
		}
	}
	return cleanup
}

func cleanupTemporary(candidates []candidate, destination *os.Root) {
	for _, candidate := range candidates {
		if candidate.temporary != "" {
			_ = destination.Remove(candidate.temporary)
		}
	}
}

func sourceSymlink(source fs.FS, name string) (bool, error) {
	parent := "."
	for _, component := range strings.Split(name, "/") {
		entries, err := fs.ReadDir(source, parent)
		if err != nil {
			return false, err
		}
		found := false
		for _, entry := range entries {
			if entry.Name() == component {
				if entry.Type()&fs.ModeSymlink != 0 {
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

func randomToken() string {
	var data [12]byte
	if _, err := rand.Read(data[:]); err != nil {
		panic("export: crypto/rand unavailable: " + err.Error())
	}
	return fmt.Sprintf("%x", data[:])
}
