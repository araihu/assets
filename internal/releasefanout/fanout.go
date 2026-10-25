// Package releasefanout verifies immutable Assets releases and dispatches them
// to the explicitly enrolled repositories.
package releasefanout

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const Repository = "araihu/assets"
const EventType = "araihu-assets-released"

var semver = regexp.MustCompile(`^v(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)(?:-(?:0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*)(?:\.(?:0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*))*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)
var repositoryName = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._-]*[a-z0-9])?$`)
var checksumLine = regexp.MustCompile(`^([0-9a-f]{64})  ([^\x00-\x1f\x7f]+)$`)

type Manifest struct {
	SchemaVersion int      `yaml:"schema_version" json:"schema_version"`
	Owner         string   `yaml:"owner" json:"owner"`
	Repositories  []string `yaml:"repositories" json:"repositories"`
}

func LoadManifest(data []byte) (Manifest, error) {
	var m Manifest
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return m, fmt.Errorf("consumer manifest: %w", err)
	}
	var rejectAliases func(*yaml.Node) error
	rejectAliases = func(node *yaml.Node) error {
		if node.Kind == yaml.AliasNode {
			return errors.New("consumer manifest aliases are not supported")
		}
		for _, child := range node.Content {
			if err := rejectAliases(child); err != nil {
				return err
			}
		}
		return nil
	}
	if err := rejectAliases(&document); err != nil {
		return m, err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&m); err != nil {
		return m, fmt.Errorf("consumer manifest: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return m, errors.New("consumer manifest must contain exactly one document")
	}
	return m, m.Validate()
}

func (m Manifest) Validate() error {
	if m.SchemaVersion != 1 || m.Owner != "araihu" || len(m.Repositories) == 0 || !slices.IsSorted(m.Repositories) {
		return errors.New("invalid consumer manifest schema, owner, or repository order")
	}
	for i, name := range m.Repositories {
		if !repositoryName.MatchString(name) || i > 0 && m.Repositories[i-1] == name {
			return errors.New("invalid or duplicate consumer repository")
		}
	}
	return nil
}

type Metadata struct {
	AssetsRepository  string `json:"assets_repository"`
	AssetsRevision    string `json:"assets_revision"`
	Release           string `json:"release"`
	ReleaseURL        string `json:"release_url"`
	ReleaseSHA256     string `json:"release_sha256"`
	ReleaseJSONSHA256 string `json:"release_json_sha256"`
}

func lowerHex(value string, size int) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == size && value == strings.ToLower(value)
}

func (m Metadata) Validate() error {
	if m.AssetsRepository != Repository || !semver.MatchString(m.Release) || !lowerHex(m.AssetsRevision, 20) ||
		!lowerHex(m.ReleaseSHA256, 32) || !lowerHex(m.ReleaseJSONSHA256, 32) ||
		m.ReleaseURL != "https://github.com/araihu/assets/releases/download/"+m.Release+"/araihu-assets-"+m.Release+".tar.gz" {
		return errors.New("invalid immutable release metadata")
	}
	return nil
}

type Plan struct {
	Consumers Manifest `json:"consumers"`
	Metadata  Metadata `json:"metadata"`
}

func (p Plan) Validate(m Manifest) error {
	if err := m.Validate(); err != nil {
		return err
	}
	if err := p.Consumers.Validate(); err != nil {
		return err
	}
	if !slices.Equal(p.Consumers.Repositories, m.Repositories) || p.Consumers.Owner != m.Owner {
		return errors.New("plan consumer selection differs from source manifest")
	}
	return p.Metadata.Validate()
}

func ReadPlan(directory string, m Manifest) (Plan, error) {
	var p Plan
	data, err := os.ReadFile(filepath.Join(directory, "plan.json"))
	if err == nil {
		err = json.Unmarshal(data, &p, json.RejectUnknownMembers(true))
	}
	if err != nil {
		return p, fmt.Errorf("read fan-out plan: %w", err)
	}
	return p, p.Validate(m)
}

func WritePlan(directory string, p Plan) error {
	if err := p.Validate(p.Consumers); err != nil {
		return err
	}
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(directory, "plan.json"), append(data, '\n'), 0o644)
}

// GitHub keeps credentials at the HTTP boundary; error messages never include
// response bodies or request headers.
type GitHub struct {
	client *http.Client
	base   string
	token  string
}

func NewGitHub(token string) (*GitHub, error) {
	if token == "" || strings.ContainsAny(token, "\r\n") {
		return nil, errors.New("GitHub token is missing or invalid")
	}
	return &GitHub{base: "https://api.github.com", token: token, client: &http.Client{
		Timeout: 2 * time.Minute,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 || req.URL.Scheme != "https" ||
				!(req.URL.Hostname() == "github.com" || req.URL.Hostname() == "api.github.com" || strings.HasSuffix(req.URL.Hostname(), ".githubusercontent.com")) {
				return errors.New("unexpected GitHub asset redirect")
			}
			if req.URL.Host != via[0].URL.Host {
				req.Header.Del("Authorization")
			}
			return nil
		},
	}}, nil
}

func (g *GitHub) request(ctx context.Context, method, path, accept string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, g.base+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+g.token)
	req.Header.Set("Accept", accept)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GitHub %s request failed", method)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		response.Body.Close()
		return nil, fmt.Errorf("GitHub %s %s: HTTP %d", method, path, response.StatusCode)
	}
	return response, nil
}

func (g *GitHub) getJSON(ctx context.Context, path string, target any) error {
	response, err := g.request(ctx, http.MethodGet, path, "application/vnd.github+json", nil)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 8<<20+1))
	if err != nil || len(data) > 8<<20 {
		return errors.New("GitHub JSON response exceeds limit or could not be read")
	}
	return json.Unmarshal(data, target)
}

func (g *GitHub) asset(ctx context.Context, id int64, limit int64) ([]byte, error) {
	response, err := g.request(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/releases/assets/%d", Repository, id), "application/octet-stream", nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil || int64(len(data)) > limit {
		return nil, errors.New("release asset exceeds limit or could not be read")
	}
	return data, nil
}

func checksums(data []byte) (map[string]string, error) {
	result := map[string]string{}
	for line := range strings.SplitSeq(strings.TrimSuffix(string(data), "\n"), "\n") {
		match := checksumLine.FindStringSubmatch(line)
		if match == nil || result[match[2]] != "" {
			return nil, errors.New("invalid or duplicate SHA256SUMS record")
		}
		result[match[2]] = match[1]
	}
	return result, nil
}

func digest(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }

func (g *GitHub) Prepare(ctx context.Context, m Manifest, release string) (Plan, error) {
	var p Plan
	if err := m.Validate(); err != nil {
		return p, err
	}
	if !semver.MatchString(release) {
		return p, errors.New("release must be a strict SemVer tag")
	}
	var info struct {
		TagName string `json:"tag_name"`
		Draft   bool   `json:"draft"`
		Assets  []struct {
			ID    int64  `json:"id"`
			Name  string `json:"name"`
			State string `json:"state"`
		} `json:"assets"`
	}
	if err := g.getJSON(ctx, "/repos/"+Repository+"/releases/tags/"+url.PathEscape(release), &info); err != nil {
		return p, err
	}
	if info.TagName != release || info.Draft {
		return p, errors.New("GitHub release tag mismatch or release is draft")
	}
	var commit struct {
		SHA string `json:"sha"`
	}
	commitPath := "/repos/" + Repository + "/commits/" + url.PathEscape(release)
	if err := g.getJSON(ctx, commitPath, &commit); err != nil {
		return p, err
	}
	archiveName := "araihu-assets-" + release + ".tar.gz"
	assets := map[string]int64{}
	for _, a := range info.Assets {
		if a.Name != archiveName && a.Name != "release.json" && a.Name != "SHA256SUMS" {
			continue
		}
		if a.ID < 1 || a.State != "uploaded" || assets[a.Name] != 0 {
			return p, errors.New("release asset is duplicate, invalid, or not uploaded")
		}
		assets[a.Name] = a.ID
	}
	if len(assets) != 3 {
		return p, errors.New("release is missing required fan-out assets")
	}
	sumsData, err := g.asset(ctx, assets["SHA256SUMS"], 1<<20)
	if err != nil {
		return p, err
	}
	sums, err := checksums(sumsData)
	if err != nil {
		return p, err
	}
	releaseData, err := g.asset(ctx, assets["release.json"], 16<<20)
	if err != nil {
		return p, err
	}
	if sums["release.json"] != digest(releaseData) {
		return p, errors.New("release.json SHA-256 mismatch")
	}
	var releaseDoc struct {
		Release string `json:"release"`
	}
	if err := json.Unmarshal(releaseData, &releaseDoc); err != nil || releaseDoc.Release != release {
		return p, errors.New("release.json tag mismatch or invalid JSON")
	}
	archive, err := g.asset(ctx, assets[archiveName], 256<<20)
	if err != nil {
		return p, err
	}
	if sums[archiveName] != digest(archive) {
		return p, errors.New("release archive SHA-256 mismatch")
	}
	var after struct {
		SHA string `json:"sha"`
	}
	if err := g.getJSON(ctx, commitPath, &after); err != nil {
		return p, err
	}
	if after.SHA != commit.SHA {
		return p, errors.New("release tag moved during verification")
	}
	p = Plan{Consumers: m, Metadata: Metadata{
		AssetsRepository: Repository, AssetsRevision: commit.SHA, Release: release,
		ReleaseURL:    "https://github.com/araihu/assets/releases/download/" + release + "/" + archiveName,
		ReleaseSHA256: sums[archiveName], ReleaseJSONSHA256: sums["release.json"],
	}}
	return p, p.Validate(m)
}

type Result struct {
	Repository string `json:"repository"`
	Dispatched bool   `json:"dispatched"`
}

func (g *GitHub) Dispatch(ctx context.Context, m Manifest, p Plan) ([]Result, error) {
	if err := p.Validate(m); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(struct {
		EventType     string   `json:"event_type"`
		ClientPayload Metadata `json:"client_payload"`
	}{EventType: EventType, ClientPayload: p.Metadata})
	if err != nil {
		return nil, err
	}
	var results []Result
	var failures []error
	for _, name := range m.Repositories {
		response, err := g.request(ctx, http.MethodPost, "/repos/"+m.Owner+"/"+name+"/dispatches", "application/vnd.github+json", bytes.NewReader(payload))
		if err == nil {
			response.Body.Close()
			if response.StatusCode != http.StatusNoContent {
				err = errors.New("dispatch did not return HTTP 204")
			}
		}
		results = append(results, Result{Repository: name, Dispatched: err == nil})
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", name, err))
		}
	}
	return results, errors.Join(failures...)
}

func Summary(results []Result, release string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "### Release fallback consumer dispatch\n\n")
	for _, result := range results {
		state := "failed"
		if result.Dispatched {
			state = "dispatched"
		}
		fmt.Fprintf(&b, "- `%s`: %s\n", result.Repository, state)
	}
	fmt.Fprintf(&b, "\nManual retry: run `Release consumer fan-out` with release `%s`.\n", release)
	return b.String()
}
