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
	want := []byte("<svg xmlns=\"http://www.w3.org/2000/svg\" style=\"display:none\"><symbol id=\"a-first\" viewBox=\"0 0 16 16\"><path d=\"M1 1h1v1z\"></path></symbol><symbol id=\"z-last\" viewBox=\"0 0 16 16\"><path d=\"M9 9h1v1z\"></path></symbol></svg>\n")
	if !bytes.Equal(got, want) {
		t.Fatalf("Build() = %s\nwant %s", got, want)
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
