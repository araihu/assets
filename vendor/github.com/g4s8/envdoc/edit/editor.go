package edit

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

const (
	MarkerBegin = "<!--envdoc:begin-->"
	MarkerEnd   = "<!--envdoc:end-->"
)

// Editor handles in-place editing of files with marker-based content replacement
type Editor struct {
	filePath string
}

// NewEditor creates a new Editor for the specified file path
func NewEditor(filePath string) *Editor {
	return &Editor{
		filePath: filePath,
	}
}

// ReplaceSection replaces the content between markers with newContent.
// If the file doesn't exist, it creates a new file with markers wrapping the content.
// If the file exists, it finds the markers and replaces only the content between them.
func (e *Editor) ReplaceSection(newContent []byte) error {
	// Check if file exists
	_, err := os.Stat(e.filePath)
	if os.IsNotExist(err) {
		return e.createFileWithMarkers(newContent)
	}
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	return e.replaceInExistingFile(newContent)
}

func (e *Editor) createFileWithMarkers(content []byte) error {
	dir := filepath.Dir(e.filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	var buf bytes.Buffer
	buf.WriteString(MarkerBegin)
	buf.WriteString("\n")
	buf.Write(content)
	if len(content) > 0 && content[len(content)-1] != '\n' {
		buf.WriteString("\n")
	}
	buf.WriteString(MarkerEnd)
	buf.WriteString("\n")

	return e.atomicWrite(buf.Bytes())
}

func validateMarkers(content []byte, beginIdx, endIdx int) error {
	if beginIdx == -1 && endIdx == -1 {
		return fmt.Errorf("no markers found in file: missing both %s and %s", MarkerBegin, MarkerEnd)
	}
	if beginIdx == -1 {
		return fmt.Errorf("missing begin marker: %s", MarkerBegin)
	}
	if endIdx == -1 {
		return fmt.Errorf("missing end marker: %s", MarkerEnd)
	}
	if beginIdx >= endIdx {
		return fmt.Errorf("markers in wrong order: begin marker must come before end marker")
	}

	// Check for duplicate markers
	if bytes.Count(content, []byte(MarkerBegin)) > 1 {
		return fmt.Errorf("duplicate begin markers found: %s appears more than once", MarkerBegin)
	}
	if bytes.Count(content, []byte(MarkerEnd)) > 1 {
		return fmt.Errorf("duplicate end markers found: %s appears more than once", MarkerEnd)
	}

	return nil
}

func (e *Editor) replaceInExistingFile(newContent []byte) error {
	existingContent, err := os.ReadFile(e.filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	beginIdx := bytes.Index(existingContent, []byte(MarkerBegin))
	endIdx := bytes.Index(existingContent, []byte(MarkerEnd))

	if err := validateMarkers(existingContent, beginIdx, endIdx); err != nil {
		return err
	}

	var buf bytes.Buffer

	buf.Write(existingContent[:beginIdx])
	buf.WriteString(MarkerBegin)
	buf.WriteString("\n")
	buf.Write(newContent)
	if len(newContent) > 0 && newContent[len(newContent)-1] != '\n' {
		buf.WriteString("\n")
	}
	endMarkerStart := endIdx
	buf.Write(existingContent[endMarkerStart:])

	return e.atomicWrite(buf.Bytes())
}

// atomicWrite writes content to a temp file then renames it to the target path
// This prevents corruption if the write fails partway through
func (e *Editor) atomicWrite(content []byte) error {
	dir := filepath.Dir(e.filePath)
	tmpFile, err := os.CreateTemp(dir, ".envdoc-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	defer func() {
		if tmpFile != nil {
			tmpFile.Close()
			os.Remove(tmpPath)
		}
	}()

	if _, err := tmpFile.Write(content); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, e.filePath); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	tmpFile = nil
	return nil
}
