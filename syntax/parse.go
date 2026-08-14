package syntax

import (
	"bytes"
	"slices"
	"strings"

	"github.com/wow-look-at-my/xml-validator/validator"
)

// Parse reads one .tml source file into a [File].
//
// Well-formedness, namespace correctness and the XML 1.1 rules are enforced by
// xml-validator before any TML-specific structure is examined, so a malformed
// file fails with a precise XML diagnostic rather than a confusing TML one.
func Parse(path string, src []byte) (*File, error) {
	doc, err := validator.ParseTree(bytes.NewReader(src))
	if err != nil {
		return nil, xmlError(path, err)
	}
	root := doc.Root
	pos := posOf(path, root)

	if root.Namespace != Namespace {
		if root.Namespace == "" {
			return nil, errorf(pos, "root element <%s> has no namespace; add xmlns=%q", root.Local, Namespace)
		}
		return nil, errorf(pos, "root element <%s> is in namespace %q, want %q", root.Local, root.Namespace, Namespace)
	}

	file := &File{Path: path}
	switch root.Local {
	case "Component":
		component, err := parseComponent(path, root)
		if err != nil {
			return nil, err
		}
		file.Component = component
	case "Theme":
		theme, err := parseTheme(path, root)
		if err != nil {
			return nil, err
		}
		file.Theme = theme
	default:
		return nil, errorf(pos, "root element must be <Component> or <Theme>, got <%s>", root.Local)
	}
	return file, nil
}

// xmlError re-points a validator diagnostic at the file it came from. The
// validator reports line and column but knows nothing about paths.
func xmlError(path string, err error) *Error {
	if verr, ok := err.(*validator.Error); ok {
		return &Error{Pos: Pos{File: path, Line: verr.Line, Col: verr.Col}, Message: verr.Message}
	}
	return &Error{Pos: Pos{File: path, Line: 1, Col: 1}, Message: err.Error()}
}

func posOf(path string, el *validator.Element) Pos {
	return Pos{File: path, Line: el.Line, Col: el.Col}
}

func parseComponent(path string, el *validator.Element) (*Component, error) {
	pos := posOf(path, el)
	if err := checkAttrs(path, el, "name"); err != nil {
		return nil, err
	}
	name, ok := attr(el, "name")
	if !ok || name == "" {
		return nil, errorf(pos, "<Component> requires a name attribute")
	}

	component := &Component{Name: name, Pos: pos}
	for _, child := range el.ChildElements() {
		childPos := posOf(path, child)
		if child.Namespace != Namespace {
			return nil, errorf(childPos, "<%s> is in namespace %q, want %q", child.Local, child.Namespace, Namespace)
		}
		switch child.Local {
		case "Import":
			imp, err := parseImport(path, child)
			if err != nil {
				return nil, err
			}
			component.Imports = append(component.Imports, imp)
		case "Property":
			property, err := parseProperty(path, child)
			if err != nil {
				return nil, err
			}
			component.Properties = append(component.Properties, property)
		case "Template":
			if component.Template != nil {
				return nil, errorf(childPos, "<Component> has more than one <Template>")
			}
			if err := checkAttrs(path, child); err != nil {
				return nil, err
			}
			component.Template = parseTemplateBody(path, child)
		case "Component":
			helper, err := parseComponent(path, child)
			if err != nil {
				return nil, err
			}
			component.Helpers = append(component.Helpers, helper)
		default:
			return nil, errorf(childPos,
				"unexpected <%s> in <Component>; expected Import, Property, Template or a nested Component", child.Local)
		}
	}
	if component.Template == nil {
		return nil, errorf(pos, "<Component> %q has no <Template>", name)
	}
	return component, nil
}

func parseImport(path string, el *validator.Element) (*Import, error) {
	pos := posOf(path, el)
	if err := checkAttrs(path, el, "src"); err != nil {
		return nil, err
	}
	src, ok := attr(el, "src")
	if !ok || src == "" {
		return nil, errorf(pos, "<Import> requires a src attribute")
	}
	return &Import{Src: src, Pos: pos}, nil
}

func parseProperty(path string, el *validator.Element) (*Property, error) {
	pos := posOf(path, el)
	if err := checkAttrs(path, el, "name", "type", "default", "required"); err != nil {
		return nil, err
	}
	name, ok := attr(el, "name")
	if !ok || name == "" {
		return nil, errorf(pos, "<Property> requires a name attribute")
	}
	typ, ok := attr(el, "type")
	if !ok || typ == "" {
		return nil, errorf(pos, "<Property> %q requires a type attribute", name)
	}

	property := &Property{Name: name, Type: typ, Pos: pos}
	property.Default, property.HasDefault = attr(el, "default")

	if raw, ok := attr(el, "required"); ok {
		required, err := parseBool(pos, "required", raw)
		if err != nil {
			return nil, err
		}
		property.Required = required
	}
	if property.Required && property.HasDefault {
		return nil, errorf(pos, "<Property> %q is required and also has a default; a required property is never defaulted", name)
	}
	return property, nil
}

func parseTheme(path string, el *validator.Element) (*Theme, error) {
	pos := posOf(path, el)
	if err := checkAttrs(path, el, "name"); err != nil {
		return nil, err
	}
	name, ok := attr(el, "name")
	if !ok || name == "" {
		return nil, errorf(pos, "<Theme> requires a name attribute")
	}

	theme := &Theme{Name: name, Pos: pos}
	for _, child := range el.ChildElements() {
		childPos := posOf(path, child)
		if child.Namespace != Namespace {
			return nil, errorf(childPos, "<%s> is in namespace %q, want %q", child.Local, child.Namespace, Namespace)
		}
		switch child.Local {
		case "Token":
			token, err := parseToken(path, child)
			if err != nil {
				return nil, err
			}
			theme.Tokens = append(theme.Tokens, token)
		case "Style":
			style, err := parseStyle(path, child)
			if err != nil {
				return nil, err
			}
			theme.Styles = append(theme.Styles, style)
		default:
			return nil, errorf(childPos, "unexpected <%s> in <Theme>; expected Token or Style", child.Local)
		}
	}
	return theme, nil
}

