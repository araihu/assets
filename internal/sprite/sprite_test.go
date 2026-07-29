package sprite

import (
	"bytes"
	"strings"
	"testing"
)

func TestBuildSortsSymbolsAndProducesStableSprite(t *testing.T) {
	z := []byte(`<svg viewBox="0 0 16 16"><path d="M9 9h1v1z"/></svg>`)
	a := []byte(`<svg viewBox="0 0 16 16"><path d="M1 1h1v1z"/></svg>`)

	got, err := Build([]Entry{{Symbol: "z-last", SVG: z}, {Symbol: "a-first", SVG: a}})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	want := []byte("<svg xmlns=\"http://www.w3.org/2000/svg\"><symbol id=\"a-first\" viewBox=\"0 0 16 16\"><path d=\"M1 1h1v1z\"></path></symbol><symbol id=\"z-last\" viewBox=\"0 0 16 16\"><path d=\"M9 9h1v1z\"></path></symbol></svg>\n")
	if !bytes.Equal(got, want) {
		t.Fatalf("Build() = %s\nwant %s", got, want)
	}
	if err := Validate(got); err != nil {
		t.Fatalf("Validate(Build()) = %v", err)
	}
}

func TestValidateRejectsUnsafeFullSpriteContracts(t *testing.T) {
	for _, tc := range []struct {
		name string
		svg  []byte
		want string
	}{
		{
			name: "root style",
			svg:  []byte(`<svg xmlns="http://www.w3.org/2000/svg" style="display:none"><symbol id="safe" viewBox="0 0 16 16"><path d="M0 0h1v1z"/></symbol></svg>` + "\n"),
			want: "fixed generated form",
		},
		{
			name: "duplicate ids",
			svg:  []byte(`<svg xmlns="http://www.w3.org/2000/svg"><symbol id="one" viewBox="0 0 16 16"><path id="shared" d="M0 0h1v1z"/></symbol><symbol id="two" viewBox="0 0 16 16"><path id="shared" d="M1 1h1v1z"/></symbol></svg>` + "\n"),
			want: "duplicate ID",
		},
		{
			name: "dangling href",
			svg:  []byte(`<svg xmlns="http://www.w3.org/2000/svg"><symbol id="one" viewBox="0 0 16 16"><use href="#missing"/></symbol></svg>` + "\n"),
			want: "has no target",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := Validate(tc.svg); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate() error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestBuildRejectsDuplicateUnsafeAndDocumentLevelSymbols(t *testing.T) {
	safe := []byte(`<svg viewBox="0 0 16 16"><path d="M0 0h1v1z"/></svg>`)
	withTitle := []byte(`<svg viewBox="0 0 16 16"><title>x</title><path d="M0 0h1v1z"/></svg>`)
	for _, tc := range []struct {
		name    string
		entries []Entry
		want    string
	}{
		{"duplicate symbol", []Entry{{Symbol: "same", SVG: safe}, {Symbol: "same", SVG: safe}}, "duplicate symbol"},
		{"invalid symbol", []Entry{{Symbol: `bad\" id=\"owned`, SVG: safe}}, "invalid symbol"},
		{"unsafe SVG", []Entry{{Symbol: "safe", SVG: []byte(`<svg viewBox="0 0 16 16"><script/></svg>`)}}, "unsafe SVG"},
		{"CSS-escaped external URL", []Entry{{Symbol: "safe", SVG: []byte(`<svg viewBox="0 0 16 16"><path fill="\75rl(https://evil.invalid/a.svg)" d="M0 0h1v1z"/></svg>`)}}, "unsafe SVG"},
		{"document level child", []Entry{{Symbol: "safe", SVG: withTitle}}, "unsafe SVG"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Build(tc.entries)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Build() error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestBuildRejectsIDsDuplicatedAcrossWholeSprite(t *testing.T) {
	withSharedID := func(id string) []byte {
		return []byte(`<svg viewBox="0 0 16 16"><path id="` + id + `" d="M0 0h1v1z"/></svg>`)
	}
	for _, tc := range []struct {
		name    string
		entries []Entry
	}{
		{
			"same child ID in two source documents",
			[]Entry{{Symbol: "first", SVG: withSharedID("shared")}, {Symbol: "second", SVG: withSharedID("shared")}},
		},
		{
			"child ID collides with symbol ID",
			[]Entry{{Symbol: "symbol-id", SVG: withSharedID("symbol-id")}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Build(tc.entries); err == nil || !strings.Contains(err.Error(), "duplicate ID") {
				t.Fatalf("Build() error = %v, want duplicate ID", err)
			}
		})
	}
}
