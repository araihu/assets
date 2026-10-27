package runneradmission

import (
	"encoding/json/v2"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/araihu/assets/internal/releasefanout"
	"golang.org/x/sys/unix"
)

func TestEnvironmentDocumentation(t *testing.T) {
	output := filepath.Join(t.TempDir(), "environment.md")
	command := exec.Command("go", "tool", "envdoc", "-dir", ".", "-types", "Config", "-output", output, "-format", "markdown")
	command.Env = append(os.Environ(), "GOFILE=config.go", "GOLINE=1")
	if data, err := command.CombinedOutput(); err != nil {
		t.Fatalf("envdoc: %v %s", err, data)
	}
	want, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile("environment.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatal("environment.md stale; run go generate ./internal/runneradmission")
	}
}

func TestConsumerEnrollmentMatchesDistribution(t *testing.T) {
	data, err := os.ReadFile("../../manifests/release-consumers.yaml")
	if err != nil {
		t.Fatal(err)
	}
	m, err := releasefanout.LoadManifest(data)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(m.Repositories, "\n") != strings.Join(consumers, "\n") {
		t.Fatal("runner admission consumer enrollment differs from distribution")
	}
}

func event(t *testing.T, c Config, action string) map[string]any {
	t.Helper()
	return map[string]any{"repository": map[string]any{"full_name": c.Repository}, "sender": map[string]any{"login": c.Actor}, "action": action}
}
func encoded(t *testing.T, p map[string]any) []byte {
	t.Helper()
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func contextConfig() Config {
	return Config{Repository: "araihu/manja", Actor: "guilycst", Event: "push", Ref: "refs/heads/main", EventPath: "event.json"}
}
func metadata() releasefanout.Metadata {
	return releasefanout.Metadata{AssetsRepository: "araihu/assets", AssetsRevision: strings.Repeat("a", 40), Release: "v0.2.5", ReleaseURL: "https://github.com/araihu/assets/releases/download/v0.2.5/araihu-assets-v0.2.5.tar.gz", ReleaseSHA256: strings.Repeat("b", 64), ReleaseJSONSHA256: strings.Repeat("c", 64)}
}

func TestTrustedEvents(t *testing.T) {
	for _, tc := range []struct {
		event, ref, action, repo, actor string
		allow                           bool
	}{
		{"push", "refs/heads/main", "", "araihu/manja", "guilycst", true},
		{"push", "refs/tags/v1.2.3", "", "araihu/manja", "guilycst", true},
		{"push", "refs/heads/feature", "", "araihu/manja", "guilycst", false},
		{"workflow_dispatch", "refs/heads/main", "", "araihu/manja", "guilycst", true},
		{"schedule", "refs/heads/main", "", "araihu/manja", "guilycst", true},
		{"workflow_dispatch", "refs/tags/v1.2.3", "", "araihu/manja", "guilycst", false},
		{"release", "refs/tags/v1.2.3", "published", "araihu/manja", "guilycst", true},
		{"release", "refs/tags/v1.2.3", "created", "araihu/manja", "guilycst", false},
		{"release", "refs/heads/main", "published", "araihu/manja", "guilycst", false},
		{"repository_dispatch", "refs/heads/main", "goshtoso-main", "araihu/assets", "guilycst", false},
		{"repository_dispatch", "refs/heads/main", "goshtoso-charts-main", "araihu/assets", "guilycst", false},
		{"repository_dispatch", "refs/heads/main", "manja-main", "araihu/assets", "guilycst", false},
		{"repository_dispatch", "refs/heads/main", "other", "araihu/assets", "guilycst", false},
		{"repository_dispatch", "refs/heads/main", "manja-main", "araihu/manja", "guilycst", false},
		{"push", "refs/heads/main", "", "foreign/manja", "guilycst", false},
		{"push", "refs/heads/main", "", "araihu/manja", "mallory", false},
		{"push", "refs/heads/main", "", "araihu/manja", "dependabot[bot]", false},
		{"push", "refs/heads/main", "", "araihu/manja", AssetsActor, false},
		{"workflow_dispatch", "refs/heads/main", "", "araihu/manja", AssetsActor, false},
		{"pull_request_target", "refs/heads/main", "", "araihu/manja", "guilycst", false},
	} {
		t.Run(tc.event+tc.ref+tc.action+tc.repo+tc.actor, func(t *testing.T) {
			c := contextConfig()
			c.Event, c.Ref, c.Repository, c.Actor = tc.event, tc.ref, tc.repo, tc.actor
			p := event(t, c, tc.action)
			p["release"] = map[string]any{"tag_name": "v1.2.3", "draft": false}
			if err := Check(c, "trusted", []string{"guilycst", AssetsActor}, encoded(t, p)); (err == nil) != tc.allow {
				t.Fatalf("allow=%v err=%v", tc.allow, err)
			}
		})
	}
}

func TestAssetsDispatchScope(t *testing.T) {
	for _, repo := range consumers {
		c := contextConfig()
		c.Actor, c.Event, c.Repository = AssetsActor, "repository_dispatch", "araihu/"+repo
		p := event(t, c, releasefanout.EventType)
		p["client_payload"] = metadata()
		if err := Check(c, "trusted", nil, encoded(t, p)); err != nil {
			t.Fatal(repo, err)
		}
	}
	for _, tc := range []string{"actor", "sender", "repository", "foreign owner", "ref", "action", "unknown field", "hash", "url", "missing metadata", "pr lane", "unknown lane", "duplicate JSON"} {
		t.Run(tc, func(t *testing.T) {
			c := contextConfig()
			c.Actor, c.Event = AssetsActor, "repository_dispatch"
			p := event(t, c, releasefanout.EventType)
			m := metadata()
			domain := "trusted"
			switch tc {
			case "actor":
				c.Actor = "mallory"
			case "sender":
				p["sender"] = map[string]any{"login": "mallory"}
			case "repository":
				c.Repository = "araihu/assets"
				p["repository"] = map[string]any{"full_name": c.Repository}
			case "foreign owner":
				c.Repository = "foreign/manja"
				p["repository"] = map[string]any{"full_name": c.Repository}
			case "ref":
				c.Ref = "refs/heads/feature"
			case "action":
				p["action"] = "manja-main"
			case "hash":
				m.ReleaseSHA256 = "invalid"
			case "url":
				m.ReleaseURL = "https://evil.invalid/archive"
			case "pr lane":
				domain = "pull-request"
			case "unknown lane":
				domain = "untrusted"
			}
			p["client_payload"] = m
			if tc == "missing metadata" {
				delete(p, "client_payload")
			}
			if tc == "unknown field" {
				b, _ := json.Marshal(m)
				var extra map[string]any
				json.Unmarshal(b, &extra)
				extra["unknown"] = "x"
				p["client_payload"] = extra
			}
			data := encoded(t, p)
			if tc == "duplicate JSON" {
				data = append([]byte(`{"action":"araihu-assets-released",`), data[1:]...)
			}
			if err := Check(c, domain, []string{"guilycst"}, data); err == nil {
				t.Fatal("admitted invalid dispatch")
			}
		})
	}
}

func pullPayload(t *testing.T, c Config, author string) map[string]any {
	p := event(t, c, "opened")
	p["pull_request"] = map[string]any{"draft": false, "user": map[string]any{"login": author}, "base": map[string]any{"ref": "main", "repo": map[string]any{"full_name": c.Repository}}, "head": map[string]any{"ref": "automation/araihu-assets-v0.2.5", "repo": map[string]any{"full_name": c.Repository, "fork": false}}}
	return p
}

func TestPullRequestIsolation(t *testing.T) {
	for _, tc := range []struct {
		actor, author string
		allow         bool
	}{
		{"guilycst", "guilycst", true}, {"dependabot[bot]", "dependabot[bot]", true}, {"guilycst", "dependabot[bot]", true},
		{AssetsActor, AssetsActor, true}, {"guilycst", AssetsActor, true}, {"mallory", AssetsActor, false}, {"mallory", "dependabot[bot]", false}, {"guilycst", "alice", false},
	} {
		t.Run(tc.actor+tc.author, func(t *testing.T) {
			c := contextConfig()
			c.Event, c.Ref, c.Actor = "pull_request", "refs/pull/42/merge", tc.actor
			p := pullPayload(t, c, tc.author)
			err := Check(c, "pull-request", []string{"guilycst"}, encoded(t, p))
			if (err == nil) != tc.allow {
				t.Fatalf("allow=%v err=%v", tc.allow, err)
			}
		})
	}
	for _, tc := range []string{"fork", "draft", "action", "base repo", "head repo", "branch", "base branch", "ref", "repository", "trusted lane", "target event", "sender", "missing fork", "missing draft"} {
		t.Run(tc, func(t *testing.T) {
			c := contextConfig()
			c.Event, c.Ref, c.Actor = "pull_request", "refs/pull/42/merge", AssetsActor
			p := pullPayload(t, c, AssetsActor)
			pr := p["pull_request"].(map[string]any)
			base := pr["base"].(map[string]any)
			head := pr["head"].(map[string]any)
			domain := "pull-request"
			switch tc {
			case "fork":
				head["repo"].(map[string]any)["fork"] = true
			case "draft":
				pr["draft"] = true
			case "missing fork":
				delete(head["repo"].(map[string]any), "fork")
			case "missing draft":
				delete(pr, "draft")
			case "action":
				p["action"] = "closed"
			case "base repo":
				base["repo"].(map[string]any)["full_name"] = "araihu/other"
			case "head repo":
				head["repo"].(map[string]any)["full_name"] = "foreign/fork"
			case "branch":
				head["ref"] = "feature/unrelated"
			case "base branch":
				base["ref"] = "develop"
			case "ref":
				c.Ref = "refs/heads/main"
			case "repository":
				c.Repository = "araihu/assets"
			case "trusted lane":
				domain = "trusted"
			case "target event":
				c.Event = "pull_request_target"
			case "sender":
				p["sender"].(map[string]any)["login"] = "mallory"
			}
			if err := Check(c, domain, []string{"guilycst"}, encoded(t, p)); err == nil {
				t.Fatal("admitted invalid PR")
			}
		})
	}
}

func TestParse(t *testing.T) {
	valid := map[string]string{"GITHUB_REPOSITORY": "araihu/manja", "GITHUB_ACTOR": "guilycst", "GITHUB_EVENT_NAME": "push", "GITHUB_REF": "refs/heads/main", "GITHUB_EVENT_PATH": "event.json"}
	if _, err := Parse(valid); err != nil {
		t.Fatal(err)
	}
	for key := range valid {
		for _, value := range []string{"", " ", "SECRET\n"} {
			t.Run(key+value, func(t *testing.T) {
				copy := map[string]string{}
				for k, v := range valid {
					copy[k] = v
				}
				copy[key] = value
				if _, err := Parse(copy); err == nil {
					t.Fatal("invalid environment admitted")
				}
			})
		}
		t.Run(key+"missing", func(t *testing.T) {
			copy := map[string]string{}
			for k, v := range valid {
				if k != key {
					copy[k] = v
				}
			}
			if _, err := Parse(copy); err == nil {
				t.Fatal("missing environment admitted")
			}
		})
	}
}

func TestAdmissionCoordination(t *testing.T) {
	for _, tc := range []string{"valid", "denied", "stale", "maintenance active", "invalid JSON", "oversized JSON", "missing domain", "missing actors", "missing event"} {
		t.Run(tc, func(t *testing.T) {
			root := t.TempDir()
			paths := Paths{filepath.Join(root, "domain"), filepath.Join(root, "actors"), root}
			c := contextConfig()
			c.EventPath = filepath.Join(root, "event.json")
			for path, data := range map[string][]byte{paths.Domain: []byte("trusted\n"), paths.Actors: []byte("guilycst\n"), c.EventPath: encoded(t, event(t, c, ""))} {
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			want := tc == "valid" || tc == "stale"
			switch tc {
			case "denied":
				c.Actor = "mallory"
			case "stale", "maintenance active":
				os.WriteFile(filepath.Join(root, ".maintenance"), nil, 0600)
				if tc == "maintenance active" {
					f, err := lock(filepath.Join(root, ".infrastructure-maintenance.lock"), unix.LOCK_EX)
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { f.Close() })
				}
			case "invalid JSON":
				os.WriteFile(c.EventPath, []byte("SECRET_PAYLOAD"), 0600)
			case "oversized JSON":
				os.WriteFile(c.EventPath, make([]byte, 16<<20+1), 0600)
			case "missing domain":
				os.Remove(paths.Domain)
			case "missing actors":
				os.Remove(paths.Actors)
			case "missing event":
				os.Remove(c.EventPath)
			}
			err := Admit(c, paths)
			if (err == nil) != want {
				t.Fatalf("allow=%v err=%v", want, err)
			}
			_, markerErr := os.Stat(filepath.Join(root, ".job-active"))
			if (markerErr == nil) != want {
				t.Fatal("job marker disagrees with admission")
			}
			if err != nil && strings.Contains(err.Error(), "SECRET") {
				t.Fatal("payload leaked")
			}
			if tc == "stale" {
				if _, err := os.Stat(filepath.Join(root, ".maintenance")); !os.IsNotExist(err) {
					t.Fatal("stale marker retained")
				}
			}
		})
	}
}
