// Package svgasset validates and deterministically normalizes distributable SVG assets.
package svgasset

import (
	"bytes"
	"crypto/sha256"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
)

var (
	identifier      = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.:-]*$`)
	allowedElements = map[string]struct{}{
		"circle": {}, "clipPath": {}, "defs": {}, "ellipse": {}, "g": {},
		"line": {}, "linearGradient": {}, "marker": {}, "mask": {}, "path": {},
		"polygon": {}, "polyline": {}, "radialGradient": {}, "rect": {}, "stop": {}, "use": {},
	}
	geometryAttributes = map[string]struct{}{
		"cx": {}, "cy": {}, "d": {}, "height": {}, "points": {}, "r": {}, "rx": {}, "ry": {}, "width": {}, "x": {}, "x1": {}, "x2": {}, "y": {}, "y1": {}, "y2": {},
	}
	documentElements = map[string]struct{}{
		"desc": {}, "metadata": {}, "title": {},
	}
	visualElements = map[string]bool{
		"circle": true, "ellipse": true, "line": true, "path": true, "polygon": true, "polyline": true, "rect": true, "use": true,
	}
	mixedCaseAttributes = map[string]struct{}{
		"clipPathUnits": {}, "gradientTransform": {}, "gradientUnits": {}, "markerHeight": {}, "markerUnits": {}, "markerWidth": {}, "preserveAspectRatio": {}, "refX": {}, "refY": {}, "viewBox": {},
	}
)

// Options controls normalization for a catalog color classification.
type Options struct {
	ColorBehavior string
}

// Document is a validated SVG document.
type Document struct {
	root node
}

type node struct {
	name     string
	attrs    []attribute
	children []node
}

type attribute struct {
	name  string
	value string
}

// Parse accepts one conservative, self-contained SVG document.
func Parse(input []byte) (Document, error) {
	if bytes.Contains(input, []byte("&")) {
		return Document{}, errors.New("SVG entities are forbidden")
	}
	decoder := xml.NewDecoder(bytes.NewReader(input))
	decoder.Strict = true

	var root *node
	stack := make([]*node, 0, 8)
	ids := make(map[string]struct{})
	seenRoot := false
	closedRoot := false
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return Document{}, fmt.Errorf("decode SVG: %w", err)
		}
		switch token := token.(type) {
		case xml.ProcInst:
			return Document{}, errors.New("SVG processing instructions are forbidden")
		case xml.Directive:
			return Document{}, errors.New("SVG directives and entities are forbidden")
		case xml.Comment:
			return Document{}, errors.New("SVG comments are forbidden")
		case xml.CharData:
			if strings.TrimSpace(string(token)) != "" {
				return Document{}, errors.New("SVG text content is forbidden")
			}
		case xml.StartElement:
			if closedRoot {
				return Document{}, errors.New("SVG contains content after root element")
			}
			if token.Name.Space != "" && token.Name.Space != "http://www.w3.org/2000/svg" {
				return Document{}, fmt.Errorf("SVG element namespace %q is forbidden", token.Name.Space)
			}
			if len(stack) == 0 {
				if seenRoot || token.Name.Local != "svg" {
					return Document{}, errors.New("SVG requires one lowercase svg root element")
				}
				seenRoot = true
			} else if err := validateElementName(token.Name.Local); err != nil {
				return Document{}, err
			}

			current, err := newNode(token, len(stack) == 0, ids)
			if err != nil {
				return Document{}, err
			}
			if len(stack) == 0 {
				root = current
			} else {
				parent := stack[len(stack)-1]
				parent.children = append(parent.children, *current)
				current = &parent.children[len(parent.children)-1]
			}
			stack = append(stack, current)
		case xml.EndElement:
			if len(stack) == 0 || stack[len(stack)-1].name != token.Name.Local {
				return Document{}, errors.New("SVG has mismatched element boundaries")
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				closedRoot = true
			}
		}
	}
	if !seenRoot || root == nil || len(stack) != 0 {
		return Document{}, errors.New("SVG requires one complete root element")
	}
	if err := validateRoot(*root); err != nil {
		return Document{}, err
	}
	return Document{root: *root}, nil
}

func validateElementName(name string) error {
	if _, forbidden := documentElements[name]; forbidden {
		return fmt.Errorf("SVG document-level element %q is forbidden", name)
	}
	if _, ok := allowedElements[name]; !ok {
		return fmt.Errorf("SVG element %q is forbidden", name)
	}
	return nil
}

func newNode(element xml.StartElement, isRoot bool, ids map[string]struct{}) (*node, error) {
	if !isRoot {
		if err := validateElementName(element.Name.Local); err != nil {
			return nil, err
		}
	}
	n := &node{name: element.Name.Local, attrs: make([]attribute, 0, len(element.Attr))}
	seenAttrs := make(map[string]struct{}, len(element.Attr))
	for _, raw := range element.Attr {
		if raw.Name.Space != "" && raw.Name.Space != "http://www.w3.org/1999/xlink" {
			return nil, fmt.Errorf("SVG attribute namespace %q is forbidden", raw.Name.Space)
		}
		name := raw.Name.Local
		lower := strings.ToLower(name)
		_, mixedCase := mixedCaseAttributes[name]
		if (mixedCase && lower != strings.ToLower(name)) || (!mixedCase && name != lower) {
			return nil, fmt.Errorf("SVG attribute %q uses unsafe case", name)
		}
		canonicalName := name
		if lower == "viewbox" {
			canonicalName = "viewBox"
		}
		if _, duplicate := seenAttrs[canonicalName]; duplicate {
			return nil, fmt.Errorf("SVG has duplicate attribute %q", name)
		}
		seenAttrs[canonicalName] = struct{}{}
		if strings.HasPrefix(lower, "on") || lower == "style" {
			return nil, fmt.Errorf("SVG attribute %q is forbidden", name)
		}
		if lower == "id" {
			if !identifier.MatchString(raw.Value) {
				return nil, fmt.Errorf("SVG id %q is invalid", raw.Value)
			}
			if _, duplicate := ids[raw.Value]; duplicate {
				return nil, fmt.Errorf("SVG has duplicate id %q", raw.Value)
			}
			ids[raw.Value] = struct{}{}
		}
		if err := validateAttributeValue(lower, raw.Value); err != nil {
			return nil, err
		}
		n.attrs = append(n.attrs, attribute{name: canonicalName, value: raw.Value})
	}
	return n, nil
}

func validateAttributeValue(name, value string) error {
	if strings.Contains(value, `\`) || strings.Contains(value, "/*") || strings.Contains(value, "*/") {
		return fmt.Errorf("SVG attribute %q contains forbidden CSS syntax", name)
	}
	lower := strings.ToLower(strings.TrimSpace(value))
	if strings.Contains(lower, "data:") {
		return fmt.Errorf("SVG attribute %q contains data URL", name)
	}
	if strings.Contains(lower, "url(") {
		if !isLocalURLReference(lower) {
			return fmt.Errorf("SVG attribute %q contains unsafe url() reference", name)
		}
	}
	if name == "href" || name == "src" {
		if !strings.HasPrefix(value, "#") || !identifier.MatchString(strings.TrimPrefix(value, "#")) {
			return fmt.Errorf("SVG attribute %q contains external reference", name)
		}
	}
	return nil
}

func isLocalURLReference(value string) bool {
	if !strings.HasPrefix(value, "url(#") || !strings.HasSuffix(value, ")") {
		return false
	}
	return identifier.MatchString(strings.TrimSuffix(strings.TrimPrefix(value, "url(#"), ")"))
}

func validateRoot(root node) error {
	viewBox := ""
	for _, attr := range root.attrs {
		switch attr.name {
		case "viewBox":
			viewBox = attr.value
		case "width", "height":
			return fmt.Errorf("SVG root fixed %s is forbidden", attr.name)
		}
	}
	if !validViewBox(viewBox) {
		return fmt.Errorf("SVG has invalid viewBox %q", viewBox)
	}
	return nil
}

func validViewBox(value string) bool {
	parts := strings.Fields(value)
	if len(parts) != 4 {
		return false
	}
	for i, part := range parts {
		n, err := strconv.ParseFloat(part, 64)
		if err != nil || math.IsNaN(n) || math.IsInf(n, 0) || (i >= 2 && n <= 0) {
			return false
		}
	}
	return true
}

// GeometrySignature returns a stable digest of geometry-bearing SVG attributes.
func (d Document) GeometrySignature() []byte {
	hash := sha256.New()
	writeGeometry(hash, d.root)
	return hash.Sum(nil)
}

func writeGeometry(dst io.Writer, n node) {
	_, _ = io.WriteString(dst, n.name)
	for _, attr := range n.attrs {
		if _, geometry := geometryAttributes[attr.name]; geometry {
			_, _ = io.WriteString(dst, "\x00"+attr.name+"="+attr.value)
		}
	}
	_, _ = io.WriteString(dst, "\x00")
	for _, child := range n.children {
		writeGeometry(dst, child)
	}
}

// Normalize serializes a validated document deterministically for its color behavior.
func (d Document) Normalize(options Options) ([]byte, error) {
	if options.ColorBehavior != "monochrome" && options.ColorBehavior != "protected" && options.ColorBehavior != "tintable" {
		return nil, fmt.Errorf("unsupported color behavior %q", options.ColorBehavior)
	}
	if options.ColorBehavior == "protected" && d.hasCurrentColor() {
		return nil, errors.New("protected multicolor asset cannot use currentColor")
	}
	root := cloneNode(d.root)
	if options.ColorBehavior == "monochrome" || options.ColorBehavior == "tintable" {
		fill, stroke := paintRoles(root)
		normalizePaint(&root, fill, stroke)
	}
	var out bytes.Buffer
	writeNode(&out, root)
	out.WriteByte('\n')
	return out.Bytes(), nil
}

func (d Document) hasCurrentColor() bool {
	return nodeHasCurrentColor(d.root)
}

func nodeHasCurrentColor(n node) bool {
	for _, attr := range n.attrs {
		if strings.Contains(strings.ToLower(attr.value), "currentcolor") {
			return true
		}
	}
	for _, child := range n.children {
		if nodeHasCurrentColor(child) {
			return true
		}
	}
	return false
}

func cloneNode(src node) node {
	dst := node{name: src.name, attrs: append([]attribute(nil), src.attrs...), children: make([]node, len(src.children))}
	for i := range src.children {
		dst.children[i] = cloneNode(src.children[i])
	}
	return dst
}

func paintRoles(root node) (bool, bool) {
	return scanPaintRoles(root, "black", "none", true)
}

func scanPaintRoles(n node, inheritedFill, inheritedStroke string, visible bool) (bool, bool) {
	fill, stroke := inheritedFill, inheritedStroke
	for _, attr := range n.attrs {
		switch attr.name {
		case "fill":
			fill = attr.value
		case "stroke":
			stroke = attr.value
		}
	}

	hasFill, hasStroke := false, false
	if visible && visualElements[n.name] {
		hasFill = n.name != "line" && !noPaint(fill)
		hasStroke = !noPaint(stroke)
	}
	childVisible := visible && n.name != "defs" && n.name != "clipPath" && n.name != "mask"
	for _, child := range n.children {
		childFill, childStroke := scanPaintRoles(child, fill, stroke, childVisible)
		hasFill = hasFill || childFill
		hasStroke = hasStroke || childStroke
	}
	return hasFill, hasStroke
}

func normalizePaint(root *node, hasFill, hasStroke bool) {
	rootFill, rootHasFill := paintAttribute(*root, "fill")
	rootStroke, rootHasStroke := paintAttribute(*root, "stroke")
	rootFillNone := !hasFill || (rootHasFill && noPaint(rootFill))
	rootStrokePaint := rootHasStroke && !noPaint(rootStroke)
	globalStroke := rootStrokePaint || (hasStroke && !hasFill)

	attrs := nonPaintAttributes(root.attrs)
	if rootFillNone {
		attrs = append(attrs, attribute{name: "fill", value: "none"})
	} else {
		attrs = append(attrs, attribute{name: "fill", value: "currentColor"})
	}
	if globalStroke {
		attrs = append(attrs, attribute{name: "stroke", value: "currentColor"})
	}
	root.attrs = attrs
	for i := range root.children {
		normalizePaintNode(&root.children[i], rootFillNone, rootStrokePaint, globalStroke)
	}
}

func normalizePaintNode(n *node, parentFillNone, parentStrokePaint, globalStroke bool) {
	fill, hasFill := paintAttribute(*n, "fill")
	stroke, hasStroke := paintAttribute(*n, "stroke")
	fillNone := parentFillNone
	if hasFill {
		fillNone = noPaint(fill)
	}
	strokePaint := parentStrokePaint
	if hasStroke {
		strokePaint = !noPaint(stroke)
	}

	attrs := make([]attribute, 0, len(n.attrs))
	for _, attr := range n.attrs {
		switch attr.name {
		case "color":
			continue
		case "fill":
			if noPaint(attr.value) {
				attrs = append(attrs, attr)
			} else if parentFillNone {
				attrs = append(attrs, attribute{name: "fill", value: "currentColor"})
			}
		case "stroke":
			if noPaint(attr.value) {
				attrs = append(attrs, attr)
			} else if !globalStroke && !parentStrokePaint {
				attrs = append(attrs, attribute{name: "stroke", value: "currentColor"})
			}
		default:
			attrs = append(attrs, attr)
		}
	}
	n.attrs = attrs
	for i := range n.children {
		normalizePaintNode(&n.children[i], fillNone, strokePaint, globalStroke)
	}
}

func paintAttribute(n node, name string) (string, bool) {
	for _, attr := range n.attrs {
		if attr.name == name {
			return attr.value, true
		}
	}
	return "", false
}

func nonPaintAttributes(attrs []attribute) []attribute {
	result := make([]attribute, 0, len(attrs)+2)
	for _, attr := range attrs {
		if attr.name != "fill" && attr.name != "stroke" && attr.name != "color" {
			result = append(result, attr)
		}
	}
	return result
}

func noPaint(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), "none")
}

// ViewBox returns the root SVG viewBox validated by Parse.
func (d Document) ViewBox() string {
	for _, attr := range d.root.attrs {
		if attr.name == "viewBox" {
			return attr.value
		}
	}
	return ""
}

// ChildrenXML returns stable markup for validated root children.
func (d Document) ChildrenXML() []byte {
	var out bytes.Buffer
	for _, child := range d.root.children {
		writeNode(&out, child)
	}
	return out.Bytes()
}

// ChildIDs returns stable identifiers that will be emitted beneath a sprite symbol.
func (d Document) ChildIDs() []string {
	var ids []string
	for _, child := range d.root.children {
		collectIDs(child, &ids)
	}
	return ids
}

func collectIDs(n node, ids *[]string) {
	if id, ok := paintAttribute(n, "id"); ok {
		*ids = append(*ids, id)
	}
	for _, child := range n.children {
		collectIDs(child, ids)
	}
}

func writeNode(out *bytes.Buffer, n node) {
	out.WriteByte('<')
	out.WriteString(n.name)
	for _, attr := range n.attrs {
		out.WriteByte(' ')
		out.WriteString(attr.name)
		out.WriteString(`="`)
		xml.EscapeText(out, []byte(attr.value))
		out.WriteByte('"')
	}
	out.WriteByte('>')
	for _, child := range n.children {
		writeNode(out, child)
	}
	out.WriteString("</")
	out.WriteString(n.name)
	out.WriteByte('>')
}
