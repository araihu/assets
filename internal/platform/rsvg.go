// Package platform creates deterministic web and native icon packages.
package platform

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	pinnedRSVGVersion  = "2.62.1"
	maxRasterDimension = 8192
	maxStderrBytes     = 4096
)

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// Request describes one bounded SVG-to-PNG conversion.
type Request struct {
	SVG           []byte
	Width, Height int
	Background    string
}

// Rasterizer rasterizes one SVG without exposing process details to generators.
type Rasterizer interface {
	Rasterize(context.Context, Request) ([]byte, error)
}

// Runner is the narrow external-process boundary used by RSVG.
type Runner interface {
	Run(context.Context, string, []string, []byte) ([]byte, []byte, error)
}

// RSVG invokes the project-pinned rsvg-convert binary.
type RSVG struct {
	Runner  Runner
	Binary  string
	Timeout time.Duration
}

// NewRSVG constructs an adapter with a testable process boundary.
func NewRSVG(runner Runner) *RSVG {
	if runner == nil {
		runner = commandRunner{}
	}
	return &RSVG{Runner: runner, Binary: "rsvg-convert", Timeout: 30 * time.Second}
}

// Rasterize converts SVG to PNG after proving the renderer version.
func (r *RSVG) Rasterize(ctx context.Context, request Request) ([]byte, error) {
	if err := validateRequest(request); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("rsvg-convert: %w", err)
	}
	if err := r.checkVersion(ctx); err != nil {
		return nil, err
	}
	conversionCtx, cancel := context.WithTimeout(ctx, r.timeout())
	defer cancel()
	args := []string{"--width", strconv.Itoa(request.Width), "--height", strconv.Itoa(request.Height), "--format", "png"}
	if request.Background != "" {
		args = append(args, "--background-color", request.Background)
	}
	args = append(args, "-")
	stdout, stderr, err := r.Runner.Run(conversionCtx, r.binary(), args, request.SVG)
	if err != nil {
		if contextErr := conversionCtx.Err(); contextErr != nil {
			return nil, fmt.Errorf("rsvg-convert: %w", contextErr)
		}
		return nil, processError("rasterize", err, stderr)
	}
	if err := conversionCtx.Err(); err != nil {
		return nil, fmt.Errorf("rsvg-convert: %w", err)
	}
	return stdout, nil
}

func (r *RSVG) checkVersion(ctx context.Context) error {
	versionCtx, cancel := context.WithTimeout(ctx, r.timeout())
	defer cancel()
	stdout, stderr, err := r.Runner.Run(versionCtx, r.binary(), []string{"--version"}, nil)
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return fmt.Errorf("rsvg-convert: binary not found; install rsvg-convert version %s", pinnedRSVGVersion)
		}
		if contextErr := versionCtx.Err(); contextErr != nil {
			return fmt.Errorf("rsvg-convert version check: %w", contextErr)
		}
		return processError("version check", err, stderr)
	}
	version := strings.TrimSpace(string(stdout))
	if newline := strings.IndexByte(version, '\n'); newline >= 0 {
		version = version[:newline]
	}
	expected := "rsvg-convert version " + pinnedRSVGVersion
	if version != expected {
		return fmt.Errorf("rsvg-convert version mismatch: expected %s, got %q", expected, version)
	}
	return nil
}

func (r *RSVG) binary() string {
	if r.Binary == "" {
		return "rsvg-convert"
	}
	return r.Binary
}

func (r *RSVG) timeout() time.Duration {
	if r.Timeout <= 0 {
		return 30 * time.Second
	}
	return r.Timeout
}

func validateRequest(request Request) error {
	if len(bytes.TrimSpace(request.SVG)) == 0 {
		return errors.New("rsvg-convert: SVG is required")
	}
	if request.Width < 1 || request.Width > maxRasterDimension || request.Height < 1 || request.Height > maxRasterDimension {
		return fmt.Errorf("rsvg-convert: dimensions must be between 1 and %d", maxRasterDimension)
	}
	if request.Background != "" && !hexColor.MatchString(request.Background) {
		return fmt.Errorf("rsvg-convert: background must be #rrggbb, got %q", request.Background)
	}
	return nil
}

func processError(operation string, err error, stderr []byte) error {
	detail := strings.TrimSpace(string(stderr))
	if len(detail) > maxStderrBytes {
		detail = detail[:maxStderrBytes] + "..."
	}
	if detail == "" {
		return fmt.Errorf("rsvg-convert %s: %w", operation, err)
	}
	return fmt.Errorf("rsvg-convert %s: %w: %s", operation, err, detail)
}

type commandRunner struct{}

func (commandRunner) Run(ctx context.Context, name string, args []string, stdin []byte) ([]byte, []byte, error) {
	command := exec.CommandContext(ctx, name, args...)
	command.Stdin = bytes.NewReader(stdin)
	var stdout bytes.Buffer
	stderr := boundedCapture{limit: maxStderrBytes + 1}
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	return stdout.Bytes(), append([]byte(nil), stderr.Bytes()...), err
}

// boundedCapture retains enough stderr to report a useful error and discards
// remaining child output without making the child fail with io.ErrShortWrite.
type boundedCapture struct {
	buffer bytes.Buffer
	limit  int
}

func (b *boundedCapture) Write(data []byte) (int, error) {
	original := len(data)
	remaining := b.limit - b.buffer.Len()
	if remaining > 0 {
		if remaining > len(data) {
			remaining = len(data)
		}
		_, _ = b.buffer.Write(data[:remaining])
	}
	return original, nil
}

func (b *boundedCapture) Bytes() []byte { return b.buffer.Bytes() }
