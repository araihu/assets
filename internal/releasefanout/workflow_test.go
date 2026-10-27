package releasefanout

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestWorkflowUsesGoToolAndScopedToken(t *testing.T) {
	data, err := os.ReadFile("../../.github/workflows/release-fanout.yml")
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct {
			Steps []struct {
				ID   string            `yaml:"id"`
				Run  string            `yaml:"run"`
				Uses string            `yaml:"uses"`
				With map[string]string `yaml:"with"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		t.Fatal(err)
	}
	plan, outputs, token, dispatch := -1, -1, -1, -1
	installer := false
	for i, step := range workflow.Jobs["dispatch"].Steps {
		if strings.HasPrefix(step.Uses, "dagger/dagger-for-github@") && step.With["version"] == "0.21.11" {
			installer = true
		}
		if strings.Contains(step.Run, "dagger call release-fanout-plan") {
			plan = i
		}
		if step.ID == "consumers" {
			outputs = i
			if !strings.Contains(step.Run, "/releasefanout\" outputs") || !strings.Contains(step.Run, "--github-output=\"$GITHUB_OUTPUT\"") {
				t.Fatal("selection is not adapted by Go tool")
			}
		}
		if step.ID == "app-token" {
			token = i
			if step.With["repositories"] != "${{ steps.consumers.outputs.repositories }}" || step.With["permission-contents"] != "write" || step.With["owner"] != "araihu" {
				t.Fatal("App token scope changed")
			}
		}
		if strings.Contains(step.Run, "dagger call release-fanout ") {
			dispatch = i
		}
		for _, forbidden := range []string{"scripts/", "set -", "echo ", "cat ", "awk ", "ruby ", "python ", "jq "} {
			if strings.Contains(step.Run, forbidden) {
				t.Fatalf("fan-out invokes shell tooling: %s", forbidden)
			}
		}
	}
	if !installer || plan < 0 || outputs <= plan || token <= outputs || dispatch <= token {
		t.Fatal("verify/output/scoped-token/dispatch order differs")
	}
	for _, path := range []string{"scripts/release-consumer-fanout.rb", "scripts/release-consumer-fanout_test.sh", "scripts/dagger/release-fanout-plan.sh", "scripts/dagger/release-fanout.sh"} {
		if _, err := os.Stat("../../" + path); !os.IsNotExist(err) {
			t.Fatalf("obsolete fan-out script retained: %s", path)
		}
	}
}
