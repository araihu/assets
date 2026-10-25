package runneradmission

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// Paths are host-owned files in production and explicit test seams locally.
type Paths struct{ Domain, Actors, Coordination string }

func DefaultPaths() Paths {
	return Paths{"/etc/hostinger-runner/trust-domain", "/etc/hostinger-runner/allowed-actors", "/opt/hostinger-cache"}
}

func readBounded(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.New("missing admission input")
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || int64(len(b)) > limit {
		return nil, errors.New("invalid admission input")
	}
	return b, nil
}

func Admit(c Config, paths Paths) error {
	domain, err := readBounded(paths.Domain, 128)
	if err != nil {
		return err
	}
	actors, err := readBounded(paths.Actors, 64<<10)
	if err != nil {
		return err
	}
	payload, err := readBounded(c.EventPath, 16<<20)
	if err != nil {
		return err
	}
	if err := Check(c, strings.TrimSpace(string(domain)), strings.Split(strings.TrimSuffix(string(actors), "\n"), "\n"), payload); err != nil {
		return err
	}
	return coordinate(paths.Coordination)
}

func lock(path string, flags int) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0600)
	if err == nil {
		err = unix.Flock(int(f.Fd()), flags)
	}
	if err != nil {
		if f != nil {
			f.Close()
		}
		return nil, err
	}
	return f, nil
}

func coordinate(root string) error {
	guard, err := lock(filepath.Join(root, ".maintenance.lock"), unix.LOCK_EX)
	if err != nil {
		return errors.New("coordination lock")
	}
	defer guard.Close()
	marker := filepath.Join(root, ".maintenance")
	if _, err := os.Lstat(marker); err == nil {
		maintenance, err := lock(filepath.Join(root, ".infrastructure-maintenance.lock"), unix.LOCK_EX|unix.LOCK_NB)
		if err != nil {
			return errors.New("lane maintenance")
		}
		defer maintenance.Close()
		if err := os.Remove(marker); err != nil {
			return errors.New("maintenance marker")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return errors.New("maintenance marker")
	}
	f, err := os.OpenFile(filepath.Join(root, ".job-active"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return errors.New("job marker")
	}
	return f.Close()
}
