package provenance

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/araihu/assets/internal/svgasset"
)

type archiveEntry struct {
	path string
	data []byte
}

func normalizeSourceSVG(raw []byte, symbol, colorBehavior string) ([]byte, svgasset.Document, error) {
	ids, err := sourceIDs(raw, symbol)
	if err != nil {
		return nil, svgasset.Document{}, err
	}
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	var output bytes.Buffer
	encoder := xml.NewEncoder(&output)
	depth := 0
	scriptDepth := 0
	rootSeen := false
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, svgasset.Document{}, fmt.Errorf("decode SVG: %w", err)
		}
		switch value := token.(type) {
		case xml.ProcInst, xml.Directive, xml.Comment:
			return nil, svgasset.Document{}, fmt.Errorf("SVG document content is forbidden")
		case xml.CharData:
			if strings.TrimSpace(string(value)) != "" {
				return nil, svgasset.Document{}, fmt.Errorf("SVG text content is forbidden")
			}
		case xml.StartElement:
			if scriptDepth > 0 {
				return nil, svgasset.Document{}, fmt.Errorf("script content is forbidden")
			}
			if value.Name.Local == "script" {
				if len(value.Attr) != 0 {
					return nil, svgasset.Document{}, fmt.Errorf("script attributes are forbidden")
				}
				scriptDepth = 1
				continue
			}
			if depth == 0 {
				if rootSeen || value.Name.Local != "svg" {
					return nil, svgasset.Document{}, fmt.Errorf("SVG requires one root")
				}
				rootSeen = true
			}
			value.Name = xml.Name{Local: value.Name.Local}
			value.Attr, err = sanitizeAttributes(value.Attr, depth == 0, ids)
			if err != nil {
				return nil, svgasset.Document{}, err
			}
			if err := encoder.EncodeToken(value); err != nil {
				return nil, svgasset.Document{}, err
			}
			depth++
		case xml.EndElement:
			if scriptDepth > 0 {
				if value.Name.Local != "script" {
					return nil, svgasset.Document{}, fmt.Errorf("script boundary is invalid")
				}
				scriptDepth = 0
				continue
			}
			depth--
			value.Name = xml.Name{Local: value.Name.Local}
			if err := encoder.EncodeToken(value); err != nil {
				return nil, svgasset.Document{}, err
			}
		}
	}
	if !rootSeen || depth != 0 || scriptDepth != 0 {
		return nil, svgasset.Document{}, fmt.Errorf("SVG is incomplete")
	}
	if err := encoder.Flush(); err != nil {
		return nil, svgasset.Document{}, err
	}
	document, err := svgasset.Parse(output.Bytes())
	if err != nil {
		return nil, svgasset.Document{}, err
	}
	normalized, err := document.Normalize(svgasset.Options{ColorBehavior: colorBehavior})
	if err != nil {
		return nil, svgasset.Document{}, err
	}
	generated, err := svgasset.Parse(normalized)
	if err != nil {
		return nil, svgasset.Document{}, err
	}
	return normalized, generated, nil
}

func sourceIDs(raw []byte, symbol string) (map[string]string, error) {
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	result := make(map[string]string)
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return result, nil
		}
		if err != nil {
			return nil, err
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		for _, attribute := range start.Attr {
			if attribute.Name.Local != "id" {
				continue
			}
			if _, duplicate := result[attribute.Value]; duplicate {
				return nil, fmt.Errorf("duplicate source ID %q", attribute.Value)
			}
			result[attribute.Value] = symbol + "-" + attribute.Value
		}
	}
}

func sanitizeAttributes(attributes []xml.Attr, root bool, ids map[string]string) ([]xml.Attr, error) {
	result := make([]xml.Attr, 0, len(attributes))
	seen := make(map[string]struct{}, len(attributes))
	for _, attribute := range attributes {
		if isNamespace(attribute) || attribute.Name.Space == "http://www.w3.org/XML/1998/namespace" {
			continue
		}
		name := attribute.Name.Local
		if root && (name == "width" || name == "height" || name == "version") {
			continue
		}
		if name == "style" {
			switch attribute.Value {
			case "mask-type:luminance":
				name, attribute.Value = "mask-type", "luminance"
			case "flex:none;line-height:1":
				continue
			default:
				return nil, fmt.Errorf("unsupported source style %q", attribute.Value)
			}
		}
		if _, duplicate := seen[name]; duplicate {
			return nil, fmt.Errorf("duplicate attribute %q", name)
		}
		seen[name] = struct{}{}
		attribute.Name = xml.Name{Local: name}
		if name == "id" {
			attribute.Value = ids[attribute.Value]
		} else {
			attribute.Value = replaceReferences(attribute.Value, ids)
		}
		result = append(result, attribute)
	}
	return result, nil
}

func replaceReferences(value string, replacements map[string]string) string {
	var output strings.Builder
	last := 0
	for index := 0; index < len(value); index++ {
		if value[index] != '#' {
			continue
		}
		end := index + 1
		for end < len(value) && isReferenceCharacter(value[end]) {
			end++
		}
		if replacement, ok := replacements[value[index+1:end]]; ok {
			output.WriteString(value[last:index])
			output.WriteByte('#')
			output.WriteString(replacement)
			last = end
			index = end - 1
		}
	}
	if last == 0 {
		return value
	}
	output.WriteString(value[last:])
	return output.String()
}

func isReferenceCharacter(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' ||
		value >= '0' && value <= '9' || value == '_' || value == '.' || value == ':' || value == '-'
}

func isNamespace(attribute xml.Attr) bool {
	return attribute.Name.Local == "xmlns" || attribute.Name.Space == "xmlns" ||
		attribute.Name.Space == "http://www.w3.org/2000/xmlns/"
}
