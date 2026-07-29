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
		{"CSS-escaped external URL", `<svg viewBox="0 0 16 16"><path fill="\75rl(https://evil.invalid/a.svg)" d="M0 0h1v1z"/></svg>`},
		{"CSS-comment external URL", `<svg viewBox="0 0 16 16"><path fill="u/**/rl(https://evil.invalid/a.svg)" d="M0 0h1v1z"/></svg>`},
		{"CSS-escaped currentColor", `<svg viewBox="0 0 16 16"><path fill="current\43 olor" d="M0 0h1v1z"/></svg>`},
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

func TestParseGeneratedPermitsOnlyAdaptiveGeneratorStyle(t *testing.T) {
	safe := []byte(`<svg viewBox="0 0 16 16"><style>@media (prefers-color-scheme: dark) {:root {--araihu-logo-auto-surface: #07111f;--araihu-logo-auto-ink: #f3f2e9;--araihu-logo-auto-signal: #c7ff4a;}}</style><path fill="var(--araihu-logo-ink, var(--araihu-logo-auto-ink, #07111f))" d="M0 0h1v1z"/></svg>`)
	if _, err := ParseGenerated(safe); err != nil {
		t.Fatalf("ParseGenerated(safe): %v", err)
	}
	unsafe := []byte(strings.Replace(string(safe), `@media (prefers-color-scheme: dark) {:root {--araihu-logo-auto-surface: #07111f;--araihu-logo-auto-ink: #f3f2e9;--araihu-logo-auto-signal: #c7ff4a;}}`, `path { fill: red; }`, 1))
	if _, err := ParseGenerated(unsafe); err == nil {
		t.Fatal("ParseGenerated() accepted an arbitrary stylesheet")
	}
}

func TestNormalizeMaintainsPaintRoles(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			"fill-only art inherits fill without an outline",
			`<svg viewBox="0 0 16 16"><path fill="#123456" d="M1 1h2v2z"/></svg>`,
			"<svg viewBox=\"0 0 16 16\" fill=\"currentColor\"><path d=\"M1 1h2v2z\"></path></svg>\n",
		},
		{
			"stroke-only art preserves no-fill and inherits stroke",
			`<svg viewBox="0 0 16 16"><path fill="none" stroke="#123456" d="M1 1h2v2z"/></svg>`,
			"<svg viewBox=\"0 0 16 16\" fill=\"none\" stroke=\"currentColor\"><path fill=\"none\" d=\"M1 1h2v2z\"></path></svg>\n",
		},
		{
			"mixed art keeps no-fill stroke geometry",
			`<svg viewBox="0 0 16 16"><path fill="#123456" d="M1 1h2v2z"/><path fill="none" stroke="#abcdef" d="M4 4h2v2z"/></svg>`,
			"<svg viewBox=\"0 0 16 16\" fill=\"currentColor\"><path d=\"M1 1h2v2z\"></path><path fill=\"none\" stroke=\"currentColor\" d=\"M4 4h2v2z\"></path></svg>\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := Parse([]byte(tc.input))
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			before := doc.GeometrySignature()
			got, err := doc.Normalize(Options{ColorBehavior: "monochrome"})
			if err != nil {
				t.Fatalf("Normalize() error = %v", err)
			}
			if string(got) != tc.want {
				t.Fatalf("Normalize() = %s\nwant %s", got, tc.want)
			}
			normalized, err := Parse(got)
			if err != nil {
				t.Fatalf("Parse(normalized) error = %v", err)
			}
			if got := normalized.GeometrySignature(); !bytes.Equal(got, before) {
				t.Fatalf("geometry changed:\n got %q\nwant %q", got, before)
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
	if !bytes.Contains(out, []byte(`<svg viewBox="0 0 16 16" fill="currentColor">`)) || !bytes.Contains(out, []byte(`stroke="currentColor"`)) {
		t.Fatalf("normalized SVG missing role-specific inheritable paint: %s", out)
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

func TestParseRejectsCSSEscapedCurrentColorForProtectedArt(t *testing.T) {
	_, err := Parse([]byte(`<svg viewBox="0 0 16 16" fill="current\43 olor"><path d="M0 0h16v16z"/></svg>`))
	if err == nil {
		t.Fatal("Parse() accepted CSS-escaped currentColor")
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
