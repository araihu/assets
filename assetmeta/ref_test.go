package assetmeta

import (
	"encoding"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

var (
	_ encoding.TextMarshaler   = Ref{}
	_ encoding.TextUnmarshaler = (*Ref)(nil)
)

func TestParseRefAcceptsResourceDownload(t *testing.T) {
	ref, err := ParseRef("alpinejs/core-js")
	if err != nil || ref != (Ref{Resource: "alpinejs", Download: "core-js"}) {
		t.Fatalf("ParseRef() = %#v, %v", ref, err)
	}
	if got := ref.String(); got != "alpinejs/core-js" {
		t.Fatalf("Ref.String() = %q", got)
	}
}

func TestParseRefRejectsInvalidSyntax(t *testing.T) {
	for _, value := range []string{"", "alpinejs", "/core-js", "alpinejs/", "alpinejs/core/js"} {
		t.Run(value, func(t *testing.T) {
			if _, err := ParseRef(value); err == nil {
				t.Fatalf("ParseRef(%q) returned nil error", value)
			}
		})
	}
}

func TestRefTextAndYAMLRoundTrips(t *testing.T) {
	want := Ref{Resource: "alpinejs", Download: "core-js"}
	text, err := want.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	if string(text) != "alpinejs/core-js" {
		t.Fatalf("MarshalText() = %q", text)
	}

	var fromText Ref
	if err := fromText.UnmarshalText(text); err != nil || fromText != want {
		t.Fatalf("UnmarshalText() = %#v, %v", fromText, err)
	}

	type document struct {
		Entry Ref `yaml:"entry"`
	}
	encoded, err := yaml.Marshal(document{Entry: want})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(encoded); got != "entry: alpinejs/core-js\n" {
		t.Fatalf("yaml.Marshal() = %q", got)
	}
	var decoded document
	if err := yaml.Unmarshal(encoded, &decoded); err != nil || decoded.Entry != want {
		t.Fatalf("yaml.Unmarshal() = %#v, %v", decoded, err)
	}
}

func TestRefMarshalingRejectsInvalidConstructedValue(t *testing.T) {
	for _, ref := range []Ref{
		{},
		{Resource: "alpinejs"},
		{Resource: "alpine/js", Download: "core"},
	} {
		if _, err := ref.MarshalText(); err == nil {
			t.Fatalf("Ref{%q, %q}.MarshalText() returned nil error", ref.Resource, ref.Download)
		}
	}

	ref := Ref{Resource: "retained", Download: "value"}
	if err := ref.UnmarshalText([]byte("invalid")); err == nil {
		t.Fatal("UnmarshalText(invalid) returned nil error")
	}
	if ref != (Ref{Resource: "retained", Download: "value"}) {
		t.Fatalf("failed UnmarshalText mutated receiver: %#v", ref)
	}
}

func TestInventoryResolveReturnsCallerOwnedCopy(t *testing.T) {
	inventory, err := NewInventory(fixtureResources())
	if err != nil {
		t.Fatal(err)
	}
	ref := Ref{Resource: "alpinejs", Download: "core-js"}
	resolved, ok := inventory.Resolve(ref)
	if !ok {
		t.Fatal("Resolve(valid) = false")
	}
	want := Resolved{Resource: fixtureResources()[0], Download: fixtureResources()[0].Downloads[0]}
	if !reflect.DeepEqual(resolved, want) {
		t.Fatalf("Resolve(valid) = %#v, want %#v", resolved, want)
	}
	resolved.Resource.Downloads[0].Hash = "changed"
	again, _ := inventory.Resolve(ref)
	if again.Resource.Downloads[0].Hash != "sha384:0123456789abcdef" {
		t.Fatalf("resolved mutation reached inventory: %#v", again)
	}
	if _, ok := inventory.Resolve(Ref{Resource: "alpinejs", Download: "missing"}); ok {
		t.Fatal("Resolve(unknown download) = true")
	}
	if _, ok := (*Inventory)(nil).Resolve(ref); ok {
		t.Fatal("nil Inventory.Resolve() = true")
	}
}

func TestValidateRefsReportsSortedUniqueUnknownReferences(t *testing.T) {
	inventory, err := NewInventory(fixtureResources())
	if err != nil {
		t.Fatal(err)
	}
	err = ValidateRefs(inventory,
		Ref{Resource: "missing", Download: "z"},
		Ref{Resource: "alpinejs", Download: "core-js"},
		Ref{Resource: "missing", Download: "a"},
		Ref{Resource: "missing", Download: "z"},
	)
	if err == nil || err.Error() != "unknown references: missing/a, missing/z" {
		t.Fatalf("ValidateRefs() error = %v", err)
	}
}

func TestValidateRefsRejectsNilInventoryAndInvalidSyntax(t *testing.T) {
	if err := ValidateRefs(nil, Ref{Resource: "alpinejs", Download: "core-js"}); err == nil || err.Error() != "inventory is nil" {
		t.Fatalf("ValidateRefs(nil) error = %v", err)
	}
	inventory, err := NewInventory(fixtureResources())
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateRefs(inventory, Ref{Resource: "alpinejs"}); err == nil {
		t.Fatal("ValidateRefs(invalid Ref) returned nil error")
	}
}
