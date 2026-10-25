// Package consumerupdate verifies an immutable Assets release and prepares only
// the files selected by a consumer-owned manifest.
package consumerupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/araihu/assets/internal/catalog"
	"github.com/araihu/assets/internal/releasefanout"
	"github.com/araihu/assets/internal/releasemeta"
)

const ManifestPath = "araihu-assets.json"

// Manifest is owned by the consumer. Dispatch data can change only Metadata.
type Manifest struct {
	SchemaVersion int                    `json:"schemaVersion"`
	Metadata      releasefanout.Metadata `json:"metadata"`
	Mappings      []Mapping              `json:"mappings"`
}

type Mapping struct {
	Source        string `json:"source"`
	Destination   string `json:"destination"`
	CanonicalName string `json:"canonicalName,omitempty"`
}

func Load(data []byte) (Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m, json.RejectUnknownMembers(true)); err != nil {
		return m, fmt.Errorf("consumer manifest: %w", err)
	}
	if m.SchemaVersion != 1 || len(m.Mappings) == 0 {
		return m, errors.New("consumer manifest schema or mappings invalid")
	}
	if err := m.Metadata.Validate(); err != nil {
		return m, err
	}
	seen := map[string]bool{}
	for _, mapping := range m.Mappings {
		if !safePath(mapping.Source) || !safePath(mapping.Destination) || strings.EqualFold(mapping.Destination, ManifestPath) || strings.EqualFold(strings.Split(mapping.Destination, "/")[0], ".git") || seen[strings.ToLower(mapping.Destination)] {
			return m, errors.New("unsafe or duplicate consumer mapping")
		}
		seen[strings.ToLower(mapping.Destination)] = true
		if mapping.CanonicalName == "" && mapping.Source != "themes/araihu.css" && mapping.Source != "themes/araihu-signal-night.css" {
			return m, errors.New("visual assets require canonicalName")
		}
	}
	return m, nil
}

func safePath(name string) bool {
	return name != "." && fs.ValidPath(name) && !strings.ContainsAny(name, "\\:\x00\r\n")
}

// EventMetadata handles GitHub's two supported triggers without shell parsing.
func EventMetadata(data []byte) (releasefanout.Metadata, error) {
	var event struct {
		Action        string         `json:"action"`
		ClientPayload jsontext.Value `json:"client_payload"`
		Inputs        jsontext.Value `json:"inputs"`
	}
	if err := json.Unmarshal(data, &event); err != nil {
		return releasefanout.Metadata{}, errors.New("invalid GitHub event")
	}
	var raw []byte
	if len(event.ClientPayload) > 0 && string(event.ClientPayload) != "null" {
		if event.Action != releasefanout.EventType || len(event.Inputs) > 0 && string(event.Inputs) != "null" {
			return releasefanout.Metadata{}, errors.New("unexpected or ambiguous assets event")
		}
		raw = event.ClientPayload
	} else {
		raw = event.Inputs
	}
	var metadata releasefanout.Metadata
	if err := json.Unmarshal(raw, &metadata, json.RejectUnknownMembers(true)); err != nil {
		return metadata, errors.New("invalid immutable event metadata")
	}
	return metadata, metadata.Validate()
}

// Fetch checks current GitHub release provenance before downloading the pinned
// public archive. Credentials are used only by releasefanout's GitHub client.
func Fetch(ctx context.Context, metadata releasefanout.Metadata, token string) ([]byte, error) {
	if err := metadata.Validate(); err != nil {
		return nil, err
	}
	github, err := releasefanout.NewGitHub(token)
	if err != nil {
		return nil, err
	}
	plan, err := github.Prepare(ctx, releasefanout.Manifest{SchemaVersion: 1, Owner: "araihu", Repositories: []string{"consumer"}}, metadata.Release)
	if err != nil {
		return nil, err
	}
	if plan.Metadata != metadata {
		return nil, errors.New("event metadata differs from published GitHub release")
	}
	client := &http.Client{Timeout: 2 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 || req.URL.Scheme != "https" || !(req.URL.Hostname() == "github.com" || strings.HasSuffix(req.URL.Hostname(), ".githubusercontent.com")) {
			return errors.New("unexpected archive redirect")
		}
		return nil
	}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metadata.ReleaseURL, nil)
	if err != nil {
		return nil, errors.New("invalid release request")
	}
	response, err := client.Do(req)
	if err != nil {
		return nil, errors.New("release archive request failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("release archive HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 256<<20+1))
	if err != nil || len(data) > 256<<20 {
		return nil, errors.New("release archive exceeds limit or cannot be read")
	}
	if digest(data) != metadata.ReleaseSHA256 {
		return nil, errors.New("release archive SHA-256 mismatch")
	}
	return data, nil
}

