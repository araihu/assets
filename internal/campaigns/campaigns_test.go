package campaigns

import (
	"os"
	"strings"
	"testing"
	"testing/fstest"
)

func TestActiveUsesInclusiveUTCDateRange(t *testing.T) {
	m := Manifest{SchemaVersion: 1, Campaigns: []Campaign{{
		ID: "halloween-2026", Enabled: true,
		StartsOn: mustDate(t, "2026-10-01"), EndsOn: mustDate(t, "2026-10-31"),
		Theme:  "araihu-halloween",
		Toggle: validToggle(), Brand: validBrand(),
	}}}
	for _, raw := range []string{"2026-10-01", "2026-10-31"} {
		active, err := m.Active(mustDate(t, raw))
		if err != nil || active == nil || active.ID != "halloween-2026" {
			t.Fatalf("Active(%s) = %#v, %v", raw, active, err)
		}
	}
	for _, raw := range []string{"2026-09-30", "2026-11-01"} {
		active, err := m.Active(mustDate(t, raw))
		if err != nil || active != nil {
			t.Fatalf("Active(%s) = %#v, %v, want nil, nil", raw, active, err)
		}
	}
}

func TestValidateRejectsEnabledOverlap(t *testing.T) {
	m := fixtureManifest("2026-10-01", "2026-10-31", "2026-10-31", "2026-11-02")
	if err := m.Validate(); err == nil || !strings.Contains(err.Error(), "overlap") {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestParseDateRejectsTimeAndNonCanonicalDate(t *testing.T) {
	for _, raw := range []string{"2026-08-01T00:00:00Z", "2026-08-01+03:00", "2026-8-01", "2026-02-30"} {
		if _, err := ParseDate(raw); err == nil || !strings.Contains(err.Error(), "YYYY-MM-DD") {
			t.Fatalf("ParseDate(%q) error = %v", raw, err)
		}
	}
	date, err := ParseDate("2026-08-01")
	if err != nil {
		t.Fatalf("ParseDate() error = %v", err)
	}
	if got, want := date.String(), "2026-08-01"; got != want || date.Location().String() != "UTC" {
		t.Fatalf("ParseDate() = %q in %s, want %q UTC", got, date.Location(), want)
	}
}

func TestLoadRejectsUnknownFieldAndMultipleDocuments(t *testing.T) {
	for _, raw := range []string{
		"schema_version: 1\ncampaigns: []\nextra: true\n",
		"schema_version: 1\ncampaigns: []\n---\nschema_version: 1\ncampaigns: []\n",
	} {
		_, err := Load(fstest.MapFS{"campaigns.yaml": {Data: []byte(raw)}}, "campaigns.yaml")
		if err == nil {
			t.Fatal("Load() error = nil")
		}
	}
}

func TestValidateFullyValidatesDisabledCampaigns(t *testing.T) {
	m := Manifest{SchemaVersion: 1, Campaigns: []Campaign{{
		ID: "disabled-proof", Enabled: false,
		StartsOn: mustDate(t, "2026-08-01"), EndsOn: mustDate(t, "2026-08-02"),
		Theme:  "",
		Toggle: validToggle(), Brand: validBrand(),
	}}}
	if err := m.Validate(); err == nil || !strings.Contains(err.Error(), "theme") {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRejectsBadCampaignFields(t *testing.T) {
	for _, mutate := range []func(*Campaign){
		func(c *Campaign) { c.ID = "first-campaign" },
		func(c *Campaign) { c.StartsOn, c.EndsOn = c.EndsOn, c.StartsOn },
		func(c *Campaign) { c.Toggle.EnabledIcon.Mode = "url" },
		func(c *Campaign) { c.Toggle.DisabledIcon.Asset = "bad\nasset" },
		func(c *Campaign) { c.Brand.Logo = "" },
	} {
		m := fixtureManifest("2026-08-01", "2026-08-02", "2026-08-03", "2026-08-04")
		mutate(&m.Campaigns[1])
		if err := m.Validate(); err == nil {
			t.Fatal("Validate() error = nil")
		}
	}
}

func TestLoadInitialCalendar(t *testing.T) {
	m, err := Load(os.DirFS("../.."), "manifests/campaigns.yaml")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got, want := len(m.Campaigns), 1; got != want || m.Campaigns[0].Enabled {
		t.Fatalf("campaigns = %#v, want one disabled campaign", m.Campaigns)
	}
}

func fixtureManifest(firstStart, firstEnd, secondStart, secondEnd string) Manifest {
	return Manifest{SchemaVersion: 1, Campaigns: []Campaign{
		{ID: "first-campaign", Enabled: true, StartsOn: mustDate(nil, firstStart), EndsOn: mustDate(nil, firstEnd), Theme: "first-theme", Toggle: validToggle(), Brand: validBrand()},
		{ID: "second-campaign", Enabled: true, StartsOn: mustDate(nil, secondStart), EndsOn: mustDate(nil, secondEnd), Theme: "second-theme", Toggle: validToggle(), Brand: validBrand()},
	}}
}

func validToggle() Toggle {
	return Toggle{
		EnabledIcon:  IconRef{Asset: "ui-hi-16-solid-sparkles", Mode: "sprite"},
		DisabledIcon: IconRef{Asset: "ui-hi-16-solid-moon", Mode: "asset"},
	}
}

func validBrand() Brand {
	return Brand{Logo: "araihu-logo-tinted-transparent-optical", Icon: "araihu-icon-tinted-transparent-optical"}
}

func mustDate(t *testing.T, raw string) Date {
	if t != nil {
		t.Helper()
	}
	date, err := ParseDate(raw)
	if err != nil {
		if t != nil {
			t.Fatalf("ParseDate(%q) error = %v", raw, err)
		}
		panic(err)
	}
	return date
}
