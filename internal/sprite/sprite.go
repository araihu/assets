// Package sprite builds deterministic SVG sprites from validated SVG assets.
package sprite

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"slices"

	"github.com/araihu/assets/internal/svgasset"
)

var symbolName = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)

// Entry is one source SVG assigned to a sprite symbol identifier.
type Entry struct {
	Symbol string
	SVG    []byte
}

// Build validates SVGs and emits one deterministic hidden sprite.
func Build(entries []Entry) ([]byte, error) {
	if len(entries) == 0 {
		return nil, errors.New("sprite has no entries")
	}
	ordered := slices.Clone(entries)
	slices.SortFunc(ordered, func(a, b Entry) int {
		if a.Symbol < b.Symbol {
			return -1
		}
		if a.Symbol > b.Symbol {
			return 1
		}
		return 0
	})

	var out bytes.Buffer
	out.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" style="display:none">`)
	for i, entry := range ordered {
		if !symbolName.MatchString(entry.Symbol) {
			return nil, fmt.Errorf("invalid symbol %q", entry.Symbol)
		}
		if i > 0 && ordered[i-1].Symbol == entry.Symbol {
			return nil, fmt.Errorf("duplicate symbol %q", entry.Symbol)
		}
		doc, err := svgasset.Parse(entry.SVG)
		if err != nil {
			return nil, fmt.Errorf("unsafe SVG for symbol %q: %w", entry.Symbol, err)
		}
		out.WriteString(`<symbol id="`)
		out.WriteString(entry.Symbol)
		out.WriteString(`" viewBox="`)
		out.WriteString(doc.ViewBox())
		out.WriteString(`">`)
		out.Write(doc.ChildrenXML())
		out.WriteString(`</symbol>`)
	}
	out.WriteString("</svg>\n")
	return out.Bytes(), nil
}
