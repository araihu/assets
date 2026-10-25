package runneradmission

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"regexp"
	"slices"
	"strings"

	"github.com/araihu/assets/internal/releasefanout"
)

const AssetsActor = "arai-hu-assets-distribution[bot]"

var consumers = []string{"goshtoso", "goshtoso-app-shells", "goshtoso-charts", "manja", "xisnove"}
var stableRelease = regexp.MustCompile(`^v(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)$`)
var pullRef = regexp.MustCompile(`^refs/pull/[1-9][0-9]*/merge$`)

type repository struct {
	FullName string `json:"full_name"`
	Fork     *bool  `json:"fork"`
}

type eventPayload struct {
	Action     string     `json:"action"`
	Repository repository `json:"repository"`
	Sender     struct {
		Login string `json:"login"`
	} `json:"sender"`
	ClientPayload jsontext.Value `json:"client_payload"`
	Release       struct {
		Draft *bool  `json:"draft"`
		Tag   string `json:"tag_name"`
	} `json:"release"`
	PR struct {
		Draft *bool `json:"draft"`
		User  struct {
			Login string `json:"login"`
		} `json:"user"`
		Base struct {
			Ref  string     `json:"ref"`
			Repo repository `json:"repo"`
		} `json:"base"`
		Head struct {
			Ref  string     `json:"ref"`
			Repo repository `json:"repo"`
		} `json:"head"`
	} `json:"pull_request"`
}

func falseValue(v *bool) bool { return v != nil && !*v }
func isConsumer(repo string) bool {
	return slices.Contains(consumers, strings.TrimPrefix(repo, "araihu/")) && strings.HasPrefix(repo, "araihu/")
}

// Check validates policy without filesystem or network side effects. Provider
// payloads may contain unrelated fields, but duplicate JSON names are rejected.
func Check(c Config, domain string, actors []string, data []byte) error {
	if !strings.HasPrefix(c.Repository, "araihu/") || c.Actor == "" || c.Ref == "" || c.Event == "" {
		return errors.New("runner context")
	}
	var p eventPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return errors.New("invalid event payload")
	}
	if p.Repository.FullName != c.Repository || p.Sender.Login != "" && p.Sender.Login != c.Actor {
		return errors.New("payload mismatch")
	}
	human := slices.Contains(actors, c.Actor) && c.Actor != AssetsActor
	if domain == "trusted" {
		return trusted(c, human, p)
	}
	if domain != "pull-request" || c.Event != "pull_request" {
		return errors.New("trust domain event")
	}
	if !slices.Contains([]string{"opened", "synchronize", "reopened", "ready_for_review"}, p.Action) ||
		!falseValue(p.PR.Draft) || p.PR.Base.Repo.FullName != c.Repository || p.PR.Head.Repo.FullName != c.Repository || !falseValue(p.PR.Head.Repo.Fork) {
		return errors.New("pull request metadata")
	}
	if p.PR.User.Login == AssetsActor {
		if (!human && c.Actor != AssetsActor) || !isConsumer(c.Repository) || !pullRef.MatchString(c.Ref) || p.PR.Base.Ref != "main" ||
			!strings.HasPrefix(p.PR.Head.Ref, "automation/araihu-assets-") ||
			!stableTag(strings.TrimPrefix(p.PR.Head.Ref, "automation/araihu-assets-")) || p.Sender.Login != c.Actor {
			return errors.New("assets pull request metadata")
		}
		return nil
	}
	if (!human && c.Actor != "dependabot[bot]") || (p.PR.User.Login != c.Actor && !(p.PR.User.Login == "dependabot[bot]" && human)) {
		return errors.New("actor")
	}
	return nil
}

func stableTag(tag string) bool {
	return stableRelease.MatchString(tag)
}

func trusted(c Config, human bool, p eventPayload) error {
	if c.Event == "repository_dispatch" && c.Actor == AssetsActor {
		var m releasefanout.Metadata
		if c.Ref != "refs/heads/main" || !isConsumer(c.Repository) || p.Action != releasefanout.EventType || p.Sender.Login != AssetsActor ||
			json.Unmarshal(p.ClientPayload, &m, json.RejectUnknownMembers(true)) != nil || m.Validate() != nil || !stableTag(m.Release) {
			return errors.New("assets dispatch metadata")
		}
		return nil
	}
	if !human {
		return errors.New("actor")
	}
	switch c.Event {
	case "push":
		if c.Ref == "refs/heads/main" || strings.HasPrefix(c.Ref, "refs/tags/") {
			return nil
		}
	case "workflow_dispatch", "schedule":
		if c.Ref == "refs/heads/main" {
			return nil
		}
	case "release":
		if strings.HasPrefix(c.Ref, "refs/tags/") && p.Action == "published" && falseValue(p.Release.Draft) && p.Release.Tag == strings.TrimPrefix(c.Ref, "refs/tags/") {
			return nil
		}
	case "repository_dispatch":
		if c.Ref == "refs/heads/main" && c.Repository == "araihu/fly-deploy" && slices.Contains([]string{"goshtoso-main", "goshtoso-charts-main", "manja-main"}, p.Action) {
			return nil
		}
	}
	return errors.New("trusted event metadata")
}