func parseToken(path string, el *validator.Element) (*Token, error) {
	pos := posOf(path, el)
	if err := checkAttrs(path, el, "name", "value", "light", "dark"); err != nil {
		return nil, err
	}
	name, ok := attr(el, "name")
	if !ok || name == "" {
		return nil, errorf(pos, "<Token> requires a name attribute")
	}

	value, hasValue := attr(el, "value")
	light, hasLight := attr(el, "light")
	dark, hasDark := attr(el, "dark")

	switch {
	case hasValue && (hasLight || hasDark):
		return nil, errorf(pos, "<Token> %q sets both value and a light/dark pair; use one or the other", name)
	case hasValue:
		return &Token{Name: name, Value: value, Pos: pos}, nil
	case hasLight && hasDark:
		return &Token{Name: name, Light: light, Dark: dark, Pos: pos}, nil
	case hasLight || hasDark:
		return nil, errorf(pos, "<Token> %q sets only one of light and dark; an adaptive token needs both", name)
	default:
		return nil, errorf(pos, "<Token> %q needs either a value or a light/dark pair", name)
	}
}

func parseStyle(path string, el *validator.Element) (*Style, error) {
	pos := posOf(path, el)
	name, ok := attr(el, "name")
	if !ok || name == "" {
		return nil, errorf(pos, "<Style> requires a name attribute")
	}

	style := &Style{Name: name, Pos: pos}
	style.Extends, _ = attr(el, "extends")
	// Every remaining attribute is a style property. They are validated by the
	// style package against the lipgloss mapping, not here.
	for _, a := range el.Attrs {
		if isNamespaceDecl(a) || a.Name == "name" || a.Name == "extends" {
			continue
		}
		style.Attrs = append(style.Attrs, Attr{Name: a.Name, Value: a.Value, Pos: pos})
	}
	return style, nil
}

// parseTemplateBody converts the children of <Template> into template nodes.
// The <Template> element itself is not represented: its children are the body.
func parseTemplateBody(path string, el *validator.Element) *Node {
	body := &Node{Kind: ElementNode, Name: "Template", Pos: posOf(path, el)}
	body.Children = parseNodes(path, el)
	return body
}

func parseNodes(path string, el *validator.Element) []*Node {
	var out []*Node
	for _, child := range el.Children {
		switch c := child.(type) {
		case *validator.Element:
			node := &Node{Kind: ElementNode, Name: c.Local, Pos: posOf(path, c)}
			for _, a := range c.Attrs {
				if isNamespaceDecl(a) {
					continue
				}
				node.Attrs = append(node.Attrs, Attr{Name: a.Name, Value: a.Value, Pos: node.Pos})
			}
			node.Children = parseNodes(path, c)
			out = append(out, node)
		case *validator.CharData:
			if text, keep := normalizeText(c.Content); keep {
				out = append(out, &Node{Kind: TextNode, Text: text, Pos: posOf(path, el)})
			}
		}
	}
	return out
}

// normalizeText decides what character data survives into the template.
//
// Source layout must not become content: whitespace that spans a line break is
// indentation, so it collapses to a single space between words and disappears
// entirely at the edges of a node. Whitespace on a single line is deliberate and
// is kept verbatim, which is what makes `<Text>a <B/> b</Text>` render correctly.
func normalizeText(raw string) (string, bool) {
	if strings.TrimSpace(raw) == "" {
		if strings.ContainsAny(raw, "\n\r") {
			return "", false
		}
		return raw, true
	}

	runes := []rune(raw)
	var b strings.Builder
	for i := 0; i < len(runes); {
		if !isSpace(runes[i]) {
			b.WriteRune(runes[i])
			i++
			continue
		}
		j := i
		spansLines := false
		for j < len(runes) && isSpace(runes[j]) {
			if runes[j] == '\n' || runes[j] == '\r' {
				spansLines = true
			}
			j++
		}
		switch {
		case !spansLines:
			b.WriteString(string(runes[i:j]))
		case i > 0 && j < len(runes):
			b.WriteRune(' ')
		}
		i = j
	}
	return b.String(), true
}

func isSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

func isNamespaceDecl(a validator.Attr) bool {
	return a.Name == "xmlns" || strings.HasPrefix(a.Name, "xmlns:")
}

func attr(el *validator.Element, name string) (string, bool) {
	for _, a := range el.Attrs {
		if a.Name == name {
			return a.Value, true
		}
	}
	return "", false
}

// checkAttrs rejects any attribute the element does not define. An unknown
// attribute is almost always a typo, and silently ignoring it would hide it.
func checkAttrs(path string, el *validator.Element, allowed ...string) error {
	for _, a := range el.Attrs {
		if isNamespaceDecl(a) {
			continue
		}
		if !slices.Contains(allowed, a.Name) {
			return errorf(posOf(path, el), "<%s> has no attribute %q", el.Local, a.Name)
		}
	}
	return nil
}

func parseBool(pos Pos, name, raw string) (bool, error) {
	switch raw {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, errorf(pos, "attribute %q must be true or false, got %q", name, raw)
	}
}