// Prepare verifies archive safety, metadata, catalog selection, and every mapped
// file before returning an allowlisted patch. It performs no filesystem writes.
func Prepare(m Manifest, metadata releasefanout.Metadata, archive []byte) (map[string][]byte, error) {
	encoded, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	if _, err := Load(encoded); err != nil {
		return nil, err
	}
	if err := metadata.Validate(); err != nil {
		return nil, err
	}
	if digest(archive) != metadata.ReleaseSHA256 {
		return nil, errors.New("release archive SHA-256 mismatch")
	}
	wanted := map[string]bool{"release.json": true, "catalog.json": true}
	for _, mapping := range m.Mappings {
		wanted[mapping.Source] = true
	}
	files, err := members(archive, wanted)
	if err != nil {
		return nil, err
	}
	if digest(files["release.json"]) != metadata.ReleaseJSONSHA256 {
		return nil, errors.New("release.json SHA-256 mismatch")
	}
	var document releasemeta.Document
	if err := json.Unmarshal(files["release.json"], &document, json.RejectUnknownMembers(true)); err != nil {
		return nil, err
	}
	if err := document.Validate(); err != nil {
		return nil, err
	}
	if document.Release != metadata.Release {
		return nil, errors.New("release.json tag mismatch")
	}
	inventory := map[string]releasemeta.File{}
	for _, file := range document.Files {
		inventory[file.Path] = file
	}
	for name, data := range files {
		if name == "release.json" {
			continue
		}
		file, ok := inventory[name]
		if !ok || digest(data) != file.SHA256 || int64(len(data)) != file.Size {
			return nil, fmt.Errorf("release file identity mismatch: %s", name)
		}
	}
	cat, err := catalog.Decode(bytes.NewReader(files["catalog.json"]))
	if err != nil {
		return nil, err
	}
	if cat.Release != metadata.Release || cat.IdentityRevision != document.IdentityRevision {
		return nil, errors.New("catalog release identity mismatch")
	}
	byName := map[string]catalog.Asset{}
	for _, asset := range cat.Assets {
		byName[asset.CanonicalName] = asset
	}
	patch := map[string][]byte{}
	for _, mapping := range m.Mappings {
		if mapping.CanonicalName != "" {
			asset, ok := byName[mapping.CanonicalName]
			if !ok || asset.Path != mapping.Source || asset.SHA256 != inventory[mapping.Source].SHA256 {
				return nil, fmt.Errorf("catalog selection mismatch: %s", mapping.CanonicalName)
			}
		}
		patch[mapping.Destination] = files[mapping.Source]
	}
	m.Metadata = metadata
	receipt, err := json.Marshal(m, jsontext.Multiline(true), jsontext.WithIndent("  "))
	if err != nil {
		return nil, err
	}
	patch[ManifestPath] = append(receipt, '\n')
	return patch, nil
}

func members(archive []byte, wanted map[string]bool) (map[string][]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	limited := &io.LimitedReader{R: gz, N: 512<<20 + 1}
	tr := tar.NewReader(limited)
	seen := map[string]bool{}
	files := map[string][]byte{}
	for count := 0; ; count++ {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, errors.New("invalid release archive")
		}
		name := header.Name
		if header.Typeflag == tar.TypeDir {
			name = strings.TrimSuffix(name, "/")
		}
		if count >= 10000 || !safePath(name) || seen[strings.ToLower(name)] || header.Size < 0 || header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeDir {
			return nil, errors.New("unsafe, duplicate, or excessive archive member")
		}
		seen[strings.ToLower(name)] = true
		if wanted[name] {
			if header.Typeflag != tar.TypeReg || header.Size > 16<<20 {
				return nil, errors.New("selected archive member is not a bounded regular file")
			}
			data, err := io.ReadAll(tr)
			if err != nil {
				return nil, err
			}
			files[name] = data
		}
	}
	// Consume the gzip trailer as well, detecting truncation and expansion past
	// the limit even when tar already reached its end-of-archive blocks.
	if _, err := io.Copy(io.Discard, limited); err != nil || limited.N <= 0 {
		return nil, errors.New("invalid or excessive expanded archive")
	}
	for name := range wanted {
		if _, ok := files[name]; !ok {
			return nil, fmt.Errorf("archive missing %s", name)
		}
	}
	return files, nil
}

// WritePatch writes a fresh patch directory. Refusing an existing output keeps
// symlinks and unrelated consumer files outside this tool's write boundary.
func WritePatch(directory string, patch map[string][]byte) error {
	if err := os.Mkdir(directory, 0o755); err != nil {
		return fmt.Errorf("create fresh patch directory: %w", err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return err
	}
	defer root.Close()
	for name, data := range patch {
		if !safePath(name) {
			return errors.New("unsafe output path")
		}
		if err := root.MkdirAll(path.Dir(name), 0o755); err != nil {
			return err
		}
		if err := root.WriteFile(name, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func digest(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }
