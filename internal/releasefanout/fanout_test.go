package releasefanout

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func consumerManifest() Manifest {
	return Manifest{SchemaVersion: 1, Owner: "araihu", Repositories: []string{"alpha", "beta", "gamma"}}
}

func validMetadata() Metadata {
	return Metadata{AssetsRepository: Repository, AssetsRevision: strings.Repeat("a", 40), Release: "v0.2.4",
		ReleaseURL:    "https://github.com/araihu/assets/releases/download/v0.2.4/araihu-assets-v0.2.4.tar.gz",
		ReleaseSHA256: strings.Repeat("b", 64), ReleaseJSONSHA256: strings.Repeat("c", 64)}
}

func TestManifest(t *testing.T) {
	valid := "schema_version: 1\nowner: araihu\nrepositories: [alpha, beta]\n"
	for _, test := range []struct {
		name, data string
		valid      bool
	}{
		{"valid", valid, true},
		{"unknown", valid + "unexpected: true\n", false},
		{"alias", "schema_version: 1\nowner: &owner araihu\nrepositories: [*owner]\n", false},
		{"duplicate key", valid + "owner: araihu\n", false},
		{"duplicate repo", strings.Replace(valid, "alpha, beta", "alpha, alpha", 1), false},
		{"unsorted", strings.Replace(valid, "alpha, beta", "beta, alpha", 1), false},
		{"traversal", strings.Replace(valid, "alpha, beta", "../alpha, beta", 1), false},
		{"wrong owner", strings.Replace(valid, "araihu", "other", 1), false},
		{"empty", strings.Replace(valid, "[alpha, beta]", "[]", 1), false},
		{"multiple documents", valid + "---\nowner: other\n", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := LoadManifest([]byte(test.data))
			if (err == nil) != test.valid {
				t.Fatalf("valid=%v, error=%v", test.valid, err)
			}
		})
	}
	data, err := os.ReadFile("../../manifests/release-consumers.yaml")
	if err != nil {
		t.Fatal(err)
	}
	m, err := LoadManifest(data)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(m.Repositories, []string{"goshtoso", "goshtoso-app-shells", "goshtoso-charts", "manja", "xisnove"}) {
		t.Fatalf("unexpected enrolled consumers: %v", m.Repositories)
	}
}

type releaseFixture struct {
	draft         bool
	wrongTag      bool
	missing       bool
	duplicate     bool
	unuploaded    bool
	badSums       bool
	badArchive    bool
	badMetadata   bool
	moveTag       bool
	duplicateJSON bool
	commits       int
	posts         int
}

func fixtureClient(t *testing.T, fixture *releaseFixture) *GitHub {
	t.Helper()
	metadata := []byte(`{"release":"v0.2.4","schemaVersion":1}`)
	archive := []byte("immutable archive bytes")
	sums := []byte(fmt.Sprintf("%s  release.json\n%s  araihu-assets-v0.2.4.tar.gz\n", digest(metadata), digest(archive)))
	if fixture.badSums {
		sums = append(sums, sums...)
	}
	if fixture.badArchive {
		archive = []byte("corrupt")
	}
	if fixture.badMetadata {
		metadata = []byte(`{"release":"v0.2.3"}`)
	}
	if fixture.duplicateJSON {
		metadata = []byte(`{"release":"v0.2.3","release":"v0.2.4"}`)
		sums = []byte(fmt.Sprintf("%s  release.json\n%s  araihu-assets-v0.2.4.tar.gz\n", digest(metadata), digest(archive)))
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-secret" {
			t.Error("missing GitHub authorization")
		}
		if r.Method == http.MethodPost {
			fixture.posts++
			w.WriteHeader(http.StatusNoContent)
			return
		}
		switch r.URL.Path {
		case "/repos/araihu/assets/releases/tags/v0.2.4":
			tag := "v0.2.4"
			if fixture.wrongTag {
				tag = "v0.2.3"
			}
			assets := []map[string]any{{"id": 1, "name": "SHA256SUMS", "state": "uploaded"}, {"id": 2, "name": "release.json", "state": "uploaded"}, {"id": 3, "name": "araihu-assets-v0.2.4.tar.gz", "state": "uploaded"}}
			if fixture.missing {
				assets = assets[:2]
			}
			if fixture.duplicate {
				assets = append(assets, assets[0])
			}
			if fixture.unuploaded {
				assets[0]["state"] = "new"
			}
			data, _ := json.Marshal(map[string]any{"tag_name": tag, "draft": fixture.draft, "assets": assets})
			w.Write(data)
		case "/repos/araihu/assets/commits/v0.2.4":
			fixture.commits++
			sha := strings.Repeat("a", 40)
			if fixture.moveTag && fixture.commits > 1 {
				sha = strings.Repeat("b", 40)
			}
			fmt.Fprintf(w, `{"sha":%q}`, sha)
		case "/repos/araihu/assets/releases/assets/1":
			w.Write(sums)
		case "/repos/araihu/assets/releases/assets/2":
			w.Write(metadata)
		case "/repos/araihu/assets/releases/assets/3":
			w.Write(archive)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	g, err := NewGitHub("test-secret")
	if err != nil {
		t.Fatal(err)
	}
	g.base = server.URL
	g.client = server.Client()
	return g
}

func TestPrepare(t *testing.T) {
	for _, test := range []struct {
		name    string
		fixture releaseFixture
		valid   bool
	}{
		{"verified", releaseFixture{}, true},
		{"draft", releaseFixture{draft: true}, false},
		{"wrong tag", releaseFixture{wrongTag: true}, false},
		{"missing asset", releaseFixture{missing: true}, false},
		{"duplicate asset", releaseFixture{duplicate: true}, false},
		{"upload incomplete", releaseFixture{unuploaded: true}, false},
		{"duplicate checksums", releaseFixture{badSums: true}, false},
		{"archive corrupted", releaseFixture{badArchive: true}, false},
		{"metadata corrupted", releaseFixture{badMetadata: true}, false},
		{"tag moved", releaseFixture{moveTag: true}, false},
		{"duplicate JSON", releaseFixture{duplicateJSON: true}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			g := fixtureClient(t, &test.fixture)
			plan, err := g.Prepare(t.Context(), consumerManifest(), "v0.2.4")
			if (err == nil) != test.valid {
				t.Fatalf("valid=%v, error=%v", test.valid, err)
			}
			if err == nil {
				if err := plan.Validate(consumerManifest()); err != nil {
					t.Fatal(err)
				}
			}
			if test.fixture.posts != 0 {
				t.Fatal("prepare dispatched before scoped token")
			}
		})
	}
	for _, tag := range []string{"v1", "v01.2.3", "v1.2.3-01", "v0.2.4\n", "v0.2.4;bad"} {
		t.Run(tag, func(t *testing.T) {
			g := fixtureClient(t, &releaseFixture{})
			if _, err := g.Prepare(t.Context(), consumerManifest(), tag); err == nil {
				t.Fatal("invalid release accepted")
			}
		})
	}
}

