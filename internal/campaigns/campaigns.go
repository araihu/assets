// Package campaigns defines the strict date-only seasonal campaign calendar.
package campaigns

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	schemaVersion = 1
	dateLayout    = "2006-01-02"
)

var lowerKebab = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)

// Date is one UTC calendar date with no time-of-day or timezone component.
type Date struct{ time.Time }

// ParseDate parses a canonical YYYY-MM-DD UTC calendar date.
func ParseDate(raw string) (Date, error) {
	parsed, err := time.Parse(dateLayout, raw)
	if err != nil || parsed.Format(dateLayout) != raw {
		return Date{}, fmt.Errorf("campaign date %q must use YYYY-MM-DD", raw)
	}
	return Date{Time: parsed.UTC()}, nil
}

// UnmarshalYAML accepts only a scalar canonical calendar date.
func (d *Date) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.ScalarNode {
		return errors.New("campaign date must use YYYY-MM-DD")
	}
	parsed, err := ParseDate(value.Value)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

// MarshalJSON serializes a Date as its canonical YYYY-MM-DD string.
func (d Date) MarshalJSON() ([]byte, error) {
	if err := validateDate(d); err != nil {
		return nil, err
	}
	return json.Marshal(d.String())
}

// UnmarshalJSON accepts only one canonical YYYY-MM-DD calendar date.
func (d *Date) UnmarshalJSON(data []byte) error {
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return errors.New("campaign date must use YYYY-MM-DD")
	}
	parsed, err := ParseDate(raw)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

// String returns the canonical YYYY-MM-DD representation.
func (d Date) String() string { return d.Format(dateLayout) }

// Manifest is the source campaigns.yaml contract.
type Manifest struct {
	SchemaVersion int        `yaml:"schema_version" json:"schemaVersion"`
	Campaigns     []Campaign `yaml:"campaigns" json:"campaigns"`
}

// IconRef identifies a catalog asset and how the runtime renders it.
type IconRef struct {
	Asset string `yaml:"asset" json:"asset"`
	Mode  string `yaml:"mode" json:"mode"`
}

// Toggle defines the icon rendered before and after opting out.
type Toggle struct {
	EnabledIcon  IconRef `yaml:"enabled_icon" json:"enabledIcon"`
	DisabledIcon IconRef `yaml:"disabled_icon" json:"disabledIcon"`
}

// Brand identifies catalog assets used for a campaign brand substitution.
type Brand struct {
	Logo string `yaml:"logo" json:"logo"`
	Icon string `yaml:"icon" json:"icon"`
}

// Campaign defines one bounded optional presentation.
type Campaign struct {
	ID       string `yaml:"id" json:"id"`
	Enabled  bool   `yaml:"enabled" json:"enabled"`
	StartsOn Date   `yaml:"starts_on" json:"startsOn"`
	EndsOn   Date   `yaml:"ends_on" json:"endsOn"`
	Theme    string `yaml:"theme" json:"theme"`
	Toggle   Toggle `yaml:"toggle" json:"toggle"`
	Brand    Brand  `yaml:"brand" json:"brand"`
}

// Load reads, strictly decodes, and validates one campaigns manifest.
func Load(fsys fs.FS, name string) (Manifest, error) {
	if !fs.ValidPath(name) {
		return Manifest{}, fmt.Errorf("campaign manifest path %q is invalid", name)
	}
	file, err := fsys.Open(name)
	if err != nil {
		return Manifest{}, fmt.Errorf("open campaigns manifest: %w", err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode campaigns manifest: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return Manifest{}, errors.New("decode campaigns manifest: multiple YAML documents")
		}
		return Manifest{}, fmt.Errorf("decode campaigns manifest: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

// Validate checks the closed schema and all campaign records, including disabled ones.
func (m Manifest) Validate() error {
	if m.SchemaVersion != schemaVersion {
		return fmt.Errorf("unsupported schema_version %d", m.SchemaVersion)
	}
	seen := make(map[string]struct{}, len(m.Campaigns))
	for i, campaign := range m.Campaigns {
		if err := validateCampaign(campaign); err != nil {
			return fmt.Errorf("campaign[%d] %q: %w", i, campaign.ID, err)
		}
		if _, exists := seen[campaign.ID]; exists {
			return fmt.Errorf("campaign[%d]: duplicate id %q", i, campaign.ID)
		}
		seen[campaign.ID] = struct{}{}
	}
	for i, campaign := range m.Campaigns {
		if !campaign.Enabled {
			continue
		}
		for j := i + 1; j < len(m.Campaigns); j++ {
			other := m.Campaigns[j]
			if other.Enabled && rangesOverlap(campaign, other) {
				return fmt.Errorf("campaigns %q and %q overlap", campaign.ID, other.ID)
			}
		}
	}
	return nil
}

// Active returns the one enabled campaign containing date, if any.
func (m Manifest) Active(date Date) (*Campaign, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	if err := validateDate(date); err != nil {
		return nil, err
	}
	for i := range m.Campaigns {
		campaign := &m.Campaigns[i]
		if campaign.Enabled && !date.Before(campaign.StartsOn.Time) && !date.After(campaign.EndsOn.Time) {
			return campaign, nil
		}
	}
	return nil, nil
}

func validateCampaign(c Campaign) error {
	if !validCatalogName(c.ID) {
		return fmt.Errorf("invalid id %q", c.ID)
	}
	if err := validateDate(c.StartsOn); err != nil {
		return fmt.Errorf("invalid starts_on: %w", err)
	}
	if err := validateDate(c.EndsOn); err != nil {
		return fmt.Errorf("invalid ends_on: %w", err)
	}
	if c.EndsOn.Before(c.StartsOn.Time) {
		return errors.New("ends_on is before starts_on")
	}
	if !validCatalogName(c.Theme) {
		return fmt.Errorf("invalid theme %q", c.Theme)
	}
	if err := validateIconRef("enabled_icon", c.Toggle.EnabledIcon); err != nil {
		return err
	}
	if err := validateIconRef("disabled_icon", c.Toggle.DisabledIcon); err != nil {
		return err
	}
	if !validCatalogName(c.Brand.Logo) {
		return fmt.Errorf("invalid brand logo %q", c.Brand.Logo)
	}
	if !validCatalogName(c.Brand.Icon) {
		return fmt.Errorf("invalid brand icon %q", c.Brand.Icon)
	}
	return nil
}

func validateDate(date Date) error {
	if date.Time.IsZero() || date.Location() != time.UTC || date.Hour() != 0 || date.Minute() != 0 || date.Second() != 0 || date.Nanosecond() != 0 {
		return errors.New("campaign date must use YYYY-MM-DD UTC")
	}
	return nil
}

func validateIconRef(name string, icon IconRef) error {
	if !validCatalogName(icon.Asset) {
		return fmt.Errorf("invalid %s asset %q", name, icon.Asset)
	}
	if icon.Mode != "asset" && icon.Mode != "sprite" {
		return fmt.Errorf("invalid %s mode %q", name, icon.Mode)
	}
	return nil
}

func validCatalogName(value string) bool {
	return lowerKebab.MatchString(value) && !strings.ContainsAny(value, "\r\n\t")
}

func rangesOverlap(a, b Campaign) bool {
	return !a.EndsOn.Before(b.StartsOn.Time) && !b.EndsOn.Before(a.StartsOn.Time)
}
