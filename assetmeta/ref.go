package assetmeta

import (
	"fmt"
	"sort"
	"strings"
)

// Ref identifies one inventory download using resource/download syntax.
type Ref struct {
	Resource string
	Download string
}

// ParseRef parses an exact resource/download reference.
func ParseRef(value string) (Ref, error) {
	if strings.Count(value, "/") != 1 {
		return Ref{}, fmt.Errorf("invalid reference %q: want resource/download", value)
	}
	resource, download, _ := strings.Cut(value, "/")
	if resource == "" || download == "" {
		return Ref{}, fmt.Errorf("invalid reference %q: want resource/download", value)
	}
	return Ref{Resource: resource, Download: download}, nil
}

// String returns resource/download syntax.
func (r Ref) String() string {
	return r.Resource + "/" + r.Download
}

// MarshalText validates and encodes a reference.
func (r Ref) MarshalText() ([]byte, error) {
	value := r.String()
	if _, err := ParseRef(value); err != nil {
		return nil, err
	}
	return []byte(value), nil
}

// UnmarshalText parses text without mutating the receiver on failure.
func (r *Ref) UnmarshalText(text []byte) error {
	if r == nil {
		return fmt.Errorf("unmarshal reference: nil receiver")
	}
	parsed, err := ParseRef(string(text))
	if err != nil {
		return err
	}
	*r = parsed
	return nil
}

// ValidateRefs validates syntax and reports every unknown reference in stable
// lexical order.
func ValidateRefs(inventory *Inventory, refs ...Ref) error {
	if inventory == nil {
		return fmt.Errorf("inventory is nil")
	}
	unknown := make(map[string]struct{})
	for _, ref := range refs {
		value := ref.String()
		if _, err := ParseRef(value); err != nil {
			return err
		}
		if _, ok := inventory.Resolve(ref); !ok {
			unknown[value] = struct{}{}
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	values := make([]string, 0, len(unknown))
	for value := range unknown {
		values = append(values, value)
	}
	sort.Strings(values)
	return fmt.Errorf("unknown references: %s", strings.Join(values, ", "))
}