func TestDispatchAttemptsEveryConsumerOnFailureAndRetry(t *testing.T) {
	var names []string
	var failBeta = true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer test-secret" {
			t.Error("invalid dispatch request")
		}
		name := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/repos/araihu/"), "/dispatches")
		names = append(names, name)
		data, _ := io.ReadAll(r.Body)
		var payload struct {
			EventType     string   `json:"event_type"`
			ClientPayload Metadata `json:"client_payload"`
		}
		if err := json.Unmarshal(data, &payload, json.RejectUnknownMembers(true)); err != nil {
			t.Error(err)
		}
		if payload.EventType != EventType || payload.ClientPayload != validMetadata() {
			t.Error("dispatch metadata drift")
		}
		if failBeta && name == "beta" {
			http.Error(w, "test-secret", http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	g, _ := NewGitHub("test-secret")
	g.base = server.URL
	g.client = server.Client()
	m := consumerManifest()
	p := Plan{Consumers: m, Metadata: validMetadata()}
	results, err := g.Dispatch(t.Context(), m, p)
	if err == nil || strings.Contains(err.Error(), "test-secret") || !slices.Equal(names, m.Repositories) {
		t.Fatalf("results=%v, error=%v, names=%v", results, err, names)
	}
	if len(results) != 3 || !results[0].Dispatched || results[1].Dispatched || !results[2].Dispatched {
		t.Fatalf("unexpected results %v", results)
	}
	names = nil
	failBeta = false
	results, err = g.Dispatch(t.Context(), m, p)
	if err != nil || !slices.Equal(names, m.Repositories) {
		t.Fatalf("retry skipped consumer: %v, %v", names, err)
	}
	if !strings.Contains(Summary(results, p.Metadata.Release), "`gamma`: dispatched") {
		t.Fatal("missing receipt")
	}
	names = nil
	p.Consumers.Repositories = []string{"outside"}
	if _, err := g.Dispatch(t.Context(), m, p); err == nil || len(names) > 0 {
		t.Fatal("consumer drift was dispatched")
	}
}

func TestPlanStrictJSONAndProvenance(t *testing.T) {
	m := consumerManifest()
	p := Plan{Consumers: m, Metadata: validMetadata()}
	directory := t.TempDir()
	if err := WritePlan(directory, p); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadPlan(directory, m); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(directory, "plan.json"))
	for _, invalid := range []string{
		`null`, string(data) + `{}`, strings.Replace(string(data), `"metadata":`, `"unknown":0,"metadata":`, 1),
		strings.Replace(string(data), `"release":"v0.2.4"`, `"release":"v0.2.3","release":"v0.2.4"`, 1),
		strings.Replace(string(data), `"release":"v0.2.4"`, `"Release":"v0.2.4"`, 1),
		strings.Replace(string(data), "https://github.com/araihu/assets/", "https://other.invalid/", 1),
		strings.Replace(string(data), strings.Repeat("a", 40), strings.Repeat("A", 40), 1),
	} {
		if err := os.WriteFile(filepath.Join(directory, "plan.json"), []byte(invalid), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadPlan(directory, m); err == nil {
			t.Fatalf("invalid plan accepted: %s", invalid)
		}
	}
}

func TestCancellationAndRedirectBoundary(t *testing.T) {
	g, _ := NewGitHub("test-secret")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := g.Prepare(ctx, consumerManifest(), "v0.2.4"); err == nil {
		t.Fatal("cancelled operation succeeded")
	}
	for _, target := range []string{"https://evil.invalid/token", "http://github.com/token", "https://githubusercontent.com.evil.invalid/token"} {
		req, _ := http.NewRequest(http.MethodGet, target, nil)
		via, _ := http.NewRequest(http.MethodGet, "https://api.github.com/assets", nil)
		if err := g.client.CheckRedirect(req, []*http.Request{via}); err == nil {
			t.Fatalf("unsafe redirect allowed %s", target)
		}
	}
	req, _ := http.NewRequest(http.MethodGet, "https://release-assets.githubusercontent.com/asset", nil)
	req.Header.Set("Authorization", "Bearer test-secret")
	via, _ := http.NewRequest(http.MethodGet, "https://api.github.com/assets", nil)
	if err := g.client.CheckRedirect(req, []*http.Request{via}); err != nil || req.Header.Get("Authorization") != "" {
		t.Fatal("redirect leaked credentials")
	}
}
