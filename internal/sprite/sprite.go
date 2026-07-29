// Package sprite builds deterministic SVG sprites from validated SVG assets.
package sprite

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"

	"github.com/araihu/assets/internal/svgasset"
)

var (
	symbolName = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)
	urlRef     = regexp.MustCompile(`(?i)url\(#([A-Za-z_][A-Za-z0-9_.:-]*)\)`)
)

const (
	spritePrefix = `<svg xmlns="http://www.w3.org/2000/svg">`
	spriteSuffix = "</svg>\n"
)

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

	ids := make(map[string]struct{}, len(ordered)*2)
	for i, entry := range ordered {
		if !symbolName.MatchString(entry.Symbol) {
			return nil, fmt.Errorf("invalid symbol %q", entry.Symbol)
		}
		if i > 0 && ordered[i-1].Symbol == entry.Symbol {
			return nil, fmt.Errorf("duplicate symbol %q", entry.Symbol)
		}
		ids[entry.Symbol] = struct{}{}
	}

	documents := make([]svgasset.Document, len(ordered))
	for i, entry := range ordered {
		doc, err := svgasset.Parse(entry.SVG)
		if err != nil {
			return nil, fmt.Errorf("unsafe SVG for symbol %q: %w", entry.Symbol, err)
		}
		for _, id := range doc.ChildIDs() {
			if _, duplicate := ids[id]; duplicate {
				return nil, fmt.Errorf("duplicate ID %q", id)
			}
			ids[id] = struct{}{}
		}
		documents[i] = doc
	}

	var out bytes.Buffer
	out.WriteString(spritePrefix)
	for i, entry := range ordered {
		doc := documents[i]
		presentation := doc.RootPresentation()
		out.WriteString(`<symbol id="`)
		out.WriteString(entry.Symbol)
		out.WriteString(`" viewBox="`)
		writeEscaped(&out, doc.ViewBox())
		out.WriteByte('"')
		if presentation.HasFill {
			out.WriteString(` fill="`)
			writeEscaped(&out, presentation.Fill)
			out.WriteByte('"')
		}
		if presentation.HasStroke {
			out.WriteString(` stroke="`)
			writeEscaped(&out, presentation.Stroke)
			out.WriteByte('"')
		}
		out.WriteByte('>')
		out.Write(doc.ChildrenXML())
		out.WriteString(`</symbol>`)
	}
	out.WriteString(spriteSuffix)
	output := out.Bytes()
	if err := Validate(output); err != nil {
		return nil, fmt.Errorf("validate generated sprite: %w", err)
	}
	return output, nil
}

// Validate reparses one complete generated sprite. Its grammar permits only a
// fixed root and validated symbol documents, so styles and external references
// cannot enter the final artifact.
func Validate(data []byte) error {
	if !bytes.HasPrefix(data, []byte(spritePrefix)) || !bytes.HasSuffix(data, []byte(spriteSuffix)) {
		return errors.New("sprite root must use the fixed generated form")
	}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	ids := make(map[string]struct{})
	var references []string
	depth := 0
	closed := false
	var symbol *symbolState
	for {
		token, err := decoder.RawToken()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("decode sprite: %w", err)
		}
		switch token := token.(type) {
		case xml.StartElement:
			if closed {
				return errors.New("sprite has content after root")
			}
			switch depth {
			case 0:
				if token.Name.Local != "svg" {
					return errors.New("sprite root is not svg")
				}
				depth = 1
			case 1:
				if token.Name.Local != "symbol" {
					return fmt.Errorf("sprite root child %q is not symbol", token.Name.Local)
				}
				id, viewBox, presentation, err := symbolAttributes(token.Attr)
				if err != nil {
					return err
				}
				if _, duplicate := ids[id]; duplicate {
					return fmt.Errorf("duplicate ID %q", id)
				}
				ids[id] = struct{}{}
				collectReferences(&references, presentation)
				symbol = &symbolState{viewBox: viewBox, presentation: presentation}
				symbol.encoder = xml.NewEncoder(&symbol.body)
				depth = 2
			default:
				if symbol == nil {
					return errors.New("sprite element is outside a symbol")
				}
				if err := symbol.encoder.EncodeToken(token); err != nil {
					return err
				}
				collectReferences(&references, token.Attr)
				depth++
			}
		case xml.EndElement:
			switch {
			case depth == 0:
				return errors.New("sprite has unexpected closing element")
			case depth == 1:
				if token.Name.Local != "svg" {
					return errors.New("sprite root closes incorrectly")
				}
				depth, closed = 0, true
			case depth == 2:
				if token.Name.Local != "symbol" || symbol == nil {
					return errors.New("sprite symbol closes incorrectly")
				}
				if err := validateSymbol(symbol); err != nil {
					return err
				}
				for _, id := range symbol.document.ChildIDs() {
					if _, duplicate := ids[id]; duplicate {
						return fmt.Errorf("duplicate ID %q", id)
					}
					ids[id] = struct{}{}
				}
				symbol = nil
				depth = 1
			default:
				if symbol == nil {
					return errors.New("sprite closing element is outside a symbol")
				}
				if err := symbol.encoder.EncodeToken(token); err != nil {
					return err
				}
				depth--
			}
		case xml.CharData:
			if symbol == nil {
				if !closed || strings.TrimSpace(string(token)) != "" {
					return errors.New("sprite has non-symbol text")
				}
				continue
			}
			if err := symbol.encoder.EncodeToken(token); err != nil {
				return err
			}
		default:
			if symbol == nil {
				return errors.New("sprite has forbidden document content")
			}
			if err := symbol.encoder.EncodeToken(token); err != nil {
				return err
			}
		}
	}
	if depth != 0 || !closed || symbol != nil {
		return errors.New("sprite document is incomplete")
	}
	for _, ref := range references {
		if _, found := ids[ref]; !found {
			return fmt.Errorf("sprite reference %q has no target", ref)
		}
	}
	return nil
}

