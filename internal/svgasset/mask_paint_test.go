package svgasset

import (
	"bytes"
	"testing"
)

func TestMonochromePreservesMaskContrast(t *testing.T) {
	doc, err := Parse([]byte(`<svg viewBox="0 0 16 16"><defs><mask id="face"><rect width="16" height="16" fill="white"/><circle cx="8" cy="8" r="2" fill="black"/></mask></defs><path d="M0 0h16v16H0z" fill="#6092ab" mask="url(#face)"/></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	out, err := doc.Normalize(Options{ColorBehavior: "monochrome"})
	if err != nil {
		t.Fatal(err)
	}
	for _, paint := range []string{`fill="white"`, `fill="black"`, `fill="currentColor"`} {
		if !bytes.Contains(out, []byte(paint)) {
			t.Fatalf("mask/visible ink lost %s: %s", paint, out)
		}
	}
	if _, err := Parse(out); err != nil {
		t.Fatal(err)
	}
}
