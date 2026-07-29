package svgasset

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestParseRejectsUnsafeOrAmbiguousSVG(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"script", `<svg viewBox="0 0 16 16"><script>alert(1)</script></svg>`},
		{"case variant script", `<svg viewBox="0 0 16 16"><SCRIPT>alert(1)</SCRIPT></svg>`},
		{"event attribute", `<svg viewBox="0 0 16 16" onLoad="x()"/>`},
		{"external href", `<svg viewBox="0 0 16 16"><use href="https://evil.invalid/x"/></svg>`},
		{"data href", `<svg viewBox="0 0 16 16"><use href="data:text/plain,x"/></svg>`},
		{"unsafe CSS URL", `<svg viewBox="0 0 16 16"><path style="fill:url(https://evil.invalid/x)" d="M0 0h1v1z"/></svg>`},
		{"image", `<svg viewBox="0 0 16 16"><image href="data:image/png;base64,x"/></svg>`},
		{"font", `<svg viewBox="0 0 16 16"><font/></svg>`},
		{"style", `<svg viewBox="0 0 16 16"><style>@import url(https://evil.invalid/x)</style></svg>`},
		{"DTD", `<!DOCTYPE svg [<!ENTITY x "y">]><svg viewBox="0 0 16 16"><path d="M0 0"/></svg>`},
		{"entity reference", `<svg viewBox="0 0 16 16"><path d="M0&#32;0h1v1z"/></svg>`},
		{"processing instruction", `<?xml-stylesheet href="https://evil.invalid/x"?><svg viewBox="0 0 16 16"><path d="M0 0"/></svg>`},
		{"duplicate IDs", `<svg viewBox="0 0 16 16"><path id="a" d="M0 0"/><path id="a" d="M1 1"/></svg>`},
		{"editor metadata", `<svg viewBox="0 0 16 16"><metadata>editor</metadata></svg>`},
		{"document content", `<svg viewBox="0 0 16 16"><title>title</title><path d="M0 0"/></svg>`},
		{"fixed width", `<svg width="16" viewBox="0 0 16 16"><path d="M0 0"/></svg>`},
		{"fixed height", `<svg height="16" viewBox="0 0 16 16"><path d="M0 0"/></svg>`},
		{"missing viewBox", `<svg><path d="M0 0"/></svg>`},
		{"zero viewBox", `<svg viewBox="0 0 0 16"><path d="M0 0"/></svg>`},
		{"case variant viewBox", `<svg VIEWBOX="0 0 16 16"><path d="M0 0"/></svg>`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Parse([]byte(tc.input)); err == nil {
				t.Fatalf("Parse() accepted unsafe SVG: %s", tc.input)
			}
		})
	}
}

func TestNormalizePreservesGeometryAndEmitsInheritablePaint(t *testing.T) {
	doc := fixture(t, "safe.svg")
	before := doc.GeometrySignature()

	out, err := doc.Normalize(Options{ColorBehavior: "monochrome"})
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	parsed, err := Parse(out)
	if err != nil {
		t.Fatalf("Parse(normalized) error = %v", err)
	}
	if got := parsed.GeometrySignature(); !bytes.Equal(got, before) {
		t.Fatalf("geometry changed:\n got %q\nwant %q", got, before)
	}
	if !bytes.Contains(out, []byte(`<svg viewBox="0 0 16 16" fill="currentColor" stroke="currentColor">`)) {
		t.Fatalf("normalized SVG missing inheritable paint: %s", out)
	}
	if !bytes.Contains(out, []byte(`d="M1.25 2.50C3 4 5.000 6 7 8Z"`)) {
		t.Fatalf("normalized SVG changed path data: %s", out)
	}
	if bytes.Contains(out, []byte(`fill="#123456"`)) || bytes.Contains(out, []byte(`stroke="#abcdef"`)) {
		t.Fatalf("normalized SVG retained non-inheritable paint: %s", out)
	}
}

func TestParseAcceptsSafeSVGGradient(t *testing.T) {
	input := []byte(`<svg viewBox="0 0 16 16"><defs><linearGradient id="gradient" gradientUnits="userSpaceOnUse"><stop offset="0" stop-color="#123456"/></linearGradient></defs><path d="M0 0h16v16z" fill="url(#gradient)"/></svg>`)
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if _, err := doc.Normalize(Options{ColorBehavior: "protected"}); err != nil {
		t.Fatalf("Normalize(protected) error = %v", err)
	}
}

func TestNormalizeProtectedRejectsCurrentColor(t *testing.T) {
	doc, err := Parse([]byte(`<svg viewBox="0 0 16 16" fill="currentColor"><path d="M0 0h16v16z"/></svg>`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if _, err := doc.Normalize(Options{ColorBehavior: "protected"}); err == nil || !strings.Contains(err.Error(), "currentColor") {
		t.Fatalf("Normalize(protected) error = %v, want currentColor rejection", err)
	}
}

func TestNormalizeRejectsUnknownColorBehavior(t *testing.T) {
	doc := fixture(t, "safe.svg")
	if _, err := doc.Normalize(Options{ColorBehavior: "rainbow"}); err == nil {
		t.Fatal("Normalize() error = nil")
	}
}

func fixture(t *testing.T, name string) Document {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", name, err)
	}
	doc, err := Parse(b)
	if err != nil {
		t.Fatalf("Parse(%q) error = %v", name, err)
	}
	return doc
}