type symbolState struct {
	viewBox      string
	presentation []xml.Attr
	body         bytes.Buffer
	encoder      *xml.Encoder
	document     svgasset.Document
}

func symbolAttributes(attrs []xml.Attr) (string, string, []xml.Attr, error) {
	var id, viewBox string
	var presentation []xml.Attr
	seen := make(map[string]struct{}, len(attrs))
	for _, attr := range attrs {
		if attr.Name.Space != "" {
			return "", "", nil, fmt.Errorf("sprite symbol attribute namespace %q is forbidden", attr.Name.Space)
		}
		if _, duplicate := seen[attr.Name.Local]; duplicate {
			return "", "", nil, fmt.Errorf("sprite symbol has duplicate attribute %q", attr.Name.Local)
		}
		seen[attr.Name.Local] = struct{}{}
		switch attr.Name.Local {
		case "id":
			id = attr.Value
		case "viewBox":
			viewBox = attr.Value
		case "fill", "stroke":
			presentation = append(presentation, attr)
		default:
			return "", "", nil, fmt.Errorf("sprite symbol attribute %q is forbidden", attr.Name.Local)
		}
	}
	if !symbolName.MatchString(id) || viewBox == "" {
		return "", "", nil, errors.New("sprite symbol id or viewBox is invalid")
	}
	return id, viewBox, presentation, nil
}

func validateSymbol(symbol *symbolState) error {
	if err := symbol.encoder.Flush(); err != nil {
		return err
	}
	var source bytes.Buffer
	source.WriteString(`<svg viewBox="`)
	writeEscaped(&source, symbol.viewBox)
	source.WriteByte('"')
	for _, attr := range symbol.presentation {
		source.WriteByte(' ')
		source.WriteString(attr.Name.Local)
		source.WriteString(`="`)
		writeEscaped(&source, attr.Value)
		source.WriteByte('"')
	}
	source.WriteByte('>')
	source.WriteString(symbol.body.String())
	source.WriteString(`</svg>`)
	document, err := svgasset.Parse(source.Bytes())
	if err != nil {
		return fmt.Errorf("validate sprite symbol: %w", err)
	}
	symbol.document = document
	return nil
}

func writeEscaped(out io.Writer, value string) {
	_ = xml.EscapeText(out, []byte(value))
}

func collectReferences(references *[]string, attrs []xml.Attr) {
	for _, attr := range attrs {
		if attr.Name.Local == "href" && strings.HasPrefix(attr.Value, "#") {
			*references = append(*references, strings.TrimPrefix(attr.Value, "#"))
		}
		for _, match := range urlRef.FindAllStringSubmatch(attr.Value, -1) {
			*references = append(*references, match[1])
		}
	}
}
