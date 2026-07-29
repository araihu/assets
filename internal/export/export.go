// Package export copies selected release files into a confined destination.
package export

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"
)

type publishPhase uint8

const (
	beforeLink publishPhase = iota
	afterLink
	beforeExistingVerify
)

// publishHook is a test-only fault/race injection seam. Production leaves it nil.
var publishHook func(publishPhase, string, *os.Root) error

type candidate struct {
	name        string
	data        []byte
	temporary   string
	preexisting bool
	created     bool
	stagedInfo  fs.FileInfo
}

// Copy writes paths from source beneath destination as one operation. source
// must be an immutable or snapshotted fs.FS: generic mutable fs.FS values,
// including os.DirFS, cannot prevent a symlink TOCTOU race. CopyRoot provides
// the rooted disk-backed variant. Existing identical files are retained;
// different files are never overwritten.
func Copy(source fs.FS, paths []string, destination *os.Root) error {
	return CopyContext(context.Background(), source, paths, destination)
}

// CopyContext writes paths only while ctx remains active. Cancellation before
// publication leaves destination files absent; a completed no-replace link is
// an indivisible publication boundary.
func CopyContext(ctx context.Context, source fs.FS, paths []string, destination *os.Root) error {
	if source == nil {
		return errors.New("export: source is nil")
	}
	if destination == nil {
		return errors.New("export: destination root is nil")
	}
	if err := checkContext(ctx); err != nil {
		return err
	}
	ordered, err := safePaths(paths)
	if err != nil {
		return err
	}
	candidates, err := preflight(ctx, source, ordered, destination)
	if err != nil {
		return err
	}
	if err := stage(ctx, candidates, destination); err != nil {
		cleanupTemporary(candidates, destination)
		return err
	}
	defer cleanupTemporary(candidates, destination)
	if err := publish(ctx, candidates, destination); err != nil {
		return errors.Join(err, rollback(candidates, destination))
	}
	return nil
}

// CopyRoot copies from a live, rooted source. os.Root confines symlink
// resolution beneath source even when the source tree changes concurrently.
func CopyRoot(source *os.Root, paths []string, destination *os.Root) error {
	return CopyRootContext(context.Background(), source, paths, destination)
}

// CopyRootContext copies from a live rooted source with cancellation checks.
func CopyRootContext(ctx context.Context, source *os.Root, paths []string, destination *os.Root) error {
	if source == nil {
		return errors.New("export: source root is nil")
	}
	return CopyContext(ctx, source.FS(), paths, destination)
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

func preflight(ctx context.Context, source fs.FS, paths []string, destination *os.Root) ([]candidate, error) {
	candidates := make([]candidate, 0, len(paths))
	for _, name := range paths {
		if err := checkContext(ctx); err != nil {
			return nil, err
		}
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
			candidates = append(candidates, candidate{name: name, data: data, preexisting: true})
			continue
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("export: inspect destination %s: %w", name, err)
		}
		candidates = append(candidates, candidate{name: name, data: data})
	}
	return candidates, nil
}

func stage(ctx context.Context, candidates []candidate, destination *os.Root) (err error) {
	for index := range candidates {
		if err := checkContext(ctx); err != nil {
			return err
		}
		if candidates[index].preexisting {
			continue
		}
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

func publish(ctx context.Context, candidates []candidate, destination *os.Root) error {
	for index := range candidates {
		if err := checkContext(ctx); err != nil {
			return err
		}
		candidate := &candidates[index]
		if candidate.preexisting {
			if publishHook != nil {
				if err := publishHook(beforeExistingVerify, candidate.name, destination); err != nil {
					return fmt.Errorf("export: publish hook %s: %w", candidate.name, err)
				}
			}
			if err := checkContext(ctx); err != nil {
				return err
			}
			info, err := destination.Lstat(candidate.name)
			if err != nil || !info.Mode().IsRegular() {
				return fmt.Errorf("export: preexisting destination changed at %s", candidate.name)
			}
			current, err := destination.ReadFile(candidate.name)
			if err != nil || !bytes.Equal(current, candidate.data) {
				return fmt.Errorf("export: preexisting destination changed at %s", candidate.name)
			}
			continue
		}
		temporaryInfo, err := destination.Lstat(candidate.temporary)
		if err != nil || !temporaryInfo.Mode().IsRegular() {
			return fmt.Errorf("export: staged temporary changed for %s", candidate.name)
		}
		candidate.stagedInfo = temporaryInfo
		if publishHook != nil {
			if err := publishHook(beforeLink, candidate.name, destination); err != nil {
				return fmt.Errorf("export: publish hook %s: %w", candidate.name, err)
			}
		}
		if err := checkContext(ctx); err != nil {
			return err
		}
		linkErr := destination.Link(candidate.temporary, candidate.name)
		if linkErr == nil {
			candidate.created = true
			if publishHook != nil {
				if err := publishHook(afterLink, candidate.name, destination); err != nil {
					return fmt.Errorf("export: publish hook %s: %w", candidate.name, err)
				}
			}
			info, statErr := destination.Lstat(candidate.name)
			if statErr != nil {
				return fmt.Errorf("export: inspect published %s: %w", candidate.name, statErr)
			}
			if !os.SameFile(candidate.stagedInfo, info) {
				return fmt.Errorf("export: published target changed for %s", candidate.name)
			}
			current, readErr := destination.ReadFile(candidate.name)
			if readErr != nil || !bytes.Equal(current, candidate.data) {
				return fmt.Errorf("export: published target bytes changed for %s", candidate.name)
			}
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

func checkContext(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("export: %w", err)
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
		if err == nil && os.SameFile(current, candidate.stagedInfo) {
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
